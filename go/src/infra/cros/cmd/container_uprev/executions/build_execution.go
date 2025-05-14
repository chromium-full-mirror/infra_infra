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
func LuciBuildExecution(targetConfig string) {
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
			err := executeContainerUprev(ctx, dockerKeyFile, label, label, targetConfig)
			if err != nil {
				logging.Infof(ctx, "error found: %s", err)
				st.SetSummaryMarkdown(err.Error())
			}
			return err
		},
	)
}

// LocalBuildExecution performs local building of the images.
func LocalBuildExecution(cipdLabel, imageTag, targetConfig string, runAsAdmin bool) {
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
		UpdateShaStorage = func(ctx context.Context, _ string, _ map[string]*common.ContainerInfoItem, _, _ string) (_ error) {
			logging.Infof(ctx, "Local execution, skipping sha storage update")
			return nil
		}

		if imageTag == common.LabelStaging || imageTag == common.LabelProd {
			logging.Infof(ctx, "Cannot tag local image execution with prod/staging")
			return
		}
	}

	err = executeContainerUprev(ctx, "", cipdLabel, imageTag, targetConfig)
	if err != nil {
		logging.Infof(ctx, "%s", err)
	}
}

// executeContainerUprev steps through the uprev configs, creates a new container,
// and uploads its sha to the storage.
func executeContainerUprev(ctx context.Context, dockerKeyFile, cipdLabel, imageTag, targetConfig string) (err error) {
	containerInfosByFirestore := map[string]internal.ContainerInfosMap{}
	configs := internal.GetConfigs()

	configsByRepoHostname := map[string][]*internal.UprevConfig{}
	for _, config := range configs {
		if targetConfig != "" && config.Name != targetConfig {
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
	imageCache := map[string]any{}
	for repoHostname, repoConfigs := range configsByRepoHostname {
		step, ctx := build.StartStep(ctx, fmt.Sprintf("Repository: %s", repoHostname))
		if err = internal.GcloudAuth(ctx, repoHostname, dockerKeyFile); err != nil {
			err = errors.Annotate(err, "failed to Gcloud auth").Err()
			return
		}

		for _, config := range repoConfigs {
			logging.Infof(ctx, "Running build for %q", config.Name)
			// Will have exactly one repository after upstream mapping.
			repo := config.Repositories[0]
			containerInfo, uprevErr := internal.UprevContainer(ctx, imageCache, config, cipdLabel, imageTag)
			if uprevErr != nil {
				logging.Infof(ctx, "error while upreving: %s", uprevErr)
				err = errors.Append(err, uprevErr)
				continue
			}
			if _, ok := containerInfosByFirestore[repo.FirestoreHost]; !ok {
				containerInfosByFirestore[repo.FirestoreHost] = internal.ContainerInfosMap{}
			}
			containerInfosByFirestore[repo.FirestoreHost][config.FirestoreName] = containerInfo
		}

		step.End(err)
	}

	for firestoreHost, containerInfos := range containerInfosByFirestore {
		step, ctx := build.StartStep(ctx, fmt.Sprintf("Firestore: %s", firestoreHost))

		if shaErr := UpdateShaStorage(ctx, firestoreHost, containerInfos, dockerKeyFile, imageTag); shaErr != nil {
			shaErr = errors.Annotate(shaErr, "failed to update SHAs").Err()
			err = errors.Append(err, shaErr)
			return
		}

		step.End(err)
	}

	return
}
