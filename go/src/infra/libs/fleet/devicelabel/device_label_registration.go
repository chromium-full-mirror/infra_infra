// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicelabel

import (
	"go.chromium.org/infra/libs/fleet"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
)

type Registration struct {
	name          string
	schedulableID string
	source        fleet.DeviceLabel_SOURCE
	reasonToAdd   string
	owner         string

	// The func to set the value of this device label
	getValue applyValueFunc
}

type applyValueFunc func(*ufspb.ChromeOSDeviceData) (*fleet.SchedulableValue, error)

var labelRegs = []*Registration{
	{
		name:          "arc",
		schedulableID: "label-arc",
		source:        fleet.DeviceLabel_SOURCE_MANUAL_INPUT,
		reasonToAdd:   "Decided by model, from DT's manual input or HaRT if the former doesn't exist",
		getValue:      applyArc,
	},
	{
		name:          "audio_beamforming",
		schedulableID: "label-audio_beamforming",
		source:        fleet.DeviceLabel_SOURCE_AUTO_DETECTED,
		reasonToAdd:   "Not sure",
		getValue:      applyAudioBeamforming,
	},
	{
		name:          "audio_box",
		schedulableID: "label-audio_box",
		source:        fleet.DeviceLabel_SOURCE_MANUAL_INPUT,
		reasonToAdd:   "Not sure",
		getValue:      applyAudioBox,
	},
	{
		name:          "audio_cable",
		schedulableID: "label-audio_cable",
		source:        fleet.DeviceLabel_SOURCE_MANUAL_INPUT,
		reasonToAdd:   "Not sure",
		getValue:      applyAudioCable,
	},
	{
		name:          "audio_loopback_dongle",
		schedulableID: "label-audio_loopback_dongle",
		source:        fleet.DeviceLabel_SOURCE_AUTO_DETECTED,
		reasonToAdd:   "Not sure",
		getValue:      applyAudioLoopbackDongle,
	},
	{
		name:          "bluetooth_state",
		schedulableID: "label-bluetooth_state",
		source:        fleet.DeviceLabel_SOURCE_AUTO_DETECTED,
		reasonToAdd:   "The State of peripheral bluetooth",
		getValue:      applyBluetoothState,
	},
	{
		name:          "cellular_modem_state",
		schedulableID: "label-cellular_modem_state",
		source:        fleet.DeviceLabel_SOURCE_AUTO_DETECTED,
		reasonToAdd:   "The state of peripheral cellular modem ",
		getValue:      applyCellularModemState,
	},
	{
		name:          "chameleon_state",
		schedulableID: "label-chameleon_state",
		source:        fleet.DeviceLabel_SOURCE_AUTO_DETECTED,
		reasonToAdd:   "The state of peripheral chameleon",
		getValue:      applyChameleonState,
	},
	{
		name:          "cr50_phase",
		schedulableID: "label-cr50_phase",
		source:        fleet.DeviceLabel_SOURCE_AUTO_DETECTED,
		reasonToAdd:   "Indicating the cr50 phase",
		getValue:      applyCr50Phase,
	},
	{
		name:          "peripheral_wifi_state",
		schedulableID: "label-peripheral_wifi_state",
		source:        fleet.DeviceLabel_SOURCE_AUTO_DETECTED,
		reasonToAdd:   "The state of peripheral wifi",
		getValue:      applyPeripheralWifiState,
	},
	{
		name:          "servo",
		schedulableID: "label-servo",
		source:        fleet.DeviceLabel_SOURCE_AUTO_DETECTED,
		reasonToAdd:   "If servo exist or not",
		getValue:      applyServo,
	},
	{
		name:          "servo_state",
		schedulableID: "label-servo_state",
		source:        fleet.DeviceLabel_SOURCE_AUTO_DETECTED,
		reasonToAdd:   "The state of peripheral servo",
		getValue:      applyServoState,
	},
	{
		name:          "servo_usb_state",
		schedulableID: "label-servo_usb_state",
		source:        fleet.DeviceLabel_SOURCE_AUTO_DETECTED,
		reasonToAdd:   "The state of peripheral servo usb",
		getValue:      applyServoUSBState,
	},
	{
		name:          "working_bluetooth_btpeer",
		schedulableID: "label-working_bluetooth_btpeer",
		source:        fleet.DeviceLabel_SOURCE_AUTO_DETECTED,
		reasonToAdd:   "The number of working bluetooth btpeer",
		getValue:      applyWorkingBluetoothBTPeer,
	},
}
