// Copyright 2022 The Chromium Authors
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

// Tests the functionality for Creating/Updating Ownership data per bot in the datastore
func TestPutOwnershipData(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)

	t.Run("add non-existent Bot ownership", func(t *testing.T) {
		ownershipData := &ufspb.OwnershipData{
			PoolName:         "pool1",
			SwarmingInstance: "test-swarming",
			SecurityLevel:    "untrusted",
			Customer:         "browser",
		}
		expectedName := "test"
		assetType := "machine"
		got, err := PutOwnershipData(ctx, ownershipData, expectedName, assetType)
		if err != nil {
			t.Fatalf("PutOwnershipData failed: %s", err)
		}
		p, err := got.GetProto()
		if err != nil {
			t.Fatalf("Unmarshalling ownership data from datatstore failed: %s", err)
		}
		pm := p.(*ufspb.OwnershipData)
		if got.Name != expectedName {
			t.Errorf("PutOwnershipData returned unexpected name:\n%s", got.Name)
		}
		if pm.PoolName != ownershipData.PoolName {
			t.Errorf("PutOwnershipData returned unexpected result for Pool Name:\n%v", pm.PoolName)
		}
		if pm.Customer != ownershipData.Customer {
			t.Errorf("PutOwnershipData returned unexpected result for Customer:\n%v", pm.Customer)
		}
		if pm.SecurityLevel != ownershipData.SecurityLevel {
			t.Errorf("PutOwnershipData returned unexpected result for Pool Name:\n%v", pm.SecurityLevel)
		}
		if pm.SwarmingInstance != ownershipData.SwarmingInstance {
			t.Errorf("PutOwnershipData returned unexpected result for Pool Name:\n%v", pm.SwarmingInstance)
		}
	})

	t.Run("add existing Bot Ownership", func(t *testing.T) {
		ownershipData := &ufspb.OwnershipData{
			PoolName:         "pool1",
			SwarmingInstance: "test-swarming",
			SecurityLevel:    "untrusted",
			Customer:         "browser",
		}
		assetType := "machine"
		expectedName := "test"
		_, err := PutOwnershipData(ctx, ownershipData, expectedName, assetType)
		if err != nil {
			t.Fatalf("PutOwnershipData failed: %s", err)
		}

		updated_ownershipData := &ufspb.OwnershipData{
			PoolName:         "pool2",
			SwarmingInstance: "test-swarming",
			SecurityLevel:    "untrusted",
			Customer:         "flex",
		}
		// Update ownership
		got, err := PutOwnershipData(ctx, updated_ownershipData, expectedName, assetType)
		if err != nil {
			t.Fatalf("PutOwnershipData failed: %s", err)
		}
		p, err := got.GetProto()
		if err != nil {
			t.Fatalf("Unmarshalling ownership data from datatstore failed: %s", err)
		}
		pm := p.(*ufspb.OwnershipData)
		if got.Name != expectedName {
			t.Errorf("PutOwnershipData returned unexpected name:\n%s", got.Name)
		}
		if pm.PoolName != updated_ownershipData.PoolName {
			t.Errorf("PutOwnershipData returned unexpected result for Pool Name:\n%v", pm.PoolName)
		}
		if pm.Customer != updated_ownershipData.Customer {
			t.Errorf("PutOwnershipData returned unexpected result for Customer:\n%v", pm.Customer)
		}
		if pm.SecurityLevel != updated_ownershipData.SecurityLevel {
			t.Errorf("PutOwnershipData returned unexpected result for Pool Name:\n%v", pm.SecurityLevel)
		}
		if pm.SwarmingInstance != updated_ownershipData.SwarmingInstance {
			t.Errorf("PutOwnershipData returned unexpected result for Pool Name:\n%v", pm.SwarmingInstance)
		}
	})

	t.Run("add empty bot name", func(t *testing.T) {
		_, err := PutOwnershipData(ctx, &ufspb.OwnershipData{}, "", "")
		if err == nil {
			t.Errorf("PutOwnershipData succeeded with empty name")
		}
		if c := status.Code(err); c != codes.Internal {
			t.Errorf("Unexpected error when calling PutOwnershipData: %s", err)
		}
	})
}

// Tests the functionality for getting Ownership data per bot from the DataStore
func TestGetOwnershipData(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)

	t.Run("get OwnershipData by existing ID", func(t *testing.T) {
		ownershipData := &ufspb.OwnershipData{
			PoolName:         "pool1",
			SwarmingInstance: "test-swarming",
			SecurityLevel:    "untrusted",
			Customer:         "browser",
		}
		assetType := "machine"
		expectedName := "test"
		_, err := PutOwnershipData(ctx, ownershipData, expectedName, assetType)
		if err != nil {
			t.Fatalf("PutOwnershipData failed: %s", err)
		}

		got, err := GetOwnershipData(ctx, expectedName)
		if err != nil {
			t.Fatalf("GetOwnershipData failed: %s", err)
		}
		if got.Name != expectedName {
			t.Errorf("GetOwnershipData returned unexpected Name:\n%s", got.Name)
		}
		p, err := got.GetProto()
		if err != nil {
			t.Fatalf("Unmarshalling ownership data from datatstore failed: %s", err)
		}
		pm := p.(*ufspb.OwnershipData)

		if pm.PoolName != ownershipData.PoolName {
			t.Errorf("PutOwnershipData returned unexpected result for Pool Name:\n%v", pm.PoolName)
		}
		if pm.Customer != ownershipData.Customer {
			t.Errorf("PutOwnershipData returned unexpected result for Customer:\n%v", pm.Customer)
		}
		if pm.SecurityLevel != ownershipData.SecurityLevel {
			t.Errorf("PutOwnershipData returned unexpected result for Pool Name:\n%v", pm.SecurityLevel)
		}
		if pm.SwarmingInstance != ownershipData.SwarmingInstance {
			t.Errorf("PutOwnershipData returned unexpected result for Pool Name:\n%v", pm.SwarmingInstance)
		}
	})

	t.Run("get OwnershipData by non-existent ID", func(t *testing.T) {
		const expectedName = "test2"
		_, err := GetOwnershipData(ctx, expectedName)
		if err == nil {
			t.Errorf("GetOwnershipData succeeded with non-existent ID: %s", expectedName)
		}
		if c := status.Code(err); c != codes.NotFound {
			t.Errorf("Unexpected error when calling GetOwnershipData: %s", err)
		}
	})
}

func TestGetAllOwnershipData(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)

	t.Run("Get all OwnershipData", func(t *testing.T) {
		ownershipData := &ufspb.OwnershipData{
			PoolName:         "pool1",
			SwarmingInstance: "test-swarming",
			SecurityLevel:    "untrusted",
			Customer:         "browser",
		}
		assetType := "machine"
		expectedName := "test"
		_, err := PutOwnershipData(ctx, ownershipData, expectedName, assetType)
		if err != nil {
			t.Fatalf("PutOwnershipData failed: %s", err)
		}

		got, _, err := ListOwnerships(ctx, 10, "", nil, false)
		if err != nil {
			t.Fatalf("ListOwnerships failed: %s", err)
		}
		if len(got) == 0 {
			t.Errorf("ListOwnerships returned no results")
		}
		if got[0].Name != expectedName {
			t.Errorf("GetOwnershipData returned unexpected Name:\n%s", got[0].Name)
		}
		p, err := got[0].GetProto()
		if err != nil {
			t.Fatalf("Unmarshalling ownership data from datatstore failed: %s", err)
		}
		pm := p.(*ufspb.OwnershipData)

		if pm.PoolName != ownershipData.PoolName {
			t.Errorf("PutOwnershipData returned unexpected result for Pool Name:\n%v", pm.PoolName)
		}
		if pm.Customer != ownershipData.Customer {
			t.Errorf("PutOwnershipData returned unexpected result for Customer:\n%v", pm.Customer)
		}
		if pm.SecurityLevel != ownershipData.SecurityLevel {
			t.Errorf("PutOwnershipData returned unexpected result for Pool Name:\n%v", pm.SecurityLevel)
		}
		if pm.SwarmingInstance != ownershipData.SwarmingInstance {
			t.Errorf("PutOwnershipData returned unexpected result for Pool Name:\n%v", pm.SwarmingInstance)
		}
	})
}
