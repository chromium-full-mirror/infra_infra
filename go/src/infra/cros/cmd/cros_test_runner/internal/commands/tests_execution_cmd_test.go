// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands_test

import (
	"context"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/containers"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/cmd/cros_test_runner/data"
	"infra/cros/cmd/cros_test_runner/internal/commands"
	"infra/cros/cmd/cros_test_runner/internal/executors"
)

func TestTestsExecutionCmd_UnsupportedSK(t *testing.T) {
	t.Parallel()
	Convey("Unsupported state keeper", t, func() {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosTestTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosTestExecutor(cont)
		cmd := commands.NewTestsExecutionCmd(exec)
		sk := &UnsupportedStateKeeper{}
		err := cmd.ExtractDependencies(ctx, sk)
		So(err, ShouldNotBeNil)
	})
}

func TestTestsExecutionCmd_MissingDeps(t *testing.T) {
	t.Parallel()
	Convey("Cmd missing deps", t, func() {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosTestTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosTestExecutor(cont)
		cmd := commands.NewTestsExecutionCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		So(err, ShouldNotBeNil)
	})
}

func TestTestsExecutionCmd_UpdateSK(t *testing.T) {
	t.Parallel()
	Convey("Cmd with no updates", t, func() {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosTestTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosTestExecutor(cont)
		cmd := commands.NewTestsExecutionCmd(exec)
		err := cmd.UpdateStateKeeper(ctx, sk)
		So(err, ShouldBeNil)
	})
}

func TestTestsExecutionCmd_ExtractDepsSuccess(t *testing.T) {
	t.Parallel()

	Convey("TestsExecutionCmd extract deps", t, func() {
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
		So(err, ShouldBeNil)
	})
}

func TestTestsExecutionCmd_UpdateSKSuccess(t *testing.T) {
	t.Parallel()

	Convey("TestsExecutionCmd update SK", t, func() {
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
		So(err, ShouldBeNil)
		So(sk.TestResponses, ShouldNotBeNil)
		So(sk.TkoPublishSrcDir, ShouldNotBeNil)
		So(sk.TestResultForRdb, ShouldNotBeNil)
		So(sk.TestResponses, ShouldEqual, wantTestResp)
		So(sk.TkoPublishSrcDir, ShouldEqual, wantTkoPublishSrcDir)
	})
}
