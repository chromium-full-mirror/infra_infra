// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/grpcutil"
	"go.chromium.org/luci/luciexe/exe"

	"go.chromium.org/infra/chromium/bootstrapper/clients/gclient"
	"go.chromium.org/infra/chromium/bootstrapper/clients/gerrit"
	"go.chromium.org/infra/chromium/bootstrapper/clients/gitiles"
	"go.chromium.org/infra/chromium/bootstrapper/clients/gob"
)

type GclientGetter func(ctx context.Context) (*gclient.Client, error)

// BuildBootstrapper provides the functionality for computing the build
// that the bootstrapped executable receives as input.
type BuildBootstrapper struct {
	gitiles       *gitiles.Client
	gerrit        *gerrit.Client
	gclientGetter GclientGetter
}

func NewBuildBootstrapper(gitiles *gitiles.Client, gerrit *gerrit.Client, gclientGetter GclientGetter) *BuildBootstrapper {
	return &BuildBootstrapper{gitiles: gitiles, gerrit: gerrit, gclientGetter: gclientGetter}
}

// gitilesCommit is a simple wrapper around *buildbucketpb.gitilesCommit with
// the gitiles URI as the string representation.
type gitilesCommit struct {
	*buildbucketpb.GitilesCommit
}

func (c *gitilesCommit) String() string {
	revision := c.Ref
	if c.Id != "" {
		revision = c.Id
	}
	return fmt.Sprintf("%s/%s/+/%s", c.Host, c.Project, revision)
}

// gerritChange is a wrapper around *buildbucketpb.gerritChange with the gerrit URI as the string
// representation and information retrieved from gerrit about the change.
type gerritChange struct {
	*buildbucketpb.GerritChange

	gitilesRevision string
}

func (c *gerritChange) String() string {
	return fmt.Sprintf("%s/c/%s/+/%d/%d", c.Host, c.Project, c.Change, c.Patchset)
}

type BootstrapConfig struct {
	// inputCommit is the top-level gitiles commit to set as the build's input (if different
	// from the config commit)
	inputCommit *gitilesCommit
	// configCommit is the gitiles commit to read the properties file from.
	configCommit *gitilesCommit
	// change is gerrit change that may potentially modify the properties
	// file.
	//
	// nil indicates that the build does not contain any gerrit changes that
	// may modify the properties file.
	change *gerritChange

	// checkForUnrolledPropertiesFile causes NotFound errors to be treated differently when
	// downloading the properties file. For projects with config defined in a dependency
	// project, the properties file won't exist in the pinned revision of the top-level project
	// until some time later when a roll happens. This will cause additional work to be done to
	// distinguish this case from replication lag.
	checkForUnrolledPropertiesFile bool
	// preferBuildProperties causes properties set in buildProperties to override the properties
	// set in builderProperties instead of the other way around
	preferBuildProperties bool
	// buildProperties is the properties that were set on the build.
	buildProperties *structpb.Struct
	// buildRequestedProperties is the properties that were requested when the build was
	// scheduled.
	buildRequestedProperties *structpb.Struct
	// builderProperties is the properties read from the builder's
	// properties file.
	builderProperties *structpb.Struct
	// ledEditedProperties is the properties that were edited using led.
	ledEditedProperties *structpb.Struct
	// ledRemovedProperties is the properties that were removed using led.
	ledRemovedProperties []string
	// Information detailing the source of the loaded config
	configSource *ConfigSource
	// skipAnalysisReasons are reasons that the bootstrapped executable
	// should skip performing analysis to reduce the targets and tests that
	// are built and run.
	skipAnalysisReasons []string
}

// GetBootstrapConfig does the necessary work to extract the properties from the
// appropriate version of the properties file.
func (b *BuildBootstrapper) GetBootstrapConfig(ctx context.Context, input *Input) (*BootstrapConfig, error) {
	var config *BootstrapConfig
	if input.propsProperties == nil {
		if !input.propertiesOptional {
			panic("invalid state: propsProperties is nil and propertiesOptional is not true")
		}
		logging.Infof(ctx, "skipping properties bootstrapping: $bootstrap/properties wasn't set while using properties optional bootstrapping")
		config = &BootstrapConfig{}
	} else {
		switch x := input.propsProperties.ConfigProject.(type) {
		case *BootstrapPropertiesProperties_TopLevelProject_:
			var err error
			if config, err = b.getTopLevelConfig(ctx, input, x.TopLevelProject); err != nil {
				return nil, err
			}

		case *BootstrapPropertiesProperties_DependencyProject_:
			var err error
			if config, err = b.getDependencyConfig(ctx, input, x.DependencyProject, input.propsProperties.PropertiesFile); err != nil {
				return nil, err
			}

		default:
			return nil, errors.Reason("config_project handling for type %T is not implemented", x).Err()
		}

		if err := b.getPropertiesFromFile(ctx, input.propsProperties.PropertiesFile, input.propsProperties.ShadowPropertiesFile, input.shadowBuild, config); err != nil {
			return nil, errors.Annotate(err, "failed to get properties from properties file %s", input.propsProperties.PropertiesFile).Err()
		}
	}

	// Polymorphic builders prefer build properties so that the properties bootstrapped for
	// another builder can't override the properties necessary for correct operation of the
	// polymorphic builder (e.g. recipe)
	config.preferBuildProperties = input.polymorphic

	config.buildProperties = input.buildProperties
	config.buildRequestedProperties = input.buildRequestedProperties
	config.ledEditedProperties = input.ledEditedProperties
	config.ledRemovedProperties = input.ledRemovedProperties

	return config, nil
}

func (b *BuildBootstrapper) getTopLevelConfig(ctx context.Context, input *Input, topLevel *BootstrapPropertiesProperties_TopLevelProject) (*BootstrapConfig, error) {
	commit, change, err := b.getCommitAndChange(ctx, input, topLevel.Repo, topLevel.Ref)
	if err != nil {
		return nil, err
	}
	return &BootstrapConfig{
		configCommit: commit,
		change:       change,
	}, nil
}

// getDependencyConfig determines the commit and change that the properties file should be extracted
// from for a dependency project. If the build input includes a change for the top-level repo that
// modifies the DEPS file, the patched DEPS file will be used and the config will indicate that
// analysis should be skipped if the pin for the dependency repo is updated.
func (b *BuildBootstrapper) getDependencyConfig(ctx context.Context, input *Input, dependency *BootstrapPropertiesProperties_DependencyProject, propsFile string) (*BootstrapConfig, error) {
	// "" for ref means commit will be nil if there isn't a change or commit for the config repo
	commit, change, err := b.getCommitAndChange(ctx, input, dependency.ConfigRepo, "")
	if err != nil {
		return nil, err
	}
	if commit != nil {
		return &BootstrapConfig{
			configCommit: commit,
			change:       change,
		}, nil
	}

	commit, change, err = b.getCommitAndChange(ctx, input, dependency.TopLevelRepo, dependency.TopLevelRef)
	if err != nil {
		return nil, err
	}

	var locator configRepoLocator
	switch x := dependency.ConfigRepoLocator.(type) {
	case *BootstrapPropertiesProperties_DependencyProject_ConfigRepoPath:
		locator = &depsConfigRepoLocator{
			bootstrapper:   b,
			configRepoPath: x.ConfigRepoPath,
		}

	case *BootstrapPropertiesProperties_DependencyProject_ConfigRepoSubmodulePath:
		locator = &submoduleConfigRepoLocator{
			bootstrapper:            b,
			configRepoSubmodulePath: x.ConfigRepoSubmodulePath,
		}

	default:
		return nil, errors.Reason("config_repo_locator handling for type %T is not implemented", x).Err()
	}

	dependencyRevision, oldDependencyRevision, err := b.getDependencyRevision(ctx, commit, change, locator)
	if err != nil {
		return nil, errors.Annotate(err, "failed to get dependency revision for %s/%s", dependency.ConfigRepo.Host, dependency.ConfigRepo.Project).Err()
	}

	// If the DEPS pin for the config repo has changed, find out if the properties file has
	// changed so that a skip analysis reason can be provided
	var skipAnalysisReasons []string
	if oldDependencyRevision != "" {
		propertiesDiff, err := b.gitiles.DownloadDiff(ctx, dependency.ConfigRepo.Host, dependency.ConfigRepo.Project, dependencyRevision, oldDependencyRevision, propsFile)
		if err != nil {
			return nil, errors.Annotate(err, "failed to determine if properties file was affected").Err()
		}
		if propertiesDiff != "" {
			skipAnalysisReasons = append(skipAnalysisReasons, fmt.Sprintf("properties file %s is affected by CL (via DEPS change)", propsFile))
		}
	}

	configCommit := &gitilesCommit{&buildbucketpb.GitilesCommit{
		Host:    dependency.ConfigRepo.Host,
		Project: dependency.ConfigRepo.Project,
	}}
	if locator.mightBeRef() {
		// We don't know if the revision is a commit hash or a ref, so just set it as ref.
		// If it is a revision, populateCommitId will clear Ref.
		configCommit.Ref = dependencyRevision
		configCommit, err = b.populateCommitId(ctx, configCommit)
		if err != nil {
			return nil, err
		}
	} else {
		configCommit.Id = dependencyRevision
	}

	return &BootstrapConfig{
		checkForUnrolledPropertiesFile: true,
		inputCommit:                    commit,
		configCommit:                   configCommit,
		skipAnalysisReasons:            skipAnalysisReasons,
	}, nil
}

type configRepoLocator interface {
	mightBeRef() bool
	getRevision(ctx context.Context, commit *gitilesCommit) (string, error)
}

type depsConfigRepoLocator struct {
	bootstrapper   *BuildBootstrapper
	configRepoPath string

	gclient *gclient.Client
}

func (d *depsConfigRepoLocator) mightBeRef() bool {
	return true
}

func (d *depsConfigRepoLocator) getRevision(ctx context.Context, commit *gitilesCommit) (string, error) {
	if d.gclient == nil {
		gclient, err := d.bootstrapper.gclientGetter(ctx)
		if err != nil {
			return "", err
		}
		d.gclient = gclient
	}
	contents, err := d.bootstrapper.downloadFile(ctx, commit, "DEPS")
	if err != nil {
		return "", err
	}
	return d.gclient.GetDep(ctx, contents, d.configRepoPath)
}

type submoduleConfigRepoLocator struct {
	bootstrapper            *BuildBootstrapper
	configRepoSubmodulePath string
}

func (s *submoduleConfigRepoLocator) mightBeRef() bool {
	return false
}

func (s *submoduleConfigRepoLocator) getRevision(ctx context.Context, commit *gitilesCommit) (string, error) {
	return s.bootstrapper.gitiles.GetSubmoduleRevision(ctx, commit.Host, commit.Project, commit.Id, s.configRepoSubmodulePath)
}

func (b *BuildBootstrapper) getDependencyRevision(ctx context.Context, topLevelRepoCommit *gitilesCommit, topLevelRepoChange *gerritChange, locator configRepoLocator) (string, string, error) {
	// If there is a change for the top-level repo, get the revision for the dependency repo
	// from the CL and its base. If they are different, than we can just use the revision from
	// the CL and indicate that there was a change to the pin.
	if topLevelRepoChange != nil {
		clDependencyRevision, err := locator.getRevision(ctx, &gitilesCommit{
			GitilesCommit: &buildbucketpb.GitilesCommit{
				Host:    topLevelRepoCommit.Host,
				Project: topLevelRepoCommit.Project,
				Id:      topLevelRepoChange.gitilesRevision,
			},
		})
		if err != nil {
			return "", "", errors.Annotate(err, "failed to get dependency revision for CL %s", topLevelRepoChange).Err()
		}
		baseRevision, err := b.gitiles.GetParentRevision(ctx, topLevelRepoCommit.Host, topLevelRepoCommit.Project, topLevelRepoChange.gitilesRevision)
		if err != nil {
			return "", "", errors.Annotate(err, "failed to get base revision for CL %s", topLevelRepoChange).Err()
		}
		baseDependencyRevision, err := locator.getRevision(ctx, &gitilesCommit{
			GitilesCommit: &buildbucketpb.GitilesCommit{
				Host:    topLevelRepoCommit.Host,
				Project: topLevelRepoCommit.Project,
				Id:      baseRevision,
			},
		})
		if err != nil {
			return "", "", errors.Annotate(err, "failed to get dependency revision for base of CL %s", topLevelRepoChange).Err()
		}
		if clDependencyRevision != baseDependencyRevision {
			return clDependencyRevision, baseDependencyRevision, nil
		}
	}

	// There is no change to the pin for the dependency repo, so just use the pin from the head
	// DEPS file
	revision, err := locator.getRevision(ctx, topLevelRepoCommit)
	if err != nil {
		return "", "", errors.Annotate(err, "failed to get dependency revision from commit %s", topLevelRepoCommit).Err()
	}
	return revision, "", nil
}

// getCommitAndChange gets the commit and change for a given repo. If there is
// no commit for the given repo and ref is empty, commit will be nil, otherwise,
// one will be constructed using the project and host of repo and the provided
// ref. If a non-nill commit is returned, its ID is guaranteed to be populated.
// The returned change will be nil if there is no change for the repo.
func (b *BuildBootstrapper) getCommitAndChange(ctx context.Context, input *Input, repo *GitilesRepo, ref string) (*gitilesCommit, *gerritChange, error) {
	change := findMatchingGerritChange(input.changes, repo)
	if change != nil {
		logging.Infof(ctx, "getting change info for config change %s", change)
		info, err := b.gerrit.GetChangeInfo(ctx, change.Host, change.Project, change.Change, int32(change.Patchset))
		if err != nil {
			return nil, nil, errors.Annotate(err, "failed to get change info for config change %s", change).Err()
		}
		ref = info.TargetRef
		change.gitilesRevision = info.GitilesRevision
	}
	commit := findMatchingGitilesCommit(input.commits, repo)
	if commit == nil {
		if ref == "" {
			return nil, nil, nil
		}
		commit = &gitilesCommit{&buildbucketpb.GitilesCommit{
			Host:    repo.Host,
			Project: repo.Project,
			Ref:     ref,
		}}
	}
	commit, err := b.populateCommitId(ctx, commit)
	if err != nil {
		return nil, nil, err
	}
	return commit, change, nil
}

// getPropertiesFromFile updates config to include the properties contained in
// the builder's properties file.
func (b *BuildBootstrapper) getPropertiesFromFile(ctx context.Context, propsFile, shadowPropsFile string, shadowBuild bool, config *BootstrapConfig) error {
	getPropertiesFromFile := func(f string) (*structpb.Struct, error) {
		var diff string
		if config.change != nil {
			var err error
			diff, err = b.getDiffForMaybeAffectedFile(ctx, config.change, f)
			if err != nil {
				return nil, err
			}
		}

		contents, err := b.downloadPropertiesFile(ctx, f, config)
		if err != nil {
			return nil, err
		}
		if diff != "" {
			config.skipAnalysisReasons = append(config.skipAnalysisReasons, fmt.Sprintf("properties file %s is affected by CL", f))
			logging.Infof(ctx, "patching properties file %s", f)
			contents, err = patchFile(ctx, f, contents, diff)
			if err != nil {
				return nil, errors.Annotate(err, "failed to patch properties file %s", f).Err()
			}
		}

		properties := &structpb.Struct{}
		logging.Infof(ctx, "unmarshalling builder properties file %s", f)
		if err := protojson.Unmarshal([]byte(contents), properties); err != nil {
			err := errors.Annotate(err, "failed to unmarshall builder properties file %s", f).Err()
			logging.Errorf(ctx, err.Error())
			logging.Errorf(ctx, "properties file contents: {%s}", contents)
			return nil, err
		}
		return properties, nil
	}

	properties, err := getPropertiesFromFile(propsFile)
	if err != nil {
		return err
	}
	if shadowBuild && shadowPropsFile != "" {
		shadowProperties, err := getPropertiesFromFile(shadowPropsFile)
		if err != nil {
			return err
		}
		for k, v := range shadowProperties.Fields {
			properties.Fields[k] = v
		}
	}

	logging.Infof(ctx, "getting last changed commit for properties file")
	changedRev, err := b.gitiles.FetchLatestRevisionForPath(ctx, config.configCommit.Host, config.configCommit.Project, config.configCommit.Id, propsFile)
	if err != nil {
		return errors.Annotate(err, "failed to get last changed commit for properties file %s", propsFile).Err()
	}
	configSource := &ConfigSource{
		LastChangedCommit: &buildbucketpb.GitilesCommit{
			Host:    config.configCommit.Host,
			Project: config.configCommit.Project,
			Ref:     config.configCommit.Ref,
			Id:      changedRev,
		},
		Path: propsFile,
	}

	config.builderProperties = properties
	config.configSource = configSource

	return nil
}

func (b *BuildBootstrapper) downloadPropertiesFile(ctx context.Context, propsFile string, config *BootstrapConfig) (string, error) {
	if !config.checkForUnrolledPropertiesFile {
		return b.downloadFile(ctx, config.configCommit, propsFile)
	}

	var contents string
	revisionKnownToExist := false

	err := gob.Execute(ctx, "download properties file", func() error {
		// We want to issue multiple requests to gitiles without retries so that we can
		// diagnose the errors we get from individual requests
		ctx := gob.DisableRetries(ctx)

		var err error
		contents, err = b.downloadFile(ctx, config.configCommit, propsFile)
		if grpcutil.Code(err) != codes.NotFound {
			return err
		}

		// In the case of the file not being found, it could be due to replication lag, or
		// it could that the revision of the top-level repo being used pins a version of the
		// config repo before the builder was added. In order to distinguish the two,
		// attempt to "download" the root of the repo: if it succeeds then we know that the
		// revision is contained in the repo.
		if !revisionKnownToExist {
			_, rootErr := b.downloadFile(ctx, config.configCommit, "")
			// gob flakiness, return the original error since that will make more sense
			// to users
			if gob.ErrorIsRetriable(rootErr) {
				return err
			}
			// Report whatever weirdness happened
			if rootErr != nil {
				return rootErr
			}
			revisionKnownToExist = true

			// Make another attempt to download the file in case the not found error was
			// due to replication lag that caught up between the two requests. This
			// could still result in some gob flakiness, in which case, this whole
			// function will be retried, but we won't need to re-check if the revision
			// exists.
			contents, err = b.downloadFile(ctx, config.configCommit, propsFile)
			if grpcutil.Code(err) != codes.NotFound {
				return err
			}
		}

		// The revision exists in the repo and we still got a not found error, so the file
		// doesn't exist at the pinned revision. Create an error with a helpful message for
		// users and a tag so the top-level code will sleep.
		err = errors.Reason(`dependency properties file %s does not exist in pinned revision %s
This should resolve once the CL that adds this builder rolls into %s/%s
If you believe you are seeing this message in error, please contact a trooper
This build will sleep for 10 minutes to avoid the builder cycling too quickly`,
			propsFile, config.configCommit, config.inputCommit.Host, config.inputCommit.Project).Err()
		err = SleepBeforeExiting.ApplyValue(err, 10*time.Minute)
		return err
	})
	if err != nil {
		return "", err
	}
	return contents, nil
}

func (b *BuildBootstrapper) downloadFile(ctx context.Context, commit *gitilesCommit, file string) (string, error) {
	if commit.Id == "" {
		return "", errors.New("commit ID not set for download")
	}
	logging.Infof(ctx, "downloading %s/%s", commit, file)
	contents, err := b.gitiles.DownloadFile(ctx, commit.Host, commit.Project, commit.Id, file)
	if err != nil {
		return "", errors.Annotate(err, "failed to get %s/%s", commit, file).Err()
	}
	return contents, nil
}

func (b *BuildBootstrapper) getDiffForMaybeAffectedFile(ctx context.Context, change *gerritChange, file string) (string, error) {
	logging.Infof(ctx, "getting diff for %s", change)
	diff, err := b.gitiles.DownloadDiff(ctx, convertGerritHostToGitilesHost(change.Host), change.Project, change.gitilesRevision, gitiles.PARENT, file)
	if err != nil {
		return "", errors.Annotate(err, "failed to get diff from %s", change).Err()
	}
	if diff != "" {
		logging.Infof(ctx, "%s was affected by %s", file, change)
	} else {
		logging.Infof(ctx, "%s was not affected by %s", file, change)
	}
	return diff, nil
}

func (b *BuildBootstrapper) populateCommitId(ctx context.Context, commit *gitilesCommit) (*gitilesCommit, error) {
	if commit.Id == "" {
		logging.Infof(ctx, "getting revision for %s", commit)
		revision, err := b.gitiles.FetchLatestRevision(ctx, commit.Host, commit.Project, commit.Ref)
		if err != nil {
			return nil, errors.Annotate(err, "failed to populate commit ID for %s", commit).Err()
		}
		commit = &gitilesCommit{proto.Clone(commit.GitilesCommit).(*buildbucketpb.GitilesCommit)}
		if revision == commit.Ref {
			commit.Ref = ""
		}
		commit.Id = revision
	}
	return commit, nil
}

func findMatchingGitilesCommit(commits []*buildbucketpb.GitilesCommit, repo *GitilesRepo) *gitilesCommit {
	for _, commit := range commits {
		if commit.Host == repo.Host && commit.Project == repo.Project {
			return &gitilesCommit{commit}
		}
	}
	return nil
}

func findMatchingGerritChange(changes []*buildbucketpb.GerritChange, repo *GitilesRepo) *gerritChange {
	for _, change := range changes {
		if convertGerritHostToGitilesHost(change.Host) == repo.Host && change.Project == repo.Project {
			return &gerritChange{GerritChange: change}
		}
	}
	return nil
}

func convertGerritHostToGitilesHost(host string) string {
	pieces := strings.SplitN(host, ".", 2)
	pieces[0] = strings.TrimSuffix(pieces[0], "-review")
	return strings.Join(pieces, ".")
}

// UpdateBuild updates the build proto to use as input for the bootstrapped executable.
//
// The build's properties will be combined from multiple sources, with earlier source in the list
// taking priority:
//   - Properties set/removed by led
//   - The properties requested at the time the build is scheduled.
//   - The $build/chromium_bootstrap property will be set with information about the bootstrapping
//     process that the bootstrapped executable can use to ensure it operates in a manner that is
//     consistent with the bootstrapping process. See chromium_bootstrap.proto for more information.
//   - The properties read from the properties file identified by the config_project and
//     properties_file fields of the build's $bootstrap/properties property.
//   - The build's input properties with the $bootstrap/properties and $bootstrap/exe properties
//     removed.
//
// Additionally, if the build's input gitiles commit matches the project that the config was read
// from, the commit will be updated to refer to the same revision that the config came from.
func (c *BootstrapConfig) UpdateBuild(build *buildbucketpb.Build, bootstrappedExe *BootstrappedExe) error {
	properties := &structpb.Struct{
		Fields: map[string]*structpb.Value{},
	}
	updateProperties := func(updates *structpb.Struct) {
		for key, value := range updates.GetFields() {
			properties.Fields[key] = value
		}
	}

	if c.preferBuildProperties {
		updateProperties(c.builderProperties)
		updateProperties(c.buildProperties)
		// buildRequestedProperties is a subset of buildProperties, so there's no need to
		// re-apply them
	} else {
		updateProperties(c.buildProperties)
		updateProperties(c.builderProperties)
		updateProperties(c.buildRequestedProperties)
	}
	updateProperties(c.ledEditedProperties)
	for _, prop := range c.ledRemovedProperties {
		delete(properties.Fields, prop)
	}

	commits := []*buildbucketpb.GitilesCommit{}
	if c.inputCommit != nil {
		commits = append(commits, c.inputCommit.GitilesCommit)
	}
	if c.configCommit != nil {
		commits = append(commits, c.configCommit.GitilesCommit)
	}
	modProperties := &ChromiumBootstrapModuleProperties{
		Commits:             commits,
		Exe:                 bootstrappedExe,
		ConfigSource:        c.configSource,
		SkipAnalysisReasons: c.skipAnalysisReasons,
	}
	if err := exe.WriteProperties(properties, map[string]interface{}{
		"$build/chromium_bootstrap": modProperties,
	}); err != nil {
		return errors.Annotate(err, "failed to write out properties for chromium_bootstrap module: {%s}", modProperties).Err()
	}

	build.Input.Properties = properties
	inputCommit := c.inputCommit
	if inputCommit == nil {
		inputCommit = c.configCommit
	}
	if shouldUpdateGitilesCommit(build, inputCommit) {
		build.Input.GitilesCommit = inputCommit.GitilesCommit
	}

	return nil
}

func shouldUpdateGitilesCommit(build *buildbucketpb.Build, commit *gitilesCommit) bool {
	if commit == nil {
		return false
	}
	buildCommit := build.Input.GitilesCommit
	if buildCommit == nil {
		return true
	}
	return buildCommit.Host == commit.Host && buildCommit.Project == commit.Project
}
