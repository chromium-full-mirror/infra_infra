// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fakeufsdata contains fake UFS data to be used in tests.
//
// Comments adorning fake responses should contain enough information to
// indicate:
// 1) the UFS RPC that they are faking.
// 2) the resource that they describe.
package fakeufsdata

import (
	models "infra/unifiedfleet/api/v1/models"
	lab "infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
)

// FakeOctopusDUTHostname is the hostname of a fake octopus DUT.
const FakeOctopusDUTHostname = "fake-octopus-dut-1"

// FakeOctopusDUTDeviceDataResponse is a fake octopus DUT with hostname "fake-octopus-dut-1".
//
// It is very useful in tests.
var FakeOctopusDUTDeviceDataResponse = &ufsAPI.GetDeviceDataResponse{
	Resource: &ufsAPI.GetDeviceDataResponse_ChromeOsDeviceData{
		ChromeOsDeviceData: &models.ChromeOSDeviceData{
			LabConfig: &models.MachineLSE{
				Lse: &models.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &models.ChromeOSMachineLSE{
						ChromeosLse: &models.ChromeOSMachineLSE_DeviceLse{
							DeviceLse: &models.ChromeOSDeviceLSE{
								Device: &models.ChromeOSDeviceLSE_Dut{
									Dut: &lab.DeviceUnderTest{
										Hostname: FakeOctopusDUTHostname,
									},
								},
							},
						},
					},
				},
			},
			Machine: &models.Machine{
				Device: &models.Machine_ChromeosMachine{
					ChromeosMachine: &models.ChromeOSMachine{
						BuildTarget: "build-target",
						Model:       "model",
					},
				},
			},
		},
	},
	ResourceType: ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE,
}

var FakeOctopusDUTWithServoDeviceDataResponse = &ufsAPI.GetDeviceDataResponse{
	Resource: &ufsAPI.GetDeviceDataResponse_ChromeOsDeviceData{
		ChromeOsDeviceData: &models.ChromeOSDeviceData{
			LabConfig: &models.MachineLSE{
				Lse: &models.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &models.ChromeOSMachineLSE{
						ChromeosLse: &models.ChromeOSMachineLSE_DeviceLse{
							DeviceLse: &models.ChromeOSDeviceLSE{
								Device: &models.ChromeOSDeviceLSE_Dut{
									Dut: &lab.DeviceUnderTest{
										Hostname: FakeOctopusDUTHostname,
										Peripherals: &lab.Peripherals{
											Servo: &lab.Servo{
												ServoHostname: "fake-labstation",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			Machine: &models.Machine{
				Device: &models.Machine_ChromeosMachine{
					ChromeosMachine: &models.ChromeOSMachine{
						BuildTarget: "build-target",
						Model:       "model",
					},
				},
			},
		},
	},
	ResourceType: ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE,
}
