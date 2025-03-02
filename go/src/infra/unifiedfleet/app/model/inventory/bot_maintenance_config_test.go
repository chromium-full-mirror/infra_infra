// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inventory

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
)

func mockBotMaintenanceConfig() *ufspb.BotMaintenanceConfig {
	return &ufspb.BotMaintenanceConfig{
		SwarmingInstance:      "test-swarming",
		Dimensions:            []string{"pool:gpu.test"},
		Owners:                []string{"gpu.test@google.com"},
		MaintenanceConfigName: "gpu_gpu.test_1",
	}
}

// Tests the functionality for Creating/Updating MaintenanceConfig data per bot in the datastore
func TestPutBotMaintenanceConfig(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)

	t.Run("add non-existent Bot maintenance config", func(t *testing.T) {
		maintenanceConfig := mockBotMaintenanceConfig()
		expectedId := "test"
		got, err := PutBotMaintenanceConfig(ctx, maintenanceConfig, expectedId)
		if err != nil {
			t.Fatalf("PutBotMaintenanceConfig failed: %s", err)
		}
		p, err := got.GetProto()
		if err != nil {
			t.Fatalf("Unmarshalling maintenance config from datatstore failed: %s", err)
		}
		pm := p.(*ufspb.BotMaintenanceConfig)
		if got.Id != expectedId {
			t.Errorf("PutBotMaintenanceConfig returned unexpected id:\n%s", got.Id)
		}
		if pm.MaintenanceConfigName != maintenanceConfig.MaintenanceConfigName {
			t.Errorf("PutBotMaintenanceConfig returned unexpected result for Maintenance config Name:\n%v", pm.MaintenanceConfigName)
		}
		if pm.SwarmingInstance != maintenanceConfig.SwarmingInstance {
			t.Errorf("PutBotMaintenanceConfig returned unexpected result for swarming instance:\n%v", pm.SwarmingInstance)
		}
	})

	t.Run("add existing Bot maintenance config", func(t *testing.T) {
		maintenanceConfig := mockBotMaintenanceConfig()
		expectedId := "test"
		got, err := PutBotMaintenanceConfig(ctx, maintenanceConfig, expectedId)
		if err != nil {
			t.Fatalf("PutBotMaintenanceConfig failed: %s", err)
		}
		p, err := got.GetProto()
		if err != nil {
			t.Fatalf("Unmarshalling maintenance config from datatstore failed: %s", err)
		}
		pm := p.(*ufspb.BotMaintenanceConfig)
		if got.Id != expectedId {
			t.Errorf("PutBotMaintenanceConfig returned unexpected id:\n%s", got.Id)
		}
		if pm.MaintenanceConfigName != maintenanceConfig.MaintenanceConfigName {
			t.Errorf("PutBotMaintenanceConfig returned unexpected result for Maintenance config Name:\n%v", pm.MaintenanceConfigName)
		}
		if pm.SwarmingInstance != maintenanceConfig.SwarmingInstance {
			t.Errorf("PutBotMaintenanceConfig returned unexpected result for swarming instance:\n%v", pm.SwarmingInstance)
		}
	})

	t.Run("add empty bot name", func(t *testing.T) {
		_, err := PutBotMaintenanceConfig(ctx, &ufspb.BotMaintenanceConfig{}, "")
		if err == nil {
			t.Errorf("PutBotMaintenanceConfig succeeded with empty name")
		}
		if c := status.Code(err); c != codes.Internal {
			t.Errorf("Unexpected error when calling PutBotMaintenanceConfig: %s", err)
		}
	})
}

// Tests the functionality for getting maintenance config per bot from the DataStore
func TestGetBotMaintenanceConfig(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)

	t.Run("get BotMaintenanceConfig by existing ID", func(t *testing.T) {
		maintenanceConfig := mockBotMaintenanceConfig()
		expectedId := "test"
		_, err := PutBotMaintenanceConfig(ctx, maintenanceConfig, expectedId)
		if err != nil {
			t.Fatalf("PutBotMaintenanceConfig failed: %s", err)
		}

		got, err := GetBotMaintenanceConfig(ctx, expectedId)
		if err != nil {
			t.Fatalf("GetBotMaintenanceConfig failed: %s", err)
		}
		if got.Id != expectedId {
			t.Errorf("GetBotMaintenanceConfig returned unexpected Id:\n%s", got.Id)
		}
		p, err := got.GetProto()
		if err != nil {
			t.Fatalf("Unmarshalling bot maintenance config from datatstore failed: %s", err)
		}

		pm := p.(*ufspb.BotMaintenanceConfig)
		if pm.MaintenanceConfigName != maintenanceConfig.MaintenanceConfigName {
			t.Errorf("PutBotMaintenanceConfig returned unexpected result for Maintenance config Name:\n%v", pm.MaintenanceConfigName)
		}
		if pm.SwarmingInstance != maintenanceConfig.SwarmingInstance {
			t.Errorf("PutBotMaintenanceConfig returned unexpected result for swarming instance:\n%v", pm.SwarmingInstance)
		}
	})

	t.Run("get BotMaintenanceConfig by non-existent ID", func(t *testing.T) {
		const expectedId = "test2"
		_, err := GetBotMaintenanceConfig(ctx, expectedId)
		if err == nil {
			t.Errorf("GetBotMaintenanceConfig succeeded with non-existent ID: %s", expectedId)
		}
		if c := status.Code(err); c != codes.NotFound {
			t.Errorf("Unexpected error when calling GetBotMaintenanceConfig: %s", err)
		}
	})
}

func TestListBotMaintenanceConfigs(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)

	t.Run("Get all OwnershipData", func(t *testing.T) {
		maintenanceConfig := mockBotMaintenanceConfig()
		expectedId := "test"
		_, err := PutBotMaintenanceConfig(ctx, maintenanceConfig, expectedId)
		if err != nil {
			t.Fatalf("PutBotMaintenanceConfig failed: %s", err)
		}

		got, _, err := ListBotMaintenanceConfigs(ctx, 10, "", nil, false)
		if err != nil {
			t.Fatalf("ListBotMaintenanceConfigs failed: %s", err)
		}
		if len(got) == 0 {
			t.Errorf("ListBotMaintenanceConfigs returned no results")
		}
		if got[0].Id != expectedId {
			t.Errorf("ListBotMaintenanceConfigs returned unexpected Id:\n%s", got[0].Id)
		}
		p, err := got[0].GetProto()
		if err != nil {
			t.Fatalf("Unmarshalling bot maintenance config from datastore failed: %s", err)
		}

		pm := p.(*ufspb.BotMaintenanceConfig)
		if pm.MaintenanceConfigName != maintenanceConfig.MaintenanceConfigName {
			t.Errorf("ListBotMaintenanceConfigs returned unexpected result for Maintenance config Name:\n%v", pm.MaintenanceConfigName)
		}
		if pm.SwarmingInstance != maintenanceConfig.SwarmingInstance {
			t.Errorf("ListBotMaintenanceConfigs returned unexpected result for swarming instance:\n%v", pm.SwarmingInstance)
		}
	})
}
