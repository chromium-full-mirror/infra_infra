// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package registration

import (
	"fmt"
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "infra/unifiedfleet/api/v1/models"
	"infra/unifiedfleet/app/config"
	. "infra/unifiedfleet/app/model/datastore"
	ufsutil "infra/unifiedfleet/app/util"
)

func mockChromeOSMachine(id, lab, board string, zone ufspb.Zone) *ufspb.Machine {
	return &ufspb.Machine{
		Name: id,
		Device: &ufspb.Machine_ChromeosMachine{
			ChromeosMachine: &ufspb.ChromeOSMachine{
				ReferenceBoard: board,
			},
		},
		Location: &ufspb.Location{
			Zone: zone,
		},
		Realm: ufsutil.ToUFSRealm(zone.String()),
	}
}

func mockChromeBrowserMachine(id, lab, name string, zone ufspb.Zone) *ufspb.Machine {
	return &ufspb.Machine{
		Name: id,
		Device: &ufspb.Machine_ChromeBrowserMachine{
			ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
				Description: name,
			},
		},
		Location: &ufspb.Location{
			Zone: zone,
		},
	}
}

func mockChromeBrowserMachineWithOwnership(id, lab, name string, ownership *ufspb.OwnershipData) *ufspb.Machine {
	machine := mockChromeBrowserMachine(id, lab, name, ufspb.Zone_ZONE_SFO36_BROWSER)
	machine.Ownership = ownership
	return machine
}

func mockAttachedDevice(id, lab, buildTarget string) *ufspb.Machine {
	return &ufspb.Machine{
		Name: id,
		Device: &ufspb.Machine_AttachedDevice{
			AttachedDevice: &ufspb.AttachedDevice{
				BuildTarget: buildTarget,
			},
		},
	}
}

func assertMachineEqual(t *ftt.Test, a *ufspb.Machine, b *ufspb.Machine) {
	assert.Loosely(t, a.GetName(), should.Equal(b.GetName()))
	assert.Loosely(t, a.GetChromeBrowserMachine().GetDescription(), should.Equal(
		b.GetChromeBrowserMachine().GetDescription()))
	assert.Loosely(t, a.GetChromeosMachine().GetReferenceBoard(), should.Equal(
		b.GetChromeosMachine().GetReferenceBoard()))
	assert.Loosely(t, a.GetAttachedDevice().GetBuildTarget(), should.Equal(
		b.GetAttachedDevice().GetBuildTarget()))
}

func assertMachineWithOwnershipEqual(t *ftt.Test, a *ufspb.Machine, b *ufspb.Machine) {
	if a.GetOwnership() == nil && b.GetOwnership() == nil {
		return
	}
	assertMachineEqual(t, a, b)
	assert.Loosely(t, a.GetOwnership().PoolName, should.Equal(b.GetOwnership().PoolName))
	assert.Loosely(t, a.GetOwnership().SwarmingInstance, should.Equal(b.GetOwnership().SwarmingInstance))
	assert.Loosely(t, a.GetOwnership().Customer, should.Equal(b.GetOwnership().Customer))
	assert.Loosely(t, a.GetOwnership().SecurityLevel, should.Equal(b.GetOwnership().SecurityLevel))
}

func getMachineNames(machines []*ufspb.Machine) []string {
	names := make([]string, len(machines))
	for i, p := range machines {
		names[i] = p.GetName()
	}
	return names
}

func TestCreateMachine(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	chromeOSMachine1 := mockChromeOSMachine("chromeos-asset-1", "chromeoslab", "samus", ufspb.Zone_ZONE_CHROMEOS4)
	chromeOSMachine2 := mockChromeOSMachine("", "chromeoslab", "samus", ufspb.Zone_ZONE_CHROMEOS6)
	attchedDevice1 := mockAttachedDevice("attached-device-1", "chromeoslab", "goldfish")
	chromeBrowserMachine1 := mockChromeBrowserMachine("chrome-asset-1", "chromelab", "machine-1", ufspb.Zone_ZONE_SFO36_BROWSER)

	ownershipData := &ufspb.OwnershipData{
		PoolName:         "pool1",
		SwarmingInstance: "test-swarming",
		Customer:         "test-customer",
		SecurityLevel:    "test-security-level",
	}
	chromeBrowserMachineWithOwnership := mockChromeBrowserMachineWithOwnership("chrome-asset-1", "chromelab", "machine-1", ownershipData)
	ftt.Run("CreateMachine", t, func(t *ftt.Test) {
		t.Run("Create new os machine", func(t *ftt.Test) {
			resp, err := CreateMachine(ctx, chromeOSMachine1)
			assert.Loosely(t, err, should.BeNil)
			assertMachineEqual(t, resp, chromeOSMachine1)
		})
		t.Run("Create new attached device", func(t *ftt.Test) {
			resp, err := CreateMachine(ctx, attchedDevice1)
			assert.Loosely(t, err, should.BeNil)
			assertMachineEqual(t, resp, attchedDevice1)
		})
		t.Run("Create existing machine", func(t *ftt.Test) {
			resp, err := CreateMachine(ctx, chromeOSMachine1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(AlreadyExists))
		})
		t.Run("Create machine - invalid ID", func(t *ftt.Test) {
			resp, err := CreateMachine(ctx, chromeOSMachine2)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
		t.Run("Create new browser machine with ownership data - ownership is not saved", func(t *ftt.Test) {
			resp, err := CreateMachine(ctx, chromeBrowserMachineWithOwnership)
			assert.Loosely(t, err, should.BeNil)
			assertMachineWithOwnershipEqual(t, resp, chromeBrowserMachine1)
		})
	})
}

func TestUpdateMachine(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	chromeOSMachine1 := mockChromeOSMachine("chromeos-asset-1", "chromeoslab", "samus", ufspb.Zone_ZONE_CHROMEOS4)
	chromeOSMachine2 := mockChromeOSMachine("chromeos-asset-1", "chromeoslab", "veyron", ufspb.Zone_ZONE_CHROMEOS6)
	chromeBrowserMachine1 := mockChromeBrowserMachine("chrome-asset-1", "chromelab", "machine-1", ufspb.Zone_ZONE_SFO36_BROWSER)

	ownershipData := &ufspb.OwnershipData{
		PoolName:         "pool1",
		SwarmingInstance: "test-swarming",
		Customer:         "test-customer",
		SecurityLevel:    "test-security-level",
	}
	chromeBrowserMachineWithOwnership := mockChromeBrowserMachineWithOwnership("chrome-asset-1", "chromelab", "machine-1", ownershipData)
	chromeOSMachine3 := mockChromeOSMachine("", "chromeoslab", "samus", ufspb.Zone_ZONE_CHROMEOS4)
	ftt.Run("UpdateMachine", t, func(t *ftt.Test) {
		t.Run("Update existing machine", func(t *ftt.Test) {
			resp, err := CreateMachine(ctx, chromeOSMachine1)
			assert.Loosely(t, err, should.BeNil)
			assertMachineEqual(t, resp, chromeOSMachine1)

			resp, err = UpdateMachine(ctx, chromeOSMachine2)
			assert.Loosely(t, err, should.BeNil)
			assertMachineEqual(t, resp, chromeOSMachine2)
		})
		t.Run("Update non-existing machine", func(t *ftt.Test) {
			resp, err := UpdateMachine(ctx, chromeBrowserMachine1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Update machine - invalid ID", func(t *ftt.Test) {
			resp, err := UpdateMachine(ctx, chromeOSMachine3)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
		t.Run("Update existing machine - does not update ownership", func(t *ftt.Test) {
			resp, err := CreateMachine(ctx, chromeBrowserMachine1)
			assert.Loosely(t, err, should.BeNil)
			assertMachineEqual(t, resp, chromeBrowserMachine1)

			resp, err = UpdateMachine(ctx, chromeBrowserMachineWithOwnership)
			assert.Loosely(t, err, should.BeNil)
			assertMachineWithOwnershipEqual(t, resp, chromeBrowserMachine1)
			assert.Loosely(t, resp.GetOwnership(), should.BeNil)
		})
	})
}

func TestUpdateMachineOwnership(t *testing.T) {
	// Tests the ownership update scenarios for a machine
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
	chromeBrowserMachine1 := mockChromeBrowserMachineWithOwnership("chrome-asset-1", "chromelab", "machine-1", ownershipData)
	chromeBrowserMachine1copy := mockChromeBrowserMachineWithOwnership("chrome-asset-1", "chromelab", "machine-1", ownershipData)
	chromeBrowserMachine2 := mockChromeBrowserMachineWithOwnership("chrome-asset-1", "chromelab", "machine-2", ownershipData2)
	chromeBrowserMachine2_oldOwnership := mockChromeBrowserMachineWithOwnership("chrome-asset-1", "chromelab", "machine-2", ownershipData)

	ftt.Run("UpdateMachine", t, func(t *ftt.Test) {
		t.Run("Update existing machine with ownership data", func(t *ftt.Test) {
			resp, err := CreateMachine(ctx, chromeBrowserMachine1)
			assert.Loosely(t, err, should.BeNil)
			assertMachineEqual(t, resp, chromeBrowserMachine1)

			// Ownership data should be updated
			resp, err = UpdateMachineOwnership(ctx, resp.Name, ownershipData)
			assert.Loosely(t, err, should.BeNil)
			assertMachineWithOwnershipEqual(t, resp, chromeBrowserMachine1copy)

			// Regular Update calls should not override ownership data
			resp, err = UpdateMachine(ctx, chromeBrowserMachine2)
			assert.Loosely(t, err, should.BeNil)
			assertMachineEqual(t, resp, chromeBrowserMachine2)

			resp, err = GetMachine(ctx, "chrome-asset-1")
			assert.Loosely(t, err, should.BeNil)
			assertMachineWithOwnershipEqual(t, resp, chromeBrowserMachine2_oldOwnership)
		})
		t.Run("Update non-existing machine with ownership", func(t *ftt.Test) {
			resp, err := UpdateMachineOwnership(ctx, "dummy", ownershipData)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Update machine with ownership - invalid ID", func(t *ftt.Test) {
			resp, err := UpdateMachineOwnership(ctx, "", ownershipData)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestGetMachine(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	chromeOSMachine1 := mockChromeOSMachine("chromeos-asset-3", "chromeoslab", "samus", ufspb.Zone_ZONE_CHROMEOS4)

	ownershipData := &ufspb.OwnershipData{
		PoolName:         "pool1",
		SwarmingInstance: "test-swarming",
		Customer:         "test-customer",
		SecurityLevel:    "test-security-level",
	}
	chromeBrowserMachine1 := mockChromeBrowserMachineWithOwnership("chrome-asset-1", "chromelab", "machine-1", ownershipData)
	chromeBrowserMachinecopy := mockChromeBrowserMachineWithOwnership("chrome-asset-1", "chromelab", "machine-1", ownershipData)
	ftt.Run("GetMachine", t, func(t *ftt.Test) {
		t.Run("Get machine by existing ID", func(t *ftt.Test) {
			resp, err := CreateMachine(ctx, chromeOSMachine1)
			assert.Loosely(t, err, should.BeNil)
			assertMachineEqual(t, resp, chromeOSMachine1)
			resp, err = GetMachine(ctx, "chromeos-asset-3")
			assert.Loosely(t, err, should.BeNil)
			assertMachineEqual(t, resp, chromeOSMachine1)
		})
		t.Run("Get machine by non-existing ID", func(t *ftt.Test) {
			resp, err := GetMachine(ctx, "chrome-asset-1")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get machine - invalid ID", func(t *ftt.Test) {
			resp, err := GetMachine(ctx, "")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
		t.Run("Get machine with ownership by existing ID", func(t *ftt.Test) {
			resp, err := CreateMachine(ctx, chromeBrowserMachine1)
			assert.Loosely(t, err, should.BeNil)
			assertMachineEqual(t, resp, chromeBrowserMachine1)
			assert.Loosely(t, resp.GetOwnership(), should.BeNil)

			// Ownership data should be updated
			resp, err = UpdateMachineOwnership(ctx, resp.Name, ownershipData)
			assert.Loosely(t, err, should.BeNil)
			assertMachineWithOwnershipEqual(t, resp, chromeBrowserMachinecopy)

			resp, err = GetMachine(ctx, "chrome-asset-1")
			assert.Loosely(t, err, should.BeNil)
			assertMachineWithOwnershipEqual(t, resp, chromeBrowserMachinecopy)
		})
	})
}

// Tests GetMachineACL, primarily focused on realm use cases
func TestGetMachineACL(t *testing.T) {
	t.Parallel()

	// manually turn on config
	alwaysUseACLConfig := config.Config{
		ExperimentalAPI: &config.ExperimentalAPI{
			GetMachineACL: 99,
		},
	}

	ctx := gaetesting.TestingContextWithAppID("go-test")
	ctx = config.Use(ctx, &alwaysUseACLConfig)
	chromeOSMachineZone4, err := CreateMachine(
		ctx,
		mockChromeOSMachine("chromeos-asset-zone4", "chromeoslab", "samus", ufspb.Zone_ZONE_CHROMEOS4),
	)
	if err != nil {
		t.Errorf("failed to create machine data: %s", err)
	}

	chromeOSMachineZone5, err := CreateMachine(
		ctx,
		mockChromeOSMachine("chromeos-asset-zone5", "chromeoslab", "samus", ufspb.Zone_ZONE_CHROMEOS5),
	)
	if err != nil {
		t.Errorf("failed to create machine data: %s", err)
	}

	// superuser has permissions in two realms.
	ctxSuperuser := mockUser(ctx, "root@lab.com")
	mockRealmPerms(ctxSuperuser, ufsutil.AtlLabAdminRealm, ufsutil.RegistrationsGet)
	mockRealmPerms(ctxSuperuser, ufsutil.AcsLabAdminRealm, ufsutil.RegistrationsGet)

	// permission in one realm.
	ctxACSLab := mockUser(ctx, "acs@lab.com")
	mockRealmPerms(ctxACSLab, ufsutil.AcsLabAdminRealm, ufsutil.RegistrationsGet)

	// permission in no realms.
	ctxNoPerms := mockUser(ctx, "bad@lab.com")

	ftt.Run("GetMachine", t, func(t *ftt.Test) {
		t.Run("User with correct perms sees both", func(t *ftt.Test) {
			resp, err := GetMachineACL(ctxSuperuser, "chromeos-asset-zone4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(chromeOSMachineZone4))
			resp, err = GetMachineACL(ctxSuperuser, "chromeos-asset-zone5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(chromeOSMachineZone5))
		})
		t.Run("User only sees realm they should access", func(t *ftt.Test) {
			resp, err := GetMachineACL(ctxACSLab, "chromeos-asset-zone4")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			resp, err = GetMachineACL(ctxACSLab, "chromeos-asset-zone5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(chromeOSMachineZone5))
		})
		t.Run("User with no realms sees nothing", func(t *ftt.Test) {
			resp, err := GetMachineACL(ctxNoPerms, "chromeos-asset-zone4")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			resp, err = GetMachineACL(ctxNoPerms, "chromeos-asset-zone5")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
	})
}

// Tests GetMachineACL, primarily focused on realm use cases
func TestBatchGetMachineACL(t *testing.T) {
	t.Parallel()

	// manually turn on config
	alwaysUseACLConfig := config.Config{
		ExperimentalAPI: &config.ExperimentalAPI{
			GetMachineACL: 99,
		},
	}

	ctx := gaetesting.TestingContextWithAppID("go-test")
	ctx = config.Use(ctx, &alwaysUseACLConfig)
	chromeOSMachineZone4, err := CreateMachine(
		ctx,
		mockChromeOSMachine("chromeos-asset-zone4", "chromeoslab", "samus", ufspb.Zone_ZONE_CHROMEOS4),
	)
	if err != nil {
		t.Errorf("failed to create machine data: %s", err)
	}

	chromeOSMachineZone5, err := CreateMachine(
		ctx,
		mockChromeOSMachine("chromeos-asset-zone5", "chromeoslab", "samus", ufspb.Zone_ZONE_CHROMEOS5),
	)
	if err != nil {
		t.Errorf("failed to create machine data: %s", err)
	}

	// superuser has permissions in two realms.
	ctxSuperuser := mockUser(ctx, "root@lab.com")
	mockRealmPerms(ctxSuperuser, ufsutil.AtlLabAdminRealm, ufsutil.RegistrationsGet)
	mockRealmPerms(ctxSuperuser, ufsutil.AcsLabAdminRealm, ufsutil.RegistrationsGet)

	// permission in one realm.
	ctxACSLab := mockUser(ctx, "acs@lab.com")
	mockRealmPerms(ctxACSLab, ufsutil.AcsLabAdminRealm, ufsutil.RegistrationsGet)

	// permission in no realms.
	ctxNoPerms := mockUser(ctx, "bad@lab.com")

	ftt.Run("GetMachine", t, func(t *ftt.Test) {
		t.Run("User with correct perms sees both", func(t *ftt.Test) {
			resp, err := BatchGetMachinesACL(ctxSuperuser, []string{"chromeos-asset-zone4", "chromeos-asset-zone5"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match([]*ufspb.Machine{chromeOSMachineZone4, chromeOSMachineZone5}))
		})
		t.Run("User only sees realm they should access", func(t *ftt.Test) {
			resp, err := BatchGetMachinesACL(ctxACSLab, []string{"chromeos-asset-zone4", "chromeos-asset-zone5"})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			resp, err = BatchGetMachinesACL(ctxACSLab, []string{"chromeos-asset-zone5"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match([]*ufspb.Machine{chromeOSMachineZone5}))
		})
		t.Run("User with no realms sees nothing", func(t *ftt.Test) {
			resp, err := BatchGetMachinesACL(ctxNoPerms, []string{"chromeos-asset-zone4", "chromeos-asset-zone5"})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
	})
}

func TestListMachines(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	machines := make([]*ufspb.Machine, 0, 4)
	for i := 0; i < 4; i++ {
		chromeOSMachine1 := mockChromeOSMachine(fmt.Sprintf("chromeos-%d", i), "chromeoslab", "samus", ufspb.Zone_ZONE_CHROMEOS4)
		resp, _ := CreateMachine(ctx, chromeOSMachine1)
		machines = append(machines, resp)
	}
	ftt.Run("ListMachines", t, func(t *ftt.Test) {
		t.Run("List machines - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachines(ctx, 5, "abc", nil, false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InvalidPageToken))
		})

		t.Run("List machines - Full listing with no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachines(ctx, 4, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machines))
		})

		t.Run("List machines - listing with pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachines(ctx, 3, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machines[:3]))

			resp, _, err = ListMachines(ctx, 2, nextPageToken, nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machines[3:]))
		})
	})
}

func TestListMachinesACL(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	machines := make([]*ufspb.Machine, 0, 20)
	for i := 0; i < 10; i++ {
		chromeOSMachine := mockChromeOSMachine(fmt.Sprintf("chromeos-0%d", i), "chromeoslab", "samus", ufspb.Zone_ZONE_CHROMEOS5)
		resp, _ := CreateMachine(ctx, chromeOSMachine)
		machines = append(machines, resp)
	}
	for i := 0; i < 10; i++ {
		chromeOSMachine := mockChromeOSMachine(fmt.Sprintf("chromeos-1%d", i), "chromeoslab", "samus", ufspb.Zone_ZONE_CHROMEOS4)
		resp, _ := CreateMachine(ctx, chromeOSMachine)
		machines = append(machines, resp)
	}

	// superuser has permissions in two realms.
	ctxSuperuser := mockUser(ctx, "root@lab.com")
	mockRealmPerms(ctxSuperuser, ufsutil.AtlLabAdminRealm, ufsutil.RegistrationsList)
	mockRealmPerms(ctxSuperuser, ufsutil.AcsLabAdminRealm, ufsutil.RegistrationsList)

	// permission in one realm.
	ctxACSLab := mockUser(ctx, "acs@lab.com")
	mockRealmPerms(ctxACSLab, ufsutil.AcsLabAdminRealm, ufsutil.RegistrationsList)

	// permission in no realms.
	ctxNoPerms := mockUser(ctx, "bad@lab.com")

	ftt.Run("ListMachinesACL", t, func(t *ftt.Test) {
		t.Run("List machines - anonymous", func(t *ftt.Test) {
			// User anonymous sees nothing
			resp, nextPageToken, err := ListMachinesACL(ctx, 100, "", nil, false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
		})
		t.Run("List machines - reject realm filter", func(t *ftt.Test) {
			// Can't filter on realm
			resp, nextPageToken, err := ListMachinesACL(ctxSuperuser, 100, "", map[string][]interface{}{"realm": {"woah..."}}, false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
		})
		t.Run("List machines - happy path, no perms", func(t *ftt.Test) {
			// Can't filter on realm
			resp, nextPageToken, err := ListMachinesACL(ctxNoPerms, 100, "", nil, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)

		})
		t.Run("List machines - happy path, one realm", func(t *ftt.Test) {
			// test pagination
			resp, nextPageToken, err := ListMachinesACL(ctxACSLab, 3, "", nil, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machines[0:3]))
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)

			resp, nextPageToken, err = ListMachinesACL(ctxACSLab, 100, nextPageToken, nil, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machines[3:10]))
			assert.Loosely(t, nextPageToken, should.BeEmpty)

		})
		t.Run("List machines - happy path, all realms", func(t *ftt.Test) {
			// test pagination
			resp, nextPageToken, err := ListMachinesACL(ctxSuperuser, 3, "", nil, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machines[0:3]))
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)

			resp, nextPageToken, err = ListMachinesACL(ctxSuperuser, 100, nextPageToken, nil, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machines[3:20]))
			assert.Loosely(t, nextPageToken, should.BeEmpty)

		})
		t.Run("List machines - happy path, two realms, filter", func(t *ftt.Test) {
			// test pagination
			resp, nextPageToken, err := ListMachinesACL(ctxSuperuser, 3, "", map[string][]interface{}{"zone": {"ZONE_CHROMEOS5"}}, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machines[0:3]))
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)

			resp, nextPageToken, err = ListMachinesACL(ctxSuperuser, 100, nextPageToken, map[string][]interface{}{"zone": {"ZONE_CHROMEOS5"}}, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machines[3:10]))
			assert.Loosely(t, nextPageToken, should.BeEmpty)
		})
		t.Run("List machines - happy path, filter out all machines", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachinesACL(ctxSuperuser, 3, "", map[string][]interface{}{"zone": {"ZONE_CHROMEOS3"}}, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
		})
	})
}

// TestListMachinesByIdPrefixSearch tests the functionality for listing
// machines by seraching for name/id prefix
func TestListMachinesByIdPrefixSearch(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	machines := make([]*ufspb.Machine, 0, 4)
	for i := 0; i < 4; i++ {
		chromeOSMachine1 := mockChromeOSMachine(fmt.Sprintf("chromeos-%d", i), "chromeoslab", "samus", ufspb.Zone_ZONE_CHROMEOS4)
		resp, _ := CreateMachine(ctx, chromeOSMachine1)
		machines = append(machines, resp)
	}
	ftt.Run("ListMachinesByIdPrefixSearch", t, func(t *ftt.Test) {
		t.Run("List machines - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachinesByIdPrefixSearch(ctx, 5, "abc", "chromeos-", false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InvalidPageToken))
		})

		t.Run("List machines - Full listing with valid prefix and no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachinesByIdPrefixSearch(ctx, 4, "", "chromeos-", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machines))
		})

		t.Run("List machines - Full listing with invalid prefix", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachinesByIdPrefixSearch(ctx, 4, "", "chromeos1-", false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("List machines - listing with valid prefix and pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachinesByIdPrefixSearch(ctx, 3, "", "chromeos-", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machines[:3]))

			resp, _, err = ListMachinesByIdPrefixSearch(ctx, 2, nextPageToken, "chromeos-", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machines[3:]))
		})
	})
}

func TestDeleteMachine(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	chromeOSMachine2 := mockChromeOSMachine("chromeos-asset-2", "chromeoslab", "samus", ufspb.Zone_ZONE_CHROMEOS4)

	ownershipData := &ufspb.OwnershipData{
		PoolName:         "pool1",
		SwarmingInstance: "test-swarming",
		Customer:         "test-customer",
		SecurityLevel:    "test-security-level",
	}
	chromeBrowserMachine1 := mockChromeBrowserMachineWithOwnership("chrome-asset-3", "chromelab", "machine-1", ownershipData)
	chromeBrowserMachinecopy := mockChromeBrowserMachineWithOwnership("chrome-asset-3", "chromelab", "machine-1", ownershipData)
	ftt.Run("DeleteMachine", t, func(t *ftt.Test) {
		t.Run("Delete machine by existing ID", func(t *ftt.Test) {
			resp, cerr := CreateMachine(ctx, chromeOSMachine2)
			assert.Loosely(t, cerr, should.BeNil)
			assertMachineEqual(t, resp, chromeOSMachine2)
			err := DeleteMachine(ctx, "chromeos-asset-2")
			assert.Loosely(t, err, should.BeNil)
			res, err := GetMachine(ctx, "chromeos-asset-2")
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete machine by non-existing ID", func(t *ftt.Test) {
			err := DeleteMachine(ctx, "chrome-asset-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete machine - invalid ID", func(t *ftt.Test) {
			err := DeleteMachine(ctx, "")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
		t.Run("Delete machine - with ownershipdata", func(t *ftt.Test) {
			resp, cerr := CreateMachine(ctx, chromeBrowserMachine1)
			assert.Loosely(t, cerr, should.BeNil)
			assertMachineWithOwnershipEqual(t, resp, chromeBrowserMachine1)

			// Ownership data should be updated
			resp, err := UpdateMachineOwnership(ctx, resp.Name, ownershipData)
			assert.Loosely(t, err, should.BeNil)
			assertMachineWithOwnershipEqual(t, resp, chromeBrowserMachinecopy)

			err = DeleteMachine(ctx, "chrome-asset-3")
			assert.Loosely(t, err, should.BeNil)
		})
	})
}

func TestBatchUpdateMachines(t *testing.T) {
	t.Parallel()
	ftt.Run("BatchUpdateMachines", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		machines := make([]*ufspb.Machine, 0, 4)
		for i := 0; i < 4; i++ {
			chromeOSMachine1 := mockChromeOSMachine(fmt.Sprintf("chromeos-%d", i), "chromeoslab", "samus", ufspb.Zone_ZONE_CHROMEOS4)
			resp, err := CreateMachine(ctx, chromeOSMachine1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(chromeOSMachine1))
			machines = append(machines, resp)
		}
		t.Run("BatchUpdate all machines", func(t *ftt.Test) {
			resp, err := BatchUpdateMachines(ctx, machines)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machines))
		})
		t.Run("BatchUpdate existing and non-existing machines", func(t *ftt.Test) {
			chromeOSMachine5 := mockChromeOSMachine("", "chromeoslab", "samus", ufspb.Zone_ZONE_CHROMEOS4)
			machines = append(machines, chromeOSMachine5)
			resp, err := BatchUpdateMachines(ctx, machines)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestQueryMachineByPropertyName(t *testing.T) {
	t.Parallel()
	ftt.Run("QueryMachineByPropertyName", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		dummyMachine := &ufspb.Machine{
			Name: "machine-1",
		}
		machine1 := &ufspb.Machine{
			Name: "machine-1",
			Device: &ufspb.Machine_ChromeBrowserMachine{
				ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
					ChromePlatform: "chromePlatform-1",
					KvmInterface: &ufspb.KVMInterface{
						Kvm: "kvm-1",
					},
					RpmInterface: &ufspb.RPMInterface{
						Rpm: "rpm-1",
					},
				},
			},
		}
		resp, cerr := CreateMachine(ctx, machine1)
		assert.Loosely(t, cerr, should.BeNil)
		assert.Loosely(t, resp, should.Match(machine1))

		machines := make([]*ufspb.Machine, 0, 1)
		machines = append(machines, dummyMachine)

		machines1 := make([]*ufspb.Machine, 0, 1)
		machines1 = append(machines1, machine1)
		t.Run("Query By existing ChromePlatform", func(t *ftt.Test) {
			resp, err := QueryMachineByPropertyName(ctx, "chrome_platform_id", "chromePlatform-1", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machines))
		})
		t.Run("Query By non-existing ChromePlatform", func(t *ftt.Test) {
			resp, err := QueryMachineByPropertyName(ctx, "chrome_platform_id", "chromePlatform-2", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Query By existing rpm", func(t *ftt.Test) {
			resp, err := QueryMachineByPropertyName(ctx, "rpm_id", "rpm-1", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machines1))
		})
		t.Run("Query By non-existing rpm", func(t *ftt.Test) {
			resp, err := QueryMachineByPropertyName(ctx, "rpm_id", "rpm-2", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Query By existing kvm", func(t *ftt.Test) {
			resp, err := QueryMachineByPropertyName(ctx, "kvm_id", "kvm-1", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machines))
		})
		t.Run("Query By non-existing kvm", func(t *ftt.Test) {
			resp, err := QueryMachineByPropertyName(ctx, "kvm_id", "kvm-2", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
	})
}

/*
func TestGetAllMachines(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	Convey("GetAllMachines", t, func() {
		Convey("Get empty machines", func() {
			resp, err := GetAllMachines(ctx)
			So(err, ShouldBeNil)
			So(resp.Passed(), ShouldHaveLength, 0)
			So(resp.Failed(), ShouldHaveLength, 0)
		})
		Convey("Get all the machines", func() {
			chromeOSMachine1 := mockChromeOSMachine("chromeos-asset-1", "chromeoslab", "samus")
			chromeMachine1 := mockChromeMachine("chrome-asset-1", "chromelab", "machine-1")
			input := []*fleet.Machine{chromeMachine1, chromeOSMachine1}
			resp, err := CreateMachines(ctx, input)
			So(err, ShouldBeNil)
			So(resp.Passed(), ShouldHaveLength, 2)
			So(resp.Failed(), ShouldHaveLength, 0)
			assertMachineEqual(resp.Passed()[0].Data.(*fleet.Machine), chromeMachine1)
			assertMachineEqual(resp.Passed()[1].Data.(*fleet.Machine), chromeOSMachine1)

			resp, err = GetAllMachines(ctx)
			So(err, ShouldBeNil)
			So(resp.Passed(), ShouldHaveLength, 2)
			So(resp.Failed(), ShouldHaveLength, 0)
			output := []*fleet.Machine{
				resp.Passed()[0].Data.(*fleet.Machine),
				resp.Passed()[1].Data.(*fleet.Machine),
			}
			wants := getMachineNames(input)
			gets := getMachineNames(output)
			So(wants, ShouldResemble, gets)
		})
	})
}
*/
