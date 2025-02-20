// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inventory

import (
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "infra/unifiedfleet/api/v1/models"
	. "infra/unifiedfleet/app/model/datastore"
)

func mockMaintenanceConfig(name string) *ufspb.MaintenanceConfig {
	return &ufspb.MaintenanceConfig{
		Name: name,
	}
}

func TestGetMaintenanceConfig(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	config1 := mockMaintenanceConfig("chromium-gpu-test_luci.chromium.gpu.ci_1")
	ftt.Run("GetMaintenanceConfig", t, func(t *ftt.Test) {
		t.Run("Get maintenance configuration by name", func(t *ftt.Test) {
			resp, err := UpdateMaintenanceConfigs(ctx, []*ufspb.MaintenanceConfig{config1})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp[0], should.Match(config1))
			respDr, err := GetMaintenanceConfig(ctx, "chromium-gpu-test_luci.chromium.gpu.ci_1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, respDr, should.Match(config1))
		})
		t.Run("Get maintenance configuration by non-existing name", func(t *ftt.Test) {
			resp, err := GetMaintenanceConfig(ctx, "chromium-gpu-test_luci.chromium.gpu.ci_2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get maintenance configuration record - invalid Name", func(t *ftt.Test) {
			resp, err := GetMaintenanceConfig(ctx, "")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestUpdateMaintenanceConfigs(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	ftt.Run("Update MaintenanceConfigs", t, func(t *ftt.Test) {
		t.Run("Update non-existing maintenanceConfigs", func(t *ftt.Test) {
			config1 := mockMaintenanceConfig("chromium-gpu-test_luci.chromium.gpu.ci_1")
			resp, err := UpdateMaintenanceConfigs(ctx, []*ufspb.MaintenanceConfig{config1})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(1))
			assert.Loosely(t, resp[0], should.Match(config1))
		})

		t.Run("Update existing MaintenanceConfig", func(t *ftt.Test) {
			config1 := mockMaintenanceConfig("chromium-gpu-test_luci.chromium.gpu.ci_1")
			resp, err := UpdateMaintenanceConfigs(ctx, []*ufspb.MaintenanceConfig{config1})
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)

			resp, err = UpdateMaintenanceConfigs(ctx, []*ufspb.MaintenanceConfig{config1})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(1))
			assert.Loosely(t, resp[0], should.Match(config1))
		})

		t.Run("Update MaintenanceConfig - invalid name", func(t *ftt.Test) {
			config1 := mockMaintenanceConfig("")
			resp, err := UpdateMaintenanceConfigs(ctx, []*ufspb.MaintenanceConfig{config1})
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Empty"))
		})
	})
}
