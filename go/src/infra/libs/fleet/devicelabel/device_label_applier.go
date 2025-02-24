// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicelabel

import (
	"fmt"

	"go.chromium.org/infra/libs/fleet"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
)

var noArcBoardMap = map[string]bool{
	"fizz-labstation":  true,
	"guado_labstation": true,
}

func applyArc(data *ufspb.ChromeOSDeviceData) (*fleet.SchedulableValue, error) {
	_, ok := noArcBoardMap[data.GetMachine().GetChromeosMachine().GetBuildTarget()]
	arc := !ok
	return &fleet.SchedulableValue{
		SwarmingLabels: []string{boolToSwarmingString(arc)},
	}, nil
}

func applyAudioBeamforming(data *ufspb.ChromeOSDeviceData) (*fleet.SchedulableValue, error) {
	s := data.GetDutState()
	return &fleet.SchedulableValue{
		SwarmingLabels: []string{s.GetAudioBeamforming()},
	}, nil
}

func applyAudioBox(data *ufspb.ChromeOSDeviceData) (*fleet.SchedulableValue, error) {
	p := data.GetLabConfig().GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals()
	return &fleet.SchedulableValue{
		SwarmingLabels: []string{boolToSwarmingString(p.GetAudio().GetAudioBox())},
	}, nil
}

func applyAudioCable(data *ufspb.ChromeOSDeviceData) (*fleet.SchedulableValue, error) {
	p := data.GetLabConfig().GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals()
	return &fleet.SchedulableValue{
		SwarmingLabels: []string{boolToSwarmingString(p.GetAudio().GetAudioCable())},
	}, nil
}

func applyAudioLoopbackDongle(data *ufspb.ChromeOSDeviceData) (*fleet.SchedulableValue, error) {
	s := data.GetDutState()
	return &fleet.SchedulableValue{
		SwarmingLabels: []string{peripheralStateToBoolString(s.GetAudioLoopbackDongle())},
	}, nil
}

func applyBluetoothState(data *ufspb.ChromeOSDeviceData) (*fleet.SchedulableValue, error) {
	s := data.GetDutState()
	return &fleet.SchedulableValue{
		SwarmingLabels: []string{hardwareStateToSwarmingString(s.GetBluetoothState())},
	}, nil
}

func applyCellularModemState(data *ufspb.ChromeOSDeviceData) (*fleet.SchedulableValue, error) {
	s := data.GetDutState()
	return &fleet.SchedulableValue{
		SwarmingLabels: []string{hardwareStateToSwarmingString(s.GetCellularModemState())},
	}, nil
}

func applyChameleonState(data *ufspb.ChromeOSDeviceData) (*fleet.SchedulableValue, error) {
	s := data.GetDutState()
	return &fleet.SchedulableValue{
		SwarmingLabels: []string{s.GetChameleon().String()},
	}, nil
}

func applyCr50Phase(data *ufspb.ChromeOSDeviceData) (*fleet.SchedulableValue, error) {
	s := data.GetDutState()
	return &fleet.SchedulableValue{
		SwarmingLabels: []string{s.GetCr50Phase().String()},
	}, nil
}

func applyPeripheralWifiState(data *ufspb.ChromeOSDeviceData) (*fleet.SchedulableValue, error) {
	s := data.GetDutState()
	return &fleet.SchedulableValue{
		SwarmingLabels: []string{s.GetWifiPeripheralState().String()},
	}, nil
}

func applyServo(data *ufspb.ChromeOSDeviceData) (*fleet.SchedulableValue, error) {
	s := data.GetDutState()
	return &fleet.SchedulableValue{
		SwarmingLabels: []string{peripheralStateToBoolString(s.GetServo())},
	}, nil
}

func applyServoState(data *ufspb.ChromeOSDeviceData) (*fleet.SchedulableValue, error) {
	s := data.GetDutState()
	return &fleet.SchedulableValue{
		SwarmingLabels: []string{s.GetServo().String()},
	}, nil
}

func applyServoUSBState(data *ufspb.ChromeOSDeviceData) (*fleet.SchedulableValue, error) {
	s := data.GetDutState()
	return &fleet.SchedulableValue{
		SwarmingLabels: []string{hardwareStateToSwarmingString(s.GetServoUsbState())},
	}, nil
}

func applyWorkingBluetoothBTPeer(data *ufspb.ChromeOSDeviceData) (*fleet.SchedulableValue, error) {
	s := data.GetDutState()
	return &fleet.SchedulableValue{
		SwarmingLabels: []string{fmt.Sprint(s.GetWorkingBluetoothBtpeer())},
	}, nil
}
