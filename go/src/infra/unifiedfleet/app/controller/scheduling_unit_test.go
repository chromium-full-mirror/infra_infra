// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"fmt"
	"testing"

	"google.golang.org/genproto/protobuf/field_mask"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "infra/unifiedfleet/api/v1/models"
	chromeosLab "infra/unifiedfleet/api/v1/models/chromeos/lab"
	. "infra/unifiedfleet/app/model/datastore"
	"infra/unifiedfleet/app/model/history"
	"infra/unifiedfleet/app/model/inventory"
)

func mockSchedulingUnit(name string) *ufspb.SchedulingUnit {
	return &ufspb.SchedulingUnit{
		Name: name,
	}
}

func TestCreateSchedulingUnit(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("CreateSchedulingUnit", t, func(t *ftt.Test) {
		t.Run("Create new SchedulingUnit - happy path", func(t *ftt.Test) {
			su := mockSchedulingUnit("su-1")
			resp, err := CreateSchedulingUnit(ctx, su)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Resemble(su))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "schedulingunits/su-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("schedulingunit"))

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "schedulingunits/su-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Create new SchedulingUnit - already existing", func(t *ftt.Test) {
			su1 := mockSchedulingUnit("su-2")
			inventory.CreateSchedulingUnit(ctx, su1)

			su2 := mockSchedulingUnit("su-2")
			_, err := CreateSchedulingUnit(ctx, su2)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("already exists"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "schedulingunits/su-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "schedulingunits/su-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
		})

		t.Run("Create new SchedulingUnit - DUT non-existing", func(t *ftt.Test) {
			_, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name: "dut-1",
			})
			assert.Loosely(t, err, should.BeNil)

			su2 := mockSchedulingUnit("su-3")
			su2.MachineLSEs = []string{"dut-1", "dut-2"}
			_, err = CreateSchedulingUnit(ctx, su2)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("There is no MachineLSE with MachineLSEID dut-2 in the system."))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "schedulingunits/su-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "schedulingunits/su-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
		})

		t.Run("Create new SchedulingUnit - DUT already associated", func(t *ftt.Test) {
			_, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name: "dut-2",
			})
			assert.Loosely(t, err, should.BeNil)

			su1 := mockSchedulingUnit("su-4")
			su1.MachineLSEs = []string{"dut-2"}
			inventory.CreateSchedulingUnit(ctx, su1)

			su2 := mockSchedulingUnit("su-5")
			su2.MachineLSEs = []string{"dut-2"}
			_, err = CreateSchedulingUnit(ctx, su2)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("already associated"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "schedulingunits/su-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "schedulingunits/su-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
		})

		t.Run("Create new SchedulingUnit - DUT specified more than once", func(t *ftt.Test) {
			_, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name: "dut-3",
			})
			assert.Loosely(t, err, should.BeNil)

			su1 := mockSchedulingUnit("su-6")
			su1.MachineLSEs = []string{"dut-3", "dut-3"}
			_, err = CreateSchedulingUnit(ctx, su1)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("specified more than once"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "schedulingunits/su-6")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "schedulingunits/su-6")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
		})

		t.Run("Create new SchedulingUnit - DUTs do not share the same hive", func(t *ftt.Test) {
			_, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name: "dut-4",
				Lse: &ufspb.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
						ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
							DeviceLse: &ufspb.ChromeOSDeviceLSE{
								Device: &ufspb.ChromeOSDeviceLSE_Dut{
									Dut: &chromeosLab.DeviceUnderTest{
										Hive: "hive-1",
									},
								},
							},
						},
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name: "dut-5",
				Lse: &ufspb.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
						ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
							DeviceLse: &ufspb.ChromeOSDeviceLSE{
								Device: &ufspb.ChromeOSDeviceLSE_Dut{
									Dut: &chromeosLab.DeviceUnderTest{
										Hive: "hive-2",
									},
								},
							},
						},
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)

			su1 := mockSchedulingUnit("su-7")
			su1.MachineLSEs = []string{"dut-4", "dut-5"}
			_, err = CreateSchedulingUnit(ctx, su1)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("have different hives"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "schedulingunits/su-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "schedulingunits/su-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
		})
	})
}

func TestUpdateSchedulingUnit(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("UpdateSchedulingUnit", t, func(t *ftt.Test) {
		t.Run("Update SchedulingUnit for existing SchedulingUnit - happy path", func(t *ftt.Test) {
			su1 := mockSchedulingUnit("su-1")
			su1.Tags = []string{"Dell"}
			inventory.CreateSchedulingUnit(ctx, su1)

			su2 := mockSchedulingUnit("su-1")
			su2.Tags = []string{"Apple"}
			resp, _ := UpdateSchedulingUnit(ctx, su2, nil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Resemble(su2))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "schedulingunits/su-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("schedulingunit.tags"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Match("[Dell]"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Match("[Apple]"))

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "schedulingunits/su-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Update SchedulingUnit for non-existing SchedulingUnit", func(t *ftt.Test) {
			su := mockSchedulingUnit("su-2")
			resp, err := UpdateSchedulingUnit(ctx, su, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "schedulingunits/su-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("UpdateSchedulingUnit - DUT non-existing", func(t *ftt.Test) {
			_, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name: "dut-3",
			})
			assert.Loosely(t, err, should.BeNil)

			su := mockSchedulingUnit("su-3")
			su.MachineLSEs = []string{"dut-3"}
			_, err = inventory.CreateSchedulingUnit(ctx, su)
			assert.Loosely(t, err, should.BeNil)

			su1 := mockSchedulingUnit("su-3")
			su1.MachineLSEs = []string{"dut-3", "dut-4"}
			_, err = UpdateSchedulingUnit(ctx, su1, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("There is no MachineLSE with MachineLSEID dut-4 in the system."))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "schedulingunits/su-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "schedulingunits/su-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
		})

		t.Run("UpdateSchedulingUnit - DUT already associated", func(t *ftt.Test) {
			_, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name: "dut-4",
			})
			assert.Loosely(t, err, should.BeNil)

			su1 := mockSchedulingUnit("su-4")
			su1.MachineLSEs = []string{"dut-4"}
			inventory.CreateSchedulingUnit(ctx, su1)

			su2 := mockSchedulingUnit("su-5")
			_, err = inventory.CreateSchedulingUnit(ctx, su2)
			assert.Loosely(t, err, should.BeNil)

			su3 := mockSchedulingUnit("su-5")
			su3.MachineLSEs = []string{"dut-4"}
			_, err = UpdateSchedulingUnit(ctx, su3, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("already associated"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "schedulingunits/su-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "schedulingunits/su-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
		})

		t.Run("UpdateSchedulingUnit - DUT specified more than once", func(t *ftt.Test) {
			_, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name: "dut-5",
			})
			assert.Loosely(t, err, should.BeNil)

			su1 := mockSchedulingUnit("su-8")
			_, err = inventory.CreateSchedulingUnit(ctx, su1)
			assert.Loosely(t, err, should.BeNil)

			su2 := mockSchedulingUnit("su-8")
			su2.MachineLSEs = []string{"dut-5", "dut-5"}
			_, err = UpdateSchedulingUnit(ctx, su2, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("specified more than once"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "schedulingunits/su-8")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "schedulingunits/su-8")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
		})

		t.Run("UpdateSchedulingUnit - DUTs do not share the same hive", func(t *ftt.Test) {
			_, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name: "dut-7",
				Lse: &ufspb.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
						ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
							DeviceLse: &ufspb.ChromeOSDeviceLSE{
								Device: &ufspb.ChromeOSDeviceLSE_Dut{
									Dut: &chromeosLab.DeviceUnderTest{
										Hive: "hive-1",
									},
								},
							},
						},
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name: "dut-8",
				Lse: &ufspb.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
						ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
							DeviceLse: &ufspb.ChromeOSDeviceLSE{
								Device: &ufspb.ChromeOSDeviceLSE_Dut{
									Dut: &chromeosLab.DeviceUnderTest{
										Hive: "hive-2",
									},
								},
							},
						},
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)

			su1 := mockSchedulingUnit("su-9")
			su1.MachineLSEs = []string{"dut-7", "dut-8"}
			_, err = CreateSchedulingUnit(ctx, su1)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("have different hives"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "schedulingunits/su-9")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "schedulingunits/su-9")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
		})

		t.Run("Update SchedulingUnit for existing SchedulingUnit - partial update(append) machinelses", func(t *ftt.Test) {
			_, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name: "dut-1",
			})
			assert.Loosely(t, err, should.BeNil)

			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name: "dut-2",
			})
			assert.Loosely(t, err, should.BeNil)

			su1 := mockSchedulingUnit("su-7")
			su1.MachineLSEs = []string{"dut-1"}
			inventory.CreateSchedulingUnit(ctx, su1)

			su2 := mockSchedulingUnit("su-7")
			su2.MachineLSEs = []string{"dut-2"}
			resp, _ := UpdateSchedulingUnit(ctx, su2, &field_mask.FieldMask{Paths: []string{"machinelses"}})
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetName(), should.Equal(su2.GetName()))
			assert.Loosely(t, resp.GetMachineLSEs(), should.Resemble([]string{"dut-1", "dut-2"}))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "schedulingunits/su-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("schedulingunit.machinelses"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("[dut-1]"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("[dut-1 dut-2]"))

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "schedulingunits/su-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Update SchedulingUnit for existing SchedulingUnit - partial update(remove) machinelses", func(t *ftt.Test) {
			_, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name: "dut-6",
			})
			assert.Loosely(t, err, should.BeNil)

			su1 := mockSchedulingUnit("su-6")
			su1.MachineLSEs = []string{"dut-6"}
			inventory.CreateSchedulingUnit(ctx, su1)

			su2 := mockSchedulingUnit("su-6")
			su2.MachineLSEs = []string{"dut-6"}
			resp, err := UpdateSchedulingUnit(ctx, su2, &field_mask.FieldMask{Paths: []string{"machinelses.remove"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetName(), should.Equal(su2.GetName()))
			assert.Loosely(t, resp.GetMachineLSEs(), should.Resemble([]string{}))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "schedulingunits/su-6")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("schedulingunit.machinelses"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("[dut-6]"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("[]"))

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "schedulingunits/su-6")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})
	})
}

func TestGetSchedulingUnit(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	su, _ := inventory.CreateSchedulingUnit(ctx, &ufspb.SchedulingUnit{
		Name: "su-1",
	})
	ftt.Run("GetSchedulingUnit", t, func(t *ftt.Test) {
		t.Run("Get SchedulingUnit by existing ID - happy path", func(t *ftt.Test) {
			resp, _ := GetSchedulingUnit(ctx, "su-1")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Resemble(su))
		})

		t.Run("Get SchedulingUnit by non-existing ID", func(t *ftt.Test) {
			_, err := GetSchedulingUnit(ctx, "su-2")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
	})
}

func TestDeleteSchedulingUnit(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	inventory.CreateSchedulingUnit(ctx, &ufspb.SchedulingUnit{
		Name: "su-1",
	})
	ftt.Run("DeleteSchedulingUnit", t, func(t *ftt.Test) {
		t.Run("Delete SchedulingUnit by existing ID - happy path", func(t *ftt.Test) {
			err := DeleteSchedulingUnit(ctx, "su-1")
			assert.Loosely(t, err, should.BeNil)

			res, err := inventory.GetSchedulingUnit(ctx, "su-1")
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "schedulingunits/su-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("schedulingunit"))

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "schedulingunits/su-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeTrue)
		})

		t.Run("Delete SchedulingUnit by non-existing ID", func(t *ftt.Test) {
			err := DeleteSchedulingUnit(ctx, "su-2")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
	})
}

func TestListSchedulingUnits(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	schedulingUnitsWithPools := make([]*ufspb.SchedulingUnit, 0, 2)
	schedulingUnits := make([]*ufspb.SchedulingUnit, 0, 4)
	for i := 0; i < 4; i++ {
		su := mockSchedulingUnit(fmt.Sprintf("su-%d", i))
		if i%2 == 0 {
			su.Pools = []string{"DUT_QUOTA"}
		}
		resp, _ := inventory.CreateSchedulingUnit(ctx, su)
		if i%2 == 0 {
			schedulingUnitsWithPools = append(schedulingUnitsWithPools, resp)
		}
		schedulingUnits = append(schedulingUnits, resp)
	}
	ftt.Run("ListSchedulingUnits", t, func(t *ftt.Test) {
		t.Run("List SchedulingUnits - filter invalid - error", func(t *ftt.Test) {
			_, _, err := ListSchedulingUnits(ctx, 5, "", "invalid=mx-1", false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Invalid field name invalid"))
		})

		t.Run("List SchedulingUnits - filter switch - happy path", func(t *ftt.Test) {
			resp, _, _ := ListSchedulingUnits(ctx, 5, "", "pools=DUT_QUOTA", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Resemble(schedulingUnitsWithPools))
		})

		t.Run("ListSchedulingUnits - Full listing - happy path", func(t *ftt.Test) {
			resp, _, _ := ListSchedulingUnits(ctx, 5, "", "", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Resemble(schedulingUnits))
		})
	})
}
