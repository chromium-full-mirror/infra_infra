// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package executions contains the various executions for the uprev service.
package executions

import (
	"context"
	"log"
	"os"
	"path"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/container_uprev/internal"
)

var UpdateShaStorage = internal.UpdateShaStorage

// LuciBuildExecution represents build executions.
func LuciBuildExecution() {
	build.Main(
		func(ctx context.Context, args []string, st *build.State) error {
			isProd := false
			for _, arg := range args {
				if arg == "-prod" {
					isProd = true
				}
			}
			log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmsgprefix)
			dockerKeyFile := common.VmLabDockerKeyFileLocation
			label := common.LabelStaging
			if isProd {
				label = common.LabelProd
			}
			err := executeContainerUprev(ctx, dockerKeyFile, label, label)
			if err != nil {
				logging.Infof(ctx, "error found: %s", err)
				st.SetSummaryMarkdown(err.Error())
			}
			return err
		},
	)
}

// LocalBuildExecution performs local building of the images.
func LocalBuildExecution(cipdLabel, imageTag string, runAsAdmin bool) {
	execPath, _ := os.Executable()
	logDir, _ := os.MkdirTemp(path.Dir(execPath), "uprev")
	emptyBuild := &buildbucketpb.Build{}
	buildState, ctx, err := build.Start(context.Background(), emptyBuild)

	logCfg := common.LoggerConfig{Out: log.Default().Writer()}
	ctx = logCfg.Use(ctx)
	defer func() {
		buildState.End(err)
		logCfg.DumpStepsToFolder(logDir)
	}()

	if !runAsAdmin {
		// Do not update the sha storage on local execution.
		UpdateShaStorage = func(ctx context.Context, containerSHAs map[string]*common.ContainerInfoItem, creds, label string) (err error) {
			logging.Infof(ctx, "Local execution, skipping sha storage update")
			return nil
		}

		if imageTag == common.LabelStaging || imageTag == common.LabelProd {
			logging.Infof(ctx, "Cannot tag local image execution with prod/staging")
			return
		}
	}

	err = executeContainerUprev(ctx, "", cipdLabel, imageTag)
	if err != nil {
		logging.Infof(ctx, "%s", err)
	}
}

// executeContainerUprev steps through the uprev configs, creates a new container,
// and uploads its sha to the storage.
func executeContainerUprev(ctx context.Context, dockerKeyFile, cipdLabel, imageTag string) (err error) {
	containerInfos := map[string]*common.ContainerInfoItem{}
	configs := internal.GetConfigs()
	for _, config := range configs {
		if err = internal.GcloudAuth(ctx, config.RepositoryHostname, dockerKeyFile); err != nil {
			err = errors.Annotate(err, "failed to Gcloud auth").Err()
			return
		}

		sha, uprevErr := internal.UprevContainer(ctx, config, cipdLabel, imageTag)
		if uprevErr != nil {
			err = errors.Append(err, uprevErr)
		} else {
			containerInfo := common.NewContainerInfoItem(config.RepositoryHostname, config.RepositoryProject, sha)
			containerInfos[config.Name] = containerInfo
		}
	}

	if shaErr := UpdateShaStorage(ctx, containerInfos, dockerKeyFile, imageTag); shaErr != nil {
		shaErr = errors.Annotate(shaErr, "failed to update SHAs").Err()
		err = errors.Append(err, shaErr)
		return
	}

	return
}
