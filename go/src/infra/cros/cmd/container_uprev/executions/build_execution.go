// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package executions contains the various executions for the uprev service.
package executions

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"slices"
	"sync"

	"go.chromium.org/luci/auth"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/container_uprev/internal"
	"go.chromium.org/infra/cros/cmd/container_uprev/internal/vars"
)

var UpdateShaStorage = internal.UpdateShaStorage

// LuciBuildExecution represents build executions.
func LuciBuildExecution() {
	build.RegisterInputProperty[*struct{}]("")
	build.Main(
		func(ctx context.Context, args []string, st *build.State) error {
			isProd := false
			for _, arg := range args {
				if arg == "-prod" {
					isProd = true
				}
			}
			log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmsgprefix)
			dockerKeyFile := common.VMLabDockerKeyFileLocation
			label := common.LabelStaging
			if isProd {
				label = common.LabelProd
			}
			err := executeContainerUprev(ctx, dockerKeyFile, label, label, []string{})
			if err != nil {
				logging.Infof(ctx, "error found: %s", err)
				st.SetSummaryMarkdown(err.Error())
			}
			return err
		},
	)
}

// LocalBuildExecution performs local building of the images.
func LocalBuildExecution(cipdLabel, imageTag string, targetConfigs []string, runAsAdmin bool) {
	execPath, _ := os.Executable()
	logDir, _ := os.MkdirTemp(path.Dir(execPath), "generated/uprev")
	emptyBuild := &buildbucketpb.Build{}
	buildState, ctx, err := build.Start(context.Background(), emptyBuild)

	logCfg := common.LoggerConfig{Out: log.Default().Writer()}
	ctx = logCfg.Use(ctx)
	defer func() {
		buildState.End(err)
		logCfg.DumpStepsToFolder(logDir)
	}()
	vars.LoginMode = auth.InteractiveLogin

	if !runAsAdmin {
		// Do not update the sha storage on local execution.
		UpdateShaStorage = func(ctx context.Context, _ string, _ *sync.Map, _, _ string) (_ error) {
			logging.Infof(ctx, "Local execution, skipping sha storage update")
			return nil
		}

		if imageTag == common.LabelStaging || imageTag == common.LabelProd {
			logging.Infof(ctx, "Cannot tag local image execution with prod/staging")
			return
		}
	}

	err = executeContainerUprev(ctx, "", cipdLabel, imageTag, targetConfigs)
	if err != nil {
		logging.Infof(ctx, "%s", err)
	}
}

// executeContainerUprev steps through the uprev configs, creates a new container,
// and uploads its sha to the storage.
func executeContainerUprev(ctx context.Context, dockerKeyFile, cipdLabel, imageTag string, targetConfigs []string) error {
	var err error
	configs := internal.GetConfigs()

	configsByRepoHostname := map[string][]*internal.UprevConfig{}
	for _, config := range configs {
		if len(targetConfigs) > 0 && !slices.Contains(targetConfigs, config.Name) {
			logging.Infof(ctx, "Skipping build of %q", config.Name)
			continue
		}
		for _, repo := range config.Repositories {
			if _, ok := configsByRepoHostname[repo.Hostname]; !ok {
				configsByRepoHostname[repo.Hostname] = []*internal.UprevConfig{}
			}
			configsByRepoHostname[repo.Hostname] = append(configsByRepoHostname[repo.Hostname], &internal.UprevConfig{
				Name:           config.Name,
				FirestoreName:  config.FirestoreName,
				Repositories:   []*internal.Repository{repo},
				ContainerName:  config.ContainerName,
				CIPDPackages:   config.CIPDPackages,
				Prepper:        config.Prepper,
				Resources:      config.Resources,
				Entrypoint:     config.Entrypoint,
				CloudRunConfig: config.CloudRunConfig,
			})
		}
	}

	// Keep track of already built container images.
	imageCache := &sync.Map{}
	containerInfosByFirestore := &sync.Map{}
	wg := &sync.WaitGroup{}
	for repoHostname, repoConfigs := range configsByRepoHostname {
		uprevContainers(ctx, wg, containerInfosByFirestore, repoConfigs, imageCache, repoHostname, dockerKeyFile, cipdLabel, imageTag)
	}

	containerInfosByFirestore.Range(func(key, val any) bool {
		firestoreHost := key.(string)
		containerInfos := val.(*sync.Map)

		step, ctx := build.StartStep(ctx, fmt.Sprintf("Firestore: %s", firestoreHost))
		var err error
		defer step.End(err)

		err = UpdateShaStorage(ctx, firestoreHost, containerInfos, dockerKeyFile, imageTag)
		return true
	})

	return err
}

func uprevContainers(ctx context.Context, wg *sync.WaitGroup, containerInfosByFirestore *sync.Map, repoConfigs []*internal.UprevConfig, imageCache *sync.Map, repoHostname, dockerKeyFile, cipdLabel, imageTag string) {
	var err error
	step, ctx := build.StartStep(ctx, fmt.Sprintf("Repository: %s", repoHostname))
	defer step.End(err)
	if err = internal.GcloudAuth(ctx, repoHostname, dockerKeyFile); err != nil {
		err = errors.Annotate(err, "failed to Gcloud auth").Err()
		return
	}

	for _, config := range repoConfigs {
		wg.Add(1)
		logging.Infof(ctx, "Running build for %q", config.Name)
		go uprevContainer(ctx, wg, containerInfosByFirestore, config, imageCache, cipdLabel, imageTag)
	}

	wg.Wait()
}

func uprevContainer(ctx context.Context, wg *sync.WaitGroup, containerInfosByFirestore *sync.Map, config *internal.UprevConfig, imageCache *sync.Map, cipdLabel, imageTag string) {
	var err error
	defer wg.Done()

	// Will have exactly one repository after upstream mapping.
	repo := config.Repositories[0]
	containerInfo, uprevErr := internal.UprevContainer(ctx, imageCache, config, cipdLabel, imageTag)
	if uprevErr != nil {
		logging.Infof(ctx, "error while upreving: %s", uprevErr)
		err = errors.Append(err, uprevErr)
		return
	}
	containerInfos, _ := containerInfosByFirestore.LoadOrStore(repo.FirestoreHost, &sync.Map{})
	containerInfos.(*sync.Map).Store(config.FirestoreName, containerInfo)
}
