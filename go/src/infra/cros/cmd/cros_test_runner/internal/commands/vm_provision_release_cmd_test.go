// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands_test

import (
	"context"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/cmd/common_lib/containers"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/cmd/cros_test_runner/data"
	"infra/cros/cmd/cros_test_runner/internal/commands"
	"infra/cros/cmd/cros_test_runner/internal/executors"
)

func TestVMProvisionReleaseCmd_NoDeps(t *testing.T) {
	t.Parallel()
	ftt.Run("No deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosVMProvisionTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosVMProvisionExecutor(cont)
		cmd := commands.NewVMProvisionReleaseCmd(exec)
		sk := &data.HwTestStateKeeper{}
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
	ftt.Run("No deps - GceRegion", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosVMProvisionTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosVMProvisionExecutor(cont)
		cmd := commands.NewVMProvisionReleaseCmd(exec)
		sk := &data.HwTestStateKeeper{LeaseVMResponse: &api.LeaseVMResponse{LeaseId: "xyz"}}
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
	ftt.Run("No deps - LeaseID", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosVMProvisionTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosVMProvisionExecutor(cont)
		cmd := commands.NewVMProvisionReleaseCmd(exec)
		sk := &data.HwTestStateKeeper{LeaseVMResponse: &api.LeaseVMResponse{Vm: &api.VM{GceRegion: "region1"}}}
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestVMProvisionReleaseCmd_Updates(t *testing.T) {
	t.Parallel()
	ftt.Run("No updates", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosVMProvisionTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosVMProvisionExecutor(cont)
		cmd := commands.NewVMProvisionReleaseCmd(exec)
		sk := &data.HwTestStateKeeper{LeaseVMResponse: &api.LeaseVMResponse{LeaseId: "xyz", Vm: &api.VM{GceRegion: "region1"}}}
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, sk.DutTopology, should.BeNil)
	})
}
