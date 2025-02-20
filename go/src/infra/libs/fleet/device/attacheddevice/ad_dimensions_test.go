// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package attacheddevice

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"go.chromium.org/infra/cros/dutstate"
	"go.chromium.org/infra/libs/skylab/inventory/swarming"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	lab "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

var devUFSState = lab.DutState{
	Id: &lab.ChromeOSDeviceID{
		Value: "test_dut",
	},
	Servo:                  lab.PeripheralState_BROKEN,
	WorkingBluetoothBtpeer: 3,
	WifiState:              lab.HardwareState_HARDWARE_ACCEPTABLE,
	RpmState:               lab.PeripheralState_WORKING,
}

var attachedDeviceData = ufsAPI.AttachedDeviceData{
	DutState: &devUFSState,
	LabConfig: &ufspb.MachineLSE{
		Hostname: "dummy_hostname",
		Lse: &ufspb.MachineLSE_AttachedDeviceLse{
			AttachedDeviceLse: &ufspb.AttachedDeviceLSE{
				OsVersion: &ufspb.OSVersion{
					Value:       "dummy_value",
					Description: "dummy_description",
					Image:       "dummy_image",
				},
				AssociatedHostname: "dummy_associated_hostname",
			},
		},
	},
	Machine: &ufspb.Machine{
		SerialNumber: "1234567890",
		Device: &ufspb.Machine_AttachedDevice{
			AttachedDevice: &ufspb.AttachedDevice{
				Manufacturer: "dummy_manufacturer",
				DeviceType:   ufspb.AttachedDeviceType_ATTACHED_DEVICE_TYPE_ANDROID_PHONE,
				BuildTarget:  "dummy_board",
				Model:        "dummy_model",
			},
		},
	},
}

var baseAttachedDeviceDims = swarming.Dimensions{
	"dut_id":                    {"dummy_hostname"},
	"dut_name":                  {"dummy_hostname"},
	"label-associated_hostname": {"dummy_associated_hostname"},
	"label-model":               {"dummy_model"},
	"label-board":               {"dummy_board"},
	"serial_number":             {"1234567890"},
	"label-device-stable":       {"True"},
}

var fullAttachedDeviceDims = swarming.Dimensions{
	"dut_id":                    {"dummy_hostname"},
	"dut_name":                  {"dummy_hostname"},
	"dut_state":                 {"ready"},
	"label-associated_hostname": {"dummy_associated_hostname"},
	"label-model":               {"dummy_model"},
	"label-board":               {"dummy_board"},
	"serial_number":             {"1234567890"},
	"label-device-stable":       {"True"},
}

func TestGetAttachedDeviceBotDimensions(t *testing.T) {
	ctx := context.Background()
	r := func(e error) { fmt.Printf("sanitize dimensions: %s\n", e) }
	tests := []struct {
		name         string
		dutState     dutstate.Info
		ufsData      *ufsAPI.AttachedDeviceData
		expectedDims swarming.Dimensions
	}{
		{
			name:         "empty DUT state",
			dutState:     dutstate.Info{},
			ufsData:      &attachedDeviceData,
			expectedDims: baseAttachedDeviceDims,
		},
		{
			name: "full Attached Device data",
			dutState: dutstate.Info{
				State: dutstate.Ready,
			},
			ufsData:      &attachedDeviceData,
			expectedDims: fullAttachedDeviceDims,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dims := GetAttachedDeviceBotDims(ctx, r, tt.dutState, tt.ufsData)
			if !reflect.DeepEqual(tt.expectedDims, dims) {
				for k, v := range tt.expectedDims {
					if !reflect.DeepEqual(v, dims[k]) {
						t.Errorf("Diff in dim %s; expected %v, got %v", k, v, dims[k])
					}
					if _, ok := dims[k]; !ok {
						t.Errorf("Missing dim %s; expected %v", k, v)
					}
				}
				for k, v := range dims {
					if !reflect.DeepEqual(v, tt.expectedDims[k]) {
						t.Errorf("Extra dim %s; expected none, got %v", k, v)
					}
				}
			}
		})
	}
}
