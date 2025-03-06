// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"fmt"
	"sync/atomic"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/unifiedfleet/app/config"
	"go.chromium.org/infra/unifiedfleet/app/model/inventory"
)

var branchNum uint32 = 0

func mockMaintenanceConfigs() *config.Config {
	return &config.Config{
		MaintenanceConfigs: &config.MaintenanceConfigs{
			GitilesHost: "test_gitiles",
			Project:     "test_project",
			Branch:      fmt.Sprintf("test_branch_%d", atomic.AddUint32(&branchNum, 1)),
			MaintenanceConfig: []*config.MaintenanceConfigs_ConfigFile{
				{
					Name:       "test_name",
					RemotePath: "test_maintenance_git_path",
				},
			},
		},
	}
}

func TestImportBotMaintenanceConfigs(t *testing.T) {
	ctx := encTestingContext()
	ftt.Run("Import BotMaintenanceConfigs", t, func(t *ftt.Test) {
		contextConfig := mockMaintenanceConfigs()
		ctx = config.Use(ctx, contextConfig)
		t.Run("happy path - Bot ID for MachineLSE", func(t *ftt.Test) {
			ctx := encTestingContext()
			ctx = config.Use(ctx, contextConfig)
			maintenanceConfigs, gitClient, err := GetMaintenanceConfigAndGitClient(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, maintenanceConfigs, should.NotBeNil)
			assert.Loosely(t, maintenanceConfigs.GetGitilesHost(), should.Match("test_gitiles"))

			resp, err := inventory.CreateMachineLSE(ctx, MockMachineLSE("mac-608-e504"))
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.MaintenanceConfigName, should.BeEmpty)

			botMaintenanceConfig, err := GetBotMaintenanceConfig(ctx, "mac-608-e504")
			assert.Loosely(t, botMaintenanceConfig, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)

			err = ImportMaintenanceConfig(ctx, maintenanceConfigs, gitClient)
			assert.Loosely(t, err, should.BeNil)

			resp, err = inventory.GetMachineLSE(ctx, "mac-608-e504")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.MaintenanceConfigName, should.Match("chrome-swarming_chrome.tests.gpu.debug_1"))

			// Import Again, should not update the Asset
			err = ImportMaintenanceConfig(ctx, maintenanceConfigs, gitClient)
			assert.Loosely(t, err, should.BeNil)
			resp2, err := inventory.GetMachineLSE(ctx, "mac-608-e504")
			assert.Loosely(t, resp2, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp2.GetUpdateTime(), should.Match(resp.GetUpdateTime()))
			assert.Loosely(t, resp2.MaintenanceConfigName, should.Match("chrome-swarming_chrome.tests.gpu.debug_1"))
		})
	})

}
