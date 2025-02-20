// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands_test

import (
	"context"
	"testing"

	"google.golang.org/protobuf/types/known/anypb"

	api "go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/cmd/common_lib/containers"
	"go.chromium.org/infra/cros/cmd/common_lib/tools/crostoolrunner"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/commands"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/executors"
)

func TestAndroidCompanionDutServiceStartCmd_UnsupportedSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Unsupported state keeper", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &UnsupportedStateKeeper{}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewAndroidDutTemplatedContainer("container/image/path", ctr)
		exec := executors.NewAndroidDutExecutor(cont)
		cmd := commands.NewAndroidCompanionDutServiceStartCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestAndroidCompanionDutServiceStartCmd_MissingDeps(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd missing deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewAndroidDutTemplatedContainer("container/image/path", ctr)
		exec := executors.NewAndroidDutExecutor(cont)
		cmd := commands.NewAndroidCompanionDutServiceStartCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestAndroidCompanionDutServiceStartCmd_UpdateSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd with no updates", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{CftTestRequest: nil}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewAndroidDutTemplatedContainer("container/image/path", ctr)
		exec := executors.NewAndroidDutExecutor(cont)
		cmd := commands.NewAndroidCompanionDutServiceStartCmd(exec)
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestAndroidCompanionDutServiceStartCmd_ExtractDepsSuccess(t *testing.T) {
	t.Parallel()

	ftt.Run("AndroidCompanionDutServiceStartCmd extract deps", t, func(t *ftt.Test) {
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
				{
					DutType: &labapi.Dut_Android_{
						Android: &labapi.Dut_Android{
							DutModel: &labapi.DutModel{
								BuildTarget: "pixel6",
							},
						},
					},
				},
			},
		}
		androidProvisionRequestMetadata := &api.AndroidProvisionRequestMetadata{
			AndroidOsImage: &api.AndroidOsImage{
				LocationOneof: &api.AndroidOsImage_OsVersion{
					OsVersion: "R98.3451.0.1",
				},
			},
		}
		companionDevices := []*api.CrosTestRequest_Device{
			{
				Dut: &labapi.Dut{
					DutType: &labapi.Dut_Android_{
						Android: &labapi.Dut_Android{
							DutModel: &labapi.DutModel{
								BuildTarget: "pixel6",
							},
						},
					},
				},
			},
		}
		primaryDevice := &api.CrosTestRequest_Device{
			Dut: dutTopo.Duts[0],
		}

		provisionMetadata, _ := anypb.New(androidProvisionRequestMetadata)

		cftTestReq := &skylab_test_runner.CFTTestRequest{
			CompanionDuts: []*skylab_test_runner.CFTTestRequest_Device{
				{
					DutModel: &labapi.DutModel{
						BuildTarget: "pixel6",
					},
					ProvisionState: &api.ProvisionState{
						ProvisionMetadata: provisionMetadata,
					},
				},
			},
		}
		sk := &data.HwTestStateKeeper{CftTestRequest: cftTestReq, CompanionDevices: companionDevices, PrimaryDevice: primaryDevice}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewAndroidDutTemplatedContainer("container/image/path", ctr)
		exec := executors.NewAndroidDutExecutor(cont)
		cmd := commands.NewAndroidCompanionDutServiceStartCmd(exec)

		// Extract deps first
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestAndroidCompanionDutServiceStartCmd_UpdateSKSuccess(t *testing.T) {
	t.Parallel()
	ftt.Run("AndroidCompanionDutServiceStartCmd update SK", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{HostName: "DUT-1234"}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewAndroidDutTemplatedContainer("container/image/path", ctr)
		exec := executors.NewAndroidDutExecutor(cont)
		cmd := commands.NewAndroidCompanionDutServiceStartCmd(exec)
		cmd.AndroidDutServerAddress = &labapi.IpEndpoint{}

		// Update SK
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, sk.AndroidDutServerAddress, should.NotBeNil)
	})
}
