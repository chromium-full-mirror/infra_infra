// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package attacheddevice

import (
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/google/go-cmp/cmp"

	"go.chromium.org/infra/libs/skylab/inventory/swarming"
	ufsapi "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

const attachedDeviceDataProto = `
lab_config: {
  hostname: "dummy_hostname"
  attached_device_lse: {
    os_version: {
      value: "dummy_value"
      description: "dummy_description"
      image: "dummy_image"
    }
    associated_hostname: "dummy_associated_hostname"
  }
}
machine: {
  serial_number: "1234567890"
  attached_device: {
    manufacturer: "dummy_manufacturer"
    device_type: ATTACHED_DEVICE_TYPE_ANDROID_PHONE
    build_target: "dummy_board"
    model: "dummy_model"
  }
}
`

var expectedDimensions = swarming.Dimensions{
	"dut_id":                    {"dummy_hostname"},
	"dut_name":                  {"dummy_hostname"},
	"label-associated_hostname": {"dummy_associated_hostname"},
	"label-model":               {"dummy_model"},
	"label-board":               {"dummy_board"},
	"serial_number":             {"1234567890"},
	"label-device-stable":       {"True"},
}

func TestConvertAttachedDeviceData(t *testing.T) {
	t.Parallel()
	var data ufsapi.AttachedDeviceData
	if err := proto.UnmarshalText(attachedDeviceDataProto, &data); err != nil {
		t.Fatalf("Error unmarshalling example text: %s", err)
	}
	got := Convert(&data)
	if diff := cmp.Diff(expectedDimensions, got); diff != "" {
		t.Errorf("Empty attached device labels mismatch (-want +got):\n%s", diff)
	}
}
