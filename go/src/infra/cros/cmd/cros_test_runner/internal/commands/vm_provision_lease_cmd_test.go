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
	vmlabapi "infra/libs/vmlab/api"
)

func TestVMProvisionLeaseCmd_NoDeps(t *testing.T) {
	t.Parallel()
	ftt.Run("No deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosVMProvisionTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosVMProvisionExecutor(cont)
		cmd := commands.NewVMProvisionLeaseCmd(exec)
		sk := &data.HwTestStateKeeper{}
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
	ftt.Run("No deps - name", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosVMProvisionTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosVMProvisionExecutor(cont)
		cmd := commands.NewVMProvisionLeaseCmd(exec)
		sk := &data.HwTestStateKeeper{DutVmGceImage: &vmlabapi.GceImage{Project: "project"}}
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
	ftt.Run("No deps - project", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosVMProvisionTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosVMProvisionExecutor(cont)
		cmd := commands.NewVMProvisionLeaseCmd(exec)
		sk := &data.HwTestStateKeeper{DutVmGceImage: &vmlabapi.GceImage{Name: "name"}}
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestVMProvisionLeaseCmd_Updates(t *testing.T) {
	t.Parallel()
	ftt.Run("No updates", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosVMProvisionTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosVMProvisionExecutor(cont)
		cmd := commands.NewVMProvisionLeaseCmd(exec)
		cmd.LeaseVMResponse = &api.LeaseVMResponse{}
		sk := &data.HwTestStateKeeper{
			DutVmGceImage: &vmlabapi.GceImage{Name: "name", Project: "project"},
			CrosTestRunnerRequest: &api.CrosTestRunnerDynamicRequest{
				Params: &api.CrosTestRunnerParams{
					PrimaryDut: &labapi.DutModel{
						BuildTarget: "betty",
					},
				},
			},
		}
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, sk.DutTopology, should.NotBeNil)
	})
}
