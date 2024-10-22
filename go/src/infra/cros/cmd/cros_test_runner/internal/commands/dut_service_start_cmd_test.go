// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands_test

import (
	"context"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/cmd/common_lib/containers"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/cmd/cros_test_runner/data"
	"infra/cros/cmd/cros_test_runner/internal/commands"
	"infra/cros/cmd/cros_test_runner/internal/executors"
)

func TestDutServiceStartCmd_UnsupportedSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Unsupported state keeper", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &UnsupportedStateKeeper{}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosDutTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosDutExecutor(cont)
		cmd := commands.NewDutServiceStartCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestDutServiceStartCmd_MissingDeps(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd missing deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosDutTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosDutExecutor(cont)
		cmd := commands.NewDutServiceStartCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestDutServiceStartCmd_UpdateSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd with no updates", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{CftTestRequest: nil}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosDutTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosDutExecutor(cont)
		cmd := commands.NewDutServiceStartCmd(exec)
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestDutServiceStartCmd_ExtractDepsSuccess(t *testing.T) {
	t.Parallel()

	ftt.Run("DutServiceStartCmd extract deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		dutTopo := &labapi.DutTopology{
			Duts: []*labapi.Dut{
				{
					CacheServer: &labapi.CacheServer{Address: &labapi.IpEndpoint{}},
					DutType: &labapi.Dut_Chromeos{
						Chromeos: &labapi.Dut_ChromeOS{
							Ssh: &labapi.IpEndpoint{},
						},
					},
				},
			},
		}
		primaryDevice := &api.CrosTestRequest_Device{
			Dut: dutTopo.Duts[0],
		}
		sk := &data.HwTestStateKeeper{PrimaryDevice: primaryDevice}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosDutTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosDutExecutor(cont)
		cmd := commands.NewDutServiceStartCmd(exec)

		// Extract deps first
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestDutServiceStartCmd_UpdateSKSuccess(t *testing.T) {
	t.Parallel()
	ftt.Run("DutServiceStartCmd update SK", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := data.NewHwTestStateKeeper()
		sk.HostName = "DUT-1234"
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosDutTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosDutExecutor(cont)
		cmd := commands.NewDutServiceStartCmd(exec)
		cmd.DutServerAddress = &labapi.IpEndpoint{}

		// Update SK
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, sk.DutServerAddress, should.NotBeNil)
	})
}
