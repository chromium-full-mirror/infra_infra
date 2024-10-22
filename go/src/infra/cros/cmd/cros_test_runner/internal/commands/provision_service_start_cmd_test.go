// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands_test

import (
	"context"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/cmd/common_lib/containers"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/cmd/cros_test_runner/data"
	"infra/cros/cmd/cros_test_runner/internal/commands"
	"infra/cros/cmd/cros_test_runner/internal/executors"
)

func TestProvisionStartCmd_UnsupportedSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Unsupported state keeper", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &UnsupportedStateKeeper{}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosProvisionTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosProvisionExecutor(cont)
		cmd := commands.NewProvisionServiceStartCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestProvisionStartCmd_MissingDeps(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd missing deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosProvisionTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosProvisionExecutor(cont)
		cmd := commands.NewProvisionServiceStartCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestProvisionStartCmd_UpdateSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd with no updates", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{CftTestRequest: nil}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosProvisionTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosProvisionExecutor(cont)
		cmd := commands.NewProvisionServiceStartCmd(exec)
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestProvisionStartCmd_ExtractDepsSuccess(t *testing.T) {
	t.Parallel()

	ftt.Run("ProvisionStartCmd extract deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{
			CftTestRequest: &skylab_test_runner.CFTTestRequest{
				PrimaryDut: &skylab_test_runner.CFTTestRequest_Device{
					ProvisionState: &api.ProvisionState{},
				},
			},
			PrimaryDevice: &api.CrosTestRequest_Device{
				Dut: &labapi.Dut{},
			}, DutServerAddress: &labapi.IpEndpoint{}}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosProvisionTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosProvisionExecutor(cont)
		cmd := commands.NewProvisionServiceStartCmd(exec)

		// Extract deps first
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}
