// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dutinfo

import (
	"infra/cros/recovery/tlw"
	ufsdevice "infra/unifiedfleet/api/v1/models/chromeos/device"
	ufslab "infra/unifiedfleet/api/v1/models/chromeos/lab"
)

// TODO(otabek@): Use bidirectional maps when will be available.

var hardwareStates = map[ufslab.HardwareState]tlw.HardwareState{
	ufslab.HardwareState_HARDWARE_NORMAL:           tlw.HardwareState_HARDWARE_NORMAL,
	ufslab.HardwareState_HARDWARE_ACCEPTABLE:       tlw.HardwareState_HARDWARE_ACCEPTABLE,
	ufslab.HardwareState_HARDWARE_NEED_REPLACEMENT: tlw.HardwareState_HARDWARE_NEED_REPLACEMENT,
	ufslab.HardwareState_HARDWARE_NOT_DETECTED:     tlw.HardwareState_HARDWARE_NOT_DETECTED,
}

func convertHardwareState(s ufslab.HardwareState) tlw.HardwareState {
	if ns, ok := hardwareStates[s]; ok {
		return ns
	}
	return tlw.HardwareState_HARDWARE_UNSPECIFIED
}

func convertHardwareStateToUFS(s tlw.HardwareState) ufslab.HardwareState {
	for us, ls := range hardwareStates {
		if ls == s {
			return us
		}
	}
	return ufslab.HardwareState_HARDWARE_UNKNOWN
}

var firmwareChannels = map[ufslab.ServoFwChannel]tlw.ServoFwChannel{
	ufslab.ServoFwChannel_SERVO_FW_STABLE: tlw.ServoFwChannel_STABLE,
	ufslab.ServoFwChannel_SERVO_FW_ALPHA:  tlw.ServoFwChannel_ALPHA,
	ufslab.ServoFwChannel_SERVO_FW_DEV:    tlw.ServoFwChannel_DEV,
	ufslab.ServoFwChannel_SERVO_FW_PREV:   tlw.ServoFwChannel_PREV,
}

func convertFirmwareChannel(s ufslab.ServoFwChannel) tlw.ServoFwChannel {
	if ns, ok := firmwareChannels[s]; ok {
		return ns
	}
	return tlw.ServoFwChannel_STABLE
}

var storageTypes = map[ufsdevice.Config_Storage]tlw.Storage_Type{
	ufsdevice.Config_STORAGE_SSD:  tlw.Storage_SSD,
	ufsdevice.Config_STORAGE_HDD:  tlw.Storage_HDD,
	ufsdevice.Config_STORAGE_MMC:  tlw.Storage_MMC,
	ufsdevice.Config_STORAGE_NVME: tlw.Storage_NVME,
	ufsdevice.Config_STORAGE_UFS:  tlw.Storage_UFS,
}

func convertStorageType(t ufsdevice.Config_Storage) tlw.Storage_Type {
	if v, ok := storageTypes[t]; ok {
		return v
	}
	return tlw.Storage_TYPE_UNSPECIFIED
}

func convertAudioLoopbackState(s ufslab.PeripheralState) tlw.DUTAudio_LoopbackState {
	if s == ufslab.PeripheralState_WORKING {
		return tlw.DUTAudio_LOOPBACK_WORKING
	}
	return tlw.DUTAudio_LOOPBACK_UNSPECIFIED
}

var servoStates = map[ufslab.PeripheralState]tlw.ServoHost_State{
	ufslab.PeripheralState_WORKING:                       tlw.ServoHost_WORKING,
	ufslab.PeripheralState_MISSING_CONFIG:                tlw.ServoHost_MISSING_CONFIG,
	ufslab.PeripheralState_WRONG_CONFIG:                  tlw.ServoHost_WRONG_CONFIG,
	ufslab.PeripheralState_NOT_CONNECTED:                 tlw.ServoHost_NOT_CONNECTED,
	ufslab.PeripheralState_NO_SSH:                        tlw.ServoHost_NO_SSH,
	ufslab.PeripheralState_BROKEN:                        tlw.ServoHost_BROKEN,
	ufslab.PeripheralState_NEED_REPLACEMENT:              tlw.ServoHost_NEED_REPLACEMENT,
	ufslab.PeripheralState_CR50_CONSOLE_MISSING:          tlw.ServoHost_CR50_CONSOLE_MISSING,
	ufslab.PeripheralState_CCD_TESTLAB_ISSUE:             tlw.ServoHost_CCD_TESTLAB_ISSUE,
	ufslab.PeripheralState_SERVOD_ISSUE:                  tlw.ServoHost_SERVOD_ISSUE,
	ufslab.PeripheralState_LID_OPEN_FAILED:               tlw.ServoHost_LID_OPEN_FAILED,
	ufslab.PeripheralState_BAD_RIBBON_CABLE:              tlw.ServoHost_BAD_RIBBON_CABLE,
	ufslab.PeripheralState_EC_BROKEN:                     tlw.ServoHost_EC_BROKEN,
	ufslab.PeripheralState_DUT_NOT_CONNECTED:             tlw.ServoHost_DUT_NOT_CONNECTED,
	ufslab.PeripheralState_TOPOLOGY_ISSUE:                tlw.ServoHost_TOPOLOGY_ISSUE,
	ufslab.PeripheralState_SBU_LOW_VOLTAGE:               tlw.ServoHost_SBU_LOW_VOLTAGE,
	ufslab.PeripheralState_CR50_NOT_ENUMERATED:           tlw.ServoHost_CR50_NOT_ENUMERATED,
	ufslab.PeripheralState_SERVO_SERIAL_MISMATCH:         tlw.ServoHost_SERVO_SERIAL_MISMATCH,
	ufslab.PeripheralState_SERVOD_PROXY_ISSUE:            tlw.ServoHost_SERVOD_PROXY_ISSUE,
	ufslab.PeripheralState_SERVO_HOST_ISSUE:              tlw.ServoHost_SERVO_HOST_ISSUE,
	ufslab.PeripheralState_SERVO_UPDATER_ISSUE:           tlw.ServoHost_SERVO_UPDATER_ISSUE,
	ufslab.PeripheralState_SERVOD_DUT_CONTROLLER_MISSING: tlw.ServoHost_SERVOD_DUT_CONTROLLER_MISSING,
	ufslab.PeripheralState_COLD_RESET_PIN_ISSUE:          tlw.ServoHost_COLD_RESET_PIN_ISSUE,
	ufslab.PeripheralState_WARM_RESET_PIN_ISSUE:          tlw.ServoHost_WARM_RESET_PIN_ISSUE,
	ufslab.PeripheralState_POWER_BUTTON_PIN_ISSUE:        tlw.ServoHost_POWER_BUTTON_PIN_ISSUE,
	ufslab.PeripheralState_DEBUG_HEADER_SERVO_MISSING:    tlw.ServoHost_DEBUG_HEADER_SERVO_MISSING,
}

func convertServoState(s ufslab.PeripheralState) tlw.ServoHost_State {
	if ns, ok := servoStates[s]; ok {
		return ns
	}
	return tlw.ServoHost_STATE_UNSPECIFIED
}

var chameleonStates = map[ufslab.PeripheralState]tlw.Chameleon_State{
	ufslab.PeripheralState_WORKING:        tlw.Chameleon_WORKING,
	ufslab.PeripheralState_BROKEN:         tlw.Chameleon_BROKEN,
	ufslab.PeripheralState_NOT_APPLICABLE: tlw.Chameleon_NOT_APPLICABLE,
}

func convertChameleonState(s ufslab.PeripheralState) tlw.Chameleon_State {
	if ns, ok := chameleonStates[s]; ok {
		return ns
	}
	return tlw.Chameleon_STATE_UNSPECIFIED
}

var audioBoxJackPluggerStates = map[ufslab.Chameleon_AudioBoxJackPlugger]tlw.Chameleon_AudioBoxJackPluggerState{
	ufslab.Chameleon_AUDIOBOX_JACKPLUGGER_WORKING:        tlw.Chameleon_AUDIOBOX_JACKPLUGGER_WORKING,
	ufslab.Chameleon_AUDIOBOX_JACKPLUGGER_BROKEN:         tlw.Chameleon_AUDIOBOX_JACKPLUGGER_BROKEN,
	ufslab.Chameleon_AUDIOBOX_JACKPLUGGER_NOT_APPLICABLE: tlw.Chameleon_AUDIOBOX_JACKPLUGGER_NOT_APPLICABLE,
}

func convertAudioBoxJackPluggerState(s ufslab.Chameleon_AudioBoxJackPlugger) tlw.Chameleon_AudioBoxJackPluggerState {
	if ns, ok := audioBoxJackPluggerStates[s]; ok {
		return ns
	}
	return tlw.Chameleon_AUDIOBOX_JACKPLUGGER_UNSPECIFIED
}

func convertAudioBoxJackPluggerStateToUFS(s tlw.Chameleon_AudioBoxJackPluggerState) ufslab.Chameleon_AudioBoxJackPlugger {
	for us, ls := range audioBoxJackPluggerStates {
		if ls == s {
			return us
		}
	}
	return ufslab.Chameleon_AUDIOBOX_JACKPLUGGER_UNSPECIFIED
}

var hmrStates = map[ufslab.PeripheralState]tlw.HumanMotionRobot_State{
	ufslab.PeripheralState_WORKING:        tlw.HumanMotionRobot_WORKING,
	ufslab.PeripheralState_BROKEN:         tlw.HumanMotionRobot_BROKEN,
	ufslab.PeripheralState_NOT_APPLICABLE: tlw.HumanMotionRobot_NOT_APPLICABLE,
}

// converts HumanMotionRobot UFS state to TLW state
func convertHumanMotionRobotStateToTLW(s ufslab.PeripheralState) tlw.HumanMotionRobot_State {
	if ns, ok := hmrStates[s]; ok {
		return ns
	}
	return tlw.HumanMotionRobot_STATE_UNSPECIFIED
}

// converts HumanMotionRobot TLW state to UFS state
func convertHumanMotionRobotStateToUFS(ts tlw.HumanMotionRobot_State) ufslab.PeripheralState {
	for ufsState, tlwState := range hmrStates {
		if ts == tlwState {
			return ufsState
		}
	}
	return ufslab.PeripheralState_UNKNOWN
}

var bluetoothPeerStates = map[ufslab.PeripheralState]tlw.BluetoothPeer_State{
	ufslab.PeripheralState_WORKING: tlw.BluetoothPeer_WORKING,
	ufslab.PeripheralState_BROKEN:  tlw.BluetoothPeer_BROKEN,
}

func convertBluetoothPeerState(s ufslab.PeripheralState) tlw.BluetoothPeer_State {
	if ns, ok := bluetoothPeerStates[s]; ok {
		return ns
	}
	return tlw.BluetoothPeer_STATE_UNSPECIFIED
}

func convertBluetoothPeerStateToUFS(s tlw.BluetoothPeer_State) ufslab.PeripheralState {
	for ufsState, tlwState := range bluetoothPeerStates {
		if s == tlwState {
			return ufsState
		}
	}
	return ufslab.PeripheralState_UNKNOWN
}

// WifiRouterStates maps the router UFS state to TLW  state
// it is used to in convertWifiRouterState to convert ufs periperal state to tlw router state
var wifiRouterStates = map[ufslab.PeripheralState]tlw.WifiRouterHost_State{
	ufslab.PeripheralState_WORKING: tlw.WifiRouterHost_WORKING,
	ufslab.PeripheralState_BROKEN:  tlw.WifiRouterHost_BROKEN,
}

// converts WifiRouter UFS state to TLW state
func convertWifiRouterState(s ufslab.PeripheralState) tlw.WifiRouterHost_State {
	if ns, ok := wifiRouterStates[s]; ok {
		return ns
	}
	return tlw.WifiRouterHost_UNSPECIFIED
}

func convertWifiRouterStateToUFS(s tlw.WifiRouterHost_State) ufslab.PeripheralState {
	for us, ls := range wifiRouterStates {
		if ls == s {
			return us
		}
	}
	return ufslab.PeripheralState_UNKNOWN
}

// peripheralWifiStates maps the ufs peripheral state to tlw peripheral wifi state
var peripheralWifiStates = map[ufslab.PeripheralState]tlw.ChromeOS_PeripheralWifiState{
	ufslab.PeripheralState_WORKING:        tlw.ChromeOS_PERIPHERAL_WIFI_STATE_WORKING,
	ufslab.PeripheralState_BROKEN:         tlw.ChromeOS_PERIPHERAL_WIFI_STATE_BROKEN,
	ufslab.PeripheralState_NOT_APPLICABLE: tlw.ChromeOS_PERIPHERAL_WIFI_STATE_NOT_APPLICABLE,
}

// convert wifiRouterState UFS state to TLW peripheralWifiState
func convertPeripheralWifiState(s ufslab.PeripheralState) tlw.ChromeOS_PeripheralWifiState {
	if ns, ok := peripheralWifiStates[s]; ok {
		return ns
	}
	return tlw.ChromeOS_PERIPHERAL_WIFI_STATE_UNSPECIFIED
}

// convertPeripheralWifiState tlw state to UFS peripheral state
func convertPeripheralWifiStateToUFS(s tlw.ChromeOS_PeripheralWifiState) ufslab.PeripheralState {
	for us, ls := range peripheralWifiStates {
		if ls == s {
			return us
		}
	}
	return ufslab.PeripheralState_UNKNOWN
}

// audioLatencyToolkitStates maps the ufs peripheral state to tlw peripheral audio latency toolkit state
var audioLatencyToolkitStates = map[ufslab.PeripheralState]tlw.AudioLatencyToolkit_State{
	ufslab.PeripheralState_WORKING:        tlw.AudioLatencyToolkit_WORKING,
	ufslab.PeripheralState_BROKEN:         tlw.AudioLatencyToolkit_BROKEN,
	ufslab.PeripheralState_NOT_APPLICABLE: tlw.AudioLatencyToolkit_NOT_APPLICABLE,
}

// converts AudioLatencyToolkit UFS state to TLW state
func convertAudioLatencyToolkitStates(s ufslab.PeripheralState) tlw.AudioLatencyToolkit_State {
	if ns, ok := audioLatencyToolkitStates[s]; ok {
		return ns
	}
	return tlw.AudioLatencyToolkit_STATE_UNSPECIFIED
}

// converts AudioLatencyToolkit TLW state to UFS state
func convertAudioLatencyToolkitStatesToUFS(s tlw.AudioLatencyToolkit_State) ufslab.PeripheralState {
	for us, ls := range audioLatencyToolkitStates {
		if ls == s {
			return us
		}
	}
	return ufslab.PeripheralState_UNKNOWN
}

var amtManagerStates = map[ufslab.PeripheralState]tlw.AMTManager_State{
	ufslab.PeripheralState_WORKING:        tlw.AMTManager_WORKING,
	ufslab.PeripheralState_BROKEN:         tlw.AMTManager_BROKEN,
	ufslab.PeripheralState_NOT_APPLICABLE: tlw.AMTManager_NOT_APPLICABLE,
}

// converts AMTManager UFS state to TLW state
func convertAMTManagerStateToTLW(s ufslab.PeripheralState) tlw.AMTManager_State {
	if ns, ok := amtManagerStates[s]; ok {
		return ns
	}
	return tlw.AMTManager_STATE_UNSPECIFIED
}

// converts AMTManager TLW state to UFS state
func convertAMTManagerStateToUFS(ts tlw.AMTManager_State) ufslab.PeripheralState {
	for ufsState, tlwState := range amtManagerStates {
		if ts == tlwState {
			return ufsState
		}
	}
	return ufslab.PeripheralState_UNKNOWN
}

var rpmStates = map[ufslab.PeripheralState]tlw.RPMOutlet_State{
	ufslab.PeripheralState_WORKING:        tlw.RPMOutlet_WORKING,
	ufslab.PeripheralState_MISSING_CONFIG: tlw.RPMOutlet_MISSING_CONFIG,
	ufslab.PeripheralState_WRONG_CONFIG:   tlw.RPMOutlet_WRONG_CONFIG,
}

func convertRPMState(s ufslab.PeripheralState) tlw.RPMOutlet_State {
	if ns, ok := rpmStates[s]; ok {
		return ns
	}
	return tlw.RPMOutlet_UNSPECIFIED
}

var cr50Phases = map[ufslab.DutState_CR50Phase]tlw.ChromeOS_Cr50Phase{
	ufslab.DutState_CR50_PHASE_PREPVT: tlw.ChromeOS_CR50_PHASE_PREPVT,
	ufslab.DutState_CR50_PHASE_PVT:    tlw.ChromeOS_CR50_PHASE_PVT,
}

func convertCr50Phase(p ufslab.DutState_CR50Phase) tlw.ChromeOS_Cr50Phase {
	if p, ok := cr50Phases[p]; ok {
		return p
	}
	return tlw.ChromeOS_CR50_PHASE_UNSPECIFIED
}

var cr50KeyEnvs = map[ufslab.DutState_CR50KeyEnv]tlw.ChromeOS_Cr50KeyEnv{
	ufslab.DutState_CR50_KEYENV_PROD: tlw.ChromeOS_CR50_KEYENV_PROD,
	ufslab.DutState_CR50_KEYENV_DEV:  tlw.ChromeOS_CR50_KEYENV_DEV,
}

func convertCr50KeyEnv(p ufslab.DutState_CR50KeyEnv) tlw.ChromeOS_Cr50KeyEnv {
	if p, ok := cr50KeyEnvs[p]; ok {
		return p
	}
	return tlw.ChromeOS_CR50_KEYENV_UNSPECIFIED
}

func convertServoTopologyItemFromUFS(i *ufslab.ServoTopologyItem) *tlw.ServoTopologyItem {
	if i == nil {
		return nil
	}
	return &tlw.ServoTopologyItem{
		Type:         i.GetType(),
		SysfsProduct: i.GetSysfsProduct(),
		Serial:       i.GetSerial(),
		UsbHubPort:   i.GetUsbHubPort(),
		FwVersion:    i.GetFwVersion(),
	}
}

func convertServoTopologyFromUFS(st *ufslab.ServoTopology) *tlw.ServoTopology {
	var t *tlw.ServoTopology
	if st != nil {
		var children []*tlw.ServoTopologyItem
		for _, child := range st.GetChildren() {
			children = append(children, convertServoTopologyItemFromUFS(child))
		}
		t = &tlw.ServoTopology{
			Root:     convertServoTopologyItemFromUFS(st.Main),
			Children: children,
		}
	}
	return t
}

func convertServoTopologyItemToUFS(i *tlw.ServoTopologyItem) *ufslab.ServoTopologyItem {
	if i == nil {
		return nil
	}
	return &ufslab.ServoTopologyItem{
		Type:         i.Type,
		SysfsProduct: i.SysfsProduct,
		Serial:       i.Serial,
		UsbHubPort:   i.UsbHubPort,
		FwVersion:    i.FwVersion,
	}
}

func convertServoTopologyToUFS(st *tlw.ServoTopology) *ufslab.ServoTopology {
	var t *ufslab.ServoTopology
	if st != nil {
		var children []*ufslab.ServoTopologyItem
		for _, child := range st.Children {
			children = append(children, convertServoTopologyItemToUFS(child))
		}
		t = &ufslab.ServoTopology{
			Main:     convertServoTopologyItemToUFS(st.Root),
			Children: children,
		}
	}
	return t
}

var ufsRepairRequstsToTlw = map[ufslab.DutState_RepairRequest]tlw.RepairRequest{
	ufslab.DutState_REPAIR_REQUEST_PROVISION:           tlw.RepairRequest_REPAIR_REQUEST_PROVISION,
	ufslab.DutState_REPAIR_REQUEST_REIMAGE_BY_USBKEY:   tlw.RepairRequest_REPAIR_REQUEST_REIMAGE_BY_USBKEY,
	ufslab.DutState_REPAIR_REQUEST_UPDATE_USBKEY_IMAGE: tlw.RepairRequest_REPAIR_REQUEST_UPDATE_USBKEY_IMAGE,
	ufslab.DutState_REPAIR_REQUEST_REFLASH_FW:          tlw.RepairRequest_REPAIR_REQUEST_REFLASH_FW,
}
var tlwRepairRequestsToUFS = map[tlw.RepairRequest]ufslab.DutState_RepairRequest{
	tlw.RepairRequest_REPAIR_REQUEST_PROVISION:           ufslab.DutState_REPAIR_REQUEST_PROVISION,
	tlw.RepairRequest_REPAIR_REQUEST_REIMAGE_BY_USBKEY:   ufslab.DutState_REPAIR_REQUEST_REIMAGE_BY_USBKEY,
	tlw.RepairRequest_REPAIR_REQUEST_UPDATE_USBKEY_IMAGE: ufslab.DutState_REPAIR_REQUEST_UPDATE_USBKEY_IMAGE,
	tlw.RepairRequest_REPAIR_REQUEST_REFLASH_FW:          ufslab.DutState_REPAIR_REQUEST_REFLASH_FW,
}

func convertRepairRequestsFromUFS(s []ufslab.DutState_RepairRequest) []tlw.RepairRequest {
	var r []tlw.RepairRequest
	for _, rr := range s {
		if v, ok := ufsRepairRequstsToTlw[rr]; ok {
			r = append(r, v)
		}
	}
	return r
}
func convertRepairRequestsToUFS(requests []tlw.RepairRequest) []ufslab.DutState_RepairRequest {
	var r []ufslab.DutState_RepairRequest
	if len(requests) == 0 {
		return r
	}
	for _, rr := range requests {
		if v, ok := tlwRepairRequestsToUFS[rr]; ok {
			r = append(r, v)
		}
	}
	return r
}

// modemTypes maps the ufs modem types to TLW modem types
var modemTypes = map[ufslab.ModemType]tlw.Cellular_ModemType{
	ufslab.ModemType_MODEM_TYPE_UNSUPPORTED:     tlw.Cellular_MODEM_TYPE_UNSUPPORTED,
	ufslab.ModemType_MODEM_TYPE_QUALCOMM_SC7180: tlw.Cellular_MODEM_TYPE_QUALCOMM_SC7180,
	ufslab.ModemType_MODEM_TYPE_FIBOCOMM_L850GL: tlw.Cellular_MODEM_TYPE_FIBOCOMM_L850GL,
	ufslab.ModemType_MODEM_TYPE_NL668:           tlw.Cellular_MODEM_TYPE_NL668,
	ufslab.ModemType_MODEM_TYPE_FM350:           tlw.Cellular_MODEM_TYPE_FM350,
	ufslab.ModemType_MODEM_TYPE_FM101:           tlw.Cellular_MODEM_TYPE_FM101,
	ufslab.ModemType_MODEM_TYPE_QUALCOMM_SC7280: tlw.Cellular_MODEM_TYPE_QUALCOMM_SC7280,
	ufslab.ModemType_MODEM_TYPE_EM060:           tlw.Cellular_MODEM_TYPE_EM060,
	ufslab.ModemType_MODEM_TYPE_RW101:           tlw.Cellular_MODEM_TYPE_RW101,
	ufslab.ModemType_MODEM_TYPE_RW135:           tlw.Cellular_MODEM_TYPE_RW135,
	ufslab.ModemType_MODEM_TYPE_LCUK54:          tlw.Cellular_MODEM_TYPE_LCUK54,
}

// convertModemTypes converts UFS state to TLW modem types
func convertModemTypes(s ufslab.ModemType) tlw.Cellular_ModemType {
	if ns, ok := modemTypes[s]; ok {
		return ns
	}
	return tlw.Cellular_MODEM_TYPE_UNSPECIFIED
}

// convertModemTypeToUFS TLW modem types to UFS modem types
func convertModemTypeToUFS(s tlw.Cellular_ModemType) ufslab.ModemType {
	for us, ls := range modemTypes {
		if ls == s {
			return us
		}
	}
	return ufslab.ModemType_MODEM_TYPE_UNSPECIFIED
}

// simTypes maps the ufs SIM types to TLW SIM type
var simTypes = map[ufslab.SIMType]tlw.Cellular_SIMType{
	ufslab.SIMType_SIM_PHYSICAL: tlw.Cellular_SIM_PHYSICAL,
	ufslab.SIMType_SIM_DIGITAL:  tlw.Cellular_SIM_DIGITAL,
}

// convertSIMTypes converts UFS SIM types to TLW SIM types.
func convertSIMTypes(s ufslab.SIMType) tlw.Cellular_SIMType {
	if ns, ok := simTypes[s]; ok {
		return ns
	}
	return tlw.Cellular_SIM_UNSPECIFIED
}

// convertSIMTypeToUFS TLW modem types to UFS SIM types
func convertSIMypeToUFS(s tlw.Cellular_SIMType) ufslab.SIMType {
	for us, ls := range simTypes {
		if ls == s {
			return us
		}
	}
	return ufslab.SIMType_SIM_UNKNOWN
}

// convertSIMProviders converts UFS SIM providers to TLW SIM providers
func convertSIMProviders(s ufslab.NetworkProvider) tlw.Cellular_NetworkProvider {
	if ns, ok := tlw.Cellular_NetworkProvider_value[s.String()]; ok {
		return tlw.Cellular_NetworkProvider(ns)
	}
	return tlw.Cellular_NETWORK_UNSPECIFIED
}

// convertSIMProviderToUFS TLW SIM providers to UFS SIM providers
func convertSIMProviderToUFS(s tlw.Cellular_NetworkProvider) ufslab.NetworkProvider {
	if ns, ok := ufslab.NetworkProvider_value[s.String()]; ok {
		return ufslab.NetworkProvider(ns)
	}
	return ufslab.NetworkProvider_NETWORK_OTHER
}

// starfishStates maps the ufs peripheral states to tlw starfish states.
var starfishStates = map[ufslab.PeripheralState]tlw.Cellular_StarfishState{
	ufslab.PeripheralState_WORKING: tlw.Cellular_STARFISH_STATE_WORKING,
	ufslab.PeripheralState_BROKEN:  tlw.Cellular_STARFISH_STATE_BROKEN,
}

// convertStarfishStates converts UFS state to TLW starfish states.
func convertStarfishStates(s ufslab.PeripheralState) tlw.Cellular_StarfishState {
	if ns, ok := starfishStates[s]; ok {
		return ns
	}
	return tlw.Cellular_STARFISH_STATE_UNSPECIFIED
}

// convertStarfishStateToUFS TLW modem types to UFS starfish states.
func convertStarfishStateToUFS(s tlw.Cellular_StarfishState) ufslab.PeripheralState {
	for us, ls := range starfishStates {
		if ls == s {
			return us
		}
	}
	return ufslab.PeripheralState_UNKNOWN
}

// simonnectionStates maps the ufs connection states to tlw connection states.
var simStates = map[ufslab.SIMProfileInfo_State]tlw.Cellular_SIMProfileInfo_State{
	ufslab.SIMProfileInfo_BROKEN:       tlw.Cellular_SIMProfileInfo_BROKEN,
	ufslab.SIMProfileInfo_LOCKED:       tlw.Cellular_SIMProfileInfo_LOCKED,
	ufslab.SIMProfileInfo_NO_NETWORK:   tlw.Cellular_SIMProfileInfo_NO_NETWORK,
	ufslab.SIMProfileInfo_WORKING:      tlw.Cellular_SIMProfileInfo_WORKING,
	ufslab.SIMProfileInfo_WRONG_CONFIG: tlw.Cellular_SIMProfileInfo_WRONG_CONFIG,
}

// convertSIMState converts UFS connection states to TLW connection states.
func convertSIMState(s ufslab.SIMProfileInfo_State) tlw.Cellular_SIMProfileInfo_State {
	if ns, ok := simStates[s]; ok {
		return ns
	}
	return tlw.Cellular_SIMProfileInfo_UNSPECIFIED

}

// convertSIMStateToUFS TLW connection states types to UFS connection states.
func convertSIMStateToUFS(s tlw.Cellular_SIMProfileInfo_State) ufslab.SIMProfileInfo_State {
	for us, ls := range simStates {
		if ls == s {
			return us
		}
	}
	return ufslab.SIMProfileInfo_UNSPECIFIED
}

var dolosStates = map[ufslab.PeripheralState]tlw.Dolos_State{
	ufslab.PeripheralState_BROKEN:                        tlw.Dolos_DOLOS_UNKNOWN,
	ufslab.PeripheralState_DOLOS_NO_POWER_SUPPLIED:       tlw.Dolos_DOLOS_NO_POWER_SUPPLIED,
	ufslab.PeripheralState_DOLOS_OUTPUT_POWER_FAILED:     tlw.Dolos_DOLOS_OUTPUT_POWER_FAILED,
	ufslab.PeripheralState_DOLOS_BMS_STATE_INVALID:       tlw.Dolos_DOLOS_BMS_STATE_INVALID,
	ufslab.PeripheralState_DOLOS_SMBUS_COMM_NOT_DETECTED: tlw.Dolos_DOLOS_SMBUS_COMM_NOT_DETECTED,
	ufslab.PeripheralState_DOLOS_EEPROM_FAILURE:          tlw.Dolos_DOLOS_EEPROM_FAILURE,
	ufslab.PeripheralState_WORKING:                       tlw.Dolos_DOLOS_OK,
	ufslab.PeripheralState_DOLOS_NO_COMMUNICATION:        tlw.Dolos_DOLOS_NO_COMMUNICATION,
	ufslab.PeripheralState_DOLOS_NOT_PRESENT:             tlw.Dolos_DOLOS_NOT_PRESENT,
	ufslab.PeripheralState_NO_SSH:                        tlw.Dolos_NO_SSH,
}

func convertDolosState(s ufslab.PeripheralState) tlw.Dolos_State {
	if ds, ok := dolosStates[s]; ok {
		return ds
	}
	return tlw.Dolos_DOLOS_UNKNOWN
}

func convertDolosStateToUFS(ts tlw.Dolos_State) ufslab.PeripheralState {
	for ufsState, tlwState := range dolosStates {
		if ts == tlwState {
			return ufsState
		}
	}
	return ufslab.PeripheralState_UNKNOWN
}

var dolosHWMajorRevision = map[ufslab.DolosHWMajorRevision]tlw.Dolos_DolosHWMajorRevision{
	ufslab.DolosHWMajorRevision_DOLOS_V1: tlw.Dolos_DOLOS_V1,
}

func convertDolosHWMajorRevision(v ufslab.DolosHWMajorRevision) tlw.Dolos_DolosHWMajorRevision {
	if tv, ok := dolosHWMajorRevision[v]; ok {
		return tv
	}
	return tlw.Dolos_DOLOS_UNSPECIFIED
}
