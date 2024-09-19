// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"github.com/DATA-DOG/go-sqlmock"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.chromium.org/chromiumos/config/go/test/api"
	schedulingAPI "go.chromium.org/chromiumos/config/go/test/scheduling"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/common/testing/typed"

	"infra/device_manager/internal/database"
	"infra/device_manager/internal/external"
	"infra/device_manager/internal/model"
	"infra/libs/skylab/inventory/swarming"
)

func TestGetDevice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	baseQuery := `
		SELECT
			id,
			device_address,
			device_type,
			device_state,
			schedulable_labels,
			created_time,
			last_updated_time,
			is_active
		FROM "Devices"`

	timeNow := time.Now()
	validCases := []struct {
		name           string
		idType         model.DeviceIDType
		expectedDevice *api.Device
		err            error
	}{
		{
			name:   "GetDeviceByID: valid return; search by hostname",
			idType: model.IDTypeHostname,
			expectedDevice: &api.Device{
				Id: "test-device-1",
				Address: &api.DeviceAddress{
					Host: "1.1.1.1",
					Port: 1,
				},
				Type:  api.DeviceType_DEVICE_TYPE_PHYSICAL,
				State: api.DeviceState_DEVICE_STATE_AVAILABLE,
				HardwareReqs: &api.HardwareRequirements{
					SchedulableLabels: map[string]*api.HardwareRequirements_LabelValues{
						"label-test": {
							Values: []string{"test-value-1"},
						},
					},
				},
			},
			err: nil,
		},
		{
			name:   "GetDeviceByID: valid return; search by DUT ID",
			idType: model.IDTypeDutID,
			expectedDevice: &api.Device{
				Id: "test-device-1",
				Address: &api.DeviceAddress{
					Host: "1.1.1.1",
					Port: 1,
				},
				Type:  api.DeviceType_DEVICE_TYPE_PHYSICAL,
				State: api.DeviceState_DEVICE_STATE_AVAILABLE,
				HardwareReqs: &api.HardwareRequirements{
					SchedulableLabels: map[string]*api.HardwareRequirements_LabelValues{
						"label-test": {
							Values: []string{"test-value-1"},
						},
					},
				},
			},
			err: nil,
		},
	}

	for _, tt := range validCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer func() {
				mock.ExpectClose()
				err = db.Close()
				if err != nil {
					t.Fatalf("failed to close db: %s", err)
				}
			}()

			rows := sqlmock.NewRows([]string{
				"id",
				"device_address",
				"device_type",
				"device_state",
				"schedulable_labels",
				"created_time",
				"last_updated_time",
				"is_active"}).
				AddRow(
					"test-device-1",
					"1.1.1.1:1",
					"DEVICE_TYPE_PHYSICAL",
					"DEVICE_STATE_AVAILABLE",
					`{"label-test":{"Values":["test-value-1"]}}`,
					timeNow,
					timeNow,
					true).
				AddRow(
					"test-device-2",
					"2.2.2.2:2",
					"DEVICE_TYPE_VIRTUAL",
					"DEVICE_STATE_LEASED",
					`{"label-test":{"Values":["test-value-2"]}}`,
					timeNow,
					timeNow,
					false)

			query := baseQuery
			switch tt.idType {
			case model.IDTypeDutID:
				query += `
					WHERE
						jsonb_path_query_array(
							schedulable_labels,
							'$.dut_id.Values[0]'
						) @> to_jsonb($1::text);`
			case model.IDTypeHostname:
				query += `
					WHERE id=$1;`
			default:
				t.Errorf("unexpected error: id type %s is not supported", tt.idType)
			}

			mock.ExpectQuery(regexp.QuoteMeta(query)).
				WithArgs("test-device-1").
				WillReturnRows(rows)

			device, err := GetDevice(ctx, db, tt.idType, "test-device-1")
			if !errors.Is(err, tt.err) {
				t.Errorf("unexpected error: %s", err)
			}
			if diff := typed.Got(device).Want(tt.expectedDevice).Diff(); diff != "" {
				t.Errorf("unexpected diff: %s", diff)
			}
		})
	}

	failedCases := []struct {
		name           string
		idType         model.DeviceIDType
		expectedDevice *api.Device
		err            error
	}{
		{
			name:           "invalid request; search by hostname, no device name match",
			idType:         model.IDTypeHostname,
			expectedDevice: &api.Device{},
			err:            fmt.Errorf("GetDeviceByID: failed to get Device"),
		},
		{
			name:           "invalid request; search by DUT ID, no device name match",
			idType:         model.IDTypeDutID,
			expectedDevice: &api.Device{},
			err:            fmt.Errorf("GetDeviceByID: failed to get Device"),
		},
	}

	for _, tt := range failedCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer func() {
				mock.ExpectClose()
				err = db.Close()
				if err != nil {
					t.Fatalf("failed to close db: %s", err)
				}
			}()

			query := baseQuery
			switch tt.idType {
			case model.IDTypeDutID:
				query += `
					WHERE
						jsonb_path_query_array(
							schedulable_labels,
							'$.dut_id.Values[0]'
						) @> to_jsonb($1::text);`
			case model.IDTypeHostname:
				query += `
					WHERE id=$1;`
			default:
				t.Errorf("unexpected error: id type %s is not supported", tt.idType)
			}

			mock.ExpectQuery(regexp.QuoteMeta(query)).
				WithArgs("test-device-2").
				WillReturnError(fmt.Errorf("GetDeviceByID: failed to get Device"))

			device, err := GetDevice(ctx, db, tt.idType, "test-device-2")
			if err.Error() != tt.err.Error() {
				t.Errorf("unexpected error: %s", err)
			}
			if diff := typed.Got(device).Want(tt.expectedDevice).Diff(); diff != "" {
				t.Errorf("unexpected diff: %s", diff)
			}
		})
	}
}

func TestListDevices(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("ListDevices", t, func(t *ftt.Test) {
		t.Run("ListDevices: valid return; page token returned", func(t *ftt.Test) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer func() {
				mock.ExpectClose()
				err = db.Close()
				if err != nil {
					t.Fatalf("failed to close db: %s", err)
				}
			}()

			var (
				pageSize int32 = 1
				timeNow        = time.Now()
			)

			createdTime, err := time.Parse("2006-01-02 15:04:05", "2024-01-01 12:00:00")
			assert.Loosely(t, err, should.BeNil)

			rows := sqlmock.NewRows([]string{
				"id",
				"device_address",
				"device_type",
				"device_state",
				"schedulable_labels",
				"created_time",
				"last_updated_time",
				"is_active"}).
				AddRow(
					"test-device-1",
					"1.1.1.1:1",
					"DEVICE_TYPE_PHYSICAL",
					"DEVICE_STATE_AVAILABLE",
					`{"label-test":{"Values":["test-value-1"]}}`,
					createdTime,
					timeNow,
					true).
				AddRow(
					"test-device-2",
					"2.2.2.2:2",
					"DEVICE_TYPE_VIRTUAL",
					"DEVICE_STATE_LEASED",
					`{"label-test":{"Values":["test-value-2"]}}`,
					createdTime,
					timeNow,
					false)

			mock.ExpectQuery(regexp.QuoteMeta(`
				SELECT
					id,
					device_address,
					device_type,
					device_state,
					schedulable_labels,
					created_time,
					last_updated_time,
					is_active
				FROM "Devices"
				ORDER BY created_time
				LIMIT $1;`)).
				WithArgs(pageSize + 1).
				WillReturnRows(rows)

			devices, err := ListDevices(ctx, db, &api.ListDevicesRequest{
				PageSize: pageSize,
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, devices, should.Match(&api.ListDevicesResponse{
				Devices: []*api.Device{
					{
						Id: "test-device-1",
						Address: &api.DeviceAddress{
							Host: "1.1.1.1",
							Port: 1,
						},
						Type:  api.DeviceType_DEVICE_TYPE_PHYSICAL,
						State: api.DeviceState_DEVICE_STATE_AVAILABLE,
						HardwareReqs: &api.HardwareRequirements{
							SchedulableLabels: map[string]*api.HardwareRequirements_LabelValues{
								"label-test": {
									Values: []string{"test-value-1"},
								},
							},
						},
					},
				},
				NextPageToken: "MjAyNC0wMS0wMVQxMjowMDowMFo=",
			}))

			decodedToken, err := database.DecodePageToken(ctx, database.PageToken(devices.GetNextPageToken()))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, decodedToken, should.Equal(createdTime.Format(time.RFC3339Nano)))
		})
		t.Run("ListDevices: valid return; no page token returned", func(t *ftt.Test) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer func() {
				mock.ExpectClose()
				err = db.Close()
				if err != nil {
					t.Fatalf("failed to close db: %s", err)
				}
			}()

			var (
				pageSize int32 = 2
				timeNow        = time.Now()
			)

			createdTime, err := time.Parse("2006-01-02 15:04:05", "2024-01-01 12:00:00")
			assert.Loosely(t, err, should.BeNil)

			rows := sqlmock.NewRows([]string{
				"id",
				"device_address",
				"device_type",
				"device_state",
				"schedulable_labels",
				"created_time",
				"last_updated_time",
				"is_active"}).
				AddRow(
					"test-device-1",
					"1.1.1.1:1",
					"DEVICE_TYPE_PHYSICAL",
					"DEVICE_STATE_AVAILABLE",
					`{"label-test":{"Values":["test-value-1"]}}`,
					createdTime,
					timeNow,
					true).
				AddRow(
					"test-device-2",
					"2.2.2.2:2",
					"DEVICE_TYPE_VIRTUAL",
					"DEVICE_STATE_LEASED",
					`{"label-test":{"Values":["test-value-2"]}}`,
					createdTime,
					timeNow,
					false)

			mock.ExpectQuery(regexp.QuoteMeta(`
				SELECT
					id,
					device_address,
					device_type,
					device_state,
					schedulable_labels,
					created_time,
					last_updated_time,
					is_active
				FROM "Devices"
				ORDER BY created_time
				LIMIT $1;`)).
				WithArgs(pageSize + 1).
				WillReturnRows(rows)

			devices, err := ListDevices(ctx, db, &api.ListDevicesRequest{
				PageSize: pageSize,
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, devices.GetNextPageToken(), should.BeEmpty)
			assert.Loosely(t, devices, should.Match(&api.ListDevicesResponse{
				Devices: []*api.Device{
					{
						Id: "test-device-1",
						Address: &api.DeviceAddress{
							Host: "1.1.1.1",
							Port: 1,
						},
						Type:  api.DeviceType_DEVICE_TYPE_PHYSICAL,
						State: api.DeviceState_DEVICE_STATE_AVAILABLE,
						HardwareReqs: &api.HardwareRequirements{
							SchedulableLabels: map[string]*api.HardwareRequirements_LabelValues{
								"label-test": {
									Values: []string{"test-value-1"},
								},
							},
						},
					},
					{
						Id: "test-device-2",
						Address: &api.DeviceAddress{
							Host: "2.2.2.2",
							Port: 2,
						},
						Type:  api.DeviceType_DEVICE_TYPE_VIRTUAL,
						State: api.DeviceState_DEVICE_STATE_LEASED,
						HardwareReqs: &api.HardwareRequirements{
							SchedulableLabels: map[string]*api.HardwareRequirements_LabelValues{
								"label-test": {
									Values: []string{"test-value-2"},
								},
							},
						},
					},
				},
			}))
		})
		t.Run("ListDevices: valid request using page token", func(t *ftt.Test) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer func() {
				mock.ExpectClose()
				err = db.Close()
				if err != nil {
					t.Fatalf("failed to close db: %s", err)
				}
			}()

			var (
				pageSize  int32 = 1
				pageToken       = "MjAyNC0wMS0wMVQxMjowMDowMFo="
				timeNow         = time.Now()
			)

			createdTime, err := time.Parse("2006-01-02 15:04:05", "2024-01-01 12:00:00")
			assert.Loosely(t, err, should.BeNil)

			// only add rows after test-device-1
			rows := sqlmock.NewRows([]string{
				"id",
				"device_address",
				"device_type",
				"device_state",
				"schedulable_labels",
				"created_time",
				"last_updated_time",
				"is_active"}).
				AddRow(
					"test-device-2",
					"2.2.2.2:2",
					"DEVICE_TYPE_VIRTUAL",
					"DEVICE_STATE_LEASED",
					`{"label-test":{"Values":["test-value-2"]}}`,
					createdTime,
					timeNow,
					false)

			mock.ExpectQuery(regexp.QuoteMeta(`
				SELECT
					id,
					device_address,
					device_type,
					device_state,
					schedulable_labels,
					created_time,
					last_updated_time,
					is_active
				FROM "Devices"
				WHERE created_time > $1
				ORDER BY created_time
				LIMIT $2;`)).
				WithArgs(createdTime.Format(time.RFC3339Nano), pageSize+1).
				WillReturnRows(rows)

			devices, err := ListDevices(ctx, db, &api.ListDevicesRequest{
				PageSize:  pageSize,
				PageToken: pageToken,
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, devices.GetNextPageToken(), should.BeEmpty)
			assert.Loosely(t, devices, should.Match(&api.ListDevicesResponse{
				Devices: []*api.Device{
					{
						Id: "test-device-2",
						Address: &api.DeviceAddress{
							Host: "2.2.2.2",
							Port: 2,
						},
						Type:  api.DeviceType_DEVICE_TYPE_VIRTUAL,
						State: api.DeviceState_DEVICE_STATE_LEASED,
						HardwareReqs: &api.HardwareRequirements{
							SchedulableLabels: map[string]*api.HardwareRequirements_LabelValues{
								"label-test": {
									Values: []string{"test-value-2"},
								},
							},
						},
					},
				},
			}))
		})
	})
}

func TestUpdateDevice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Set up fake PubSub server
	srv := pstest.NewServer()
	defer func() {
		err := srv.Close()
		if err != nil {
			t.Logf("failed to close fake pubsub server: %s", err)
		}
	}()

	conn, err := grpc.Dial(srv.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("could not start fake pubsub server")
	}
	defer func() {
		err = conn.Close()
		if err != nil {
			t.Logf("failed to close fake pubsub connection: %s", err)
		}
	}()

	psClient, err := pubsub.NewClient(ctx, "project", option.WithGRPCConn(conn))
	if err != nil {
		t.Fatalf("could not connect to fake pubsub server")
	}
	defer func() {
		err = psClient.Close()
		if err != nil {
			t.Logf("failed to close fake pubsub client: %s", err)
		}
	}()

	_, err = psClient.CreateTopic(ctx, external.DeviceEventsPubSubTopic)
	if err != nil {
		t.Fatalf("failed to create fake pubsub topic")
	}

	ftt.Run("UpdateDevice", t, func(t *ftt.Test) {
		t.Run("UpdateDevice: valid update", func(t *ftt.Test) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer func() {
				mock.ExpectClose()
				err = db.Close()
				if err != nil {
					t.Fatalf("failed to close db: %s", err)
				}
			}()

			mock.ExpectBegin()

			var txOpts *sql.TxOptions
			tx, err := db.BeginTx(ctx, txOpts)
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub db transaction", err)
			}

			timeNow := time.Now()
			rows := sqlmock.NewRows([]string{
				"id",
				"dut_id",
				"device_address",
				"device_type",
				"device_state",
				"schedulable_labels",
				"is_active",
				"created_time",
				"last_updated_time",
				"last_notification_time"}).
				AddRow(
					"test-device-1",
					"test-dut-id",
					"2.2.2.2:2",
					"DEVICE_TYPE_VIRTUAL",
					"DEVICE_STATE_LEASED",
					`{"dut_id":{"Values":["test-dut-id"]}}`,
					false,
					timeNow,
					timeNow,
					timeNow)

			labelBytes, err := json.Marshal(model.SchedulableLabels{
				"dut_id": model.LabelValues{
					Values: []string{"test-dut-id"},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, string(labelBytes), should.Match(`{"dut_id":{"Values":["test-dut-id"]}}`))

			mock.ExpectQuery(regexp.QuoteMeta(`
				UPDATE
					"Devices"
				SET
					dut_id=COALESCE(NULLIF($2, ''), device_address),
					device_address=COALESCE(NULLIF($3, ''), device_address),
					device_type=COALESCE(NULLIF($4, ''), device_type),
					device_state=COALESCE(NULLIF($5, ''), device_state),
					schedulable_labels=COALESCE($6::jsonb, schedulable_labels),
					last_updated_time=NOW(),
					is_active=COALESCE($7, is_active)
				WHERE
					id=$1
				RETURNING
					id,
					dut_id,
					device_address,
					device_type,
					device_state,
					schedulable_labels,
					is_active,
					created_time,
					last_updated_time,
					last_notification_time;`)).
				WithArgs(
					"test-device-1",
					"test-dut-id",
					"2.2.2.2:2",
					"DEVICE_TYPE_VIRTUAL",
					"DEVICE_STATE_LEASED",
					labelBytes,
					false).
				WillReturnRows(rows)

			err = UpdateDevice(ctx, tx, model.Device{
				ID:            "test-device-1",
				DutID:         "test-dut-id",
				DeviceAddress: "2.2.2.2:2",
				DeviceType:    "DEVICE_TYPE_VIRTUAL",
				DeviceState:   "DEVICE_STATE_LEASED",
				SchedulableLabels: model.SchedulableLabels{
					"dut_id": model.LabelValues{
						Values: []string{"test-dut-id"},
					},
				},
				IsActive: false,
			})
			assert.Loosely(t, err, should.BeNil)
		})
	})
}

func TestIsDeviceAvailable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("IsDeviceAvailable", t, func(t *ftt.Test) {
		t.Run("IsDeviceAvailable: device is available", func(t *ftt.Test) {
			rsp := IsDeviceAvailable(ctx, "DEVICE_STATE_AVAILABLE")
			assert.Loosely(t, rsp, should.Equal(true))
		})
		t.Run("IsDeviceAvailable: device is not available", func(t *ftt.Test) {
			rsp := IsDeviceAvailable(ctx, "DEVICE_STATE_LEASED")
			assert.Loosely(t, rsp, should.Equal(false))
		})
	})
}

func Test_stringToDeviceAddress(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("stringToDeviceAddress", t, func(t *ftt.Test) {
		t.Run("stringToDeviceAddress: valid address", func(t *ftt.Test) {
			addr, err := stringToDeviceAddress(ctx, "1.1.1.1:1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, addr, should.Match(&api.DeviceAddress{
				Host: "1.1.1.1",
				Port: 1,
			}))
		})
		t.Run("stringToDeviceAddress: invalid address; no port", func(t *ftt.Test) {
			addr, err := stringToDeviceAddress(ctx, "1.1.1.1.1.1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("failed to split host and port"))
			assert.Loosely(t, addr, should.Match(&api.DeviceAddress{}))
		})
		t.Run("stringToDeviceAddress: invalid address; bad port", func(t *ftt.Test) {
			addr, err := stringToDeviceAddress(ctx, "1.1.1.1:abc")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("port abc is not convertible to integer"))
			assert.Loosely(t, addr, should.Match(&api.DeviceAddress{}))
		})
	})
}

func Test_deviceAddressToString(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("deviceAddressToString", t, func(t *ftt.Test) {
		t.Run("deviceAddressToString: valid address", func(t *ftt.Test) {
			addr := deviceAddressToString(ctx, &api.DeviceAddress{
				Host: "1.1.1.1",
				Port: 1,
			})
			assert.Loosely(t, addr, should.Equal("1.1.1.1:1"))
		})
		t.Run("deviceAddressToString: ipv6 address", func(t *ftt.Test) {
			addr := deviceAddressToString(ctx, &api.DeviceAddress{
				Host: "1:2:3",
				Port: 1,
			})
			assert.Loosely(t, addr, should.Equal("[1:2:3]:1"))
		})
	})
}

func Test_stringToDeviceType(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("stringToDeviceType", t, func(t *ftt.Test) {
		t.Run("stringToDeviceType: valid types", func(t *ftt.Test) {
			for _, deviceType := range []string{
				"DEVICE_TYPE_UNSPECIFIED",
				"DEVICE_TYPE_VIRTUAL",
				"DEVICE_TYPE_PHYSICAL",
			} {
				apiType := stringToDeviceType(ctx, deviceType)
				assert.Loosely(t, apiType, should.Equal(api.DeviceType_value[deviceType]))
			}
		})
		t.Run("stringToDeviceType: unknown type", func(t *ftt.Test) {
			apiType := stringToDeviceType(ctx, "UNKNOWN_TYPE")
			assert.Loosely(t, apiType, should.Equal(api.DeviceType_DEVICE_TYPE_UNSPECIFIED))
		})
	})
}

func Test_stringToDeviceState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("stringToDeviceState", t, func(t *ftt.Test) {
		t.Run("stringToDeviceState: valid types", func(t *ftt.Test) {
			for _, deviceState := range []string{
				"DEVICE_STATE_UNSPECIFIED",
				"DEVICE_STATE_AVAILABLE",
				"DEVICE_STATE_LEASED",
			} {
				apiState := stringToDeviceState(ctx, deviceState)
				assert.Loosely(t, apiState, should.Equal(api.DeviceState_value[deviceState]))
			}
		})
		t.Run("stringToDeviceState: unknown state", func(t *ftt.Test) {
			apiState := stringToDeviceState(ctx, "UNKNOWN_STATE")
			assert.Loosely(t, apiState, should.Equal(api.DeviceState_DEVICE_STATE_UNSPECIFIED))
		})
	})
}

func Test_labelsToHardwareReqs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("labelsToHardwareReqs", t, func(t *ftt.Test) {
		t.Run("labelsToHardwareReqs: valid labels", func(t *ftt.Test) {
			labels := model.SchedulableLabels{
				"label-test": model.LabelValues{
					Values: []string{
						"test-value-1",
						"test-value-2",
					},
				},
			}
			dims := labelsToHardwareReqs(ctx, labels)
			assert.That(t, dims, should.Match(&api.HardwareRequirements{
				SchedulableLabels: map[string]*api.HardwareRequirements_LabelValues{
					"label-test": {
						Values: []string{
							"test-value-1",
							"test-value-2",
						},
					},
				},
			}))
		})
		t.Run("labelsToHardwareReqs: empty labels", func(t *ftt.Test) {
			labels := model.SchedulableLabels{}
			dims := labelsToHardwareReqs(ctx, labels)
			assert.That(t, dims, should.Match(&api.HardwareRequirements{
				SchedulableLabels: map[string]*api.HardwareRequirements_LabelValues{},
			}))
		})
	})
}

func Test_labelsToSwarmingDims(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("labelsToSwarmingDims", t, func(t *ftt.Test) {
		t.Run("labelsToSwarmingDims: valid labels", func(t *ftt.Test) {
			labels := model.SchedulableLabels{
				"label-test": model.LabelValues{
					Values: []string{
						"test-value-1",
						"test-value-2",
					},
				},
			}
			dims := labelsToSwarmingDims(ctx, labels)
			assert.That(t, dims, should.Match(&schedulingAPI.SwarmingDimensions{
				DimsMap: map[string]*schedulingAPI.DimValues{
					"label-test": {
						Values: []string{
							"test-value-1",
							"test-value-2",
						},
					},
				},
			}))
		})
		t.Run("labelsToSwarmingDims: empty labels", func(t *ftt.Test) {
			labels := model.SchedulableLabels{}
			dims := labelsToSwarmingDims(ctx, labels)
			assert.That(t, dims, should.Match(&schedulingAPI.SwarmingDimensions{
				DimsMap: map[string]*schedulingAPI.DimValues{},
			}))
		})
	})
}

func TestSwarmingDimsToLabels(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("SwarmingDimsToLabels", t, func(t *ftt.Test) {
		t.Run("SwarmingDimsToLabels: valid dims", func(t *ftt.Test) {
			dims := swarming.Dimensions{
				"label-test": []string{
					"test-value-1",
					"test-value-2",
				},
			}
			labels := SwarmingDimsToLabels(ctx, dims)
			assert.Loosely(t, labels, should.Match(model.SchedulableLabels{
				"label-test": model.LabelValues{
					Values: []string{
						"test-value-1",
						"test-value-2",
					},
				},
			}))
		})
		t.Run("SwarmingDimsToLabels: empty dims", func(t *ftt.Test) {
			dims := swarming.Dimensions{}
			labels := SwarmingDimsToLabels(ctx, dims)
			assert.Loosely(t, labels, should.Match(model.SchedulableLabels{}))
		})
	})
}

func Test_deviceModelToAPIDevice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("deviceModelToAPIDevice", t, func(t *ftt.Test) {
		t.Run("deviceModelToAPIDevice: valid device", func(t *ftt.Test) {
			modelDevice := model.Device{
				ID:            "test-device-1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				DeviceState:   "DEVICE_STATE_AVAILABLE",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
				},
			}

			apiDevice := deviceModelToAPIDevice(ctx, modelDevice)

			assert.Loosely(t, apiDevice, should.Match(&api.Device{
				Id: "test-device-1",
				Address: &api.DeviceAddress{
					Host: "1.1.1.1",
					Port: 1,
				},
				Type:  api.DeviceType_DEVICE_TYPE_PHYSICAL,
				State: api.DeviceState_DEVICE_STATE_AVAILABLE,
				HardwareReqs: &api.HardwareRequirements{
					SchedulableLabels: map[string]*api.HardwareRequirements_LabelValues{
						"label-test": {
							Values: []string{"test-value-1"},
						},
					},
				},
			}))
		})
		t.Run("deviceModelToAPIDevice: invalid fields", func(t *ftt.Test) {
			modelDevice := model.Device{
				ID:                "test-device-invalid",
				DeviceAddress:     "1.1",
				DeviceType:        "UNKNOWN",
				DeviceState:       "UNKNOWN",
				SchedulableLabels: model.SchedulableLabels{},
			}

			apiDevice := deviceModelToAPIDevice(ctx, modelDevice)

			assert.Loosely(t, apiDevice, should.Match(&api.Device{
				Id:      "test-device-invalid",
				Address: &api.DeviceAddress{},
				HardwareReqs: &api.HardwareRequirements{
					SchedulableLabels: map[string]*api.HardwareRequirements_LabelValues{},
				},
			}))
		})
		t.Run("deviceModelToAPIDevice: empty device", func(t *ftt.Test) {
			modelDevice := model.Device{
				ID: "test-device-empty",
			}

			apiDevice := deviceModelToAPIDevice(ctx, modelDevice)

			assert.Loosely(t, apiDevice, should.Match(&api.Device{
				Id:      "test-device-empty",
				Address: &api.DeviceAddress{},
				HardwareReqs: &api.HardwareRequirements{
					SchedulableLabels: map[string]*api.HardwareRequirements_LabelValues{},
				},
			}))
		})
	})
}

func TestExtractSingleValuedDimension(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("ExtractSingleValuedDimension", t, func(t *ftt.Test) {
		t.Run("pass: one dim", func(t *ftt.Test) {
			dims := map[string]*api.HardwareRequirements_LabelValues{
				"dut_id": {
					Values: []string{
						"test-id",
					},
				},
			}
			res, err := ExtractSingleValuedDimension(ctx, dims, "dut_id")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Equal("test-id"))
		})
		t.Run("fail: too many dims", func(t *ftt.Test) {
			dims := map[string]*api.HardwareRequirements_LabelValues{
				"dut_id": {
					Values: []string{
						"id-1",
						"id-2",
					},
				},
			}
			res, err := ExtractSingleValuedDimension(ctx, dims, "dut_id")
			assert.Loosely(t, err, should.ErrLike("ExtractSingleValuedDimension: multiple values for dimension dut_id"))
			assert.Loosely(t, res, should.BeEmpty)
		})
		t.Run("fail: empty dim", func(t *ftt.Test) {
			dims := map[string]*api.HardwareRequirements_LabelValues{
				"dut_id": {
					Values: []string{},
				},
			}
			res, err := ExtractSingleValuedDimension(ctx, dims, "dut_id")
			assert.Loosely(t, err, should.ErrLike("ExtractSingleValuedDimension: no value for dimension dut_id"))
			assert.Loosely(t, res, should.BeEmpty)
		})
	})
}
