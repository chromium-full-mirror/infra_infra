// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configs

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/registry"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/common_configs"
	"infra/cros/cmd/cros_test_runner/data"
)

func init() {
	registry.RegisterCmpOption(cmp.AllowUnexported(common_configs.CommandExecutorPairedConfig{}))
}

func TestGenerateHwConfigs(t *testing.T) {
	ftt.Run("GenerateHwConfigs", t, func(t *ftt.Test) {
		ctx := context.Background()
		hwConfigs := GenerateHwConfigs(ctx, nil, nil, false)

		assert.Loosely(t, hwConfigs, should.NotBeNil)
		assert.Loosely(t, hwConfigs.MainConfigs, should.NotBeNil)
		assert.Loosely(t, len(hwConfigs.MainConfigs), should.BeGreaterThan(0))
	})

	ftt.Run("GenerateHwConfigs with CrosTestRunnerRequest", t, func(t *ftt.Test) {
		ctx := context.Background()
		req := &api.CrosTestRunnerDynamicRequest{
			OrderedTasks: []*api.CrosTestRunnerDynamicRequest_Task{
				{
					OrderedContainerRequests: []*api.ContainerRequest{
						{
							DynamicIdentifier: "container",
						},
					},
					Task: &api.CrosTestRunnerDynamicRequest_Task_Provision{
						Provision: &api.ProvisionTask{},
					},
				},
			},
		}
		hwConfigs := GenerateHwConfigs(ctx, nil, req, false)

		assert.Loosely(t, hwConfigs, should.NotBeNil)
		assert.Loosely(t, hwConfigs.MainConfigs, should.NotBeNil)
		assert.Loosely(t, len(hwConfigs.MainConfigs), should.BeGreaterThan(0))
		assert.Loosely(t, hwConfigs.MainConfigs, should.ContainMatch(GenericProvision_GenericProvisionExecutor))
	})

	ftt.Run("hwConfigsForPlatform for VM", t, func(t *ftt.Test) {
		hwConfigs := hwConfigsForPlatform(nil, common.BotProviderGce, false)

		assert.Loosely(t, hwConfigs.MainConfigs, should.ContainMatch(VMProvisionRelease_CrosVMProvisionExecutor.WithRequired(true)))
		assert.Loosely(t, hwConfigs.MainConfigs, should.NotContain(DutServerStart_CrosDutExecutor))
		assert.Loosely(t, hwConfigs.MainConfigs, should.NotContain(UpdateDutState_NoExecutor.WithRequired(true)))
	})

	ftt.Run("hwConfigsForPlatform for HW", t, func(t *ftt.Test) {
		hwConfigs := hwConfigsForPlatform(nil, common.BotProviderDrone, false)

		assert.Loosely(t, hwConfigs.MainConfigs, should.ContainMatch(DutServerStart_CrosDutExecutor))
		assert.Loosely(t, hwConfigs.MainConfigs, should.ContainMatch(UpdateDutState_NoExecutor.WithRequired(true)))
	})
}

func TestGeneratePreLocalConfigs(t *testing.T) {
	ftt.Run("GeneratePreLocalConfigs", t, func(t *ftt.Test) {
		ctx := context.Background()
		preLocalConfigs := GeneratePreLocalConfigs(ctx)

		assert.Loosely(t, preLocalConfigs, should.NotBeNil)
		assert.Loosely(t, preLocalConfigs.MainConfigs, should.NotBeNil)
		assert.Loosely(t, len(preLocalConfigs.MainConfigs), should.BeGreaterThan(0))
	})
}

func TestGenerateLocalConfigs(t *testing.T) {
	ftt.Run("GenerateLocalConfigs", t, func(t *ftt.Test) {
		ctx := context.Background()
		localConfigs := GenerateLocalConfigs(ctx, &data.LocalTestStateKeeper{Args: &data.LocalArgs{}})

		assert.Loosely(t, localConfigs, should.NotBeNil)
		assert.Loosely(t, localConfigs.MainConfigs, should.NotBeNil)
		assert.Loosely(t, localConfigs.CleanupConfigs, should.NotBeNil)
		assert.Loosely(t, len(localConfigs.MainConfigs), should.BeGreaterThan(0))
		assert.Loosely(t, len(localConfigs.CleanupConfigs), should.BeGreaterThan(0))
	})
}
