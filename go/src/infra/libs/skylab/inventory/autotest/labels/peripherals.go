// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package labels

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"go.chromium.org/chromiumos/infra/proto/go/lab"

	"infra/libs/skylab/inventory"
)

func init() {
	converters = append(converters, boolPeripheralsConverter)
	converters = append(converters, otherPeripheralsConverter)

	reverters = append(reverters, boolPeripheralsReverter)
	reverters = append(reverters, otherPeripheralsReverter)
}

func boolPeripheralsConverter(ls *inventory.SchedulableLabels) []string {
	var labels []string
	p := ls.GetPeripherals()
	if p.GetAudioBoard() {
		labels = append(labels, "audio_board")
	}
	if p.GetAudioBox() {
		labels = append(labels, "audio_box")
	}
	if p.GetAudioCable() {
		labels = append(labels, "audio_cable")
	}
	if p.GetAudioLoopbackDongle() {
		labels = append(labels, "audio_loopback_dongle")
	}
	if p.GetCamerabox() {
		labels = append(labels, "camerabox")
	}
	if p.GetChameleon() {
		labels = append(labels, "chameleon")
	}
	if p.GetConductive() {
		// Special case
		labels = append(labels, "conductive:True")
	} else {
		labels = append(labels, "conductive:False")
	}
	if p.GetHuddly() {
		labels = append(labels, "huddly")
	}
	if p.GetMimo() {
		labels = append(labels, "mimo")
	}
	if p.GetServo() {
		labels = append(labels, "servo")
	}
	if p.GetSmartUsbhub() {
		labels = append(labels, "smart_usbhub")
	}
	if p.GetStylus() {
		labels = append(labels, "stylus")
	}
	if p.GetWificell() {
		labels = append(labels, "wificell")
	}
	if p.GetRouter_802_11Ax() {
		labels = append(labels, "router_802_11ax")
	}
	return labels
}

func otherPeripheralsConverter(ls *inventory.SchedulableLabels) []string {
	var labels []string
	p := ls.GetPeripherals()

	for _, v := range p.GetChameleonType() {
		const plen = 15 // len("CHAMELEON_TYPE_")
		lv := "chameleon:" + strings.ToLower(v.String()[plen:])
		labels = append(labels, lv)
	}
	if invSState := p.GetServoState(); invSState != inventory.PeripheralState_UNKNOWN {
		if labSState, ok := lab.PeripheralState_name[int32(invSState)]; ok {
			lv := "servo_state:" + labSState
			labels = append(labels, lv)
		}
	}

	if invCState := p.GetChameleonState(); invCState != inventory.PeripheralState_UNKNOWN {
		if labCState, ok := lab.PeripheralState_name[int32(invCState)]; ok {
			lv := "chameleon_state:" + labCState
			labels = append(labels, lv)
		}
	}

	if invJackPluggerState := p.GetAudioboxJackpluggerState(); invJackPluggerState != inventory.Peripherals_AUDIOBOX_JACKPLUGGER_UNSPECIFIED {
		labJackPluggerState := invJackPluggerState.String() // AUDIOBOX_JACKPLUGGER_{ UNSPECIFIED, WORKING ... }
		const plen = len("AUDIOBOX_JACKPLUGGER_")
		labels = append(labels, fmt.Sprintf("audiobox_jackplugger_state:%s", labJackPluggerState[plen:]))
	}

	if invTRRSType := p.GetTrrsType(); invTRRSType != inventory.Peripherals_TRRS_TYPE_UNSPECIFIED {
		labTRRSType := invTRRSType.String() // TRRS_TYPE_{ CTIA, OMTP, ... }
		const plen = len("TRRS_TYPE_")
		labels = append(labels, fmt.Sprintf("trrs_type:%s", labTRRSType[plen:]))
	}

	if invAudioLatencyToolkitState := p.GetAudioLatencyToolkitState(); invAudioLatencyToolkitState != inventory.PeripheralState_UNKNOWN {
		if labAudioLatencyToolkitState, ok := lab.PeripheralState_name[int32(invAudioLatencyToolkitState)]; ok {
			lv := "audio_latency_toolkit_state:" + labAudioLatencyToolkitState
			labels = append(labels, lv)
		}
	}

	if invHmrState := p.GetHmrState(); invHmrState != inventory.PeripheralState_UNKNOWN {
		if labHmrState, ok := lab.PeripheralState_name[int32(invHmrState)]; ok {
			lv := "hmr_state:" + labHmrState
			labels = append(labels, lv)
		}
	}

	if servoType := p.GetServoType(); servoType != "" {
		labels = append(labels, fmt.Sprintf("servo_type:%s", servoType))
	}

	if servoTopology := p.GetServoTopology(); servoTopology != nil {
		topologyJSONBytes, err := json.Marshal(servoTopology)
		if err == nil {
			topology := base64.StdEncoding.EncodeToString(topologyJSONBytes)
			labels = append(labels, fmt.Sprintf("servo_topology:%s", topology))
		}
	}

	if servoComponent := p.GetServoComponent(); servoComponent != nil {
		for _, v := range servoComponent {
			labels = append(labels, fmt.Sprintf("servo_component:%s", v))
		}
	}

	if rpmState := p.GetRpmState(); rpmState != inventory.PeripheralState_UNKNOWN {
		if labRpmState, ok := lab.PeripheralState_name[int32(rpmState)]; ok {
			lv := "rpm_state:" + labRpmState
			labels = append(labels, lv)
		}
	}

	if servoUSBState := p.GetServoUsbState(); servoUSBState != inventory.HardwareState_HARDWARE_UNKNOWN {
		if labState, ok := lab.HardwareState_name[int32(p.GetServoUsbState())]; ok {
			name := labState[len("HARDWARE_"):]
			labels = append(labels, "servo_usb_state:"+name)
		}
	}

	if storageState := p.GetStorageState(); storageState != inventory.HardwareState_HARDWARE_UNKNOWN {
		if labState, ok := lab.HardwareState_name[int32(storageState)]; ok {
			name := labState[len("HARDWARE_"):]
			labels = append(labels, "storage_state:"+name)
		}
	}

	if batteryState := p.GetBatteryState(); batteryState != inventory.HardwareState_HARDWARE_UNKNOWN {
		if labState, ok := lab.HardwareState_name[int32(batteryState)]; ok {
			name := labState[len("HARDWARE_"):]
			labels = append(labels, "battery_state:"+name)
		}
	}

	if wifiState := p.GetWifiState(); wifiState != inventory.HardwareState_HARDWARE_UNKNOWN {
		if labState, ok := lab.HardwareState_name[int32(wifiState)]; ok {
			name := labState[len("HARDWARE_"):]
			labels = append(labels, "wifi_state:"+name)
		}
	}

	if bluetoothState := p.GetBluetoothState(); bluetoothState != inventory.HardwareState_HARDWARE_UNKNOWN {
		if labState, ok := lab.HardwareState_name[int32(bluetoothState)]; ok {
			name := labState[len("HARDWARE_"):]
			labels = append(labels, "bluetooth_state:"+name)
		}
	}

	if modemState := p.GetCellularModemState(); modemState != inventory.HardwareState_HARDWARE_UNKNOWN {
		if labState, ok := lab.HardwareState_name[int32(modemState)]; ok {
			name := labState[len("HARDWARE_"):]
			labels = append(labels, "cellular_modem_state:"+name)
		}
	}

	if n := p.GetWorkingBluetoothBtpeer(); n > 0 {
		labels = append(labels, fmt.Sprintf("working_bluetooth_btpeer:%d", n))
	}

	if n := p.GetWorkingSims(); n > 0 {
		labels = append(labels, fmt.Sprintf("working_sims:%d", n))
	}

	if facing := p.GetCameraboxFacing(); facing != inventory.Peripherals_CAMERABOX_FACING_UNKNOWN {
		const plen = 17 // len("CAMERABOX_FACING_")
		lv := "camerabox_facing:" + strings.ToLower(facing.String()[plen:])
		labels = append(labels, lv)
	}

	if light := p.GetCameraboxLight(); light != inventory.Peripherals_CAMERABOX_LIGHT_UNKNOWN {
		const plen = 16 // len("CAMERABOX_LIGHT_")
		lv := "camerabox_light:" + strings.ToLower(light.String()[plen:])
		labels = append(labels, lv)
	}

	if peripheralBtpeerState := p.GetPeripheralWifiState(); peripheralBtpeerState != inventory.PeripheralState_UNKNOWN {
		if pbsState, ok := lab.PeripheralState_name[int32(peripheralBtpeerState)]; ok {
			labels = append(labels, "peripheral_btpeer_state:"+pbsState)
		}
	}

	if peripheralWifiState := p.GetPeripheralWifiState(); peripheralWifiState != inventory.PeripheralState_UNKNOWN {
		if pwsState, ok := lab.PeripheralState_name[int32(peripheralWifiState)]; ok {
			labels = append(labels, "peripheral_wifi_state:"+pwsState)
		}
	}

	for _, v := range p.GetWifiRouterFeatures() {
		labels = append(labels, "wifi_router_features:"+v.String())
	}

	for _, v := range p.GetWifiRouterModels() {
		labels = append(labels, "wifi_router_models:"+v)
	}

	return labels
}

func boolPeripheralsReverter(ls *inventory.SchedulableLabels, labels []string) []string {
	p := ls.GetPeripherals()
	for i := 0; i < len(labels); i++ {
		k, v := splitLabel(labels[i])
		switch k {
		case "audio_board":
			*p.AudioBoard = true
		case "audio_box":
			*p.AudioBox = true
		case "audio_cable":
			*p.AudioCable = true
		case "audio_loopback_dongle":
			*p.AudioLoopbackDongle = true
		case "camerabox":
			*p.Camerabox = true
		case "chameleon":
			if v != "" {
				continue
			}
			*p.Chameleon = true
		case "conductive":
			// Special case
			if v == "True" {
				*p.Conductive = true
			}
		case "huddly":
			*p.Huddly = true
		case "mimo":
			*p.Mimo = true
		case "servo":
			*p.Servo = true
		case "smart_usbhub":
			*p.SmartUsbhub = true
		case "stylus":
			*p.Stylus = true
		case "wificell":
			*p.Wificell = true
		case "router_802_11ax":
			*p.Router_802_11Ax = true
		default:
			continue
		}
		labels = removeLabel(labels, i)
		i--
	}
	return labels
}

func otherPeripheralsReverter(ls *inventory.SchedulableLabels, labels []string) []string {
	p := ls.GetPeripherals()
	for i := 0; i < len(labels); i++ {
		k, v := splitLabel(labels[i])
		switch k {
		case "chameleon":
			if v == "" {
				continue
			}
			vn := "CHAMELEON_TYPE_" + strings.ToUpper(v)
			type t = inventory.Peripherals_ChameleonType
			vals := inventory.Peripherals_ChameleonType_value
			p.ChameleonType = append(p.ChameleonType, t(vals[vn]))
		case "servo_state":
			if v == "" {
				continue
			}
			if labSStateVal, ok := lab.PeripheralState_value[strings.ToUpper(v)]; ok {
				servoState := inventory.PeripheralState(labSStateVal)
				p.ServoState = &servoState
			}
		case "chameleon_state":
			if v == "" {
				continue
			}
			if labCStateVal, ok := lab.PeripheralState_value[strings.ToUpper(v)]; ok {
				cState := inventory.PeripheralState(labCStateVal)
				p.ChameleonState = &cState
			}
		case "audiobox_jackplugger_state":
			if v == "" {
				continue
			}
			labJackPluggerState := "AUDIOBOX_JACKPLUGGER_" + v
			if invJackPluggerVal, ok := inventory.Peripherals_AudioBoxJackPlugger_value[labJackPluggerState]; ok {
				invJackPluggerState := inventory.Peripherals_AudioBoxJackPlugger(invJackPluggerVal)
				p.AudioboxJackpluggerState = &invJackPluggerState
			}
		case "trrs_type":
			if v == "" {
				continue
			}
			labTRRSType := "TRRS_TYPE_" + v
			if invTRRSVal, ok := inventory.Peripherals_TRRSType_value[labTRRSType]; ok {
				invTRRSType := inventory.Peripherals_TRRSType(invTRRSVal)
				p.TrrsType = &invTRRSType
			}
		case "audio_latency_toolkit_state":
			if v == "" {
				continue
			}
			if labAudioLatencyToolkitState, ok := lab.PeripheralState_value[strings.ToUpper(v)]; ok {
				audioLatencyToolkitState := inventory.PeripheralState(labAudioLatencyToolkitState)
				p.AudioLatencyToolkitState = &audioLatencyToolkitState
			}
		case "hmr_state":
			if v == "" {
				continue
			}
			if labHmrState, ok := lab.PeripheralState_value[strings.ToUpper(v)]; ok {
				hmrState := inventory.PeripheralState(labHmrState)
				p.HmrState = &hmrState
			}
		case "rpm_state":
			if labRpmState, ok := lab.PeripheralState_value[strings.ToUpper(v)]; ok {
				rpmState := inventory.PeripheralState(labRpmState)
				p.RpmState = &rpmState
			}
		case "storage_state":
			if labSStateVal, ok := lab.HardwareState_value["HARDWARE_"+strings.ToUpper(v)]; ok {
				state := inventory.HardwareState(labSStateVal)
				p.StorageState = &state
			}
		case "servo_usb_state":
			if labSStateVal, ok := lab.HardwareState_value["HARDWARE_"+strings.ToUpper(v)]; ok {
				state := inventory.HardwareState(labSStateVal)
				p.ServoUsbState = &state
			}
		case "battery_state":
			if labSStateVal, ok := lab.HardwareState_value["HARDWARE_"+strings.ToUpper(v)]; ok {
				state := inventory.HardwareState(labSStateVal)
				p.BatteryState = &state
			}
		case "wifi_state":
			if labSStateVal, ok := lab.HardwareState_value["HARDWARE_"+strings.ToUpper(v)]; ok {
				state := inventory.HardwareState(labSStateVal)
				p.WifiState = &state
			}
		case "bluetooth_state":
			if labSStateVal, ok := lab.HardwareState_value["HARDWARE_"+strings.ToUpper(v)]; ok {
				state := inventory.HardwareState(labSStateVal)
				p.BluetoothState = &state
			}
		case "cellular_modem_state":
			if labSStateVal, ok := lab.HardwareState_value["HARDWARE_"+strings.ToUpper(v)]; ok {
				state := inventory.HardwareState(labSStateVal)
				p.CellularModemState = &state
			}
		case "servo_type":
			p.ServoType = &v
		case "servo_topology":
			var topology *inventory.ServoTopology
			if v != "" {
				jsonBytes, err := base64.StdEncoding.DecodeString(v)
				if err == nil {
					topology = &inventory.ServoTopology{}
					json.Unmarshal(jsonBytes, topology)
				}
			}
			p.ServoTopology = topology
		case "servo_component":
			p.ServoComponent = append(p.ServoComponent, v)
		case "working_bluetooth_btpeer":
			i, err := strconv.Atoi(v)
			if err != nil {
				*p.WorkingBluetoothBtpeer = 0
			}
			*p.WorkingBluetoothBtpeer = int32(i)
		case "working_sims":
			i, err := strconv.Atoi(v)
			if err != nil {
				*p.WorkingSims = 0
			}
			*p.WorkingSims = int32(i)
		case "camerabox_facing":
			vn := "CAMERABOX_FACING_" + strings.ToUpper(v)
			if index, ok := inventory.Peripherals_CameraboxFacing_value[vn]; ok {
				facing := inventory.Peripherals_CameraboxFacing(index)
				p.CameraboxFacing = &facing
			}
		case "camerabox_light":
			vn := "CAMERABOX_LIGHT_" + strings.ToUpper(v)
			if index, ok := inventory.Peripherals_CameraboxLight_value[vn]; ok {
				light := inventory.Peripherals_CameraboxLight(index)
				p.CameraboxLight = &light
			}
		case "peripheral_btpeer_state":
			if stateValue, ok := lab.PeripheralState_value[strings.ToUpper(v)]; ok {
				state := inventory.PeripheralState(stateValue)
				p.PeripheralBtpeerState = &state
			}
		case "peripheral_wifi_state":
			if stateValue, ok := lab.PeripheralState_value[strings.ToUpper(v)]; ok {
				state := inventory.PeripheralState(stateValue)
				p.PeripheralWifiState = &state
			}
		case "wifi_router_features":
			if featureValue, ok := inventory.Peripherals_WifiRouterFeature_value[v]; ok {
				p.WifiRouterFeatures = append(p.GetWifiRouterFeatures(), inventory.Peripherals_WifiRouterFeature(featureValue))
			}
		case "wifi_router_models":
			p.WifiRouterModels = append(p.WifiRouterModels, v)
		default:
			continue
		}
		labels = removeLabel(labels, i)
		i--
	}
	return labels
}
