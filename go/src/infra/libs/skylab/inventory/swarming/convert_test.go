// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package swarming

import (
	"fmt"
	"testing"

	"github.com/golang/protobuf/proto"

	"go.chromium.org/luci/common/testing/typed"

	"infra/libs/skylab/inventory"
)

const fullTextProto = `
variant: "somevariant"
test_coverage_hints {
  usb_detect: true
  use_lid: true
  test_usbprinting: true
  test_usbaudio: true
  test_hdmiaudio: true
  test_audiojack: true
  recovery_test: true
  meet_app: true
  hangout_app: true
  chromesign: true
  chaos_nightly: true
  chaos_dut: true
}
self_serve_pools: "poolval"
stability: true
reference_design: "reef"
wifi_chip: "wireless_xxxx"
wifi_on_site: true
hwid_component: [
	"cellular/fake_cellular"
]
platform: "platformval"
phase: 8
peripherals: {
  wificell: true
  stylus: true
  servo: true
  servo_component: ["servo_v4", "ccd_cr50"]
  servo_state: 1
  servo_type: ""
  servo_usb_state: 3
  wifi_state: 2
  bluetooth_state: 3
  cellular_modem_state: 3
  sim_state: 4
  working_sims: 2
  starfish_state: 3
  smart_usbhub: false
  mimo: true
  huddly: true
  conductive: true
  chameleon_type: 9
  chameleon_connection_types: 3
  chameleon_connection_types: 4
  chameleon: true
  chameleon_state: 1
  audiobox_jackplugger_state: 1
  camerabox: true
  camerabox_facing: 1
  camerabox_light: 1
  audio_loopback_dongle: true
  audio_cable: true
  audio_box: true
  audio_board: true
  trrs_type: 1
  audio_latency_toolkit_state: 1
  router_802_11ax: true
  working_bluetooth_btpeer: 3
  peripheral_btpeer_state: 1
  peripheral_wifi_state: 1
  wifi_router_features: [2,3,4,5,999]
  sim_features: [0,1,2]
  wifi_router_models: ["gale","OPENWRT[Ubiquiti_Unifi_6_Lite]"]
  hmr_state: 1
  pasit_components: "CAMERA-1"
  pasit_components: "MONITOR-1"
  pasit_components: "MONITOR-2"
  pasit_components: "DOCKING_STATION-1"
  amt_manager_state: 1
  audio_beamforming: "intelligo"
  camera_state: 2
}
os_type: 2
model: "modelval"
sku: "skuval"
hwid_sku: "eve_IntelR_CoreTM_i7_7Y75_CPU_1_30GHz_16GB"
dlm_sku_id: "12345"
brand: "HOMH"
ec_type: 1
cr50_ro_keyid: "a"
cr50_ro_version: "11.12.13"
cr50_rw_keyid: "b"
cr50_rw_version: "21.22.23"
cr50_phase: 2
cts_cpu: 1
cts_cpu: 2
cts_abi: 1
cts_abi: 2
critical_pools: 1
critical_pools: 2
capabilities {
  webcam: true
  video_acceleration: 6
  video_acceleration: 8
  touchpad: true
  touchscreen: true
  telephony: "telephonyval"
  storage: "storageval"
  starfish_slot_mapping: "1_verizon,2_tmobile,4_att"
  power: "powerval"
  modem: "modemval"
  lucidsleep: true
  hotwording: true
  internal_display: true
  graphics: "graphicsval"
  gpu_family: "gpufamilyval"
  gpu_id: "gpuidval"
  flashrom: true
  fingerprint: true
  fingerprint_board: "fingerprintboardval"
  fingerprint_mcu: "fingerprintmcuval"
  fingerprint_sensor: "fingerprintsensorval"
  detachablebase: true
  carrier: 2
  supported_carriers: 2
  supported_carriers: 4
  supported_carriers: 5
  bluetooth: true
  atrus: true
  cbx: 0
  cbx_branding: 0
  form_factor: 1
}
board: "boardval"
bot_size: 1
arc: true
callbox: true
licenses: {
  type: 2
  identifier: ""
}
licenses: {
  type: 1
  identifier: ""
}
modeminfo: {
  type: 1
  imei: "imei"
  supported_bands: "bands"
	sim_count: 1
  model_variant: "somecellularvariant"
}
siminfo: [{
	slot_id: 1
	type: 1
	eid: "eid"
	test_esim: true
	profile_info: {
		iccid: "iccid"
		sim_pin: "pin"
		sim_puk: "puk"
		carrier_name: 1
		own_number: "1234567890"
	}
},
{
	slot_id: 2
	type: 2
	eid: "eid2"
	test_esim: false
	profile_info: [{
		iccid: "iccid2"
		sim_pin: "pin2"
		sim_puk: "puk2"
		carrier_name: 2
		own_number: "1234567890"
	},
	{
		iccid: "iccid3"
		sim_pin: "pin3"
		sim_puk: "puk3"
		carrier_name: 3
		own_number:   ""
	}]
}]
`

// revertedFullTextProto does not contain servo_topology as we only use it to
// derive servo component. We are not exposing servo topology at the moment
// so we are omitting the reverters for servo topology and component.
const revertedFullTextProto = `
variant: "somevariant"
test_coverage_hints {
  usb_detect: true
  use_lid: true
  test_usbprinting: true
  test_usbaudio: true
  test_hdmiaudio: true
  test_audiojack: true
  recovery_test: true
  meet_app: true
  hangout_app: true
  chromesign: true
  chaos_nightly: true
  chaos_dut: true
}
self_serve_pools: "poolval"
stability: true
reference_design: "reef"
wifi_chip: "wireless_xxxx"
wifi_on_site: true
hwid_component: [
	"cellular/fake_cellular"
]
platform: "platformval"
phase: 8
peripherals: {
  wificell: true
  stylus: true
  servo: true
  servo_component: ["servo_v4", "ccd_cr50"]
  servo_state: 1
  servo_type: ""
  servo_usb_state: 3
  wifi_state: 2
  bluetooth_state: 3
  cellular_modem_state: 3
  sim_state: 4
  working_sims: 2
  starfish_state: 3
  smart_usbhub: false
  mimo: true
  huddly: true
  conductive: true
  chameleon_type: 9
  chameleon_connection_types: 3
  chameleon_connection_types: 4
  chameleon: true
  chameleon_state: 1
  audiobox_jackplugger_state: 1
  camerabox: true
  camerabox_facing: 1
  camerabox_light: 1
  audio_loopback_dongle: true
  audio_cable: true
  audio_box: true
  audio_board: true
  trrs_type: 1
  audio_latency_toolkit_state: 1
  router_802_11ax: true
  working_bluetooth_btpeer: 3
  peripheral_btpeer_state: 1
  peripheral_wifi_state: 1
  wifi_router_features: [2,3,4,5,999]
  sim_features: [0,1,2]
  wifi_router_models: ["gale","OPENWRT[Ubiquiti_Unifi_6_Lite]"]
  hmr_state: 1
  pasit_components: "CAMERA-1"
  pasit_components: "MONITOR-1"
  pasit_components: "MONITOR-2"
  pasit_components: "DOCKING_STATION-1"
  amt_manager_state: 1
  audio_beamforming: "intelligo"
  camera_state: 2
}
os_type: 2
model: "modelval"
sku: "skuval"
hwid_sku: "eve_IntelR_CoreTM_i7_7Y75_CPU_1_30GHz_16GB"
dlm_sku_id: "12345"
brand: "HOMH"
ec_type: 1
cr50_ro_keyid: "a"
cr50_ro_version: "11.12.13"
cr50_rw_keyid: "b"
cr50_rw_version: "21.22.23"
cr50_phase: 2
cts_cpu: 1
cts_cpu: 2
cts_abi: 1
cts_abi: 2
critical_pools: 1
critical_pools: 2
capabilities {
  webcam: true
  video_acceleration: 6
  video_acceleration: 8
  touchpad: true
  touchscreen: true
  telephony: "telephonyval"
  storage: "storageval"
  starfish_slot_mapping: "1_verizon,2_tmobile,4_att"
  power: "powerval"
  modem: "modemval"
  lucidsleep: true
  hotwording: true
  internal_display: true
  graphics: "graphicsval"
  gpu_family: "gpufamilyval"
  gpu_id: "gpuidval"
  flashrom: true
  fingerprint: true
  fingerprint_board: "fingerprintboardval"
  fingerprint_mcu: "fingerprintmcuval"
  fingerprint_sensor: "fingerprintsensorval"
  detachablebase: true
  carrier: 2
  supported_carriers: 2
  supported_carriers: 4
  supported_carriers: 5
  bluetooth: true
  atrus: true
  cbx: 0
  cbx_branding: 0
  form_factor: 1
}
board: "boardval"
bot_size: 1
arc: true
callbox: true
licenses: {
  type: 2
  identifier: ""
}
licenses: {
  type: 1
  identifier: ""
}
modeminfo: {
  type: 1
  imei: "imei"
  supported_bands: "bands"
	sim_count: 1
  model_variant: "somecellularvariant"
}
siminfo: [{
	slot_id: 1
	type: 1
	eid: "eid"
	test_esim: true
	profile_info: {
		iccid: "iccid"
		sim_pin: "pin"
		sim_puk: "puk"
		carrier_name: 1
		own_number:   "1234567890"
	}
},
{
	slot_id: 2
	type: 2
	eid: "eid2"
	test_esim: false
	profile_info: [{
		iccid: "iccid2"
		sim_pin: "pin2"
		sim_puk: "puk2"
		carrier_name: 2
		own_number: "1234567890"
	},
	{
		iccid: "iccid3"
		sim_pin: "pin3"
		sim_puk: "puk3"
		carrier_name: 3
		own_number:   ""
	}]
}]
`

var fullDimensions = Dimensions{
	"label-arc":                         {"True"},
	"label-atrus":                       {"True"},
	"label-audio_board":                 {"True"},
	"label-audio_box":                   {"True"},
	"label-audio_cable":                 {"True"},
	"label-audio_latency_toolkit_state": {"WORKING"},
	"label-audio_loopback_dongle":       {"True"},
	"label-audiobox_jackplugger_state":  {"WORKING"},
	"label-bluetooth":                   {"True"},
	"label-board":                       {"boardval"},
	"label-bot_size":                    {"BOT_SIZE_LARGE"},
	"label-callbox":                     {"True"},
	"label-camerabox":                   {"True"},
	"label-camerabox_facing":            {"CAMERABOX_FACING_BACK"},
	"label-camerabox_light":             {"CAMERABOX_LIGHT_LED"},
	"label-carrier":                     {"CARRIER_TMOBILE"},
	"label-supported_carriers":          {"CARRIER_TMOBILE", "CARRIER_SPRINT", "CARRIER_TESTESIM"},
	"label-cellular_modem":              {"fake_cellular"},
	"label-chameleon":                   {"True"},
	"label-chameleon_type": {
		"CHAMELEON_TYPE_V2",
	},
	"label-chameleon_connection_types": {
		"CHAMELEON_CONNECTION_TYPE_HDMI",
		"CHAMELEON_CONNECTION_TYPE_DP",
	},
	"label-chameleon_state":         {"WORKING"},
	"label-chaos_dut":               {"True"},
	"label-chaos_nightly":           {"True"},
	"label-chromesign":              {"True"},
	"label-conductive":              {"True"},
	"label-cts_abi":                 {"CTS_ABI_ARM", "CTS_ABI_X86"},
	"label-cts_cpu":                 {"CTS_CPU_ARM", "CTS_CPU_X86"},
	"label-detachablebase":          {"True"},
	"label-device-stable":           {"True"},
	"label-ec_type":                 {"EC_TYPE_CHROME_OS"},
	"label-fingerprint":             {"True"},
	"label-fingerprint_board":       {"fingerprintboardval"},
	"label-fingerprint_mcu":         {"fingerprintmcuval"},
	"label-fingerprint_sensor":      {"fingerprintsensorval"},
	"label-flashrom":                {"True"},
	"label-form_factor":             {"FORM_FACTOR_CLAMSHELL"},
	"label-gpu_family":              {"gpufamilyval"},
	"label-gpu_id":                  {"gpuidval"},
	"label-graphics":                {"graphicsval"},
	"label-hangout_app":             {"True"},
	"label-hwid_sku":                {"eve_IntelR_CoreTM_i7_7Y75_CPU_1_30GHz_16GB"},
	"label-dlm_sku_id":              {"12345"},
	"label-hmr_state":               {"WORKING"},
	"label-hotwording":              {"True"},
	"label-huddly":                  {"True"},
	"label-internal_display":        {"True"},
	"label-meet_app":                {"True"},
	"label-mimo":                    {"True"},
	"label-model":                   {"modelval"},
	"label-modem":                   {"modemval"},
	"label-modem_imei":              {"imei"},
	"label-modem_sim_count":         {"1"},
	"label-modem_supported_bands":   {"bands"},
	"label-modem_type":              {"MODEM_TYPE_QUALCOMM_SC7180"},
	"label-license":                 {"LICENSE_TYPE_MS_OFFICE_STANDARD", "LICENSE_TYPE_WINDOWS_10_PRO"},
	"label-lucidsleep":              {"True"},
	"label-os_type":                 {"OS_TYPE_CROS"},
	"label-peripheral_btpeer_state": {"WORKING"},
	"label-peripheral_wifi_state":   {"WORKING"},
	"label-wifi_router_features":    {"WIFI_ROUTER_FEATURE_IEEE_802_11_A", "WIFI_ROUTER_FEATURE_IEEE_802_11_B", "WIFI_ROUTER_FEATURE_IEEE_802_11_G", "WIFI_ROUTER_FEATURE_IEEE_802_11_N", "999"},
	"label-sim_features":            {"SIM_FEATURE_UNSPECIFIED", "SIM_FEATURE_LIVE_NETWORK", "SIM_FEATURE_SMS"},
	"label-wifi_router_models":      {"gale", "OPENWRT[Ubiquiti_Unifi_6_Lite]"},
	"label-phase":                   {"PHASE_MP"},
	"label-platform":                {"platformval"},
	"label-pool":                    {"DUT_POOL_CQ", "DUT_POOL_BVT", "poolval"},
	"label-power":                   {"powerval"},
	"label-recovery_test":           {"True"},
	"label-reference_design":        {"reef"},
	"label-touchpad":                {"True"},
	"label-touchscreen":             {"True"},
	"label-servo":                   {"True"},
	"label-wifi_state":              {"ACCEPTABLE"},
	"label-bluetooth_state":         {"NEED_REPLACEMENT"},
	"label-cellular_modem_state":    {"NEED_REPLACEMENT"},
	"label-sim_state":               {"WRONG_CONFIG"},
	"label-working_sims":            {"1", "2"},
	"label-starfish_state":          {"BROKEN"},
	"label-servo_state":             {"WORKING"},
	"label-servo_component":         {"servo_v4", "ccd_cr50"},
	"label-servo_usb_state":         {"NEED_REPLACEMENT"},
	"label-sim_1_0_carrier_name":    {"NETWORK_TEST"},
	"label-sim_1_0_iccid":           {"iccid"},
	"label-sim_1_0_own_number":      {"1234567890"},
	"label-sim_1_0_pin":             {"pin"},
	"label-sim_1_0_puk":             {"puk"},
	"label-sim_1_eid":               {"eid"},
	"label-sim_1_test_esim":         {"True"},
	"label-sim_1_num_profiles":      {"1"},
	"label-sim_1_type":              {"SIM_PHYSICAL"},
	"label-sim_2_0_carrier_name":    {"NETWORK_ATT"},
	"label-sim_2_0_iccid":           {"iccid2"},
	"label-sim_2_0_own_number":      {"1234567890"},
	"label-sim_2_0_pin":             {"pin2"},
	"label-sim_2_0_puk":             {"puk2"},
	"label-sim_2_1_carrier_name":    {"NETWORK_TMOBILE"},
	"label-sim_2_1_iccid":           {"iccid3"},
	"label-sim_2_1_pin":             {"pin3"},
	"label-sim_2_1_puk":             {"puk3"},
	"label-sim_2_eid":               {"eid2"},
	"label-sim_2_num_profiles":      {"2"},
	"label-sim_2_type":              {"SIM_DIGITAL"},
	"label-sim_slot_id":             {"1", "2"},
	"label-sku":                     {"skuval"},
	"label-brand":                   {"HOMH"},
	"label-router_802_11ax":         {"True"},
	"label-starfish_slot_mapping":   {"1_verizon,2_tmobile,4_att"},
	"label-storage":                 {"storageval"},
	"label-stylus":                  {"True"},
	"label-telephony":               {"telephonyval"},
	"label-test_audiojack":          {"True"},
	"label-test_hdmiaudio":          {"True"},
	"label-test_usbaudio":           {"True"},
	"label-test_usbprinting":        {"True"},
	"label-trrs_type":               {"CTIA"},
	"label-usb_detect":              {"True"},
	"label-use_lid":                 {"True"},
	"label-variant":                 {"somevariant"},
	"label-cellular_variant":        {"somecellularvariant"},
	"label-video_acceleration": {
		"VIDEO_ACCELERATION_ENC_VP9",
		"VIDEO_ACCELERATION_ENC_VP9_2",
	},
	"label-webcam":                   {"True"},
	"label-wificell":                 {"True"},
	"label-cr50_phase":               {"CR50_PHASE_PVT"},
	"label-cr50_ro_keyid":            {"a"},
	"label-cr50_ro_version":          {"11.12.13"},
	"label-cr50_rw_keyid":            {"b"},
	"label-cr50_rw_version":          {"21.22.23"},
	"label-wifi_chip":                {"wireless_xxxx"},
	"label-wifi_on_site":             {"True"},
	"label-working_bluetooth_btpeer": {"1", "2", "3"},
	"label-pasit_components":         {"CAMERA-1", "MONITOR-1", "MONITOR-2", "DOCKING_STATION-1"},
	"label-amt_manager_state":        {"WORKING"},
	"label-audio_beamforming":        {"intelligo"},
	"label-camera_state":             {"ACCEPTABLE"},
}

func TestConvertEmpty(t *testing.T) {
	t.Parallel()
	ls := inventory.SchedulableLabels{}
	got := Convert(&ls)
	if len(got) > 0 {
		t.Errorf("Got nonempty dimensions %#v", got)
	}
}

func TestConvertFull(t *testing.T) {
	t.Parallel()
	var ls inventory.SchedulableLabels
	if err := proto.UnmarshalText(fullTextProto, &ls); err != nil {
		t.Fatalf("Error unmarshalling example text: %s", err)
	}
	got := Convert(&ls)
	if diff := typed.Got(got).Want(fullDimensions).Diff(); diff != "" {
		t.Errorf("labels differ -want +got, %s", diff)
	}
}

var servoStateConvertStateCases = []struct {
	stateValue    int32
	expectedEmpty bool
	expectValue   string
}{
	{0, true, ""},
	{1, false, "WORKING"},
	{2, false, "NOT_CONNECTED"},
	{3, false, "BROKEN"},
	{4, false, "WRONG_CONFIG"},
	{99, true, ""}, //wrong value
}

func TestConvertServoStateWorking(t *testing.T) {
	for _, testCase := range servoStateConvertStateCases {
		t.Run("State value is "+string(testCase.stateValue), func(t *testing.T) {
			var ls inventory.SchedulableLabels
			var dims Dimensions
			protoText := fmt.Sprintf(`peripherals: { servo_state: %v}`, testCase.stateValue)
			if err := proto.UnmarshalText(protoText, &ls); err != nil {
				t.Fatalf("Error unmarshalling example text: %s", err)
			}
			if testCase.expectedEmpty {
				dims = Dimensions{}
			} else {
				dims = Dimensions{"label-servo_state": {testCase.expectValue}}
			}
			got := Convert(&ls)
			if diff := typed.Got(got).Want(dims).Diff(); diff != "" {
				t.Errorf(
					"Convert state from %d got labels differ -want +got, %s",
					testCase.stateValue,
					diff)
			}
		})
	}
}

func TestRevertEmpty(t *testing.T) {
	t.Parallel()
	want := inventory.NewSchedulableLabels()
	got := Revert(make(Dimensions))
	if diff := typed.Got(got).Want(want).Diff(); diff != "" {
		t.Errorf("labels differ -want +got, %s", diff)
	}
}

var servoStateRevertCaseTests = []struct {
	labelValue  string
	expectState inventory.PeripheralState
}{
	{"Something", inventory.PeripheralState_UNKNOWN},
	{"WorKing", inventory.PeripheralState_WORKING},
	{"working", inventory.PeripheralState_WORKING},
	{"WORKING", inventory.PeripheralState_WORKING},
	{"Not_Connected", inventory.PeripheralState_NOT_CONNECTED},
	{"noT_CONnected", inventory.PeripheralState_NOT_CONNECTED},
	{"BroKen", inventory.PeripheralState_BROKEN},
	{"BROKEN", inventory.PeripheralState_BROKEN},
	{"broken", inventory.PeripheralState_BROKEN},
	{"Wrong_config", inventory.PeripheralState_WRONG_CONFIG},
	{"WRONG_CONFIG", inventory.PeripheralState_WRONG_CONFIG},
}

func TestRevertServoStateInCaseEffect(t *testing.T) {
	for _, testCase := range servoStateRevertCaseTests {
		t.Run(testCase.labelValue, func(t *testing.T) {
			want := inventory.NewSchedulableLabels()
			*want.Peripherals.ServoState = testCase.expectState
			dims := Dimensions{
				"label-servo_state": {testCase.labelValue},
			}
			got := Revert(dims)
			if diff := typed.Got(got).Want(want).Diff(); diff != "" {
				t.Errorf(
					"Revert value from %v made labels differ -want +got, %s",
					testCase.labelValue,
					diff)
			}
		})
	}
}

func TestRevertFull(t *testing.T) {
	t.Parallel()
	var want inventory.SchedulableLabels
	if err := proto.UnmarshalText(revertedFullTextProto, &want); err != nil {
		t.Fatalf("Error unmarshalling example text: %s", err)
	}
	got := Revert(cloneDimensions(fullDimensions))
	if diff := typed.Got(got).Want(&want).Diff(); diff != "" {
		t.Errorf("labels differ -want +got, %s", diff)
	}
}

func cloneDimensions(d Dimensions) Dimensions {
	ret := make(Dimensions)
	for k, v := range d {
		ret[k] = make([]string, len(v))
		copy(ret[k], v)
	}
	return ret
}

const fullTextProtoSpecial = `
variant: ""
`

var fullDimensionsSpecial = Dimensions{}

func TestConvertSpecial(t *testing.T) {
	t.Parallel()
	var ls inventory.SchedulableLabels
	if err := proto.UnmarshalText(fullTextProtoSpecial, &ls); err != nil {
		t.Fatalf("Error unmarshalling example text: %s", err)
	}
	got := Convert(&ls)
	if diff := typed.Got(got).Want(fullDimensionsSpecial).Diff(); diff != "" {
		t.Errorf("labels differ -want +got, %s", diff)
	}
}
