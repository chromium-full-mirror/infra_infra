// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands_test

import (
	"context"
	"fmt"
	"testing"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	bbpb "go.chromium.org/luci/buildbucket/proto"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/containers"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/cmd/cros_test_runner/data"
	"infra/cros/cmd/cros_test_runner/internal/commands"
	"infra/cros/cmd/cros_test_runner/internal/executors"
)

func TestGcsPublishPublishCmd_UnsupportedSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Unsupported state keeper", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &UnsupportedStateKeeper{}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := executors.NewCrosPublishExecutor(
			cont,
			executors.CrosGcsPublishExecutorType)
		cmd := commands.NewGcsPublishUploadCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestGcsPublishPublishCmd_MissingDeps(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd missing deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := executors.NewCrosPublishExecutor(
			cont,
			executors.CrosGcsPublishExecutorType)
		cmd := commands.NewGcsPublishUploadCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestGcsPublishPublishCmd_UpdateSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd with no updates", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{CftTestRequest: nil}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := executors.NewCrosPublishExecutor(
			cont,
			executors.CrosGcsPublishExecutorType)
		cmd := commands.NewGcsPublishUploadCmd(exec)
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestGcsPublishPublishCmd_ExtractDepsSuccess(t *testing.T) {
	t.Parallel()

	ftt.Run("GcsPublishStartCmd extract deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantGcsURL := "gs://gcs/url"
		board := "brya"
		model := "gimble"
		wantParentTaskID := "123456789"
		wantBuild := "brya-release/R131-16063.0.0"
		wantALRun := true
		wantEnableXTSArchiver := true

		buildPb := &bbpb.Build{
			Tags: []*buildbucketpb.StringPair{
				{Key: "build", Value: wantBuild},
				{Key: "parent_task_id", Value: wantParentTaskID},
			},
			Input: &buildbucketpb.Build_Input{
				Experiments: []string{common.EnableXTSArchiverExperiment},
			},
		}
		buildState, ctx, err := build.Start(ctx, buildPb)
		defer func() { buildState.End(err) }()

		sk := &data.HwTestStateKeeper{
			GcsURL:     wantGcsURL,
			IsAlRun:    wantALRun,
			BuildState: buildState,
			PrimaryDutModel: &labapi.DutModel{
				BuildTarget: board,
				ModelName:   model,
			},
		}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := executors.NewCrosPublishExecutor(
			cont,
			executors.CrosGcsPublishExecutorType)
		cmd := commands.NewGcsPublishUploadCmd(exec)

		// Extract deps first
		err = cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cmd.GcsURL, should.Equal(wantGcsURL))
		assert.Loosely(t, cmd.IsALRun, should.Equal(wantALRun))
		assert.Loosely(t, cmd.Product, should.Equal(fmt.Sprintf("%s.%s", board, model)))
		assert.Loosely(t, cmd.Build, should.Equal(wantBuild))
		assert.Loosely(t, cmd.ParentSwarmingTaskID, should.Equal(wantParentTaskID))
		assert.Loosely(t, cmd.EnableXTSArchiver, should.Equal(wantEnableXTSArchiver))
	})
}
