// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ufspb

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "infra/unifiedfleet/api/v1/models"
	chromeosLab "infra/unifiedfleet/api/v1/models/chromeos/lab"
)

func TestValidateHostnames(t *testing.T) {
	ftt.Run("ValidateHostnames", t, func(t *ftt.Test) {
		t.Run("Different hostnames - successful path", func(t *ftt.Test) {
			const h1, h2 = "h1", "h2"
			err := validateHostnames([]string{h1, h2}, "")
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Duplicated hostnames - returns error", func(t *ftt.Test) {
			const h1, h2 = "h1", "h1"
			err := validateHostnames([]string{h1, h2}, "")
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Empty hostname - returns error", func(t *ftt.Test) {
			const h1, h2 = "", "h1"
			err := validateHostnames([]string{h1, h2}, "")
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Nil input - successful path", func(t *ftt.Test) {
			err := validateHostnames(nil, "")
			assert.Loosely(t, err, should.BeNil)
		})
	})
}

func TestValidateUpdateDeviceRecoveryDataRequest(t *testing.T) {
	ftt.Run("ValidateDutId", t, func(t *ftt.Test) {
		t.Run("ChromeOS device - successful path", func(t *ftt.Test) {
			req := &UpdateDeviceRecoveryDataRequest{
				DeviceId:     "deviceId-1",
				ResourceType: UpdateDeviceRecoveryDataRequest_RESOURCE_TYPE_CHROMEOS_DEVICE,
				DeviceRecoveryData: &UpdateDeviceRecoveryDataRequest_Chromeos{
					Chromeos: &ChromeOsRecoveryData{
						DutState: &chromeosLab.DutState{
							Id: &chromeosLab.ChromeOSDeviceID{
								Value: "deviceId-1",
							},
						},
					},
				},
			}
			err := req.validateDutId()
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("ChromeOS device - empty device Id - returns error", func(t *ftt.Test) {
			req := &UpdateDeviceRecoveryDataRequest{
				DeviceId:     "",
				ResourceType: UpdateDeviceRecoveryDataRequest_RESOURCE_TYPE_CHROMEOS_DEVICE,
				DeviceRecoveryData: &UpdateDeviceRecoveryDataRequest_Chromeos{
					Chromeos: &ChromeOsRecoveryData{
						DutState: &chromeosLab.DutState{
							Id: &chromeosLab.ChromeOSDeviceID{
								Value: "deviceId-1",
							},
						},
					},
				},
			}
			err := req.validateDutId()
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("ChromeOS device - invalid device Id - returns error", func(t *ftt.Test) {
			req := &UpdateDeviceRecoveryDataRequest{
				DeviceId:     "",
				ResourceType: UpdateDeviceRecoveryDataRequest_RESOURCE_TYPE_CHROMEOS_DEVICE,
				DeviceRecoveryData: &UpdateDeviceRecoveryDataRequest_Chromeos{
					Chromeos: &ChromeOsRecoveryData{
						DutState: &chromeosLab.DutState{
							Id: &chromeosLab.ChromeOSDeviceID{
								Value: "deviceId-foo",
							},
						},
					},
				},
			}
			err := req.validateDutId()
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("ChromeOS device - missing dut state - returns error", func(t *ftt.Test) {
			req := &UpdateDeviceRecoveryDataRequest{
				DeviceId:     "",
				ResourceType: UpdateDeviceRecoveryDataRequest_RESOURCE_TYPE_CHROMEOS_DEVICE,
				DeviceRecoveryData: &UpdateDeviceRecoveryDataRequest_Chromeos{
					Chromeos: &ChromeOsRecoveryData{},
				},
			}
			err := req.validateDutId()
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("ChromeOS device - mismatching device and dut stats Ids - returns error", func(t *ftt.Test) {
			req := &UpdateDeviceRecoveryDataRequest{
				DeviceId:     "deviceId-1",
				ResourceType: UpdateDeviceRecoveryDataRequest_RESOURCE_TYPE_CHROMEOS_DEVICE,
				DeviceRecoveryData: &UpdateDeviceRecoveryDataRequest_Chromeos{
					Chromeos: &ChromeOsRecoveryData{
						DutState: &chromeosLab.DutState{
							Id: &chromeosLab.ChromeOSDeviceID{
								Value: "deviceId-2",
							},
						},
					},
				},
			}
			err := req.validateDutId()
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Attached device - successful path", func(t *ftt.Test) {
			req := &UpdateDeviceRecoveryDataRequest{
				DeviceId:     "deviceId-1",
				ResourceType: UpdateDeviceRecoveryDataRequest_RESOURCE_TYPE_ATTACHED_DEVICE,
			}
			err := req.validateDutId()
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Attached device - invalid device Id - returns error", func(t *ftt.Test) {
			req := &UpdateDeviceRecoveryDataRequest{
				DeviceId:     "deviceId-***",
				ResourceType: UpdateDeviceRecoveryDataRequest_RESOURCE_TYPE_ATTACHED_DEVICE,
			}
			err := req.validateDutId()
			assert.Loosely(t, err, should.NotBeNil)
		})
	})
}
func TestValidateUpdateTestDataRequest(t *testing.T) {
	ftt.Run("ValidateDutId", t, func(t *ftt.Test) {
		t.Run("ChromeOS device - successful path", func(t *ftt.Test) {
			req := &UpdateTestDataRequest{
				DeviceId: "deviceId-1",
				Hostname: "hostname_1",
				DeviceData: &UpdateTestDataRequest_ChromeosData{
					ChromeosData: &UpdateTestDataRequest_ChromeOs{
						DutState: &chromeosLab.DutState{
							Id: &chromeosLab.ChromeOSDeviceID{
								Value: "deviceId-2",
							},
						},
					},
				},
			}
			err := req.Validate()
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetDeviceId(), should.Equal(req.GetChromeosData().GetDutState().GetId().GetValue()))
			assert.Loosely(t, req.GetHostname(), should.Equal(req.GetChromeosData().GetDutState().GetHostname()))

		})
		t.Run("ChromeOS device - empty device Id - returns error", func(t *ftt.Test) {
			req := &UpdateTestDataRequest{
				DeviceId: "",
				Hostname: "",
			}
			err := req.Validate()
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("ChromeOS device - empty hostname - returns error", func(t *ftt.Test) {
			req := &UpdateTestDataRequest{
				DeviceId: "device-1",
				Hostname: "",
			}
			err := req.Validate()
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("ChromeOS device - missing dut state - returns error", func(t *ftt.Test) {
			req := &UpdateTestDataRequest{
				DeviceId: "device-1",
				Hostname: "hostname-1",
				DeviceData: &UpdateTestDataRequest_ChromeosData{
					ChromeosData: &UpdateTestDataRequest_ChromeOs{},
				},
			}
			err := req.Validate()
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Android device - successful path", func(t *ftt.Test) {
			req := &UpdateTestDataRequest{
				DeviceId: "device-1",
				Hostname: "hostname-1",
				DeviceData: &UpdateTestDataRequest_AndroidData{
					AndroidData: &UpdateTestDataRequest_Android{},
				},
			}
			err := req.Validate()
			assert.Loosely(t, err, should.BeNil)
		})
	})
}

func TestCreateAssetValidate(t *testing.T) {
	ftt.Run("CreateAssetRequest Validate", t, func(t *ftt.Test) {
		t.Run("Valid request - successful path", func(t *ftt.Test) {
			req := &CreateAssetRequest{
				Asset: &ufspb.Asset{
					Name: "assets/asset-1",
					Location: &ufspb.Location{
						Rack: "rack",
						Zone: ufspb.Zone_ZONE_CHROMEOS1,
					},
				},
			}
			err := req.Validate()
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Empty asset - returns error", func(t *ftt.Test) {
			req := &CreateAssetRequest{}
			err := req.Validate()
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Empty asset name - returns error", func(t *ftt.Test) {
			req := &CreateAssetRequest{
				Asset: &ufspb.Asset{
					Location: &ufspb.Location{
						Rack: "rack",
						Zone: ufspb.Zone_ZONE_CHROMEOS1,
					},
				},
			}
			err := req.Validate()
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Invalid asset name format - returns error", func(t *ftt.Test) {
			req := &CreateAssetRequest{
				Asset: &ufspb.Asset{
					Name: "asset-1",
					Location: &ufspb.Location{
						Rack: "rack",
						Zone: ufspb.Zone_ZONE_CHROMEOS1,
					},
				},
			}
			err := req.Validate()
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Invalid asset name characters - returns error", func(t *ftt.Test) {
			req := &CreateAssetRequest{
				Asset: &ufspb.Asset{
					Name: "assets/asset-@#%^&",
					Location: &ufspb.Location{
						Rack: "rack",
						Zone: ufspb.Zone_ZONE_CHROMEOS1,
					},
				},
			}
			err := req.Validate()
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Missing location - returns error", func(t *ftt.Test) {
			req := &CreateAssetRequest{
				Asset: &ufspb.Asset{
					Name: "assets/asset-1",
				},
			}
			err := req.Validate()
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Location zone unspecified - returns error", func(t *ftt.Test) {
			req := &CreateAssetRequest{
				Asset: &ufspb.Asset{
					Name: "assets/asset-1",
					Location: &ufspb.Location{
						Rack: "rack",
						Zone: ufspb.Zone_ZONE_UNSPECIFIED,
					},
				},
			}
			err := req.Validate()
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Location rack empty - returns error", func(t *ftt.Test) {
			req := &CreateAssetRequest{
				Asset: &ufspb.Asset{
					Name: "assets/asset-1",
					Location: &ufspb.Location{
						Rack: "",
						Zone: ufspb.Zone_ZONE_CHROMEOS1,
					},
				},
			}
			err := req.Validate()
			assert.Loosely(t, err, should.NotBeNil)
		})
	})
}
