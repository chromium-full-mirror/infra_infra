// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package jobs

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"infra/device_manager/internal/database"
	"infra/device_manager/internal/frontend"
	"infra/device_manager/internal/model"
	inventory "infra/libs/skylab/inventory"
	ufspb "infra/unifiedfleet/api/v1/models"
	ufschromeoslab "infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	ufsutil "infra/unifiedfleet/app/util"
)

func Test_isDeviceNeedsUpdate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tests := []struct {
		name      string
		oldDevice model.Device
		newDevice model.Device
		wantBool  bool
		panicMsg  string
	}{
		{
			name: "fail; different Device IDs",
			oldDevice: model.Device{
				ID: "d1",
			},
			newDevice: model.Device{
				ID: "d2",
			},
			wantBool: false,
			panicMsg: "comparing two different devices d1 and d2",
		},
		{
			name: "pass; no update on different DeviceAddress",
			oldDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive: true,
			},
			newDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "2.2.2.2:2",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive: true,
			},
			wantBool: false,
			panicMsg: "",
		},
		{
			name: "pass; no update on different DeviceType",
			oldDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive: true,
			},
			newDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_VIRTUAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive: true,
			},
			wantBool: false,
			panicMsg: "",
		},
		{
			name: "pass; different SchedulableLabels",
			oldDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive: true,
			},
			newDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-different-1"},
					},
				},
				IsActive: true,
			},
			wantBool: true,
			panicMsg: "",
		},
		{
			name: "pass; different IsActive",
			oldDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive: true,
			},
			newDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive: false,
			},
			wantBool: true,
			panicMsg: "",
		},
		{
			name: "pass; Device fields are the same",
			oldDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive: true,
			},
			newDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive: true,
			},
			wantBool: false,
			panicMsg: "",
		},
		{
			name: "pass; Device fields are the same and SchedulableLabels are in different order",
			oldDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
					"label-test-2": model.LabelValues{
						Values: []string{"test-value-2"},
					},
				},
				IsActive: true,
			},
			newDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test-2": model.LabelValues{
						Values: []string{"test-value-2"},
					},
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive: true,
			},
			wantBool: false,
			panicMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.panicMsg != "" && r == nil {
					t.Errorf("The code did not panic")
				} else if tt.panicMsg != "" && r != nil {
					if err, ok := r.(error); ok && !strings.Contains(err.Error(), tt.panicMsg) {
						t.Errorf("Wrong panic message: got %q, want %q", err.Error(), tt.panicMsg)
					}
				}
			}()

			t.Parallel()

			gotBool := areLabelsOrActiveStateDifferent(ctx, tt.oldDevice, tt.newDevice)

			if gotBool != tt.wantBool {
				t.Errorf("areLabelsOrActiveStateDifferent() gotBool = %v, wantBool %v", gotBool, tt.wantBool)
			}
		})
	}
}

func TestImportUFSDevices(t *testing.T) {
	t.Parallel()

	dutName := "dut1"
	tests := []struct {
		name    string
		ufs     *fakeUFSClient
		setupDB func(sqlmock.Sqlmock)
	}{
		{
			name: "import a dut",
			ufs: &fakeUFSClient{
				lses: []string{dutName},
				dd: map[string]*ufsAPI.GetDeviceDataResponse{
					dutName: {
						ResourceType: ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE,
						Resource: &ufsAPI.GetDeviceDataResponse_ChromeOsDeviceData{
							ChromeOsDeviceData: &ufspb.ChromeOSDeviceData{
								LabConfig: newMachineLSE(dutName),
								SchedulableLabels: map[string]*ufspb.SchedulableLabelValues{
									"dut_id": {LabelValues: []string{"C1111"}},
								},
								DutV1: &inventory.DeviceUnderTest{Common: &inventory.CommonDeviceSpecs{Id: &dutName}},
							},
						},
					},
				},
			},
			setupDB: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT (.+) FROM "Devices"`).WillReturnRows(sqlmock.NewRows(nil))
				m.ExpectQuery(`SELECT (.+) FROM "Devices"`).WillReturnError(sql.ErrNoRows)
				m.ExpectExec(`INSERT INTO "Devices" .+ DO UPDATE`).WillReturnResult(sqlmock.NewResult(1, 1))
				m.ExpectQuery(`SELECT (.+) FROM "Devices"`).WillReturnRows(newRows(dutName))
			},
		},
		{
			name: "a device is removed from UFS",
			ufs:  &fakeUFSClient{},
			setupDB: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT (.+) FROM "Devices"`).WillReturnRows(newRows("dut2"))
				m.ExpectQuery(`SELECT (.+) FROM "Devices"`).WillReturnRows(newRows("dut2"))
				m.ExpectExec(`INSERT INTO "Devices" .+ DO UPDATE`).WillReturnResult(sqlmock.NewResult(1, 1))
				m.ExpectQuery(`SELECT (.+) FROM "Devices"`).WillReturnRows(newRows("dut2"))
			},
		},
		{
			name: "import a scheduling unit",
			ufs: &fakeUFSClient{
				lses: []string{"dut3a", "dut3b"},
				sus: map[string]*ufspb.SchedulingUnit{
					"su1": {
						Name:        "su1",
						MachineLSEs: []string{"dut3a", "dut3b"},
					},
				},
				dd: map[string]*ufsAPI.GetDeviceDataResponse{
					"su1": {
						ResourceType: ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_SCHEDULING_UNIT,
						Resource: &ufsAPI.GetDeviceDataResponse_SchedulingUnit{
							SchedulingUnit: &ufspb.SchedulingUnit{Name: "su1"}},
					},
				},
			},
			setupDB: func(m sqlmock.Sqlmock) {
				m.MatchExpectationsInOrder(false) // We use goroutines to update DB.
				m.ExpectQuery(`SELECT (.+) FROM "Devices".+LIMIT`).WillReturnRows(newRows("dut3a", "dut3b"))
				m.ExpectQuery(`SELECT (.+) FROM "Devices".+WHERE`).WithArgs("su1").WillReturnError(sql.ErrNoRows)
				m.ExpectQuery(`SELECT (.+) FROM "Devices".+WHERE`).WithArgs("su1").WillReturnRows(newRows("su1"))
				m.ExpectQuery(`SELECT (.+) FROM "Devices".+WHERE`).WithArgs("dut3a").WillReturnRows(newRows("dut3a"))
				m.ExpectQuery(`SELECT (.+) FROM "Devices".+WHERE`).WithArgs("dut3a").WillReturnRows(newRows("dut3a"))
				m.ExpectQuery(`SELECT (.+) FROM "Devices".+WHERE`).WithArgs("dut3b").WillReturnRows(newRows("dut3b"))
				m.ExpectQuery(`SELECT (.+) FROM "Devices".+WHERE`).WithArgs("dut3b").WillReturnRows(newRows("dut3b"))
				// We need to update su1, dut3a, and dut3b, so there are 3 writes.
				m.ExpectExec(`INSERT INTO "Devices" .+ DO UPDATE`).WillReturnResult(sqlmock.NewResult(1, 1))
				m.ExpectExec(`INSERT INTO "Devices" .+ DO UPDATE`).WillReturnResult(sqlmock.NewResult(1, 1))
				m.ExpectExec(`INSERT INTO "Devices" .+ DO UPDATE`).WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer db.Close()

			tc.setupDB(mock)
			clients := frontend.ServiceClients{
				UFSClient: *tc.ufs,
				DBClient:  &database.Client{Conn: db},
			}

			ctx := logging.SetLevel(context.Background(), logging.Debug)
			ctx = gologger.StdConfig.Use(ctx)
			if err := ImportUFSDevices(ctx, clients, ""); err != nil {
				t.Errorf("importUFSDevices() = %s, want nil", err)
			}
			if err = mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unmet expectation error: %s", err)
			}
		})
	}
}

type fakeUFSClient struct {
	ufsAPI.FleetClient
	lses []string
	sus  map[string]*ufspb.SchedulingUnit
	dd   map[string]*ufsAPI.GetDeviceDataResponse
}

func (c fakeUFSClient) ListMachineLSEs(ctx context.Context, in *ufsAPI.ListMachineLSEsRequest, opts ...grpc.CallOption) (*ufsAPI.ListMachineLSEsResponse, error) {
	lses := make([]*ufspb.MachineLSE, len(c.lses))
	for i, name := range c.lses {
		lses[i] = newMachineLSE(name)
	}
	return &ufsAPI.ListMachineLSEsResponse{MachineLSEs: lses}, nil
}

func (c fakeUFSClient) GetMachineLSE(ctx context.Context, in *ufsAPI.GetMachineLSERequest, opts ...grpc.CallOption) (*ufspb.MachineLSE, error) {
	for _, name := range c.lses {
		if in.GetName() == ufsutil.AddPrefix(ufsutil.MachineLSECollection, name) {
			return newMachineLSE(name), nil
		}
	}
	return nil, fmt.Errorf("no machine LSE found for %s", in.GetName())
}

func (c fakeUFSClient) ListSchedulingUnits(ctx context.Context, in *ufsAPI.ListSchedulingUnitsRequest, opts ...grpc.CallOption) (*ufsAPI.ListSchedulingUnitsResponse, error) {
	sus := make([]*ufspb.SchedulingUnit, len(c.sus))
	i := 0
	for _, v := range c.sus {
		sus[i] = proto.Clone(v).(*ufspb.SchedulingUnit)
		i++
	}
	return &ufsAPI.ListSchedulingUnitsResponse{SchedulingUnits: sus}, nil
}

func (c fakeUFSClient) GetDeviceData(ctx context.Context, in *ufsAPI.GetDeviceDataRequest, opts ...grpc.CallOption) (*ufsAPI.GetDeviceDataResponse, error) {
	key := ""
	if id := in.GetDeviceId(); id != "" {
		key = id
	} else if name := in.GetHostname(); name != "" {
		key = name
	}
	if v, ok := c.dd[key]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("no device %v", in)
}

func newMachineLSE(name string) *ufspb.MachineLSE {
	return &ufspb.MachineLSE{
		Name:     name,
		Hostname: name,
		Machines: []string{name},
		Lse: &ufspb.MachineLSE_ChromeosMachineLse{
			ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
				ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
					DeviceLse: &ufspb.ChromeOSDeviceLSE{
						Device: &ufspb.ChromeOSDeviceLSE_Dut{
							Dut: &ufschromeoslab.DeviceUnderTest{
								Hostname: name,
							},
						},
					},
				},
			},
		},
		Zone:          "ZONE_CHROMEOS6",
		ResourceState: ufspb.State_STATE_REGISTERED,
		UpdateTime:    timestamppb.Now(),
	}
}

func newRows(dutNames ...string) *sqlmock.Rows {
	cols := []string{
		"id", "dut_id", "device_address", "device_type", "device_state", "schedulable_labels", "created_time", "last_updated_time", "is_active",
	}
	r := sqlmock.NewRows(cols)
	for _, n := range dutNames {
		vals := []driver.Value{n, fmt.Sprintf("C_%s", n), "", "DEVICE_TYPE_PHYSICAL", "DEVICE_STATE_AVAILABLE", fmt.Sprintf(`{"dut_id": {"Values": ["C_%s"]}}`, n), time.Now(), time.Now(), true}
		r.AddRow(vals...)
	}
	return r
}
