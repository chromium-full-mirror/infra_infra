// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inventory

import (
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	. "go.chromium.org/infra/unifiedfleet/app/model/datastore"
)

func mockVM(id string) *ufspb.VM {
	return &ufspb.VM{
		Name: id,
	}
}

func mockVMWithOwnership(id string, ownership *ufspb.OwnershipData) *ufspb.VM {
	machine := mockVM(id)
	machine.Ownership = ownership
	return machine
}

func assertVMWithOwnershipEqual(t *ftt.Test, a *ufspb.VM, b *ufspb.VM) {
	if a.GetOwnership() == nil && b.GetOwnership() == nil {
		return
	}
	assert.Loosely(t, a.GetOwnership().PoolName, should.Equal(b.GetOwnership().PoolName))
	assert.Loosely(t, a.GetOwnership().SwarmingInstance, should.Equal(b.GetOwnership().SwarmingInstance))
	assert.Loosely(t, a.GetOwnership().Customer, should.Equal(b.GetOwnership().Customer))
	assert.Loosely(t, a.GetOwnership().SecurityLevel, should.Equal(b.GetOwnership().SecurityLevel))
}

func TestBatchUpdateVMs(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	vm1 := mockVM("vm-1")
	vm2 := mockVM("vm-2")
	vm3 := mockVM("")
	ftt.Run("Batch Update VM", t, func(t *ftt.Test) {
		t.Run("BatchUpdate all vms", func(t *ftt.Test) {
			resp, err := BatchUpdateVMs(ctx, []*ufspb.VM{vm1, vm2})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match([]*ufspb.VM{vm1, vm2}))
		})
		t.Run("BatchUpdate existing vms", func(t *ftt.Test) {
			vm2.MacAddress = "123"
			_, err := BatchUpdateVMs(ctx, []*ufspb.VM{vm1, vm2})
			assert.Loosely(t, err, should.BeNil)
			vm, err := GetVM(ctx, "vm-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, vm.GetMacAddress(), should.Equal("123"))
		})
		t.Run("BatchUpdate invalid vms", func(t *ftt.Test) {
			resp, err := BatchUpdateVMs(ctx, []*ufspb.VM{vm1, vm2, vm3})
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestUpdateVMOwnership(t *testing.T) {
	// Tests the ownership update scenarios for a VM
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	ownershipData := &ufspb.OwnershipData{
		PoolName:         "pool1",
		SwarmingInstance: "test-swarming",
		Customer:         "test-customer",
		SecurityLevel:    "test-security-level",
	}
	ownershipData2 := &ufspb.OwnershipData{
		PoolName:         "pool2",
		SwarmingInstance: "test-swarming",
		Customer:         "test-customer",
		SecurityLevel:    "test-security-level",
	}
	vm1 := mockVM("vm-1")
	vm1_ownership := mockVMWithOwnership("vm-1", ownershipData)
	vm2 := mockVMWithOwnership("vm-1", ownershipData2)
	ftt.Run("UpdateVM", t, func(t *ftt.Test) {
		t.Run("Update existing VM with ownership data", func(t *ftt.Test) {
			resp, err := BatchUpdateVMs(ctx, []*ufspb.VM{vm1})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match([]*ufspb.VM{vm1}))

			// Ownership data should be updated
			vmResp, err := UpdateVMOwnership(ctx, resp[0].Name, ownershipData)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, vmResp.GetOwnership(), should.NotBeNil)
			assertVMWithOwnershipEqual(t, vmResp, vm1_ownership)

			// Regular Update calls should not override ownership data
			resp, err = BatchUpdateVMs(ctx, []*ufspb.VM{vm2})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match([]*ufspb.VM{vm2}))

			vmResp, err = GetVM(ctx, "vm-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, vmResp.GetOwnership(), should.NotBeNil)
			assertVMWithOwnershipEqual(t, vmResp, vm1_ownership)
		})
		t.Run("Update non-existing VM with ownership", func(t *ftt.Test) {
			resp, err := UpdateVMOwnership(ctx, "dummy", ownershipData)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Update VM with ownership - invalid ID", func(t *ftt.Test) {
			resp, err := UpdateVMOwnership(ctx, "", ownershipData)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestGetVM(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	ftt.Run("GetVM", t, func(t *ftt.Test) {
		t.Run("Get machineLSE by non-existing ID", func(t *ftt.Test) {
			resp, err := GetMachineLSE(ctx, "empty")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get machineLSE - invalid ID", func(t *ftt.Test) {
			resp, err := GetMachineLSE(ctx, "")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestListVMs(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)

	vm1 := &ufspb.VM{
		Name:          "vm-1",
		ResourceState: ufspb.State_STATE_DECOMMISSIONED,
	}
	vm2 := &ufspb.VM{
		Name: "vm-2",
		Tags: []string{"tag-1", "tag-2"},
	}
	vm3 := &ufspb.VM{
		Name:   "vm-3",
		Memory: 1234,
	}
	vm4 := mockVM("vm-4")
	vms := []*ufspb.VM{vm1, vm2, vm3, vm4}

	ftt.Run("ListVMs", t, func(t *ftt.Test) {
		_, err := BatchUpdateVMs(ctx, vms)
		assert.Loosely(t, err, should.BeNil)
		t.Run("List vms - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListVMs(ctx, 5, 5, "abc", nil, false, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InvalidPageToken))
		})

		t.Run("List vms - Full listing with no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListVMs(ctx, 4, 4, "", nil, false, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, resp, should.Match(vms))
		})

		t.Run("List vms - listing with pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListVMs(ctx, 3, 3, "", nil, false, nil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(vms[:3]))

			resp, _, err = ListVMs(ctx, 2, 2, nextPageToken, nil, false, nil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(vms[3:]))
		})
	})
	ftt.Run("ListVMs with Filters", t, func(t *ftt.Test) {
		_, err := BatchUpdateVMs(ctx, vms)
		assert.Loosely(t, err, should.BeNil)
		filterMap := make(map[string][]interface{})
		t.Run("List vms - Filter by state", func(t *ftt.Test) {
			filterMap["state"] = []interface{}{"STATE_DECOMMISSIONED"}
			resp, nextPageToken, err := ListVMs(ctx, 1, 2, "", filterMap, false, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match([]*ufspb.VM{vm1}))
		})
		t.Run("List vms - Filter by tags", func(t *ftt.Test) {
			filterMap["tags"] = []interface{}{"tag-1"}
			resp, nextPageToken, err := ListVMs(ctx, 1, 2, "", filterMap, false, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match([]*ufspb.VM{vm2}))
		})
		t.Run("List vms - Filter by memory", func(t *ftt.Test) {
			filterMap["memory"] = []interface{}{1234}
			resp, nextPageToken, err := ListVMs(ctx, 1, 2, "", filterMap, false, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match([]*ufspb.VM{vm3}))
		})
	})
}

// TestListVMsByIdPrefixSearch tests the functionality for listing
// VMs by searching for name/id prefix
func TestListVMsByIdPrefixSearch(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	vm1 := &ufspb.VM{
		Name:          "vm-1",
		ResourceState: ufspb.State_STATE_DECOMMISSIONED,
	}
	vm2 := &ufspb.VM{
		Name: "vm-2",
		Tags: []string{"tag-1", "tag-2"},
	}
	vm3 := &ufspb.VM{
		Name:   "vm-3",
		Memory: 1234,
	}
	vm4 := mockVM("vm-4")
	vms := []*ufspb.VM{vm1, vm2, vm3, vm4}
	ftt.Run("ListMachinesByIdPrefixSearch", t, func(t *ftt.Test) {
		_, err := BatchUpdateVMs(ctx, vms)
		assert.Loosely(t, err, should.BeNil)
		t.Run("List vms - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListVMsByIdPrefixSearch(ctx, 5, 2, "abc", "vm-", false, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InvalidPageToken))
		})

		t.Run("List vms - Full listing with valid prefix and no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListVMsByIdPrefixSearch(ctx, 4, 4, "", "vm-", false, nil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(vms))
		})

		t.Run("List vms - Full listing with invalid prefix", func(t *ftt.Test) {
			resp, nextPageToken, err := ListVMsByIdPrefixSearch(ctx, 4, 2, "", "vm1-", false, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("List vms - listing with valid prefix and pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListVMsByIdPrefixSearch(ctx, 3, 3, "", "vm-", false, nil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(vms[:3]))

			resp, _, err = ListVMsByIdPrefixSearch(ctx, 2, 2, nextPageToken, "vm-", false, nil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(vms[3:]))
		})
	})
}

func TestDeleteVMs(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	vm1 := mockVM("vm-delete1")
	ownershipData := &ufspb.OwnershipData{
		PoolName:         "pool1",
		SwarmingInstance: "test-swarming",
		Customer:         "test-customer",
		SecurityLevel:    "test-security-level",
	}
	vm1_ownership := mockVMWithOwnership("vm-delete1", ownershipData)
	ftt.Run("DeleteVMs", t, func(t *ftt.Test) {
		t.Run("Delete VM by existing ID", func(t *ftt.Test) {
			_, err := BatchUpdateVMs(ctx, []*ufspb.VM{vm1})
			assert.Loosely(t, err, should.BeNil)
			DeleteVMs(ctx, []string{"vm-delete1"})
			vm, err := GetVM(ctx, "vm-delete1")
			assert.Loosely(t, vm, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete vms by non-existing ID", func(t *ftt.Test) {
			res := DeleteVMs(ctx, []string{"vm-delete2"})
			assert.Loosely(t, res.Failed(), should.HaveLength(1))
		})
		t.Run("Delete machineLSE - invalid ID", func(t *ftt.Test) {
			res := DeleteVMs(ctx, []string{""})
			assert.Loosely(t, res.Failed(), should.HaveLength(1))
		})
		t.Run("Delete VM - with ownershipdata", func(t *ftt.Test) {
			vmResp, err := BatchUpdateVMs(ctx, []*ufspb.VM{vm1})
			assert.Loosely(t, err, should.BeNil)

			// Ownership data should be updated
			resp, err := UpdateVMOwnership(ctx, vmResp[0].Name, ownershipData)
			assert.Loosely(t, err, should.BeNil)
			assertVMWithOwnershipEqual(t, resp, vm1_ownership)

			DeleteVMs(ctx, []string{"vm-delete1"})
			vm, err := GetVM(ctx, "vm-delete1")
			assert.Loosely(t, vm, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
	})
}

func TestQueryVMByPropertyName(t *testing.T) {
	t.Parallel()
	ftt.Run("QueryVMByPropertyName", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		vm1 := mockVM("vm-queryByProperty1")
		vm1.MacAddress = "00:50:56:17:00:00"
		_, err := BatchUpdateVMs(ctx, []*ufspb.VM{vm1})
		assert.Loosely(t, err, should.BeNil)

		t.Run("Query By existing mac address", func(t *ftt.Test) {
			resp, err := QueryVMByPropertyName(ctx, "mac_address", "00:50:56:17:00:00", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(1))
			assert.Loosely(t, resp[0], should.Match(vm1))
		})
		t.Run("Query By non-existing mac address", func(t *ftt.Test) {
			resp, err := QueryVMByPropertyName(ctx, "mac_address", "00:50:56:xx:yy:zz", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
	})
}
