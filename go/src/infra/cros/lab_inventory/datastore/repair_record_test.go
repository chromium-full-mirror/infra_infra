package datastore

import (
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	timestamp "github.com/golang/protobuf/ptypes/timestamp"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	invlibs "go.chromium.org/infra/cros/lab_inventory/protos"
)

func mockDeviceManualRepairRecord(hostname string, assetTag string, createdTime int64) *invlibs.DeviceManualRepairRecord {
	return &invlibs.DeviceManualRepairRecord{
		Hostname:                        hostname,
		AssetTag:                        assetTag,
		RepairTargetType:                invlibs.DeviceManualRepairRecord_TYPE_DUT,
		RepairState:                     invlibs.DeviceManualRepairRecord_STATE_NOT_STARTED,
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
		UpdatedTime:   &timestamp.Timestamp{Seconds: createdTime, Nanos: 0},
		CompletedTime: &timestamp.Timestamp{Seconds: 222, Nanos: 0},
	}
}

func mockUpdatedRecord(hostname string, assetTag string, createdTime int64) *invlibs.DeviceManualRepairRecord {
	return &invlibs.DeviceManualRepairRecord{
		Hostname:                        hostname,
		AssetTag:                        assetTag,
		RepairTargetType:                invlibs.DeviceManualRepairRecord_TYPE_DUT,
		RepairState:                     invlibs.DeviceManualRepairRecord_STATE_COMPLETED,
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
		TimeTaken:     30,
		CreatedTime:   &timestamp.Timestamp{Seconds: createdTime, Nanos: 0},
		UpdatedTime:   &timestamp.Timestamp{Seconds: 333, Nanos: 0},
		CompletedTime: &timestamp.Timestamp{Seconds: 333, Nanos: 0},
	}
}

func TestAddRecord(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	record1 := mockDeviceManualRepairRecord("chromeos-addRec-aa", "addRec-111", 1)
	record2 := mockDeviceManualRepairRecord("chromeos-addRec-bb", "addRec-222", 1)
	record3 := mockDeviceManualRepairRecord("chromeos-addRec-cc", "addRec-333", 1)
	record4 := mockDeviceManualRepairRecord("chromeos-addRec-dd", "addRec-444", 1)
	record5 := mockDeviceManualRepairRecord("", "", 1)

	rec1ID, _ := GenerateRepairRecordID(record1.Hostname, record1.AssetTag, ptypes.TimestampString(record1.CreatedTime))
	rec2ID, _ := GenerateRepairRecordID(record2.Hostname, record2.AssetTag, ptypes.TimestampString(record2.CreatedTime))
	ids1 := []string{rec1ID, rec2ID}

	rec3ID, _ := GenerateRepairRecordID(record3.Hostname, record3.AssetTag, ptypes.TimestampString(record3.CreatedTime))
	rec4ID, _ := GenerateRepairRecordID(record4.Hostname, record4.AssetTag, ptypes.TimestampString(record4.CreatedTime))
	ids2 := []string{rec3ID, rec4ID}
	ftt.Run("Add device manual repair record to datastore", t, func(t *ftt.Test) {
		t.Run("Add multiple device manual repair records to datastore", func(t *ftt.Test) {
			records := []*invlibs.DeviceManualRepairRecord{record1, record2}
			res, err := AddDeviceManualRepairRecords(ctx, records)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.HaveLength(2))

			// Put and Get order should be the same as the order in which the records
			// were passed in as arguments.
			for i, r := range res {
				assert.Loosely(t, r.Err, should.BeNil)
				assert.Loosely(t, r.Entity.Hostname, should.Equal(records[i].GetHostname()))
				assert.Loosely(t, r.Entity.AssetTag, should.Equal(records[i].GetAssetTag()))
				assert.Loosely(t, r.Entity.RepairState, should.Equal("STATE_NOT_STARTED"))

				updatedTime, _ := ptypes.Timestamp(records[i].GetUpdatedTime())
				assert.That(t, r.Entity.UpdatedTime, should.Match(updatedTime))
			}

			res = GetDeviceManualRepairRecords(ctx, ids1)
			assert.Loosely(t, res, should.HaveLength(2))
			for i, r := range res {
				assert.Loosely(t, r.Err, should.BeNil)
				assert.Loosely(t, r.Entity.Hostname, should.Equal(records[i].GetHostname()))
				assert.Loosely(t, r.Entity.AssetTag, should.Equal(records[i].GetAssetTag()))
				assert.Loosely(t, r.Entity.RepairState, should.Equal("STATE_NOT_STARTED"))

				updatedTime, _ := ptypes.Timestamp(records[i].GetUpdatedTime())
				assert.That(t, r.Entity.UpdatedTime, should.Match(updatedTime))
			}
		})
		t.Run("Add existing record to datastore", func(t *ftt.Test) {
			req := []*invlibs.DeviceManualRepairRecord{record3}
			res, err := AddDeviceManualRepairRecords(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.HaveLength(1))
			assert.Loosely(t, res[0].Err, should.BeNil)

			// Verify adding existing record.
			req = []*invlibs.DeviceManualRepairRecord{record3, record4}
			res, err = AddDeviceManualRepairRecords(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, res, should.HaveLength(2))
			assert.Loosely(t, res[0].Err, should.NotBeNil)
			assert.Loosely(t, res[0].Err.Error(), should.ContainSubstring("Record exists in the datastore"))
			assert.Loosely(t, res[1].Err, should.BeNil)
			assert.Loosely(t, res[1].Entity.Hostname, should.Equal(record4.GetHostname()))

			// Check both records are in datastore.
			res = GetDeviceManualRepairRecords(ctx, ids2)
			assert.Loosely(t, res, should.HaveLength(2))
			for i, r := range res {
				assert.Loosely(t, r.Err, should.BeNil)
				assert.Loosely(t, r.Entity.Hostname, should.Equal(req[i].GetHostname()))
				assert.Loosely(t, r.Entity.AssetTag, should.Equal(req[i].GetAssetTag()))
				assert.Loosely(t, r.Entity.RepairState, should.Equal("STATE_NOT_STARTED"))

				updatedTime, _ := ptypes.Timestamp(req[i].GetUpdatedTime())
				assert.Loosely(t, r.Entity.UpdatedTime, should.Match(updatedTime))
			}
		})
		t.Run("Add record without hostname to datastore", func(t *ftt.Test) {
			req := []*invlibs.DeviceManualRepairRecord{record5}
			res, err := AddDeviceManualRepairRecords(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.HaveLength(1))
			assert.Loosely(t, res[0].Err, should.NotBeNil)
			assert.Loosely(t, res[0].Err.Error(), should.ContainSubstring("Hostname cannot be empty"))
		})
	})
}

func TestGetRecord(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	record1 := mockDeviceManualRepairRecord("chromeos-getRec-aa", "getRec-111", 1)
	record2 := mockDeviceManualRepairRecord("chromeos-getRec-bb", "getRec-222", 1)
	rec1ID, _ := GenerateRepairRecordID(record1.Hostname, record1.AssetTag, ptypes.TimestampString(record1.CreatedTime))
	rec2ID, _ := GenerateRepairRecordID(record2.Hostname, record2.AssetTag, ptypes.TimestampString(record2.CreatedTime))
	ids1 := []string{rec1ID, rec2ID}
	ftt.Run("Get device manual repair record from datastore", t, func(t *ftt.Test) {
		t.Run("Get non-existent device manual repair record from datastore", func(t *ftt.Test) {
			records := []*invlibs.DeviceManualRepairRecord{record1}
			res, err := AddDeviceManualRepairRecords(ctx, records)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.HaveLength(1))

			res = GetDeviceManualRepairRecords(ctx, ids1)
			assert.Loosely(t, res, should.HaveLength(2))
			assert.Loosely(t, res[0].Err, should.BeNil)
			assert.Loosely(t, res[1].Err, should.NotBeNil)
			assert.Loosely(t, res[1].Err.Error(), should.ContainSubstring("datastore: no such entity"))

			updatedTime, _ := ptypes.Timestamp(record1.CreatedTime)
			assert.Loosely(t, res[0].Entity.UpdatedTime, should.Match(updatedTime))
		})
		t.Run("Get record with empty id", func(t *ftt.Test) {
			res := GetDeviceManualRepairRecords(ctx, []string{""})
			assert.Loosely(t, res, should.HaveLength(1))
			assert.Loosely(t, res[0].Err, should.NotBeNil)
			assert.Loosely(t, res[0].Err.Error(), should.ContainSubstring("datastore: invalid key"))
		})
	})
}

func TestGetRecordByPropertyName(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)

	record1 := mockDeviceManualRepairRecord("chromeos-getByProp-aa", "getByProp-111", 1)
	record2 := mockUpdatedRecord("chromeos-getByProp-aa", "getByProp-111", 2)
	record3 := mockDeviceManualRepairRecord("chromeos-getByProp-aa", "getByProp-222", 1)
	records := []*invlibs.DeviceManualRepairRecord{record1, record2, record3}

	// Set up records in datastore and test
	AddDeviceManualRepairRecords(ctx, records)

	ftt.Run("Get device manual repair record from datastore by property name", t, func(t *ftt.Test) {
		t.Run("Get repair record by Hostname", func(t *ftt.Test) {
			// Query should return record1, record2, record3
			res, err := GetRepairRecordByPropertyName(ctx, map[string]string{"hostname": "chromeos-getByProp-aa"}, -1, 0, []string{})
			assert.Loosely(t, res, should.HaveLength(3))
			assert.Loosely(t, err, should.BeNil)
			for _, r := range res {
				assert.Loosely(t, r.Err, should.BeNil)
				assert.Loosely(t, r.Entity.Hostname, should.Equal("chromeos-getByProp-aa"))
				assert.Loosely(t, r.Record.GetHostname(), should.Equal("chromeos-getByProp-aa"))
				assert.Loosely(t, []string{"STATE_NOT_STARTED", "STATE_COMPLETED"}, should.Contain(r.Entity.RepairState))
				assert.Loosely(t, []string{"STATE_NOT_STARTED", "STATE_COMPLETED"}, should.Contain(r.Record.GetRepairState().String()))
				assert.Loosely(t, []string{"getByProp-111", "getByProp-222"}, should.Contain(r.Entity.AssetTag))
				assert.Loosely(t, []string{"getByProp-111", "getByProp-222"}, should.Contain(r.Record.GetAssetTag()))
			}
		})
		t.Run("Get repair record by Hostname limit to 1", func(t *ftt.Test) {
			// Query should return record1
			res, err := GetRepairRecordByPropertyName(ctx, map[string]string{"hostname": "chromeos-getByProp-aa"}, 1, 0, []string{})
			assert.Loosely(t, res, should.HaveLength(1))
			assert.Loosely(t, err, should.BeNil)
			r := res[0]
			assert.Loosely(t, r.Err, should.BeNil)
			assert.Loosely(t, r.Entity.Hostname, should.Equal("chromeos-getByProp-aa"))
			assert.Loosely(t, r.Record.GetHostname(), should.Equal("chromeos-getByProp-aa"))
			assert.Loosely(t, []string{"STATE_NOT_STARTED", "STATE_COMPLETED"}, should.Contain(r.Entity.RepairState))
			assert.Loosely(t, []string{"STATE_NOT_STARTED", "STATE_COMPLETED"}, should.Contain(r.Record.GetRepairState().String()))
			assert.Loosely(t, []string{"getByProp-111", "getByProp-222"}, should.Contain(r.Entity.AssetTag))
			assert.Loosely(t, []string{"getByProp-111", "getByProp-222"}, should.Contain(r.Record.GetAssetTag()))
		})
		t.Run("Get repair record by AssetTag", func(t *ftt.Test) {
			// Query should return record1, record2
			res, err := GetRepairRecordByPropertyName(ctx, map[string]string{"asset_tag": "getByProp-111"}, -1, 0, []string{})
			assert.Loosely(t, res, should.HaveLength(2))
			assert.Loosely(t, err, should.BeNil)
			for _, r := range res {
				assert.Loosely(t, r.Err, should.BeNil)
				assert.Loosely(t, r.Entity.Hostname, should.Equal("chromeos-getByProp-aa"))
				assert.Loosely(t, r.Record.GetHostname(), should.Equal("chromeos-getByProp-aa"))
				assert.Loosely(t, r.Entity.AssetTag, should.Equal("getByProp-111"))
				assert.Loosely(t, r.Record.GetAssetTag(), should.Equal("getByProp-111"))
				assert.Loosely(t, []string{"STATE_NOT_STARTED", "STATE_COMPLETED"}, should.Contain(r.Entity.RepairState))
				assert.Loosely(t, []string{"STATE_NOT_STARTED", "STATE_COMPLETED"}, should.Contain(r.Record.GetRepairState().String()))
			}
		})
		t.Run("Get repair record by RepairState", func(t *ftt.Test) {
			// Query should return record2
			res, err := GetRepairRecordByPropertyName(ctx, map[string]string{"repair_state": "STATE_COMPLETED"}, -1, 0, []string{})
			assert.Loosely(t, res, should.HaveLength(1))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res[0].Err, should.BeNil)
			assert.Loosely(t, res[0].Entity.Hostname, should.Equal("chromeos-getByProp-aa"))
			assert.Loosely(t, res[0].Record.GetHostname(), should.Equal("chromeos-getByProp-aa"))
			assert.Loosely(t, res[0].Entity.AssetTag, should.Equal("getByProp-111"))
			assert.Loosely(t, res[0].Record.GetAssetTag(), should.Equal("getByProp-111"))
			assert.Loosely(t, res[0].Entity.RepairState, should.Equal("STATE_COMPLETED"))
			assert.Loosely(t, res[0].Record.GetRepairState(), should.Equal(invlibs.DeviceManualRepairRecord_STATE_COMPLETED))
		})
		t.Run("Get repair record by multiple properties", func(t *ftt.Test) {
			// Query should return record1 and record2
			res, err := GetRepairRecordByPropertyName(ctx,
				map[string]string{
					"hostname":  "chromeos-getByProp-aa",
					"asset_tag": "getByProp-111",
				}, -1, 0, []string{})
			assert.Loosely(t, res, should.HaveLength(2))
			assert.Loosely(t, err, should.BeNil)
			for _, r := range res {
				assert.Loosely(t, r.Err, should.BeNil)
				assert.Loosely(t, r.Entity.Hostname, should.Equal("chromeos-getByProp-aa"))
				assert.Loosely(t, r.Record.GetHostname(), should.Equal("chromeos-getByProp-aa"))
				assert.Loosely(t, r.Entity.AssetTag, should.Equal("getByProp-111"))
				assert.Loosely(t, r.Record.GetAssetTag(), should.Equal("getByProp-111"))
				assert.Loosely(t, []string{"STATE_NOT_STARTED", "STATE_COMPLETED"}, should.Contain(r.Entity.RepairState))
				assert.Loosely(t, []string{"STATE_NOT_STARTED", "STATE_COMPLETED"}, should.Contain(r.Record.GetRepairState().String()))
			}
		})
		t.Run("Get repair record by multiple properties with offset", func(t *ftt.Test) {
			// Query should return record2
			res, err := GetRepairRecordByPropertyName(ctx,
				map[string]string{
					"hostname":  "chromeos-getByProp-aa",
					"asset_tag": "getByProp-111",
				}, -1, 1, []string{})
			assert.Loosely(t, res, should.HaveLength(1))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res[0].Err, should.BeNil)
			assert.Loosely(t, res[0].Entity.Hostname, should.Equal("chromeos-getByProp-aa"))
			assert.Loosely(t, res[0].Record.GetHostname(), should.Equal("chromeos-getByProp-aa"))
			assert.Loosely(t, res[0].Entity.AssetTag, should.Equal("getByProp-111"))
			assert.Loosely(t, res[0].Record.GetAssetTag(), should.Equal("getByProp-111"))
			assert.Loosely(t, res[0].Entity.RepairState, should.Equal("STATE_COMPLETED"))
			assert.Loosely(t, res[0].Record.GetRepairState().String(), should.Equal("STATE_COMPLETED"))
		})
	})
}

func TestUpdateRecord(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	record1 := mockDeviceManualRepairRecord("chromeos-updateRec-aa", "updateRec-111", 1)
	record1Update := mockUpdatedRecord("chromeos-updateRec-aa", "updateRec-111", 1)

	record2 := mockDeviceManualRepairRecord("chromeos-updateRec-bb", "updateRec-222", 1)

	record3 := mockDeviceManualRepairRecord("chromeos-updateRec-cc", "updateRec-333", 1)
	record3Update := mockUpdatedRecord("chromeos-updateRec-cc", "updateRec-333", 1)
	record4 := mockDeviceManualRepairRecord("chromeos-updateRec-dd", "updateRec-444", 1)

	record5 := mockDeviceManualRepairRecord("", "", 1)
	ftt.Run("Update record in datastore", t, func(t *ftt.Test) {
		t.Run("Update existing record to datastore", func(t *ftt.Test) {
			rec1ID, _ := GenerateRepairRecordID(record1.Hostname, record1.AssetTag, ptypes.TimestampString(record1.CreatedTime))
			req := []*invlibs.DeviceManualRepairRecord{record1}
			res, err := AddDeviceManualRepairRecords(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, res, should.HaveLength(1))
			assert.Loosely(t, res[0].Err, should.BeNil)

			res = GetDeviceManualRepairRecords(ctx, []string{rec1ID})
			assert.Loosely(t, res, should.HaveLength(1))
			assert.Loosely(t, res[0].Err, should.BeNil)
			assert.Loosely(t, res[0].Entity.RepairState, should.Equal("STATE_NOT_STARTED"))

			updatedTime1, _ := ptypes.Timestamp(record1.CreatedTime)
			assert.Loosely(t, res[0].Entity.UpdatedTime, should.Match(updatedTime1))

			// Update and check
			reqUpdate := map[string]*invlibs.DeviceManualRepairRecord{rec1ID: record1Update}
			res, err = UpdateDeviceManualRepairRecords(ctx, reqUpdate)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.HaveLength(1))
			assert.Loosely(t, res[0].Err, should.BeNil)

			res = GetDeviceManualRepairRecords(ctx, []string{rec1ID})
			assert.Loosely(t, res, should.HaveLength(1))
			assert.Loosely(t, res[0].Err, should.BeNil)
			assert.Loosely(t, res[0].Entity.RepairState, should.Equal("STATE_COMPLETED"))

			updatedTime1, _ = ptypes.Timestamp(&timestamp.Timestamp{Seconds: 333, Nanos: 0})
			assert.Loosely(t, res[0].Entity.UpdatedTime, should.Match(updatedTime1))
		})
		t.Run("Update non-existent record in datastore", func(t *ftt.Test) {
			rec2ID, _ := GenerateRepairRecordID(record2.Hostname, record2.AssetTag, ptypes.TimestampString(record2.CreatedTime))
			reqUpdate := map[string]*invlibs.DeviceManualRepairRecord{rec2ID: record2}
			res, err := UpdateDeviceManualRepairRecords(ctx, reqUpdate)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.HaveLength(1))
			assert.Loosely(t, res[0].Err, should.NotBeNil)
			assert.Loosely(t, res[0].Err.Error(), should.ContainSubstring("datastore: no such entity"))

			res = GetDeviceManualRepairRecords(ctx, []string{rec2ID})
			assert.Loosely(t, res, should.HaveLength(1))
			assert.Loosely(t, res[0].Err, should.NotBeNil)
		})
		t.Run("Update multiple records to datastore", func(t *ftt.Test) {
			rec3ID, _ := GenerateRepairRecordID(record3.Hostname, record3.AssetTag, ptypes.TimestampString(record3.CreatedTime))
			rec4ID, _ := GenerateRepairRecordID(record4.Hostname, record4.AssetTag, ptypes.TimestampString(record4.CreatedTime))
			req := []*invlibs.DeviceManualRepairRecord{record3, record4}
			res, err := AddDeviceManualRepairRecords(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, res, should.HaveLength(2))
			assert.Loosely(t, res[0].Err, should.BeNil)
			assert.Loosely(t, res[1].Err, should.BeNil)

			reqUpdate := map[string]*invlibs.DeviceManualRepairRecord{
				rec3ID: record3Update,
				rec4ID: record4,
			}
			res, err = UpdateDeviceManualRepairRecords(ctx, reqUpdate)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.HaveLength(2))
			assert.Loosely(t, res[0].Err, should.BeNil)
			assert.Loosely(t, res[1].Err, should.BeNil)

			res = GetDeviceManualRepairRecords(ctx, []string{rec3ID, rec4ID})
			assert.Loosely(t, res, should.HaveLength(2))
			assert.Loosely(t, res[0].Err, should.BeNil)
			assert.Loosely(t, res[1].Err, should.BeNil)
			assert.Loosely(t, res[0].Entity.RepairState, should.Equal("STATE_COMPLETED"))
			assert.Loosely(t, res[1].Entity.RepairState, should.Equal("STATE_NOT_STARTED"))

			updatedTime3, _ := ptypes.Timestamp(&timestamp.Timestamp{Seconds: 333, Nanos: 0})
			assert.Loosely(t, res[0].Entity.UpdatedTime, should.Match(updatedTime3))

			updatedTime4, _ := ptypes.Timestamp(record4.CreatedTime)
			assert.Loosely(t, res[1].Entity.UpdatedTime, should.Match(updatedTime4))
		})
		t.Run("Update record without ID to datastore", func(t *ftt.Test) {
			rec5ID, _ := GenerateRepairRecordID(record5.Hostname, record5.AssetTag, ptypes.TimestampString(record5.CreatedTime))
			reqUpdate := map[string]*invlibs.DeviceManualRepairRecord{rec5ID: record5}
			res, err := UpdateDeviceManualRepairRecords(ctx, reqUpdate)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.HaveLength(1))

			// Error should occur when trying to get old entity from datastore
			assert.Loosely(t, res[0].Err, should.NotBeNil)
			assert.Loosely(t, res[0].Err.Error(), should.ContainSubstring("datastore: no such entity"))

			res = GetDeviceManualRepairRecords(ctx, []string{rec5ID})
			assert.Loosely(t, res, should.HaveLength(1))
			assert.Loosely(t, res[0].Err, should.NotBeNil)
			assert.Loosely(t, res[0].Err.Error(), should.ContainSubstring("datastore: no such entity"))
		})
	})
}

func TestManualRepairIndexes(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)

	record1 := mockUpdatedRecord("chromeos-indexTest-aa", "indexTest-111", 1)
	record2 := mockUpdatedRecord("chromeos-indexTest-bb", "indexTest-222", 1)
	record3 := mockDeviceManualRepairRecord("chromeos-indexTest-cc", "indexTest-222", 1)
	record4 := mockUpdatedRecord("chromeos-indexTest-dd", "indexTest-444", 1)

	records := []*invlibs.DeviceManualRepairRecord{record1, record2, record3, record4}
	_, _ = AddDeviceManualRepairRecords(ctx, records)

	ftt.Run("Query device manual repair record from datastore using indexes", t, func(t *ftt.Test) {
		t.Run("Query by repair_state", func(t *ftt.Test) {
			q := datastore.NewQuery(DeviceManualRepairRecordEntityKind).
				Eq("repair_state", invlibs.DeviceManualRepairRecord_STATE_COMPLETED.String())

			var entities []*DeviceManualRepairRecordEntity
			err := datastore.GetAll(ctx, q, &entities)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, entities, should.HaveLength(3))
		})
		t.Run("Query by updated_time", func(t *ftt.Test) {
			rec4Update := mockUpdatedRecord("chromeos-indexTest-dd", "indexTest-444", 1)
			rec4Update.UpdatedTime, _ = ptypes.TimestampProto(time.Unix(555, 0).UTC())

			rec4ID, _ := GenerateRepairRecordID(rec4Update.Hostname, rec4Update.AssetTag, ptypes.TimestampString(rec4Update.CreatedTime))
			reqUpdate := map[string]*invlibs.DeviceManualRepairRecord{rec4ID: rec4Update}
			_, err := UpdateDeviceManualRepairRecords(ctx, reqUpdate)
			assert.Loosely(t, err, should.BeNil)

			q := datastore.NewQuery(DeviceManualRepairRecordEntityKind).
				Gte("updated_time", time.Unix(500, 0).UTC())

			var entities []*DeviceManualRepairRecordEntity
			err = datastore.GetAll(ctx, q, &entities)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, entities, should.HaveLength(1))
			assert.Loosely(t, entities[0].ID, should.Equal(rec4ID))
		})
	})
}
