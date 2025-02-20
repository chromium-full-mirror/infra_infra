// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicelabel

import (
	"testing"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/libs/fleet"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufslabconfigpb "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
)

// TestConvert tests that Convert can successfully converts a UFS entry to a label-based representation.
func TestConvert(t *testing.T) {
	t.Parallel()

	device, err := ConvertChromeOS(fakeChromeOSDataForConvert)
	assert.Loosely(t, err, should.BeNil)
	assert.Loosely(t, device, should.NotBeNil)
	assert.Loosely(t, len(device.GetDeviceLabels()), should.Equal(5))
	for _, l := range device.GetDeviceLabels() {
		verifyForConvert(t, l.GetSchedulableId(), l.GetSchedulableValue())
	}
}

func verifyForConvert(t *testing.T, k string, v *fleet.SchedulableValue) {
	assert.Loosely(t, v.GetSwarmingLabels(), should.Match(expectedLabelsForConvert[k]))
}

var fakeChromeOSDataForConvert = &ufspb.ChromeOSDeviceData{
	Machine: &ufspb.Machine{
		Name: "fake-machine",
		Device: &ufspb.Machine_ChromeosMachine{
			ChromeosMachine: &ufspb.ChromeOSMachine{
				BuildTarget: "fizz-labstation",
			},
		},
	},
	LabConfig: &ufspb.MachineLSE{
		Lse: &ufspb.MachineLSE_ChromeosMachineLse{
			ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
				ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
					DeviceLse: &ufspb.ChromeOSDeviceLSE{
						Device: &ufspb.ChromeOSDeviceLSE_Dut{
							Dut: &ufslabconfigpb.DeviceUnderTest{
								Hostname: "fake-host",
								Peripherals: &ufslabconfigpb.Peripherals{
									Audio: &ufslabconfigpb.Audio{
										AudioBox:   false,
										AudioCable: true,
									},
								},
							},
						},
					},
				},
			},
		},
	},
	DutState: &ufslabconfigpb.DutState{
		AudioBeamforming:    "fake-beaming",
		AudioLoopbackDongle: ufslabconfigpb.PeripheralState_BAD_RIBBON_CABLE,
	},
}

var expectedLabelsForConvert = map[string][]string{
	"label-arc":                   {"False"},
	"label-audio_beamforming":     {"fake-beaming"},
	"label-audio_box":             {"False"},
	"label-audio_cable":           {"True"},
	"label-audio_loopback_dongle": {"True"},
}
