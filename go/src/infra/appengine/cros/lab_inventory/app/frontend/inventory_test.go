// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	timestamp "github.com/golang/protobuf/ptypes/timestamp"

	"go.chromium.org/chromiumos/infra/proto/go/device"
	"go.chromium.org/chromiumos/infra/proto/go/lab"
	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	ds "go.chromium.org/luci/gae/service/datastore"

	api "infra/appengine/cros/lab_inventory/api/v1"
	"infra/appengine/cros/lab_inventory/app/config"
	"infra/cros/lab_inventory/datastore"
	"infra/cros/lab_inventory/deviceconfig"
	invlibs "infra/cros/lab_inventory/protos"
)

type testFixture struct {
	T testing.TB
	C context.Context

	Inventory          *InventoryServerImpl
	DecoratedInventory *api.DecoratedInventory
}

func newTestFixtureWithContext(ctx context.Context, t testing.TB) (testFixture, func()) {
	tf := testFixture{T: t, C: ctx}
	mc := gomock.NewController(t)

	tf.Inventory = &InventoryServerImpl{}
	tf.DecoratedInventory = &api.DecoratedInventory{
		Service: tf.Inventory,
		Prelude: checkAccess,
	}

	validate := func() {
		mc.Finish()
	}
	return tf, validate
}

func testingContext() context.Context {
	c := gaetesting.TestingContextWithAppID("dev~infra-lab-inventory")
	c = config.Use(c, &config.Config{
		Readers: &config.LuciAuthGroup{
			Value: "fake_group",
		},
	})
	return c
}

type devcfgEntity struct {
	_kind     string `gae:"$kind,DevConfig"`
	ID        string `gae:"$id"`
	DevConfig []byte `gae:",noindex"`
	Updated   time.Time
}

func TestDeviceConfigsExists(t *testing.T) {
	t.Parallel()

	ftt.Run("Test exists device config in datastore", t, func(t *ftt.Test) {
		ctx := testingContext()
		tf, validate := newTestFixtureWithContext(ctx, t)
		defer validate()
		err := ds.Put(ctx, []devcfgEntity{
			{ID: "kunimitsu.lars.variant1"},
			{ID: "sarien.arcada.variant2"},
			{
				ID:        "platform.model.variant3",
				DevConfig: []byte("bad data"),
			},
		})
		assert.Loosely(t, err, should.BeNil)

		t.Run("Happy path", func(t *ftt.Test) {
			resp, err := tf.Inventory.DeviceConfigsExists(ctx, &api.DeviceConfigsExistsRequest{
				ConfigIds: []*device.ConfigId{
					{
						PlatformId: &device.PlatformId{Value: "lars"},
						ModelId:    &device.ModelId{Value: "lars"},
						VariantId:  &device.VariantId{Value: "variant1"},
					},
					{
						PlatformId: &device.PlatformId{Value: "arcada"},
						ModelId:    &device.ModelId{Value: "arcada"},
						VariantId:  &device.VariantId{Value: "variant2"},
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Exists[0], should.BeTrue)
			assert.Loosely(t, resp.Exists[1], should.BeTrue)
		})

		t.Run("check for nonexisting data", func(t *ftt.Test) {
			resp, err := tf.Inventory.DeviceConfigsExists(ctx, &api.DeviceConfigsExistsRequest{
				ConfigIds: []*device.ConfigId{
					{
						PlatformId: &device.PlatformId{Value: "platform"},
						ModelId:    &device.ModelId{Value: "model"},
						VariantId:  &device.VariantId{Value: "variant-nonexisting"},
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Exists[0], should.BeFalse)
		})

		t.Run("check for existing and nonexisting data", func(t *ftt.Test) {
			resp, err := tf.Inventory.DeviceConfigsExists(ctx, &api.DeviceConfigsExistsRequest{
				ConfigIds: []*device.ConfigId{
					{
						PlatformId: &device.PlatformId{Value: "platform"},
						ModelId:    &device.ModelId{Value: "model"},
						VariantId:  &device.VariantId{Value: "variant-nonexisting"},
					},
					{
						PlatformId: &device.PlatformId{Value: "arcada"},
						ModelId:    &device.ModelId{Value: "arcada"},
						VariantId:  &device.VariantId{Value: "variant2"},
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Exists[0], should.BeFalse)
			assert.Loosely(t, resp.Exists[1], should.BeTrue)
		})
	})
}

func mockDeviceManualRepairRecord(hostname string, assetTag string, createdTime int64, completed bool) *invlibs.DeviceManualRepairRecord {
	var state invlibs.DeviceManualRepairRecord_RepairState
	var updatedTime timestamp.Timestamp
	var completedTime timestamp.Timestamp
	if completed {
		state = invlibs.DeviceManualRepairRecord_STATE_COMPLETED
		updatedTime = timestamp.Timestamp{Seconds: 444, Nanos: 0}
		completedTime = timestamp.Timestamp{Seconds: 444, Nanos: 0}
	} else {
		state = invlibs.DeviceManualRepairRecord_STATE_IN_PROGRESS
		updatedTime = timestamp.Timestamp{Seconds: 222, Nanos: 0}
		completedTime = timestamp.Timestamp{Seconds: 444, Nanos: 0}
	}

	return &invlibs.DeviceManualRepairRecord{
		Hostname:                        hostname,
		AssetTag:                        assetTag,
		RepairTargetType:                invlibs.DeviceManualRepairRecord_TYPE_DUT,
		RepairState:                     state,
		BuganizerBugUrl:                 "https://b/12345678",
		ChromiumBugUrl:                  "https://crbug.com/12345678",
		DutRepairFailureDescription:     "Mock DUT repair failure description.",
		DutVerifierFailureDescription:   "Mock DUT verifier failure description.",
		ServoRepairFailureDescription:   "Mock Servo repair failure description.",
		ServoVerifierFailureDescription: "Mock Servo verifier failure description.",
		Diagnosis:                       "Mock diagnosis.",
		RepairProcedure:                 "Mock repair procedure.",
		LabstationRepairActions: []invlibs.LabstationRepairAction{
			invlibs.LabstationRepairAction_LABSTATION_POWER_CYCLE,
			invlibs.LabstationRepairAction_LABSTATION_REIMAGE,
			invlibs.LabstationRepairAction_LABSTATION_UPDATE_CONFIG,
			invlibs.LabstationRepairAction_LABSTATION_REPLACE,
		},
		IssueFixed:    true,
		UserLdap:      "testing-account",
		TimeTaken:     15,
		CreatedTime:   &timestamp.Timestamp{Seconds: createdTime, Nanos: 0},
		UpdatedTime:   &updatedTime,
		CompletedTime: &completedTime,
	}
}

func mockServo(servoHost string) *lab.Servo {
	return &lab.Servo{
		ServoHostname: servoHost,
		ServoPort:     8888,
		ServoSerial:   "SERVO1",
		ServoType:     "v3",
	}
}

func mockDut(hostname, id, servoHost string) *lab.ChromeOSDevice {
	return &lab.ChromeOSDevice{
		Id: &lab.ChromeOSDeviceID{
			Value: id,
		},
		Device: &lab.ChromeOSDevice_Dut{
			Dut: &lab.DeviceUnderTest{
				Hostname: hostname,
				Peripherals: &lab.Peripherals{
					Servo:       mockServo(servoHost),
					SmartUsbhub: false,
				},
			},
		},
	}
}

func mockLabstation(hostname, id string) *lab.ChromeOSDevice {
	return &lab.ChromeOSDevice{
		Id: &lab.ChromeOSDeviceID{
			Value: id,
		},
		Device: &lab.ChromeOSDevice_Labstation{
			Labstation: &lab.Labstation{
				Hostname: hostname,
			},
		},
	}
}

func TestGetDeviceManualRepairRecord(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()

	ds.GetTestable(ctx).Consistent(true)

	record1 := mockDeviceManualRepairRecord("chromeos-getRecords-aa", "getRecords-111", 1, false)
	record2 := mockDeviceManualRepairRecord("chromeos-getRecords-bb", "getRecords-222", 1, false)
	record3 := mockDeviceManualRepairRecord("chromeos-getRecords-bb", "getRecords-333", 1, false)
	records := []*invlibs.DeviceManualRepairRecord{record1, record2, record3}

	// Set up records in datastore
	datastore.AddDeviceManualRepairRecords(ctx, records)

	ftt.Run("Test get device manual repair records", t, func(t *ftt.Test) {
		t.Run("Get record using single hostname", func(t *ftt.Test) {
			req := &api.GetDeviceManualRepairRecordRequest{
				Hostname: "chromeos-getRecords-aa",
			}
			resp, err := tf.Inventory.GetDeviceManualRepairRecord(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.DeviceRepairRecord, should.NotBeNil)
		})
		t.Run("Get first record when hostname has multiple active records", func(t *ftt.Test) {
			req := &api.GetDeviceManualRepairRecordRequest{
				Hostname: "chromeos-getRecords-bb",
			}
			resp, err := tf.Inventory.GetDeviceManualRepairRecord(tf.C, req)
			assert.Loosely(t, resp.DeviceRepairRecord, should.NotBeNil)
			assert.Loosely(t, resp.DeviceRepairRecord.GetAssetTag(), should.Equal("getRecords-222"))
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("More than one active record found"))
		})
		t.Run("Get record using non-existent hostname", func(t *ftt.Test) {
			req := &api.GetDeviceManualRepairRecordRequest{
				Hostname: "chromeos-getRecords-cc",
			}
			resp, err := tf.Inventory.GetDeviceManualRepairRecord(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("No record found"))
		})
		t.Run("Get record using empty hostname", func(t *ftt.Test) {
			req := &api.GetDeviceManualRepairRecordRequest{
				Hostname: "",
			}
			resp, err := tf.Inventory.GetDeviceManualRepairRecord(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("No record found"))
		})
	})
}

func TestCreateDeviceManualRepairRecord(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()

	ds.GetTestable(ctx).Consistent(true)

	// Empty datastore
	record1 := mockDeviceManualRepairRecord("chromeos-createRecords-aa", "", 1, false)
	record2 := mockDeviceManualRepairRecord("", "", 1, false)

	// Set up records in datastore
	ftt.Run("Test add devices using an empty datastore", t, func(t *ftt.Test) {
		t.Run("Add single record", func(t *ftt.Test) {
			propFilter := map[string]string{"hostname": record1.Hostname}
			req := &api.CreateDeviceManualRepairRecordRequest{DeviceRepairRecord: record1}
			rsp, err := tf.Inventory.CreateDeviceManualRepairRecord(tf.C, req)
			assert.Loosely(t, rsp.String(), should.BeEmpty)
			assert.Loosely(t, err, should.BeNil)

			// Check added record
			getRes, err := datastore.GetRepairRecordByPropertyName(ctx, propFilter, -1, 0, []string{})
			assert.Loosely(t, getRes, should.HaveLength(1))
			assert.Loosely(t, getRes[0].Record.GetHostname(), should.Equal("chromeos-createRecords-aa"))
			assert.Loosely(t, getRes[0].Record.GetAssetTag(), should.Equal("n/a"))
			assert.Loosely(t, getRes[0].Record.GetCreatedTime(), should.NotResemble(&timestamp.Timestamp{Seconds: 1, Nanos: 0}))
		})
		t.Run("Add single record without hostname", func(t *ftt.Test) {
			req := &api.CreateDeviceManualRepairRecordRequest{DeviceRepairRecord: record2}
			rsp, err := tf.Inventory.CreateDeviceManualRepairRecord(tf.C, req)
			assert.Loosely(t, rsp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Hostname cannot be empty"))

			// No record should be added
			propFilter := map[string]string{"hostname": record2.Hostname}
			getRes, err := datastore.GetRepairRecordByPropertyName(ctx, propFilter, -1, 0, []string{})
			assert.Loosely(t, getRes, should.HaveLength(0))
		})
		t.Run("Add single record to a host with an open record", func(t *ftt.Test) {
			// Check existing record
			propFilter := map[string]string{"hostname": record1.Hostname}
			getRes, err := datastore.GetRepairRecordByPropertyName(ctx, propFilter, -1, 0, []string{})
			assert.Loosely(t, getRes, should.HaveLength(1))
			assert.Loosely(t, getRes[0].Record.GetHostname(), should.Equal("chromeos-createRecords-aa"))

			req := &api.CreateDeviceManualRepairRecordRequest{DeviceRepairRecord: record1}
			rsp, err := tf.Inventory.CreateDeviceManualRepairRecord(tf.C, req)
			assert.Loosely(t, rsp, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("A record already exists for host chromeos-createRecords-aa"))
		})
	})

	// Datastore with DeviceEntity
	record3 := mockDeviceManualRepairRecord("chromeos-createRecords-bb", "", 1, false)
	record4 := mockDeviceManualRepairRecord("chromeos-createRecords-cc", "", 1, false)
	record5 := mockDeviceManualRepairRecord("", "", 1, false)
	record6 := mockDeviceManualRepairRecord("chromeos-createRecords-ee", "", 1, true)

	ftt.Run("Test add devices using an non-empty datastore", t, func(t *ftt.Test) {
		dut1 := mockDut("chromeos-createRecords-bb", "mockDutAssetTag-111", "labstation1")
		dut2 := mockDut("chromeos-createRecords-cc", "", "labstation1")
		dut3 := mockDut("chromeos-createRecords-ee", "mockDutAssetTag-222", "labstation1")
		labstation1 := mockLabstation("labstation1", "assetId-111")
		dut1.DeviceConfigId = &device.ConfigId{ModelId: &device.ModelId{Value: "model1"}}
		dut2.DeviceConfigId = &device.ConfigId{ModelId: &device.ModelId{Value: "model2"}}
		dut3.DeviceConfigId = &device.ConfigId{ModelId: &device.ModelId{Value: "model3"}}
		labstation1.DeviceConfigId = &device.ConfigId{
			ModelId: &device.ModelId{Value: "model5"},
		}
		devsToAdd := []*lab.ChromeOSDevice{dut1, dut2, dut3, labstation1}
		_, err := datastore.AddDevices(ctx, devsToAdd, false)
		if err != nil {
			t.Fatal(err)
		}
		t.Run("Add single record", func(t *ftt.Test) {
			propFilter := map[string]string{"hostname": record3.Hostname}
			req := &api.CreateDeviceManualRepairRecordRequest{DeviceRepairRecord: record3}
			rsp, err := tf.Inventory.CreateDeviceManualRepairRecord(tf.C, req)
			assert.Loosely(t, rsp.String(), should.BeEmpty)
			assert.Loosely(t, err, should.BeNil)

			// Check added record
			getRes, err := datastore.GetRepairRecordByPropertyName(ctx, propFilter, -1, 0, []string{})
			assert.Loosely(t, getRes, should.HaveLength(1))
			assert.Loosely(t, getRes[0].Record.GetHostname(), should.Equal("chromeos-createRecords-bb"))
			assert.Loosely(t, getRes[0].Record.GetAssetTag(), should.Equal("mockDutAssetTag-111"))
			assert.Loosely(t, getRes[0].Record.GetCreatedTime(), should.NotResemble(&timestamp.Timestamp{Seconds: 1, Nanos: 0}))
		})
		t.Run("Add single record using dut without asset tag", func(t *ftt.Test) {
			propFilter := map[string]string{"hostname": record4.Hostname}
			req := &api.CreateDeviceManualRepairRecordRequest{DeviceRepairRecord: record4}
			rsp, err := tf.Inventory.CreateDeviceManualRepairRecord(tf.C, req)
			assert.Loosely(t, rsp.String(), should.BeEmpty)
			assert.Loosely(t, err, should.BeNil)

			// Asset tag should be uuid generated for dut
			getRes, err := datastore.GetRepairRecordByPropertyName(ctx, propFilter, -1, 0, []string{})
			assert.Loosely(t, getRes, should.HaveLength(1))
			assert.Loosely(t, getRes[0].Record.GetHostname(), should.Equal("chromeos-createRecords-cc"))
			assert.Loosely(t, getRes[0].Record.GetAssetTag(), should.NotEqual(""))
			assert.Loosely(t, getRes[0].Record.GetAssetTag(), should.NotEqual("n/a"))
			assert.Loosely(t, getRes[0].Record.GetCreatedTime(), should.NotResemble(&timestamp.Timestamp{Seconds: 1, Nanos: 0}))
		})
		t.Run("Add single record with no hostname", func(t *ftt.Test) {
			req := &api.CreateDeviceManualRepairRecordRequest{DeviceRepairRecord: record5}
			rsp, err := tf.Inventory.CreateDeviceManualRepairRecord(tf.C, req)
			assert.Loosely(t, rsp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Hostname cannot be empty"))
		})
		t.Run("Add single record with completed repair state", func(t *ftt.Test) {
			propFilter := map[string]string{"hostname": record6.Hostname}
			req := &api.CreateDeviceManualRepairRecordRequest{DeviceRepairRecord: record6}
			rsp, err := tf.Inventory.CreateDeviceManualRepairRecord(tf.C, req)
			assert.Loosely(t, rsp.String(), should.BeEmpty)
			assert.Loosely(t, err, should.BeNil)

			// Completed time should be same as created
			getRes, err := datastore.GetRepairRecordByPropertyName(ctx, propFilter, -1, 0, []string{})
			assert.Loosely(t, getRes, should.HaveLength(1))
			assert.Loosely(t, getRes[0].Record.GetHostname(), should.Equal("chromeos-createRecords-ee"))
			assert.Loosely(t, getRes[0].Record.GetAssetTag(), should.Equal("mockDutAssetTag-222"))
			assert.Loosely(t, getRes[0].Record.GetCreatedTime(), should.NotResemble(&timestamp.Timestamp{Seconds: 1, Nanos: 0}))
			assert.Loosely(t, getRes[0].Record.GetCreatedTime(), should.Resemble(getRes[0].Record.GetCompletedTime()))
		})
	})
}

func TestUpdateDeviceManualRepairRecord(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()

	ds.GetTestable(ctx).Consistent(true)

	// Empty datastore
	record1 := mockDeviceManualRepairRecord("chromeos-updateRecords-aa", "updateRec-111", 1, false)
	record1Complete := mockDeviceManualRepairRecord("chromeos-updateRecords-aa", "updateRec-111", 1, true)
	record2 := mockDeviceManualRepairRecord("chromeos-updateRecords-bb", "updateRec-222", 1, false)
	record2Complete := mockDeviceManualRepairRecord("chromeos-updateRecords-bb", "updateRec-222", 1, true)
	record3 := mockDeviceManualRepairRecord("chromeos-updateRecords-cc", "updateRec-333", 1, false)
	record3Update := mockDeviceManualRepairRecord("chromeos-updateRecords-cc", "updateRec-333", 1, false)
	record4 := mockDeviceManualRepairRecord("chromeos-updateRecords-dd", "updateRec-444", 1, false)

	// Set up records in datastore
	records := []*invlibs.DeviceManualRepairRecord{record1, record2, record3}
	datastore.AddDeviceManualRepairRecords(ctx, records)

	ftt.Run("Test update devices using an non-empty datastore", t, func(t *ftt.Test) {
		t.Run("Update single record with completed repair state", func(t *ftt.Test) {
			propFilter := map[string]string{"hostname": record1.Hostname}
			getRes, err := datastore.GetRepairRecordByPropertyName(ctx, propFilter, -1, 0, []string{})
			req := &api.UpdateDeviceManualRepairRecordRequest{
				Id:                 getRes[0].Entity.ID,
				DeviceRepairRecord: record1Complete,
			}
			rsp, err := tf.Inventory.UpdateDeviceManualRepairRecord(tf.C, req)
			assert.Loosely(t, rsp.String(), should.BeEmpty)
			assert.Loosely(t, err, should.BeNil)

			// Check updated record
			getRes, err = datastore.GetRepairRecordByPropertyName(ctx, propFilter, -1, 0, []string{})
			assert.Loosely(t, getRes, should.HaveLength(1))
			assert.Loosely(t, getRes[0].Record.GetHostname(), should.Equal("chromeos-updateRecords-aa"))
			assert.Loosely(t, getRes[0].Record.GetRepairState(), should.Equal(invlibs.DeviceManualRepairRecord_STATE_COMPLETED))
			assert.Loosely(t, getRes[0].Record.GetUpdatedTime(), should.NotResemble(&timestamp.Timestamp{Seconds: 222, Nanos: 0}))
			assert.Loosely(t, getRes[0].Record.GetUpdatedTime(), should.Resemble(getRes[0].Record.GetCompletedTime()))
		})
		t.Run("Update single record with no id", func(t *ftt.Test) {
			req := &api.UpdateDeviceManualRepairRecordRequest{
				Id:                 "",
				DeviceRepairRecord: record2Complete,
			}
			rsp, err := tf.Inventory.UpdateDeviceManualRepairRecord(tf.C, req)
			assert.Loosely(t, rsp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("ID cannot be empty"))

			// Check updated record and make sure it is unchanged
			getRes, err := datastore.GetRepairRecordByPropertyName(ctx, map[string]string{"hostname": record2.Hostname}, -1, 0, []string{})
			assert.Loosely(t, getRes, should.HaveLength(1))
			assert.Loosely(t, getRes[0].Record.GetHostname(), should.Equal("chromeos-updateRecords-bb"))
			assert.Loosely(t, getRes[0].Record.GetRepairState(), should.Equal(invlibs.DeviceManualRepairRecord_STATE_IN_PROGRESS))
			assert.Loosely(t, getRes[0].Record.GetUpdatedTime(), should.Resemble(&timestamp.Timestamp{Seconds: 222, Nanos: 0}))
		})
		t.Run("Update single record", func(t *ftt.Test) {
			propFilter := map[string]string{"hostname": record3.Hostname}
			getRes, err := datastore.GetRepairRecordByPropertyName(ctx, propFilter, -1, 0, []string{})
			record3Update.TimeTaken = 20
			req := &api.UpdateDeviceManualRepairRecordRequest{
				Id:                 getRes[0].Entity.ID,
				DeviceRepairRecord: record3Update,
			}
			rsp, err := tf.Inventory.UpdateDeviceManualRepairRecord(tf.C, req)
			assert.Loosely(t, rsp.String(), should.BeEmpty)
			assert.Loosely(t, err, should.BeNil)

			// Check updated record and make sure fields are changed properly
			getRes, err = datastore.GetRepairRecordByPropertyName(ctx, propFilter, -1, 0, []string{})
			assert.Loosely(t, getRes, should.HaveLength(1))
			assert.Loosely(t, getRes[0].Record.GetHostname(), should.Equal("chromeos-updateRecords-cc"))
			assert.Loosely(t, getRes[0].Record.GetRepairState(), should.Equal(invlibs.DeviceManualRepairRecord_STATE_IN_PROGRESS))
			assert.Loosely(t, getRes[0].Record.GetTimeTaken(), should.Equal(20))
			assert.Loosely(t, getRes[0].Record.GetUpdatedTime(), should.NotResemble(&timestamp.Timestamp{Seconds: 222, Nanos: 0}))
			assert.Loosely(t, getRes[0].Record.GetCompletedTime(), should.Resemble(&timestamp.Timestamp{Seconds: 444, Nanos: 0}))
		})
		t.Run("Update single non-existent record", func(t *ftt.Test) {
			propFilter := map[string]string{"hostname": record4.Hostname}
			getRes, err := datastore.GetRepairRecordByPropertyName(ctx, propFilter, -1, 0, []string{})
			assert.Loosely(t, getRes, should.HaveLength(0))

			req := &api.UpdateDeviceManualRepairRecordRequest{
				Id:                 "test-id",
				DeviceRepairRecord: record4,
			}
			rsp, err := tf.Inventory.UpdateDeviceManualRepairRecord(tf.C, req)
			assert.Loosely(t, rsp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("No open record exists for host chromeos-updateRecords-dd"))
		})
	})
}

func TestListManualRepairRecords(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()

	ds.GetTestable(ctx).AutoIndex(true)
	ds.GetTestable(ctx).Consistent(true)

	// Updated times should go in descending order of record1 > record2 = record3
	record1 := mockDeviceManualRepairRecord("chromeos-getRecords-aa", "getRecords-111", 1, true)
	record2 := mockDeviceManualRepairRecord("chromeos-getRecords-aa", "getRecords-111", 2, false)
	record3 := mockDeviceManualRepairRecord("chromeos-getRecords-aa", "getRecords-222", 3, false)
	records := []*invlibs.DeviceManualRepairRecord{record1, record2, record3}

	// Set up records in datastore
	datastore.AddDeviceManualRepairRecords(ctx, records)

	ftt.Run("Test list device manual repair records", t, func(t *ftt.Test) {
		t.Run("List records using hostname and asset tag", func(t *ftt.Test) {
			req := &api.ListManualRepairRecordsRequest{
				Hostname: "chromeos-getRecords-aa",
				AssetTag: "getRecords-111",
				Limit:    5,
			}
			resp, err := tf.Inventory.ListManualRepairRecords(tf.C, req)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.RepairRecords, should.NotBeNil)
			assert.Loosely(t, resp.RepairRecords, should.HaveLength(2))
			assert.Loosely(t, resp.RepairRecords[0].GetHostname(), should.Equal("chromeos-getRecords-aa"))
			assert.Loosely(t, resp.RepairRecords[0].GetAssetTag(), should.Equal("getRecords-111"))
			assert.Loosely(t, resp.RepairRecords[0].GetRepairState(), should.Equal(invlibs.DeviceManualRepairRecord_STATE_IN_PROGRESS))
			assert.Loosely(t, resp.RepairRecords[1].GetHostname(), should.Equal("chromeos-getRecords-aa"))
			assert.Loosely(t, resp.RepairRecords[1].GetAssetTag(), should.Equal("getRecords-111"))
			assert.Loosely(t, resp.RepairRecords[1].GetRepairState(), should.Equal(invlibs.DeviceManualRepairRecord_STATE_COMPLETED))
		})
		t.Run("List records using hostname and asset tag with offset", func(t *ftt.Test) {
			req := &api.ListManualRepairRecordsRequest{
				Hostname: "chromeos-getRecords-aa",
				AssetTag: "getRecords-111",
				Limit:    1,
				Offset:   1,
			}
			resp, err := tf.Inventory.ListManualRepairRecords(tf.C, req)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.RepairRecords, should.NotBeNil)
			assert.Loosely(t, resp.RepairRecords, should.HaveLength(1))
			assert.Loosely(t, resp.RepairRecords[0].GetHostname(), should.Equal("chromeos-getRecords-aa"))
			assert.Loosely(t, resp.RepairRecords[0].GetAssetTag(), should.Equal("getRecords-111"))
			assert.Loosely(t, resp.RepairRecords[0].GetRepairState(), should.Equal(invlibs.DeviceManualRepairRecord_STATE_COMPLETED))
		})
		t.Run("List records using all filters", func(t *ftt.Test) {
			req := &api.ListManualRepairRecordsRequest{
				Hostname:    "chromeos-getRecords-aa",
				AssetTag:    "getRecords-111",
				Limit:       5,
				UserLdap:    "testing-account",
				RepairState: "STATE_COMPLETED",
			}
			resp, err := tf.Inventory.ListManualRepairRecords(tf.C, req)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.RepairRecords, should.NotBeNil)
			assert.Loosely(t, resp.RepairRecords, should.HaveLength(1))
			assert.Loosely(t, resp.RepairRecords[0].GetHostname(), should.Equal("chromeos-getRecords-aa"))
			assert.Loosely(t, resp.RepairRecords[0].GetAssetTag(), should.Equal("getRecords-111"))
			assert.Loosely(t, resp.RepairRecords[0].GetRepairState(), should.Equal(invlibs.DeviceManualRepairRecord_STATE_COMPLETED))
		})
		t.Run("List records using hostname and asset tag with limit 1", func(t *ftt.Test) {
			req := &api.ListManualRepairRecordsRequest{
				Hostname: "chromeos-getRecords-aa",
				AssetTag: "getRecords-111",
				Limit:    1,
			}
			resp, err := tf.Inventory.ListManualRepairRecords(tf.C, req)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.RepairRecords, should.NotBeNil)
			assert.Loosely(t, resp.RepairRecords, should.HaveLength(1))
			assert.Loosely(t, resp.RepairRecords[0].GetHostname(), should.Equal("chromeos-getRecords-aa"))
			assert.Loosely(t, resp.RepairRecords[0].GetAssetTag(), should.Equal("getRecords-111"))
			assert.Loosely(t, resp.RepairRecords[0].GetRepairState(), should.Equal(invlibs.DeviceManualRepairRecord_STATE_IN_PROGRESS))
		})
		t.Run("List records that do not exist", func(t *ftt.Test) {
			req := &api.ListManualRepairRecordsRequest{
				Hostname: "chromeos-getRecords-bb",
				AssetTag: "getRecords-111",
				Limit:    5,
			}
			resp, err := tf.Inventory.ListManualRepairRecords(tf.C, req)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.RepairRecords, should.HaveLength(0))
		})
	})
}

func TestBatchGetManualRepairRecords(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()

	ds.GetTestable(ctx).Consistent(true)

	record1 := mockDeviceManualRepairRecord("chromeos-getRecords-xx", "getRecords-111", 1, false)
	record2 := mockDeviceManualRepairRecord("chromeos-getRecords-yy", "getRecords-222", 1, false)
	record3 := mockDeviceManualRepairRecord("chromeos-getRecords-zz", "getRecords-333", 1, false)
	record4 := mockDeviceManualRepairRecord("chromeos-getRecords-zz", "getRecords-444", 1, false)
	records := []*invlibs.DeviceManualRepairRecord{record1, record2, record3, record4}

	// Set up records in datastore
	datastore.AddDeviceManualRepairRecords(ctx, records)

	ftt.Run("Test batch get manual repair records", t, func(t *ftt.Test) {
		t.Run("Get record using multiple hostnames", func(t *ftt.Test) {
			req := &api.BatchGetManualRepairRecordsRequest{
				Hostnames: []string{
					"chromeos-getRecords-xx",
					"chromeos-getRecords-yy",
				},
			}
			resp, err := tf.Inventory.BatchGetManualRepairRecords(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.RepairRecords, should.HaveLength(2))
			assert.Loosely(t, resp.RepairRecords[0].ErrorMsg, should.BeEmpty)
			assert.Loosely(t, resp.RepairRecords[0].RepairRecord, should.NotBeNil)
			assert.Loosely(t, resp.RepairRecords[0].RepairRecord.Hostname, should.Equal("chromeos-getRecords-xx"))
			assert.Loosely(t, resp.RepairRecords[0].Hostname, should.Equal("chromeos-getRecords-xx"))
			assert.Loosely(t, resp.RepairRecords[1].ErrorMsg, should.BeEmpty)
			assert.Loosely(t, resp.RepairRecords[1].RepairRecord, should.NotBeNil)
			assert.Loosely(t, resp.RepairRecords[1].RepairRecord.Hostname, should.Equal("chromeos-getRecords-yy"))
			assert.Loosely(t, resp.RepairRecords[1].Hostname, should.Equal("chromeos-getRecords-yy"))
		})
		t.Run("Get first record when hostname has multiple active records", func(t *ftt.Test) {
			req := &api.BatchGetManualRepairRecordsRequest{
				Hostnames: []string{
					"chromeos-getRecords-zz",
				},
			}
			resp, err := tf.Inventory.BatchGetManualRepairRecords(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.RepairRecords, should.HaveLength(1))
			assert.Loosely(t, resp.RepairRecords[0].RepairRecord.Hostname, should.Equal("chromeos-getRecords-zz"))
			assert.Loosely(t, resp.RepairRecords[0].Hostname, should.Equal("chromeos-getRecords-zz"))
		})
		t.Run("Get record using a non-existent hostname", func(t *ftt.Test) {
			req := &api.BatchGetManualRepairRecordsRequest{
				Hostnames: []string{
					"chromeos-getRecords-xx",
					"chromeos-getRecords-cc",
				},
			}
			resp, err := tf.Inventory.BatchGetManualRepairRecords(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.RepairRecords, should.HaveLength(2))
			assert.Loosely(t, resp.RepairRecords[0].ErrorMsg, should.BeEmpty)
			assert.Loosely(t, resp.RepairRecords[0].RepairRecord, should.NotBeNil)
			assert.Loosely(t, resp.RepairRecords[0].RepairRecord.Hostname, should.Equal("chromeos-getRecords-xx"))
			assert.Loosely(t, resp.RepairRecords[0].Hostname, should.Equal("chromeos-getRecords-xx"))
			assert.Loosely(t, resp.RepairRecords[1].ErrorMsg, should.ContainSubstring("No record found"))
			assert.Loosely(t, resp.RepairRecords[1].RepairRecord, should.BeNil)
			assert.Loosely(t, resp.RepairRecords[1].Hostname, should.Equal("chromeos-getRecords-cc"))
		})
	})
}

func TestBatchCreateManualRepairRecords(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()

	ds.GetTestable(ctx).Consistent(true)

	// Empty datastore
	record1 := mockDeviceManualRepairRecord("chromeos-createRecords-zz", "", 1, false)
	record2 := mockDeviceManualRepairRecord("chromeos-createRecords-yy", "", 1, false)
	record3 := mockDeviceManualRepairRecord("chromeos-createRecords-xx", "", 1, false)
	record4 := mockDeviceManualRepairRecord("chromeos-createRecords-ww", "", 1, false)
	record5 := mockDeviceManualRepairRecord("", "", 1, false)

	// Set up records in datastore
	ftt.Run("Test add devices using an empty datastore", t, func(t *ftt.Test) {
		t.Run("Add single record", func(t *ftt.Test) {
			createReq := &api.BatchCreateManualRepairRecordsRequest{
				RepairRecords: []*invlibs.DeviceManualRepairRecord{record1},
			}
			createRsp, err := tf.Inventory.BatchCreateManualRepairRecords(tf.C, createReq)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, createRsp, should.NotBeNil)
			assert.Loosely(t, createRsp.RepairRecords, should.HaveLength(1))
			assert.Loosely(t, createRsp.RepairRecords[0].ErrorMsg, should.BeEmpty)
			assert.Loosely(t, createRsp.RepairRecords[0].RepairRecord, should.NotBeNil)
			assert.Loosely(t, createRsp.RepairRecords[0].RepairRecord.Hostname, should.Equal("chromeos-createRecords-zz"))
			assert.Loosely(t, createRsp.RepairRecords[0].Hostname, should.Equal("chromeos-createRecords-zz"))

			// Check added record
			getReq := &api.BatchGetManualRepairRecordsRequest{
				Hostnames: []string{
					"chromeos-createRecords-zz",
				},
			}
			getRsp, err := tf.Inventory.BatchGetManualRepairRecords(tf.C, getReq)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, getRsp, should.NotBeNil)
			assert.Loosely(t, getRsp.RepairRecords, should.HaveLength(1))
			assert.Loosely(t, getRsp.RepairRecords[0].ErrorMsg, should.BeEmpty)
			assert.Loosely(t, getRsp.RepairRecords[0].RepairRecord, should.NotBeNil)
			assert.Loosely(t, getRsp.RepairRecords[0].RepairRecord.Hostname, should.Equal("chromeos-createRecords-zz"))
			assert.Loosely(t, getRsp.RepairRecords[0].Hostname, should.Equal("chromeos-createRecords-zz"))
		})
		t.Run("Add multiple records", func(t *ftt.Test) {
			createReq := &api.BatchCreateManualRepairRecordsRequest{
				RepairRecords: []*invlibs.DeviceManualRepairRecord{record2, record3},
			}
			createRsp, err := tf.Inventory.BatchCreateManualRepairRecords(tf.C, createReq)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, createRsp, should.NotBeNil)
			assert.Loosely(t, createRsp.RepairRecords, should.HaveLength(2))
			assert.Loosely(t, createRsp.RepairRecords[0].ErrorMsg, should.BeEmpty)
			assert.Loosely(t, createRsp.RepairRecords[0].RepairRecord, should.NotBeNil)
			assert.Loosely(t, createRsp.RepairRecords[0].RepairRecord.Hostname, should.Equal("chromeos-createRecords-xx"))
			assert.Loosely(t, createRsp.RepairRecords[0].Hostname, should.Equal("chromeos-createRecords-xx"))
			assert.Loosely(t, createRsp.RepairRecords[1].ErrorMsg, should.BeEmpty)
			assert.Loosely(t, createRsp.RepairRecords[1].RepairRecord, should.NotBeNil)
			assert.Loosely(t, createRsp.RepairRecords[1].RepairRecord.Hostname, should.Equal("chromeos-createRecords-yy"))
			assert.Loosely(t, createRsp.RepairRecords[1].Hostname, should.Equal("chromeos-createRecords-yy"))

			// Check added record
			getReq := &api.BatchGetManualRepairRecordsRequest{
				Hostnames: []string{
					"chromeos-createRecords-yy",
					"chromeos-createRecords-xx",
				},
			}
			getRsp, err := tf.Inventory.BatchGetManualRepairRecords(tf.C, getReq)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, getRsp, should.NotBeNil)
			assert.Loosely(t, getRsp.RepairRecords, should.HaveLength(2))
			assert.Loosely(t, getRsp.RepairRecords[0].ErrorMsg, should.BeEmpty)
			assert.Loosely(t, getRsp.RepairRecords[0].RepairRecord, should.NotBeNil)
			assert.Loosely(t, getRsp.RepairRecords[0].RepairRecord.Hostname, should.Equal("chromeos-createRecords-yy"))
			assert.Loosely(t, getRsp.RepairRecords[0].Hostname, should.Equal("chromeos-createRecords-yy"))
			assert.Loosely(t, getRsp.RepairRecords[1].ErrorMsg, should.BeEmpty)
			assert.Loosely(t, getRsp.RepairRecords[1].RepairRecord, should.NotBeNil)
			assert.Loosely(t, getRsp.RepairRecords[1].RepairRecord.Hostname, should.Equal("chromeos-createRecords-xx"))
			assert.Loosely(t, getRsp.RepairRecords[1].Hostname, should.Equal("chromeos-createRecords-xx"))
		})
		t.Run("Add multiple records; one with an open record", func(t *ftt.Test) {
			// Check existing record
			propFilter := map[string]string{"hostname": record1.Hostname}
			getRes, err := datastore.GetRepairRecordByPropertyName(ctx, propFilter, -1, 0, []string{})
			assert.Loosely(t, getRes, should.HaveLength(1))
			assert.Loosely(t, getRes[0].Record.GetHostname(), should.Equal("chromeos-createRecords-zz"))

			createReq := &api.BatchCreateManualRepairRecordsRequest{
				RepairRecords: []*invlibs.DeviceManualRepairRecord{record1, record4},
			}
			createRsp, err := tf.Inventory.BatchCreateManualRepairRecords(tf.C, createReq)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, createRsp, should.NotBeNil)
			assert.Loosely(t, createRsp.RepairRecords, should.HaveLength(2))
			assert.Loosely(t, createRsp.RepairRecords[0].ErrorMsg, should.BeEmpty)
			assert.Loosely(t, createRsp.RepairRecords[0].RepairRecord, should.NotBeNil)
			assert.Loosely(t, createRsp.RepairRecords[0].RepairRecord.Hostname, should.Equal("chromeos-createRecords-ww"))
			assert.Loosely(t, createRsp.RepairRecords[0].Hostname, should.Equal("chromeos-createRecords-ww"))
			assert.Loosely(t, createRsp.RepairRecords[1].ErrorMsg, should.ContainSubstring("A record already exists for host chromeos-createRecords-zz"))
			assert.Loosely(t, createRsp.RepairRecords[1].RepairRecord, should.BeNil)
			assert.Loosely(t, createRsp.RepairRecords[1].Hostname, should.Equal("chromeos-createRecords-zz"))
		})
		t.Run("Add single record without hostname", func(t *ftt.Test) {
			createReq := &api.BatchCreateManualRepairRecordsRequest{
				RepairRecords: []*invlibs.DeviceManualRepairRecord{record5},
			}
			createRsp, err := tf.Inventory.BatchCreateManualRepairRecords(tf.C, createReq)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, createRsp, should.NotBeNil)
			assert.Loosely(t, createRsp.RepairRecords, should.HaveLength(1))
			assert.Loosely(t, createRsp.RepairRecords[0].ErrorMsg, should.ContainSubstring("Hostname cannot be empty"))
			assert.Loosely(t, createRsp.RepairRecords[0].Hostname, should.BeEmpty)

			// No record should be added
			propFilter := map[string]string{"hostname": record5.Hostname}
			getRes, err := datastore.GetRepairRecordByPropertyName(ctx, propFilter, -1, 0, []string{})
			assert.Loosely(t, getRes, should.HaveLength(0))
		})
	})
}

func mockDevCfg(board string, model string, variant string) *device.Config {
	return &device.Config{
		Id: &device.ConfigId{
			PlatformId: &device.PlatformId{Value: board},
			ModelId:    &device.ModelId{Value: model},
			VariantId:  &device.VariantId{Value: variant},
		},
	}
}

func mockDevCfgEntity(devCfg *device.Config) (*devcfgEntity, error) {
	cfgBytes, err := proto.Marshal(devCfg)
	if err != nil {
		return nil, err
	}

	return &devcfgEntity{
		ID:        deviceconfig.GetDeviceConfigIDStr(devCfg.GetId()),
		DevConfig: cfgBytes,
	}, nil

}

func TestListDeviceConfigs(t *testing.T) {
	t.Parallel()

	ftt.Run("When device configs exist in datastore", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContext()
		ds.GetTestable(ctx).Consistent(true)
		tf, validate := newTestFixtureWithContext(ctx, t)
		defer validate()

		devCfg1 := mockDevCfg("board1", "model1", "variant1")
		cfgEntity1, err := mockDevCfgEntity(devCfg1)
		assert.Loosely(t, err, should.BeNil)

		devCfg2 := mockDevCfg("board2", "model2", "variant2")
		cfgEntity2, err := mockDevCfgEntity(devCfg2)
		assert.Loosely(t, err, should.BeNil)

		err = ds.Put(ctx, []devcfgEntity{
			*cfgEntity1,
			*cfgEntity2,
		})
		assert.Loosely(t, err, should.BeNil)

		t.Run("ListDeviceConfigs should return all configs", func(t *ftt.Test) {
			expected := &api.ListDeviceConfigsResponse{
				DeviceConfigs: []*device.Config{devCfg1, devCfg2},
			}
			resp2, err := tf.Inventory.ListDeviceConfigs(ctx, &api.ListDeviceConfigsRequest{})
			assert.Loosely(t, err, should.BeNil)
			if resp2 != nil {
				sort.Slice(resp2.DeviceConfigs, func(i int, j int) bool {
					// Id field is unique for device configs in real life, so we can use it to sort.
					return resp2.DeviceConfigs[i].GetId().String() < resp2.DeviceConfigs[j].GetId().String()
				})
			}
			assert.Loosely(t, resp2, should.Resemble(expected))
		})
	})
}
