// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package testplan

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/strslice"
	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/chromiumos/config/go/build/api"
	testpb "go.chromium.org/chromiumos/config/go/test/api"
	planpb "go.chromium.org/chromiumos/config/go/test/plan"
	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/data/stringset"
	"go.chromium.org/luci/common/errors"
	lgs "go.chromium.org/luci/common/gcloud/gs"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/proto/structmask"
	"go.chromium.org/luci/common/sync/parallel"

	"go.chromium.org/infra/cros/internal/cmd"
	"go.chromium.org/infra/cros/internal/docker"
	"go.chromium.org/infra/cros/internal/gerrit"
	"go.chromium.org/infra/cros/internal/gs"
	"go.chromium.org/infra/cros/internal/shared"
	"go.chromium.org/infra/tools/dirmd"
)

type validator struct {
	gerritClient                    gerrit.Client
	bbClient                        bbpb.BuildsClient
	containerRunner                 *docker.ContainerRunner
	gsClient                        gs.Client
	ctfImage                        string
	tmpdirFn                        func(string, string) (string, error)
	checkTagCriteriaNonEmptyEnabled bool
	bbMutex                         sync.Mutex
}

// NewValidator returns a validator with default configuration, which can be
// used to validate ChromeOS test configs in dirmd.Mappings.
func NewValidator(gerritClient gerrit.Client, bbClient bbpb.BuildsClient, gsClient gs.Client, cmdRunner cmd.CommandRunner) *validator {
	return &validator{
		gerritClient:                    gerritClient,
		bbClient:                        bbClient,
		gsClient:                        gsClient,
		containerRunner:                 docker.NewContainerRunner(cmdRunner),
		tmpdirFn:                        os.MkdirTemp,
		checkTagCriteriaNonEmptyEnabled: false,
	}
}

func (v *validator) SetCheckTagCriteriaNonEmptyEnabled(enabled bool) *validator {
	v.checkTagCriteriaNonEmptyEnabled = enabled
	return v
}

// WithTmpdirFn overrides the function the validator uses to create temp dirs;
// for example when the validator calls Docker containers it binds a temp dir
// on the host to the image for I/O.
//
// Modifies the validator and also returns a pointer to it.
//
// By default the tempdir function is os.MkdirTemp, and the new tmpdir fn must
// have the same signature. The most common reason to override the tempdir
// function is for injecting test data.
func (v *validator) WithTmpdirFn(f func(string, string) (string, error)) *validator {
	v.tmpdirFn = f
	return v
}

// ValidateMapping validates ChromeOS test config in mapping.
func (v *validator) ValidateMapping(
	ctx context.Context,
	mapping *dirmd.Mapping,
	repoRoot string,
) error {
	validationFns := []func(context.Context, string, string, *planpb.SourceTestPlan) error{
		v.validateAtLeastOneTestPlanStarlarkFile,
		v.validatePathRegexps,
		v.validateStarlarkFileExists,
		v.validateTemplateParameters,
	}

	return parallel.WorkPool(0, func(c chan<- func() error) {
		for dir, metadata := range mapping.Dirs {
			logging.Infof(ctx, "validating dir %q", dir)

			for _, sourceTestPlan := range metadata.GetChromeos().GetCq().GetSourceTestPlans() {
				for _, fn := range validationFns {
					sourceTestPlan := sourceTestPlan
					c <- func() error {
						return fn(ctx, dir, repoRoot, sourceTestPlan)
					}
				}
			}
		}
	})
}

func (v *validator) validateAtLeastOneTestPlanStarlarkFile(_ context.Context, _, _ string, plan *planpb.SourceTestPlan) error {
	if len(plan.GetTestPlanStarlarkFiles()) == 0 {
		return fmt.Errorf("at least one TestPlanStarlarkFile must be specified")
	}

	for _, file := range plan.GetTestPlanStarlarkFiles() {
		if !strings.HasSuffix(file.GetPath(), ".star") {
			return fmt.Errorf("all TestPlanStarlarkFile must specify \".star\" files, got %q", file.GetPath())
		}
	}

	return nil
}

func (v *validator) validatePathRegexps(ctx context.Context, dir, repoRoot string, plan *planpb.SourceTestPlan) error {
	for _, pattern := range append(plan.PathRegexps, plan.PathRegexpExcludes...) {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return errors.Annotate(err, "failed to compile path regexp %q", pattern).Err()
		}

		if dir != "." && !strings.HasPrefix(pattern, dir) {
			return fmt.Errorf(
				"path_regexp(_exclude)s defined in a directory that is not "+
					"the root of the repo must have the sub-directory as a prefix. "+
					"Invalid regexp %q in directory %q",
				pattern, dir,
			)
		}

		matchedPath := false
		if err := filepath.WalkDir(filepath.Join(repoRoot, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if re.Match([]byte(path)) {
				logging.Debugf(ctx, "found match for pattern %q: %q", pattern, path)
				matchedPath = true
				return fs.SkipAll
			}

			return nil
		}); err != nil {
			return err
		}

		if !matchedPath {
			logging.Warningf(ctx, "pattern %q doesn't match any files in directory %q", pattern, dir)
		}
	}

	return nil
}

func (v *validator) validateStarlarkFileExists(ctx context.Context, _, _ string, plan *planpb.SourceTestPlan) error {
	for _, file := range plan.GetTestPlanStarlarkFiles() {
		_, err := v.gerritClient.DownloadFileFromGitiles(ctx, file.GetHost(), file.GetProject(), "HEAD", file.GetPath(), shared.LongerOpts)
		if err != nil {
			return fmt.Errorf("failed downloading file %q", file)
		}
	}

	return nil
}

// ensureCTFImage sets v.ctfImage to the most recent cros-test-finder image
// used by dedede-snapshot. If this function has already been called
// and v.ctfImage is set, this function is a no-op (besides getting the
// lock to check v.ctfImage).
//
// The found build must have the artifact_link and
// $chromeos/build_menu.build_target.name properties set. The container
// metadata is assumed to be at <artifact_link>/metadata/containers.jsonpb
// and it is assumed there is info for cros-test-finder in the metadata.
func (v *validator) ensureCTFImage(ctx context.Context) error {
	v.bbMutex.Lock()
	defer v.bbMutex.Unlock()
	if v.ctfImage != "" {
		logging.Debugf(ctx, "CTF image is already set to %q", v.ctfImage)
		return nil
	}

	logging.Debugf(ctx, "finding new CTF image")

	bbResp, err := v.bbClient.SearchBuilds(ctx, &bbpb.SearchBuildsRequest{
		Predicate: &bbpb.BuildPredicate{
			Builder: &bbpb.BuilderID{
				Project: "chromeos",
				Bucket:  "postsubmit",
				Builder: "dedede-snapshot",
			},
			Status: bbpb.Status_SUCCESS,
			Tags:   []*bbpb.StringPair{{Key: "relevance", Value: "relevant"}},
		},
		Mask: &bbpb.BuildMask{
			InputProperties: []*structmask.StructMask{
				{
					Path: []string{"$chromeos/build_menu", "build_target", "name"},
				},
			},
			OutputProperties: []*structmask.StructMask{
				{
					Path: []string{"artifact_link"},
				},
			},
		},
		PageSize: 1,
	})

	if err != nil {
		return fmt.Errorf("SearchBuilds failed: %w", err)
	}

	if len(bbResp.Builds) != 1 {
		return fmt.Errorf("expected exactly one build from SearchBuilds, got %q", bbResp)
	}

	build := bbResp.Builds[0]
	logging.Debugf(ctx, "found build %q", build)

	outputProps := build.GetOutput().GetProperties().GetFields()
	artifactLink, ok := outputProps["artifact_link"]
	if !ok {
		return fmt.Errorf("artifact_link output property not found on build %d", build.Id)
	}

	inputProps := build.GetInput().GetProperties().GetFields()
	buildTarget, ok := inputProps["$chromeos/build_menu"].GetStructValue().GetFields()["build_target"].GetStructValue().GetFields()["name"]
	if !ok {
		return fmt.Errorf("$chromeos/build_menu.build_target.name input property not found on build %d", build.Id)
	}

	metadataPath := artifactLink.GetStringValue() + "/metadata/containers.jsonpb"
	metadataBytes, err := v.gsClient.Read(lgs.Path(metadataPath))
	if err != nil {
		return fmt.Errorf("failed to read %q: %w", metadataPath, err)
	}

	containerMetadata := &api.ContainerMetadata{}
	if err := protojson.Unmarshal(metadataBytes, containerMetadata); err != nil {
		return fmt.Errorf("failed to unmarshal %q: %w", metadataPath, err)
	}
	containerImageInfo, ok := containerMetadata.GetContainers()[buildTarget.GetStringValue()].GetImages()["cros-test-finder"]
	if !ok {
		return fmt.Errorf("cros-test-finder container not found in %q", containerMetadata)
	}

	v.ctfImage = fmt.Sprintf(
		"%s/%s/%s@%s",
		containerImageInfo.GetRepository().GetHostname(),
		containerImageInfo.GetRepository().GetProject(),
		containerImageInfo.GetName(),
		containerImageInfo.GetDigest(),
	)

	logging.Infof(ctx, "found CTF image %q", v.ctfImage)

	return nil
}

// callCrosTestFinder runs cros-test-finder with req as input and returns the
// response.
func (v *validator) callCrosTestFinder(
	ctx context.Context,
	req *testpb.CrosTestFinderRequest,
) (*testpb.CrosTestFinderResponse, error) {
	if err := v.ensureCTFImage(ctx); err != nil {
		return nil, err
	}

	tmpDir, err := v.tmpdirFn("", "ctf*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	// Write the request to a file and create a file for the response.
	reqJson, err := protojson.Marshal(req)
	if err != nil {
		return nil, err
	}

	reqFilePath := filepath.Join(tmpDir, "request.json")
	if err = os.WriteFile(reqFilePath, reqJson, os.ModePerm); err != nil {
		return nil, err
	}

	respFilePath := filepath.Join(tmpDir, "result.json")

	var stderrBuf bytes.Buffer
	logging.Debugf(ctx, "running image %q", v.ctfImage)
	if err := v.containerRunner.RunContainer(
		ctx,
		&container.Config{
			Image: v.ctfImage,
			Cmd:   strslice.StrSlice{"cros-test-finder"},
		},
		&container.HostConfig{
			Mounts: []mount.Mount{
				{
					Type:   mount.TypeBind,
					Source: tmpDir,
					Target: "/tmp/test/cros-test-finder",
				},
			},
		},
		&api.ContainerImageInfo{
			Repository: &api.GcrRepository{
				Hostname: "us-docker.pkg.dev",
				Project:  "cros-registry/test-services",
			},
			Name: "cros-test-finder",
		},
		&docker.RuntimeOptions{
			UseConfigureDocker: true,
			NoSudo:             true,
			StdoutBuf:          io.Discard,
			StderrBuf:          &stderrBuf,
		},
	); err != nil {
		logging.Errorf(ctx, "cros-test-finder failed, stderr: %v", stderrBuf.String())
		return nil, err
	}

	respJson, err := os.ReadFile(respFilePath)
	if err != nil {
		return nil, err
	}

	resp := &testpb.CrosTestFinderResponse{}
	if err := protojson.Unmarshal(respJson, resp); err != nil {
		return nil, fmt.Errorf("error unmarshalling proto read from %q: %w", respFilePath, err)
	}
	return resp, nil
}

// checkTagCriteriaNonEmpty uses cros-test-finder to check that the
// TestCaseTagCriteria in templateParameters match at least one test.
func (v *validator) checkTagCriteriaNonEmpty(
	ctx context.Context,
	templateParameters *planpb.SourceTestPlan_TestPlanStarlarkFile_TemplateParameters,
) error {
	suiteName := templateParameters.GetSuiteName()
	logging.Infof(ctx, "checking tag criteria for suite %q are non-empty", suiteName)
	if strings.HasPrefix(suiteName, "tast_gce") || strings.HasPrefix(suiteName, "tast_vm") {
		logging.Warningf(ctx, "cros-test-finder doesn't currently work with VM suites (%s), skipping validation", suiteName)
		return nil
	}

	ctfReq := &testpb.CrosTestFinderRequest{
		TestSuites: []*testpb.TestSuite{
			{
				Name: suiteName,
				Spec: &testpb.TestSuite_TestCaseTagCriteria_{
					TestCaseTagCriteria: templateParameters.GetTagCriteria(),
				},
			},
		},
	}

	ctfResp, err := v.callCrosTestFinder(
		ctx, ctfReq,
	)
	if err != nil {
		return fmt.Errorf("error calling cros-test-finder: %w", err)
	}

	for _, testSuite := range ctfResp.GetTestSuites() {
		if len(testSuite.GetTestCases().GetTestCases()) == 0 {
			return fmt.Errorf("no test cases found for test suite %q", testSuite)
		}
	}

	logging.Infof(ctx, "found test cases for suite %q", suiteName)

	return nil
}

func (v *validator) validateTemplateParameters(ctx context.Context, dir, repoRoot string, plan *planpb.SourceTestPlan) error {
	// Get the FieldDescriptor for template_parameters to check whether
	// TemplateParameters has been set for a given TestPlanStarlarkFile.
	templateParametersDesc := (&planpb.SourceTestPlan_TestPlanStarlarkFile{}).
		ProtoReflect().Descriptor().Fields().ByName("template_parameters")
	if templateParametersDesc == nil {
		panic("failed to find template_parameters descriptor")
	}

	for _, file := range plan.GetTestPlanStarlarkFiles() {
		if !file.ProtoReflect().Has(templateParametersDesc) {
			continue
		}

		templateParameters := file.GetTemplateParameters()
		if templateParameters.GetTagCriteria() == nil && templateParameters.GetProgram() == "" {
			return fmt.Errorf("%s: either tag_criteria or program must be set on TemplateParameters", file.Path)
		}

		if templateParameters.GetTagCriteria() != nil {
			if err := v.validateTagCriteriaTemplateParameters(ctx, file); err != nil {
				return fmt.Errorf("error validating TagCriteria in TemplateParameters: %w", err)
			}
		}

		if templateParameters.GetProgram() != "" {
			if err := v.validateProgramTemplateParameters(templateParameters.GetProgram(), dir, repoRoot); err != nil {
				return fmt.Errorf("error validating program in TemplateParameters: %w", err)
			}
		}
	}

	return nil
}

// validateTagCriteriaTemplateParameters validates that the TagCriteria on
// file's TemplateParameters are set correctly. This method should only be
// called when the TagCriteria are non-nil and non-empty.
func (v *validator) validateTagCriteriaTemplateParameters(
	ctx context.Context,
	file *planpb.SourceTestPlan_TestPlanStarlarkFile,
) error {
	templateParameters := file.GetTemplateParameters()
	if templateParameters.GetSuiteName() == "" {
		return errors.New("suite_name must not be empty")
	}

	tagExcludes := templateParameters.GetTagCriteria().GetTagExcludes()
	if !stringset.NewFromSlice(tagExcludes...).Has("informational") {
		return fmt.Errorf(`tag_excludes must exclude "informational", got %q`, tagExcludes)
	}

	starlarkContent, err := v.gerritClient.DownloadFileFromGitiles(ctx, file.GetHost(), file.GetProject(), "HEAD", file.GetPath(), shared.LongerOpts)
	if err != nil {
		return fmt.Errorf("failed downloading file %q", file)
	}

	if !strings.Contains(starlarkContent, "testplan.get_suite_name()") && !strings.Contains(starlarkContent, "testplan.get_tag_criteria()") {
		return fmt.Errorf("file %q is not templated, setting TemplateParameters has no effect", file)
	}

	if v.checkTagCriteriaNonEmptyEnabled {
		return v.checkTagCriteriaNonEmpty(ctx, templateParameters)
	}

	logging.Infof(ctx, "tag criteria non-empty check disabled")
	return nil
}

var overlayRegex = regexp.MustCompile(`overlay-(\w+)(-private)?`)

// validateProgramTemplateParameters validates that the program
// TemplateParameter is set correctly. This method should only be called when
// program is not the empty string.
func (v *validator) validateProgramTemplateParameters(
	program string,
	dir, repoRoot string,
) error {
	fullPath := filepath.Join(repoRoot, dir)
	matches := overlayRegex.FindStringSubmatch(fullPath)
	if matches == nil {
		return fmt.Errorf("program TemplateParameter is only allowed in overlay directories. Got: %q", fullPath)
	}

	if matches[1] != program {
		return fmt.Errorf("program TemplateParameter must match the overlay it is in. Got parameter %q, expected %q", program, matches[1])
	}

	return nil
}
