// Copyright 2023 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package swarming

import (
	"fmt"
	"strconv"
	"strings"

	"go.chromium.org/chromiumos/infra/proto/go/lab"

	"go.chromium.org/infra/libs/skylab/inventory"
)

func init() {
	converters = append(converters, boolPeripheralsConverter)
	reverters = append(reverters, boolPeripheralsReverter)
	converters = append(converters, otherPeripheralsConverter)
	reverters = append(reverters, otherPeripheralsReverter)
}

// servoComponentAliases is standardized names for components that vary, but serve the same purpose.
var servoComponentAliases = map[string]string{
	"servo_micro": "debug",
	"c2d2":        "debug",
	"servo_v4":    "servo_pd",
	"servo_v4p1":  "servo_pd",
}

var nonRevertableServoComponents = map[string]bool{
	"debug":    true,
	"servo_pd": true,
}

func boolPeripheralsConverter(dims Dimensions, ls *inventory.SchedulableLabels) {
	p := ls.GetPeripherals()
	if p.GetAudioBoard() {
		dims["label-audio_board"] = []string{"True"}
	}
	if p.GetAudioBox() {
		dims["label-audio_box"] = []string{"True"}
	}
	if p.GetAudioCable() {
		dims["label-audio_cable"] = []string{"True"}
	}
	if p.GetAudioLoopbackDongle() {
		dims["label-audio_loopback_dongle"] = []string{"True"}
	}
	if p.GetCamerabox() {
		dims["label-camerabox"] = []string{"True"}
	}
	if p.GetChameleon() {
		dims["label-chameleon"] = []string{"True"}
	}
	if p.GetConductive() {
		dims["label-conductive"] = []string{"True"}
	}
	if p.GetHmrWalt() {
		dims["label-hmr_walt"] = []string{"True"}
	}
	if p.GetHuddly() {
		dims["label-huddly"] = []string{"True"}
	}
	if p.GetMimo() {
		dims["label-mimo"] = []string{"True"}
	}
	if p.GetServo() {
		dims["label-servo"] = []string{"True"}
	}
	if p.GetStylus() {
		dims["label-stylus"] = []string{"True"}
	}
	if p.GetWificell() {
		dims["label-wificell"] = []string{"True"}
	}
	if p.GetRouter_802_11Ax() {
		dims["label-router_802_11ax"] = []string{"True"}
	}
}

func boolPeripheralsReverter(ls *inventory.SchedulableLabels, d Dimensions) Dimensions {
	p := ls.Peripherals
	d = assignLastBoolValueAndDropKey(d, p.AudioBoard, "label-audio_board")
	d = assignLastBoolValueAndDropKey(d, p.AudioBox, "label-audio_box")
	d = assignLastBoolValueAndDropKey(d, p.AudioCable, "label-audio_cable")
	d = assignLastBoolValueAndDropKey(d, p.AudioLoopbackDongle, "label-audio_loopback_dongle")
	d = assignLastBoolValueAndDropKey(d, p.Camerabox, "label-camerabox")
	d = assignLastBoolValueAndDropKey(d, p.Chameleon, "label-chameleon")
	d = assignLastBoolValueAndDropKey(d, p.Conductive, "label-conductive")
	d = assignLastBoolValueAndDropKey(d, p.HmrWalt, "label-hmr_walt")
	d = assignLastBoolValueAndDropKey(d, p.Huddly, "label-huddly")
	d = assignLastBoolValueAndDropKey(d, p.Mimo, "label-mimo")
	d = assignLastBoolValueAndDropKey(d, p.Servo, "label-servo")
	d = assignLastBoolValueAndDropKey(d, p.Stylus, "label-stylus")
	d = assignLastBoolValueAndDropKey(d, p.Wificell, "label-wificell")
	d = assignLastBoolValueAndDropKey(d, p.Router_802_11Ax, "label-router_802_11ax")
	return d
}

func otherPeripheralsConverter(dims Dimensions, ls *inventory.SchedulableLabels) {
	p := ls.GetPeripherals()
	for _, v := range p.GetChameleonType() {
		appendDim(dims, "label-chameleon_type", v.String())
	}
	for _, v := range p.GetChameleonConnectionTypes() {
		appendDim(dims, "label-chameleon_connection_types", v.String())
	}

	if invSState := p.GetServoState(); invSState != inventory.PeripheralState_UNKNOWN {
		if labSState, ok := lab.PeripheralState_name[int32(invSState)]; ok {
			dims["label-servo_state"] = []string{labSState}
		}
	}

	if chamState := p.GetChameleonState(); chamState != inventory.PeripheralState_UNKNOWN {
		if labSState, ok := lab.PeripheralState_name[int32(chamState)]; ok {
			dims["label-chameleon_state"] = []string{labSState}
		}
	}

	if invJackPluggerState := p.GetAudioboxJackpluggerState(); invJackPluggerState != inventory.Peripherals_AUDIOBOX_JACKPLUGGER_UNSPECIFIED {
		labJackPluggerState := invJackPluggerState.String() // AUDIOBOX_JACKPLUGGER_{ UNSPECIFIED, WORKING, ... }
		const plen = len("AUDIOBOX_JACKPLUGGER_")
		dims["label-audiobox_jackplugger_state"] = []string{labJackPluggerState[plen:]}
	}

	if invTRRSType := p.GetTrrsType(); invTRRSType != inventory.Peripherals_TRRS_TYPE_UNSPECIFIED {
		labTRRSType := invTRRSType.String() // TRRS_TYPE_{ CTIA, OMTP, ... }
		const plen = len("TRRS_TYPE_")
		dims["label-trrs_type"] = []string{labTRRSType[plen:]}
	}

	if invAudioLatencyToolkitState := p.GetAudioLatencyToolkitState(); invAudioLatencyToolkitState != inventory.PeripheralState_UNKNOWN {
		if labAudioLatencyToolkitState, ok := lab.PeripheralState_name[int32(invAudioLatencyToolkitState)]; ok {
			dims["label-audio_latency_toolkit_state"] = []string{labAudioLatencyToolkitState}
		}
	}

	if hmrState := p.GetHmrState(); hmrState != inventory.PeripheralState_UNKNOWN {
		if labSState, ok := lab.PeripheralState_name[int32(hmrState)]; ok {
			dims["label-hmr_state"] = []string{labSState}
		}
	}

	if hmrToolType := p.GetHmrToolType(); hmrToolType != inventory.Peripherals_HMR_TOOL_TYPE_UNKNOWN {
		labHmrToolType := hmrToolType.String() // HMR_TOOL_TYPE_{ STYLUS, FAKE_FINGER, ... }
		const plen = len("HMR_TOOL_TYPE_")
		dims["label-hmr_tool_type"] = []string{labHmrToolType[plen:]}
	}

	if hmrGen := p.GetHmrGen(); hmrGen != inventory.Peripherals_HMR_GEN_UNKNOWN {
		labHmrGen := hmrGen.String() // HMR_GEN_{ 1, 2, ... }
		const plen = len("HMR_")     // ignore "HMR_" prefix to keep only GEN_[N]
		dims["label-hmr_gen"] = []string{labHmrGen[plen:]}
	}

	n := p.GetWorkingBluetoothBtpeer()
	btpeers := make([]string, n)
	for i := range btpeers {
		btpeers[i] = fmt.Sprint(i + 1)
	}
	// Empty dimensions may cause swarming page to fail to load: crbug.com/1056285
	if len(btpeers) > 0 {
		dims["label-working_bluetooth_btpeer"] = btpeers
	}

	sims := make([]string, p.GetWorkingSims())
	for i := range p.GetWorkingSims() {
		sims[i] = fmt.Sprint(i + 1)
	}
	if len(sims) > 0 {
		dims["label-working_sims"] = sims
	}

	if facing := p.GetCameraboxFacing(); facing != inventory.Peripherals_CAMERABOX_FACING_UNKNOWN {
		dims["label-camerabox_facing"] = []string{facing.String()}
	}

	if light := p.GetCameraboxLight(); light != inventory.Peripherals_CAMERABOX_LIGHT_UNKNOWN {
		dims["label-camerabox_light"] = []string{light.String()}
	}

	components := make(map[string]bool)
	for _, v := range p.GetServoComponent() {
		if components[v] {
			continue
		}
		components[v] = true
		appendDim(dims, "label-servo_component", v)
		if alias, ok := servoComponentAliases[v]; ok && !components[alias] {
			components[alias] = true
			appendDim(dims, "label-servo_component", alias)
		}
	}

	hardwareStatePrefixLength := len("HARDWARE_")
	if servoUSBState := p.GetServoUsbState(); servoUSBState != inventory.HardwareState_HARDWARE_UNKNOWN {
		if usbState, ok := lab.HardwareState_name[int32(p.GetServoUsbState())]; ok {
			appendDim(dims, "label-servo_usb_state", usbState[hardwareStatePrefixLength:])
		}
	}

	if wifiState := p.GetWifiState(); wifiState != inventory.HardwareState_HARDWARE_UNKNOWN {
		if wState, ok := lab.HardwareState_name[int32(wifiState)]; ok {
			appendDim(dims, "label-wifi_state", wState[hardwareStatePrefixLength:])
		}
	}

	if bluetoothState := p.GetBluetoothState(); bluetoothState != inventory.HardwareState_HARDWARE_UNKNOWN {
		if btState, ok := lab.HardwareState_name[int32(bluetoothState)]; ok {
			appendDim(dims, "label-bluetooth_state", btState[hardwareStatePrefixLength:])
		}
	}
	if modemState := p.GetCellularModemState(); modemState != inventory.HardwareState_HARDWARE_UNKNOWN {
		if state, ok := lab.HardwareState_name[int32(modemState)]; ok {
			appendDim(dims, "label-cellular_modem_state", state[hardwareStatePrefixLength:])
		}
	}
	if peripheralCellularState := p.GetSimState(); peripheralCellularState != inventory.PeripheralState_UNKNOWN {
		if state, ok := lab.PeripheralState_name[int32(peripheralCellularState)]; ok {
			dims["label-sim_state"] = []string{state}
		}
	}
	if starfishState := p.GetStarfishState(); starfishState != inventory.PeripheralState_UNKNOWN {
		if state, ok := lab.PeripheralState_name[int32(starfishState)]; ok {
			appendDim(dims, "label-starfish_state", state)
		}
	}
	if peripheralBtpeerState := p.GetPeripheralBtpeerState(); peripheralBtpeerState != inventory.PeripheralState_UNKNOWN {
		if pwsState, ok := lab.PeripheralState_name[int32(peripheralBtpeerState)]; ok {
			dims["label-peripheral_btpeer_state"] = []string{pwsState}
		}
	}
	if peripheralWifiState := p.GetPeripheralWifiState(); peripheralWifiState != inventory.PeripheralState_UNKNOWN {
		if pwsState, ok := lab.PeripheralState_name[int32(peripheralWifiState)]; ok {
			dims["label-peripheral_wifi_state"] = []string{pwsState}
		}
	}
	for _, v := range p.GetSimFeatures() {
		appendDim(dims, "label-sim_features", v.String())
	}
	for _, v := range p.GetWifiRouterFeatures() {
		appendDim(dims, "label-wifi_router_features", v.String())
	}
	for _, v := range p.GetWifiRouterModels() {
		appendDim(dims, "label-wifi_router_models", v)
	}

	for _, v := range p.GetPasitComponents() {
		if v != "" {
			appendDim(dims, "label-pasit_components", v)
		}
	}
	if amtManagerState := p.GetAmtManagerState(); amtManagerState != inventory.PeripheralState_UNKNOWN {
		if state, ok := lab.PeripheralState_name[int32(amtManagerState)]; ok {
			dims["label-amt_manager_state"] = []string{state}
		}
	}

	if audioBeamforming := p.GetAudioBeamforming(); audioBeamforming != "" {
		appendDim(dims, "label-audio_beamforming", audioBeamforming)
	}

	if cameraState := p.GetCameraState(); cameraState != inventory.HardwareState_HARDWARE_UNKNOWN {
		if state, ok := lab.HardwareState_name[int32(cameraState)]; ok {
			appendDim(dims, "label-camera_state", state[hardwareStatePrefixLength:])
		}
	}
}

func labToInvEnum[T ~int32](labValueStr string, invEnum map[string]int32) T {
	if invVal, ok := invEnum[labValueStr]; ok {
		return T(invVal)
	}
	return T(0) // default of enums is 0
}

func otherPeripheralsReverter(ls *inventory.SchedulableLabels, d Dimensions) Dimensions {
	p := ls.Peripherals

	p.ChameleonType = make([]inventory.Peripherals_ChameleonType, len(d["label-chameleon_type"]))
	for i, v := range d["label-chameleon_type"] {
		if ct, ok := inventory.Peripherals_ChameleonType_value[v]; ok {
			p.ChameleonType[i] = inventory.Peripherals_ChameleonType(ct)
		}
	}
	delete(d, "label-chameleon_type")

	p.ChameleonConnectionTypes = make([]inventory.Peripherals_ChameleonConnectionType, len(d["label-chameleon_connection_types"]))
	for i, v := range d["label-chameleon_connection_types"] {
		p.ChameleonConnectionTypes[i] = labToInvEnum[inventory.Peripherals_ChameleonConnectionType](v, inventory.Peripherals_ChameleonConnectionType_value)
	}
	delete(d, "label-chameleon_connection_types")

	if chamStateName, ok := getLastStringValue(d, "label-chameleon_state"); ok {
		chamState := inventory.PeripheralState_UNKNOWN
		if ssIndex, ok := lab.PeripheralState_value[strings.ToUpper(chamStateName)]; ok {
			chamState = inventory.PeripheralState(ssIndex)
		}
		p.ChameleonState = &chamState
		delete(d, "label-chameleon_state")
	}

	if labJackPluggerName, ok := getLastStringValue(d, "label-audiobox_jackplugger_state"); ok {
		labValueStr := "AUDIOBOX_JACKPLUGGER_" + labJackPluggerName
		invVal := labToInvEnum[inventory.Peripherals_AudioBoxJackPlugger](labValueStr, inventory.Peripherals_AudioBoxJackPlugger_value)
		p.AudioboxJackpluggerState = &invVal
		delete(d, "label-audiobox_jackplugger_state")
	}

	if labTRRSTypeName, ok := getLastStringValue(d, "label-trrs_type"); ok {
		labValueStr := "TRRS_TYPE_" + labTRRSTypeName
		invVal := labToInvEnum[inventory.Peripherals_TRRSType](labValueStr, inventory.Peripherals_TRRSType_value)
		p.TrrsType = &invVal
		delete(d, "label-trrs_type")
	}

	if labAudioLatencyToolkitStateName, ok := getLastStringValue(d, "label-audio_latency_toolkit_state"); ok {
		audioLatencyToolkitState := inventory.PeripheralState_UNKNOWN
		if audioLatencyToolkitStateIndex, ok := lab.PeripheralState_value[strings.ToUpper(labAudioLatencyToolkitStateName)]; ok {
			audioLatencyToolkitState = inventory.PeripheralState(audioLatencyToolkitStateIndex)
		}
		p.AudioLatencyToolkitState = &audioLatencyToolkitState
		delete(d, "label-audio_latency_toolkit_state")
	}

	if hmrStateName, ok := getLastStringValue(d, "label-hmr_state"); ok {
		hmrState := inventory.PeripheralState_UNKNOWN
		if ssIndex, ok := lab.PeripheralState_value[strings.ToUpper(hmrStateName)]; ok {
			hmrState = inventory.PeripheralState(ssIndex)
		}
		p.HmrState = &hmrState
		delete(d, "label-hmr_state")
	}

	if labHmrToolTypeName, ok := getLastStringValue(d, "label-hmr_tool_type"); ok {
		labValStr := "HMR_TOOL_TYPE_" + labHmrToolTypeName
		fmt.Printf("labValStr %s", labValStr)
		invVal := labToInvEnum[inventory.Peripherals_HMRToolType](labValStr, inventory.Peripherals_HMRToolType_value)
		p.HmrToolType = &invVal
		delete(d, "label-hmr_tool_type")
	}

	if labHmrGenName, ok := getLastStringValue(d, "label-hmr_gen"); ok {
		labValStr := "HMR_" + labHmrGenName
		invVal := labToInvEnum[inventory.Peripherals_HMRGen](labValStr, inventory.Peripherals_HMRGen_value)
		p.HmrGen = &invVal
		delete(d, "label-hmr_gen")
	}

	if labSStateName, ok := getLastStringValue(d, "label-servo_state"); ok {
		servoState := inventory.PeripheralState_UNKNOWN
		if ssIndex, ok := lab.PeripheralState_value[strings.ToUpper(labSStateName)]; ok {
			servoState = inventory.PeripheralState(ssIndex)
		}
		p.ServoState = &servoState
		delete(d, "label-servo_state")
	}

	if amtManagerStateName, ok := getLastStringValue(d, "label-amt_manager_state"); ok {
		amtState := inventory.PeripheralState_UNKNOWN
		if amtIndex, ok := lab.PeripheralState_value[strings.ToUpper(amtManagerStateName)]; ok {
			amtState = inventory.PeripheralState(amtIndex)
		}
		p.AmtManagerState = &amtState
		delete(d, "label-amt_manager_state")

	}

	btpeers := d["label-working_bluetooth_btpeer"]
	max := 0
	for _, v := range btpeers {
		if i, err := strconv.Atoi(v); err == nil && i > max {
			max = i
		}
	}
	*p.WorkingBluetoothBtpeer = int32(max)
	delete(d, "label-working_bluetooth_btpeer")

	max = 0
	for _, v := range d["label-working_sims"] {
		if i, err := strconv.Atoi(v); err == nil && i > max {
			max = i
		}
	}
	*p.WorkingSims = int32(max)
	delete(d, "label-working_sims")

	if facingName, ok := getLastStringValue(d, "label-camerabox_facing"); ok {
		if index, ok := inventory.Peripherals_CameraboxFacing_value[strings.ToUpper(facingName)]; ok {
			facing := inventory.Peripherals_CameraboxFacing(index)
			p.CameraboxFacing = &facing
		}
		delete(d, "label-camerabox_facing")
	}

	if lightName, ok := getLastStringValue(d, "label-camerabox_light"); ok {
		if index, ok := inventory.Peripherals_CameraboxLight_value[strings.ToUpper(lightName)]; ok {
			light := inventory.Peripherals_CameraboxLight(index)
			p.CameraboxLight = &light
		}
		delete(d, "label-camerabox_light")
	}

	for _, v := range d["label-servo_component"] {
		if !nonRevertableServoComponents[v] {
			p.ServoComponent = append(p.ServoComponent, v)
		}
	}
	delete(d, "label-servo_component")

	if servoUSBState, ok := getLastStringValue(d, "label-servo_usb_state"); ok {
		if labSStateVal, ok := lab.HardwareState_value["HARDWARE_"+strings.ToUpper(servoUSBState)]; ok {
			state := inventory.HardwareState(labSStateVal)
			p.ServoUsbState = &state
		}
		delete(d, "label-servo_usb_state")
	}

	if wifiState, ok := getLastStringValue(d, "label-wifi_state"); ok {
		if labSStateVal, ok := lab.HardwareState_value["HARDWARE_"+strings.ToUpper(wifiState)]; ok {
			state := inventory.HardwareState(labSStateVal)
			p.WifiState = &state
		}
		delete(d, "label-wifi_state")
	}
	if bluetoothState, ok := getLastStringValue(d, "label-bluetooth_state"); ok {
		if labSStateVal, ok := lab.HardwareState_value["HARDWARE_"+strings.ToUpper(bluetoothState)]; ok {
			state := inventory.HardwareState(labSStateVal)
			p.BluetoothState = &state
		}
		delete(d, "label-bluetooth_state")
	}
	if modemState, ok := getLastStringValue(d, "label-cellular_modem_state"); ok {
		if labSStateVal, ok := lab.HardwareState_value["HARDWARE_"+strings.ToUpper(modemState)]; ok {
			state := inventory.HardwareState(labSStateVal)
			p.CellularModemState = &state
		}
		delete(d, "label-cellular_modem_state")
	}
	if starfishState, ok := getLastStringValue(d, "label-starfish_state"); ok {
		if sIndex, ok := lab.PeripheralState_value[strings.ToUpper(starfishState)]; ok {
			state := inventory.PeripheralState(sIndex)
			p.StarfishState = &state
		}
		delete(d, "label-starfish_state")
	}
	if pbsStateName, ok := getLastStringValue(d, "label-peripheral_btpeer_state"); ok {
		pbsState := inventory.PeripheralState_UNKNOWN
		if sIndex, ok := lab.PeripheralState_value[strings.ToUpper(pbsStateName)]; ok {
			pbsState = inventory.PeripheralState(sIndex)
		}
		p.PeripheralBtpeerState = &pbsState
		delete(d, "label-peripheral_btpeer_state")
	}
	if stateName, ok := getLastStringValue(d, "label-sim_state"); ok {
		state := inventory.PeripheralState_UNKNOWN
		if sIndex, ok := lab.PeripheralState_value[strings.ToUpper(stateName)]; ok {
			state = inventory.PeripheralState(sIndex)
		}
		p.SimState = &state
		delete(d, "label-sim_state")
	}
	if pwsStateName, ok := getLastStringValue(d, "label-peripheral_wifi_state"); ok {
		pwsState := inventory.PeripheralState_UNKNOWN
		if sIndex, ok := lab.PeripheralState_value[strings.ToUpper(pwsStateName)]; ok {
			pwsState = inventory.PeripheralState(sIndex)
		}
		p.PeripheralWifiState = &pwsState
		delete(d, "label-peripheral_wifi_state")
	}

	p.WifiRouterFeatures = make([]inventory.Peripherals_WifiRouterFeature, len(d["label-wifi_router_features"]))
	for i, v := range d["label-wifi_router_features"] {
		int32Value, ok := inventory.Peripherals_WifiRouterFeature_value[v]
		if !ok {
			// Could an int if the infra enum copy is out of sync, so try to parse it.
			intValue, err := strconv.Atoi(v)
			if err != nil {
				intValue = int(inventory.Peripherals_WIFI_ROUTER_FEATURE_INVALID.Number())
			}
			int32Value = int32(intValue)
		}
		p.WifiRouterFeatures[i] = inventory.Peripherals_WifiRouterFeature(int32Value)
	}
	delete(d, "label-wifi_router_features")

	p.SimFeatures = make([]inventory.Peripherals_SIMFeature, len(d["label-sim_features"]))
	for i, v := range d["label-sim_features"] {
		int32Value, ok := inventory.Peripherals_SIMFeature_value[v]
		if !ok {
			// Could an int if the infra enum copy is out of sync, so try to parse it.
			intValue, err := strconv.Atoi(v)
			if err != nil {
				intValue = int(inventory.Peripherals_SIM_FEATURE_UNSPECIFIED.Number())
			}
			int32Value = int32(intValue)
		}
		p.SimFeatures[i] = inventory.Peripherals_SIMFeature(int32Value)
	}
	delete(d, "label-sim_features")

	p.WifiRouterModels = make([]string, len(d["label-wifi_router_models"]))
	copy(p.WifiRouterModels, d["label-wifi_router_models"])
	delete(d, "label-wifi_router_models")

	p.PasitComponents = make([]string, len(d["label-pasit_components"]))
	copy(p.PasitComponents, d["label-pasit_components"])
	delete(d, "label-pasit_components")

	if audioBeamforming, ok := getLastStringValue(d, "label-audio_beamforming"); ok {
		p.AudioBeamforming = &audioBeamforming
		delete(d, "label-audio_beamforming")
	}

	if cameraState, ok := getLastStringValue(d, "label-camera_state"); ok {
		if labSStateVal, ok := lab.HardwareState_value["HARDWARE_"+strings.ToUpper(cameraState)]; ok {
			state := inventory.HardwareState(labSStateVal)
			p.CameraState = &state
		}
		delete(d, "label-camera_state")
	}

	return d
}
