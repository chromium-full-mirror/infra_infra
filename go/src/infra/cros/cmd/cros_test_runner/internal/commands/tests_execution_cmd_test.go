// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands_test

import (
	"context"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/containers"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/cmd/cros_test_runner/data"
	"infra/cros/cmd/cros_test_runner/internal/commands"
	"infra/cros/cmd/cros_test_runner/internal/executors"
)

func TestTestsExecutionCmd_UnsupportedSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Unsupported state keeper", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosTestTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosTestExecutor(cont)
		cmd := commands.NewTestsExecutionCmd(exec)
		sk := &UnsupportedStateKeeper{}
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestTestsExecutionCmd_MissingDeps(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd missing deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosTestTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosTestExecutor(cont)
		cmd := commands.NewTestsExecutionCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestTestsExecutionCmd_UpdateSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd with no updates", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosTestTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosTestExecutor(cont)
		cmd := commands.NewTestsExecutionCmd(exec)
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestTestsExecutionCmd_ExtractDepsSuccess(t *testing.T) {
	t.Parallel()

	ftt.Run("TestsExecutionCmd extract deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{
			CftTestRequest: &skylab_test_runner.CFTTestRequest{
				TestSuites: []*api.TestSuite{
					&testapi.TestSuite{},
				},
			},
			PrimaryDevice: &api.CrosTestRequest_Device{
				Dut: &labapi.Dut{},
			},
			DutServerAddress: &labapi.IpEndpoint{},
		}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosTestTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosTestExecutor(cont)
		cmd := commands.NewTestsExecutionCmd(exec)

		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestTestsExecutionCmd_UpdateSKSuccess(t *testing.T) {
	t.Parallel()

	ftt.Run("TestsExecutionCmd update SK", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{
			Injectables: common.NewInjectableStorage(),
		}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosTestTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosTestExecutor(cont)
		cmd := commands.NewTestsExecutionCmd(exec)

		wantTestResp := &testapi.CrosTestResponse{
			TestCaseResults: []*testapi.TestCaseResult{
				{
					TestCaseId: &testapi.TestCase_Id{},
				},
			},
		}
		wantTkoPublishSrcDir := "tko/src/dir"
		cmd.TestResponses = wantTestResp
		cmd.TkoPublishSrcDir = wantTkoPublishSrcDir

		// Update SK
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, sk.TestResponses, should.NotBeNil)
		assert.Loosely(t, sk.TkoPublishSrcDir, should.NotEqual(""))
		assert.Loosely(t, sk.TestResultForRdb, should.NotBeNil)
		assert.Loosely(t, sk.TestResponses, should.Equal(wantTestResp))
		assert.Loosely(t, sk.TkoPublishSrcDir, should.Equal(wantTkoPublishSrcDir))
	})
}
