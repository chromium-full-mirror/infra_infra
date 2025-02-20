// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands_test

import (
	"context"
	"fmt"
	"testing"

	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/containers"
	"go.chromium.org/infra/cros/cmd/common_lib/tools/crostoolrunner"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/commands"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/executors"
)

func TestCpconPublishPublishCmd_UnsupportedSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Unsupported state keeper", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &UnsupportedStateKeeper{}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewGenericTemplatedContainer(
			containers.CrosPublishTemplatedContainerType,
			"container/image/path",
			"cros-publish",
			ctr)
		exec := executors.NewCrosPublishExecutor(
			cont,
			executors.CrosPublishExecutorType)
		cmd := commands.NewCpconPublishUploadCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestCpconPublishPublishCmd_MissingDeps(t *testing.T) {
	t.Setenv("SWARMING_TASK_ID", "")

	ftt.Run("Cmd missing deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewGenericTemplatedContainer(
			containers.CrosPublishTemplatedContainerType,
			"container/image/path",
			"cros-publish",
			ctr)
		exec := executors.NewCrosPublishExecutor(
			cont,
			executors.CrosPublishExecutorType)
		cmd := commands.NewCpconPublishUploadCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestCpconPublishPublishCmd_UpdateSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd with no updates", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{CftTestRequest: nil}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewGenericTemplatedContainer(
			containers.CrosPublishTemplatedContainerType,
			"container/image/path",
			"cros-publish",
			ctr)
		exec := executors.NewCrosPublishExecutor(
			cont,
			executors.CrosPublishExecutorType)
		cmd := commands.NewCpconPublishUploadCmd(exec)
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestCpconPublishPublishCmd_ExtractDepsSuccess(t *testing.T) {
	wantSwarmingTaskId := "123456789abcdef0"
	t.Setenv("SWARMING_TASK_ID", wantSwarmingTaskId)

	ftt.Run("ProvisionStartCmd extract deps", t, func(t *ftt.Test) {

		ctx := context.Background()
		wantGcsURL := "gs://this-is-a-gcs-path/results"
		wantBuild := "brya-release/R131-16063.0.0"
		wantSuite := "cts"
		buildPb := &bbpb.Build{
			Tags: []*bbpb.StringPair{
				{Key: "build", Value: wantBuild},
				{Key: "parent_task_id", Value: wantSwarmingTaskId},
				{Key: "label-suite", Value: wantSuite},
			},
		}
		buildState, ctx, err := build.Start(ctx, buildPb)
		defer func() { buildState.End(err) }()
		sk := &data.HwTestStateKeeper{
			GcsURL:             wantGcsURL,
			CpconPublishSrcDir: "this/is/a/fake/path",
			BuildState:         buildState,
		}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewGenericTemplatedContainer(
			containers.CrosPublishTemplatedContainerType,
			"container/image/path",
			"cros-publish",
			ctr)
		exec := executors.NewCrosPublishExecutor(
			cont,
			executors.CrosPublishExecutorType)
		cmd := commands.NewCpconPublishUploadCmd(exec)

		// Extract deps first
		err = cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cmd.CpconJobName, should.Equal(fmt.Sprintf("swarming-%s", wantSwarmingTaskId)))
		assert.Loosely(t, cmd.GcsURL, should.Equal(wantGcsURL))
		assert.Loosely(t, cmd.Build, should.Equal(wantBuild))
		assert.Loosely(t, cmd.Suite, should.Equal(wantSuite))
		assert.Loosely(t, cmd.ParentSwarmingTaskID, should.Equal(wantSwarmingTaskId))
	})

}
