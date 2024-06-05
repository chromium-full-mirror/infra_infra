// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package labels

import (
	"fmt"
	"sort"
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
stability: false
reference_design: "reef"
hwid_component: [
	"cellular/fake_cellular"
]
wifi_chip: "wireless_xxxx"
wifi_on_site: false
platform: "platformval"
phase: 4
peripherals: {
  wificell: true
  stylus: true
  servo: true
  servo_component: "ccd_cr50"
  servo_component: "servo_v4"
  servo_state: 3
  servo_topology: {
	main: {
		usb_hub_port: "6.4.1"
		serial: "C1903145591"
		type: "servo_v4"
		sysfs_product: "Servo V4"
	}
	children: {
		usb_hub_port: "6.4.2"
		serial: "0681D03A-92DCCD64"
		type: "ccd_cr50"
		sysfs_product: "Cr50"
	}
  }
  servo_type: "servo_v3"
  rpm_state: 1
  smart_usbhub: true
  storage_state: 1
  servo_usb_state: 3
  battery_state: 3
  wifi_state: 3
  bluetooth_state: 3
  cellular_modem_state: 3
  mimo: true
  huddly: true
  conductive: true
  chameleon_type: 2
  chameleon_type: 5
  chameleon: true
  chameleon_state: 1
  audiobox_jackplugger_state: 1
  trrs_type: 1
  camerabox: true
  camerabox_facing: 1
  camerabox_light: 1
  audio_loopback_dongle: true
  audio_box: true
  audio_board: true
  audio_cable: true
  audio_latency_toolkit_state: 1
  router_802_11ax: true
  working_bluetooth_btpeer: 3
  hmr_state: 1
  peripheral_btpeer_state: 1
  peripheral_wifi_state: 1
  wifi_router_features: [2,3,4,5]
  wifi_router_models: ["OPENWRT[Ubiquiti_Unifi_6_Lite]","gale"]
}
os_type: 2
model: "modelval"
sku: "skuval"
hwid_sku: "eve_IntelR_CoreTM_i7_7Y75_CPU_1_30GHz_16GB"
dlm_sku_id: "12345"
brand: "HOMH"
ec_type: 1
cts_cpu: 1
cts_cpu: 2
cts_abi: 1
cts_abi: 2
critical_pools: 2
critical_pools: 1
cr50_phase: 2
cr50_ro_keyid: "prod"
cr50_ro_version: "1.2.3"
cr50_rw_keyid: "0xde88588d"
cr50_rw_version: "9.8.7"
capabilities {
  webcam: true
  video_acceleration: 6
  video_acceleration: 8
  touchpad: true
  touchscreen: true
  fingerprint: true
  telephony: "telephonyval"
  storage: "storageval"
  power: "powerval"
  modem: "modemval"
  lucidsleep: true
  hotwording: true
  graphics: "graphicsval"
  internal_display: true
  gpu_family: "gpufamilyval"
  gpu_id: ""
  flashrom: true
  detachablebase: true
  carrier: 2
  starfish_slot_mapping: "1_verizon,2_sprint,4_att"
  bluetooth: true
  atrus: true
  cbx: 1
  cbx_branding: 2
  form_factor: 0
}
board: "boardval"
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
		own_number: "2345678901"
	},
	{
		iccid: "iccid3"
		sim_pin: "pin3"
		sim_puk: "puk3"
		carrier_name: 3
		own_number: "3456789012"
	}]
}]
`

var fullLabels = []string{
	"arc",
	"atrus",
	"audio_board",
	"audio_box",
	"audio_cable",
	"audio_latency_toolkit_state:WORKING",
	"audio_loopback_dongle",
	"audiobox_jackplugger_state:WORKING",
	"battery_state:NEED_REPLACEMENT",
	"bluetooth",
	"bluetooth_state:NEED_REPLACEMENT",
	"board:boardval",
	"brand-code:HOMH",
	"callbox",
	"camerabox",
	"camerabox_facing:back",
	"camerabox_light:led",
	"carrier:tmobile",
	"cbx:True",
	"cbx_branding:hard",
	"cellular_modem_state:NEED_REPLACEMENT",
	"cellular_variant:somecellularvariant",
	"chameleon",
	"chameleon:dp",
	"chameleon:hdmi",
	"chameleon_state:WORKING",
	"chaos_dut",
	"chaos_nightly",
	"chromesign",
	"conductive:True",
	"cr50-ro-keyid:prod",
	"cr50-ro-version:1.2.3",
	"cr50-rw-keyid:0xde88588d",
	"cr50-rw-version:9.8.7",
	"cr50:pvt",
	"cts_abi_arm",
	"cts_abi_x86",
	"cts_cpu_arm",
	"cts_cpu_x86",
	"detachablebase",
	"device-sku:skuval",
	"dlm-sku-id:12345",
	"ec:cros",
	"fingerprint",
	"flashrom",
	"gpu_family:gpufamilyval",
	"graphics:graphicsval",
	"hangout_app",
	"hmr_state:WORKING",
	"hotwording",
	"huddly",
	"hw_video_acc_enc_vp9",
	"hw_video_acc_enc_vp9_2",
	"hwid_component:cellular/fake_cellular",
	"internal_display",
	"license_ms_office_standard",
	"license_windows_10_pro",
	"lucidsleep",
	"meet_app",
	"mimo",
	"model:modelval",
	"modem:modemval",
	"modem_imei:imei",
	"modem_sim_count:1",
	"modem_supported_bands:bands",
	"modem_type:qualcomm_sc7180",
	"os:cros",
	"peripheral_btpeer_state:WORKING",
	"peripheral_wifi_state:WORKING",
	"phase:DVT2",
	"platform:platformval",
	"pool:bvt",
	"pool:cq",
	"pool:poolval",
	"power:powerval",
	"recovery_test",
	"reference_design:reef",
	"router_802_11ax",
	"rpm_state:WORKING",
	"servo",
	"servo_component:ccd_cr50",
	"servo_component:servo_v4",
	"servo_state:BROKEN",
	"servo_topology:eyJtYWluIjp7InR5cGUiOiJzZXJ2b192NCIsInN5c2ZzX3Byb2R1Y3QiOiJTZXJ2byBWNCIsInNlcmlhbCI6IkMxOTAzMTQ1NTkxIiwidXNiX2h1Yl9wb3J0IjoiNi40LjEifSwiY2hpbGRyZW4iOlt7InR5cGUiOiJjY2RfY3I1MCIsInN5c2ZzX3Byb2R1Y3QiOiJDcjUwIiwic2VyaWFsIjoiMDY4MUQwM0EtOTJEQ0NENjQiLCJ1c2JfaHViX3BvcnQiOiI2LjQuMiJ9XX0=",
	"servo_type:servo_v3",
	"servo_usb_state:NEED_REPLACEMENT",
	"sim_1_0_carrier_name:NETWORK_TEST",
	"sim_1_0_iccid:iccid",
	"sim_1_0_own_number:1234567890",
	"sim_1_0_pin:pin",
	"sim_1_0_puk:puk",
	"sim_1_eid:eid",
	"sim_1_num_profiles:1",
	"sim_1_test_esim:True",
	"sim_1_type:SIM_PHYSICAL",
	"sim_2_0_carrier_name:NETWORK_ATT",
	"sim_2_0_iccid:iccid2",
	"sim_2_0_own_number:2345678901",
	"sim_2_0_pin:pin2",
	"sim_2_0_puk:puk2",
	"sim_2_1_carrier_name:NETWORK_TMOBILE",
	"sim_2_1_iccid:iccid3",
	"sim_2_1_own_number:3456789012",
	"sim_2_1_pin:pin3",
	"sim_2_1_puk:puk3",
	"sim_2_eid:eid2",
	"sim_2_num_profiles:2",
	"sim_2_type:SIM_DIGITAL",
	"sim_slot_id:1",
	"sim_slot_id:2",
	"sku:eve_IntelR_CoreTM_i7_7Y75_CPU_1_30GHz_16GB",
	"smart_usbhub",
	"starfish_slot_mapping:1_verizon,2_sprint,4_att",
	"storage:storageval",
	"storage_state:NORMAL",
	"stylus",
	"supported_carrier:att",
	"supported_carrier:sprint",
	"supported_carrier:tmobile",
	"supported_carrier:verizon",
	"telephony:telephonyval",
	"test_audiojack",
	"test_hdmiaudio",
	"test_usbaudio",
	"test_usbprinting",
	"touchpad",
	"touchscreen",
	"trrs_type:CTIA",
	"usb_detect",
	"use_lid",
	"variant:somevariant",
	"webcam",
	"wifi_chip:wireless_xxxx",
	"wifi_router_features:WIFI_ROUTER_FEATURE_IEEE_802_11_A",
	"wifi_router_features:WIFI_ROUTER_FEATURE_IEEE_802_11_B",
	"wifi_router_features:WIFI_ROUTER_FEATURE_IEEE_802_11_G",
	"wifi_router_features:WIFI_ROUTER_FEATURE_IEEE_802_11_N",
	"wifi_router_models:OPENWRT[Ubiquiti_Unifi_6_Lite]",
	"wifi_router_models:gale",
	"wifi_state:NEED_REPLACEMENT",
	"wificell",
	"working_bluetooth_btpeer:3",
}

var baseExpectedLabels = []string{"conductive:False"}

func TestConvertEmptyLabels(t *testing.T) {
	t.Parallel()
	ls := inventory.SchedulableLabels{}
	got := Convert(&ls)
	if diff := typed.Got(got).Want(baseExpectedLabels).Diff(); diff != "" {
		t.Errorf(
			"Convert base labels %#v got labels differ -want +got, %s",
			baseExpectedLabels,
			diff)
	}
}

func TestConvertFull(t *testing.T) {
	t.Parallel()
	var ls inventory.SchedulableLabels
	if err := proto.UnmarshalText(fullTextProto, &ls); err != nil {
		t.Fatalf("Error unmarshalling example text: %s", err)
	}
	got := Convert(&ls)
	sort.Sort(sort.StringSlice(got))
	want := make([]string, len(fullLabels))
	copy(want, fullLabels)
	if diff := typed.Got(got).Want(want).Diff(); diff != "" {
		t.Errorf("labels differ -want +got, %s", diff)
	}
}

var servoStateConvertStateCases = []struct {
	stateValue   int32
	expectLabels []string
}{
	{0, []string{}},
	{1, []string{"servo_state:WORKING"}},
	{2, []string{"servo_state:NOT_CONNECTED"}},
	{3, []string{"servo_state:BROKEN"}},
	{4, []string{"servo_state:WRONG_CONFIG"}},
	{99, []string{}}, //wrong value
}

func TestConvertServoStateWorking(t *testing.T) {
	for _, testCase := range servoStateConvertStateCases {
		t.Run("State value is "+string(testCase.stateValue), func(t *testing.T) {
			var ls inventory.SchedulableLabels
			protoText := fmt.Sprintf(`peripherals: { servo_state: %v}`, testCase.stateValue)
			if err := proto.UnmarshalText(protoText, &ls); err != nil {
				t.Fatalf("Error unmarshalling example text: %s", err)
			}
			want := append(baseExpectedLabels, testCase.expectLabels...)
			got := Convert(&ls)
			if diff := typed.Got(got).Want(want).Diff(); diff != "" {
				t.Errorf(
					"Convert servo_state %#v got labels differ -want +got, %s",
					testCase.stateValue,
					diff)
			}
		})
	}
}

var storageStateConvertStateCases = []struct {
	stateValue   int32
	expectLabels []string
}{
	{0, []string{}},
	{1, []string{"storage_state:NORMAL"}},
	{2, []string{"storage_state:ACCEPTABLE"}},
	{3, []string{"storage_state:NEED_REPLACEMENT"}},
	{5, []string{}}, //wrong value
}

func TestConvertStorageState(t *testing.T) {
	for _, testCase := range storageStateConvertStateCases {
		t.Run("State value is "+string(testCase.stateValue), func(t *testing.T) {
			var ls inventory.SchedulableLabels
			protoText := fmt.Sprintf(`peripherals: { storage_state: %v}`, testCase.stateValue)
			if err := proto.UnmarshalText(protoText, &ls); err != nil {
				t.Fatalf("Error unmarshalling example text: %s", err)
			}
			want := append(baseExpectedLabels, testCase.expectLabels...)
			got := Convert(&ls)
			if diff := typed.Got(got).Want(want).Diff(); diff != "" {
				t.Errorf(
					"Convert storage_state %#v got labels differ -want +got, %s",
					testCase.stateValue,
					diff)
			}
		})
	}
}

var servoUSBStateConvertStateCases = []struct {
	stateValue   int32
	expectLabels []string
}{
	{0, []string{}},
	{1, []string{"servo_usb_state:NORMAL"}},
	{2, []string{"servo_usb_state:ACCEPTABLE"}},
	{3, []string{"servo_usb_state:NEED_REPLACEMENT"}},
	{5, []string{}}, //wrong value
}

func TestConvertServoUSBState(t *testing.T) {
	for _, testCase := range servoUSBStateConvertStateCases {
		t.Run("State value is "+string(testCase.stateValue), func(t *testing.T) {
			var ls inventory.SchedulableLabels
			protoText := fmt.Sprintf(`peripherals: { servo_usb_state: %v}`, testCase.stateValue)
			if err := proto.UnmarshalText(protoText, &ls); err != nil {
				t.Fatalf("Error unmarshalling example text: %s", err)
			}
			want := append(baseExpectedLabels, testCase.expectLabels...)
			got := Convert(&ls)
			if diff := typed.Got(got).Want(want).Diff(); diff != "" {
				t.Errorf(
					"Convert servo_usb_state %#v got labels differ -want +got, %s",
					testCase.stateValue,
					diff)
			}
		})
	}
}

var servoTypeConvertStateCases = []struct {
	val          string
	expectLabels []string
}{
	{"", []string{}},
	{"servo_v3", []string{"servo_type:servo_v3"}},
	{"servo_V4", []string{"servo_type:servo_V4"}},
	{"servo_micro", []string{"servo_type:servo_micro"}},
}

func TestConvertServoTypeWorking(t *testing.T) {
	for _, testCase := range servoTypeConvertStateCases {
		t.Run("ServoType value is "+string(testCase.val), func(t *testing.T) {
			var ls inventory.SchedulableLabels
			protoText := fmt.Sprintf(`peripherals: { servo_type: %#v}`, testCase.val)
			if err := proto.UnmarshalText(protoText, &ls); err != nil {
				t.Fatalf("Error unmarshalling example text: %s", err)
			}
			want := append(baseExpectedLabels, testCase.expectLabels...)
			got := Convert(&ls)
			if diff := typed.Got(got).Want(want).Diff(); diff != "" {
				t.Errorf(
					"Convert servo_type %#v got labels differ -want +got, %s",
					testCase.val,
					diff)
			}
		})
	}
}

// TestConvertModemInfo validates modeminfo converter
func TestConvertModemInfo(t *testing.T) {
	var modemInfoConvertCases = []struct {
		name         string
		testState    string
		expectLabels []string
	}{
		{"NO Modem", "", []string{}},
		{"SC7180", "type: 1", []string{"modem_type:qualcomm_sc7180", "modem_imei:", "modem_supported_bands:", "modem_sim_count:0", "cellular_variant:"}},
		{"L850GL", "type: 2 imei:\"imei\"", []string{"modem_type:fibocomm_l850gl", "modem_imei:imei", "modem_supported_bands:", "modem_sim_count:0", "cellular_variant:"}},
		{"NL668", "type: 3 imei:\"imei\", sim_count:1, model_variant:\"somecellularvariant\"", []string{"modem_type:nl668", "modem_imei:imei", "modem_supported_bands:", "modem_sim_count:1", "cellular_variant:somecellularvariant"}},
		{"FM350", "type: 4 imei:\"imei\", supported_bands:\"bands\" sim_count:1", []string{"modem_type:fm350", "modem_imei:imei", "modem_supported_bands:bands", "modem_sim_count:1", "cellular_variant:"}},
	}
	t.Parallel()
	for _, testCase := range modemInfoConvertCases {
		t.Run("Modem Type "+string(testCase.name), func(t *testing.T) {
			var ls inventory.SchedulableLabels
			protoText := ""
			if testCase.testState != "" {
				protoText = fmt.Sprintf(`modeminfo: { %s }`, testCase.testState)
			}
			if err := proto.UnmarshalText(protoText, &ls); err != nil {
				t.Fatalf("Error unmarshalling example text: %s", err)
			}
			want := append(testCase.expectLabels, baseExpectedLabels...)
			got := Convert(&ls)
			if diff := typed.Got(got).Want(want).Diff(); diff != "" {
				t.Errorf(
					"Convert ModemInfo %#v got labels differ -want +got, %s",
					testCase.testState,
					diff)
			}
		})
	}
}

// Test cases for ModemInfo proto revert

// TestRevertModemInfoLabels validates modeminfo revert
func TestRevertModemInfoLabels(t *testing.T) {
	var modemInfoRevertTestCases = []struct {
		labelValue           []string
		expectType           inventory.ModemType
		expectImei           string
		expectSupportedBands string
		expectSIMCount       int
		expectVariant        string
	}{
		{[]string{}, 0, "", "", 0, ""},
		{[]string{"modem_imei:imei"}, 0, "imei", "", 0, ""},
		{[]string{"modem_imei:imei", "modem_type:qualcomm_sc7180"}, 1, "imei", "", 0, ""},
		{[]string{"modem_imei:imei", "modem_type:qualcomm_sc7180", "modem_supported_bands:bands", "cellular_variant:somecellularvariant"}, 1, "imei", "bands", 0, "somecellularvariant"},
		{[]string{"modem_imei:imei", "modem_type:qualcomm_sc7180", "modem_sim_count:1"}, 1, "imei", "", 1, ""},
	}
	t.Parallel()
	for _, testCase := range modemInfoRevertTestCases {
		t.Run(testCase.expectImei, func(t *testing.T) {
			want := inventory.NewSchedulableLabels()
			if len(testCase.labelValue) > 0 {
				want.Modeminfo = inventory.NewModeminfo()
				*want.Modeminfo.Type = testCase.expectType
				*want.Modeminfo.Imei = testCase.expectImei
				*want.Modeminfo.SupportedBands = testCase.expectSupportedBands
				*want.Modeminfo.SimCount = int32(testCase.expectSIMCount)
				*want.Modeminfo.ModelVariant = testCase.expectVariant
			}
			got := Revert(testCase.labelValue)
			t.Log(got)
			if diff := typed.Got(got).Want(want).Diff(); diff != "" {
				t.Errorf(
					"Revert servo_state from %v made labels differ -want +got, %s",
					testCase.labelValue,
					diff)
			}
		})
	}
}

func TestRevertEmpty(t *testing.T) {
	t.Parallel()
	want := inventory.NewSchedulableLabels()
	got := Revert(nil)
	if diff := typed.Got(got).Want(want).Diff(); diff != "" {
		t.Errorf("labels differ -want +got, %s", diff)
	}
}

func TestRevertServoStateWithWrongCase(t *testing.T) {
	t.Parallel()
	want := inventory.NewSchedulableLabels()
	*want.Peripherals.ServoState = inventory.PeripheralState_NOT_CONNECTED
	labels := []string{"servo_state:Not_Connected"}
	got := Revert(labels)
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

func TestRevertServoStateWithWrongValue(t *testing.T) {
	for _, testCase := range servoStateRevertCaseTests {
		t.Run(testCase.labelValue, func(t *testing.T) {
			want := inventory.NewSchedulableLabels()
			*want.Peripherals.ServoState = testCase.expectState
			labels := []string{fmt.Sprintf("servo_state:%s", testCase.labelValue)}
			got := Revert(labels)
			if diff := typed.Got(got).Want(want).Diff(); diff != "" {
				t.Errorf(
					"Revert servo_state from %v made labels differ -want +got, %s",
					testCase.labelValue,
					diff)
			}
		})
	}
}

var servoTypeRevertCaseTests = []struct {
	labelValue  string
	expectState string
	isNil       bool
}{
	{"", "", true},
	{"", "", false},
	{"Servo_v", "Servo_v", false},
	{"SerVO_v3", "SerVO_v3", false},
}

func TestRevertServoTypeValues(t *testing.T) {
	for _, testCase := range servoTypeRevertCaseTests {
		t.Run(testCase.labelValue, func(t *testing.T) {
			want := inventory.NewSchedulableLabels()
			*want.Peripherals.ServoType = testCase.expectState
			var labels []string
			if testCase.isNil {
				labels = []string{}
			} else {
				labels = []string{fmt.Sprintf("servo_type:%s", testCase.labelValue)}
			}
			got := Revert(labels)
			if diff := typed.Got(got).Want(want).Diff(); diff != "" {
				t.Errorf(
					"Revert servo_type from %v made labels differ -want +got, %s",
					testCase.labelValue,
					diff)
			}
		})
	}
}

func TestRevertFull(t *testing.T) {
	t.Parallel()
	var want inventory.SchedulableLabels
	if err := proto.UnmarshalText(fullTextProto, &want); err != nil {
		t.Fatalf("Error unmarshalling example text: %s", err)
	}
	labels := make([]string, len(fullLabels))
	copy(labels, fullLabels)
	got := Revert(labels)
	if diff := typed.Got(got).Want(&want).Diff(); diff != "" {
		t.Errorf("labels differ -want +got, %s", diff)
	}
}

const fullTextProtoSpecial = `
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
stability: false
reference_design: "reef"
wifi_chip: "wireless_xxxx"
wifi_on_site: false
platform: "platformval"
phase: 4
peripherals: {
  wificell: true
  stylus: true
  servo: true
  servo_state: 3
  servo_type: "servo_v4"
  smart_usbhub: true
  storage_state: 2
  servo_usb_state: 3
  battery_state: 3
  wifi_state: 3
  bluetooth_state: 3
  cellular_modem_state: 3
  mimo: true
  huddly: true
  conductive: true
  chameleon_type: 2
  chameleon_type: 5
  chameleon: true
  chameleon_state: 1
  audiobox_jackplugger_state: 1
  trrs_type: 1
  camerabox: true
  camerabox_facing: 1
  camerabox_light: 1
  audio_loopback_dongle: true
  audio_box: true
  audio_board: true
  audio_cable: true
  audio_latency_toolkit_state: 1
  router_802_11ax: true
  working_bluetooth_btpeer: 3
  hmr_state: 1
  peripheral_btpeer_state: 1
  peripheral_wifi_state: 1
  wifi_router_features: [2,3,4,5]
  wifi_router_models: ["OPENWRT[Ubiquiti_Unifi_6_Lite]", "gale"]
}
os_type: 2
model: "modelval"
sku: "skuval"
hwid_sku: "eve_IntelR_CoreTM_i7_7Y75_CPU_1_30GHz_16GB"
dlm_sku_id: "12345"
brand: "HOMH"
ec_type: 1
cts_cpu: 1
cts_cpu: 2
cts_abi: 1
cts_abi: 2
critical_pools: 2
critical_pools: 1
cr50_phase: 2
cr50_ro_keyid: "prod"
cr50_ro_version: "1.2.3"
cr50_rw_keyid: "0xde88588d"
cr50_rw_version: "9.8.7"
capabilities {
  webcam: true
  video_acceleration: 6
  video_acceleration: 8
  touchpad: true
  touchscreen: true
  fingerprint: true
  telephony: "telephonyval"
  storage: "storageval"
  power: "powerval"
  modem: "modemval"
  lucidsleep: true
  hotwording: true
  graphics: "graphicsval"
  internal_display: true
  gpu_family: "gpufamilyval"
  gpu_id: ""
  flashrom: true
  detachablebase: true
  carrier: 2
  starfish_slot_mapping: "1_verizon,2_sprint,4_att"
  bluetooth: true
  atrus: true
  cbx: 0
  cbx_branding: 0
  form_factor: 0
}
board: "boardval"
arc: true
callbox: true
licenses: {
  type: 2
  identifier: ""
}
licenses: {
  type : 1
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
		own_number: ""
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
		own_number: ""
	},
	{
		iccid: "iccid3"
		sim_pin: "pin3"
		sim_puk: "puk3"
		carrier_name: 3
		own_number: ""
	}]
}]
`

var fullLabelsSpecial = []string{
	"arc",
	"atrus",
	"audio_board",
	"audio_box",
	"audio_cable",
	"audio_latency_toolkit_state:WORKING",
	"audio_loopback_dongle",
	"battery_state:NEED_REPLACEMENT",
	"wifi_state:NEED_REPLACEMENT",
	"bluetooth_state:NEED_REPLACEMENT",
	"bluetooth",
	"board:boardval",
	"brand-code:HOMH",
	"callbox",
	"camerabox",
	"camerabox_facing:back",
	"camerabox_light:led",
	"carrier:tmobile",
	"cellular_modem_state:NEED_REPLACEMENT",
	"cellular_variant:somecellularvariant",
	"chameleon",
	"chameleon:dp",
	"chameleon:hdmi",
	"chameleon_state:WORKING",
	"audiobox_jackplugger_state:WORKING",
	"trrs_type:CTIA",
	"chaos_dut",
	"chaos_nightly",
	"chromesign",
	"conductive:True",
	"cr50-ro-keyid:prod",
	"cr50-ro-version:1.2.3",
	"cr50-rw-keyid:0xde88588d",
	"cr50-rw-version:9.8.7",
	"cr50:pvt",
	"cts_abi_arm",
	"cts_abi_x86",
	"cts_cpu_arm",
	"cts_cpu_x86",
	"detachablebase",
	"device-sku:skuval",
	"dlm-sku-id:12345",
	"ec:cros",
	"fingerprint",
	"flashrom",
	"gpu_family:gpufamilyval",
	"graphics:graphicsval",
	"hangout_app",
	"hmr_state:WORKING",
	"hotwording",
	"huddly",
	"hw_video_acc_enc_vp9",
	"hw_video_acc_enc_vp9_2",
	"internal_display",
	"license_ms_office_standard",
	"license_windows_10_pro",
	"lucidsleep",
	"meet_app",
	"mimo",
	"model:modelval",
	"modem:modemval",
	"modem_imei:imei",
	"modem_sim_count:1",
	"modem_supported_bands:bands",
	"modem_type:qualcomm_sc7180",
	"os:cros",
	"peripheral_btpeer_state:WORKING",
	"peripheral_wifi_state:WORKING",
	"phase:DVT2",
	"platform:platformval",
	"pool:bvt",
	"pool:cq",
	"pool:poolval",
	"power:powerval",
	"recovery_test",
	"reference_design:reef",
	"router_802_11ax",
	"servo",
	"servo_state:broken",
	"servo_type:servo_v4",
	"servo_usb_state:NEED_REPLACEMENT",
	"sim_1_0_carrier_name:NETWORK_TEST",
	"sim_1_0_iccid:iccid",
	"sim_1_0_own_number:",
	"sim_1_0_pin:pin",
	"sim_1_0_puk:puk",
	"sim_1_eid:eid",
	"sim_1_num_profiles:1",
	"sim_1_test_esim:True",
	"sim_1_type:SIM_PHYSICAL",
	"sim_2_0_carrier_name:NETWORK_ATT",
	"sim_2_0_iccid:iccid2",
	"sim_2_0_own_number:",
	"sim_2_0_pin:pin2",
	"sim_2_0_puk:puk2",
	"sim_2_1_carrier_name:NETWORK_TMOBILE",
	"sim_2_1_iccid:iccid3",
	"sim_2_1_own_number:",
	"sim_2_1_pin:pin3",
	"sim_2_1_puk:puk3",
	"sim_2_eid:eid2",
	"sim_2_num_profiles:2",
	"sim_2_type:SIM_DIGITAL",
	"sim_slot_id:1",
	"sim_slot_id:2",
	"sku:eve_IntelR_CoreTM_i7_7Y75_CPU_1_30GHz_16GB",
	"smart_usbhub",
	"starfish_slot_mapping:1_verizon,2_sprint,4_att",
	"storage:storageval",
	"storage_state:ACCEPTABLE",
	"stylus",
	"telephony:telephonyval",
	"test_audiojack",
	"test_hdmiaudio",
	"test_usbaudio",
	"test_usbprinting",
	"touchpad",
	"touchscreen",
	"usb_detect",
	"use_lid",
	"variant:",
	"webcam",
	"wifi_chip:wireless_xxxx",
	"wifi_router_features:WIFI_ROUTER_FEATURE_IEEE_802_11_A",
	"wifi_router_features:WIFI_ROUTER_FEATURE_IEEE_802_11_B",
	"wifi_router_features:WIFI_ROUTER_FEATURE_IEEE_802_11_G",
	"wifi_router_features:WIFI_ROUTER_FEATURE_IEEE_802_11_N",
	"wifi_router_models:OPENWRT[Ubiquiti_Unifi_6_Lite]",
	"wifi_router_models:gale",
	"wificell",
	"working_bluetooth_btpeer:3",
}

// Test the special cases in revert, including
// * empty variant
func TestRevertSpecial(t *testing.T) {
	t.Parallel()
	var want inventory.SchedulableLabels
	if err := proto.UnmarshalText(fullTextProtoSpecial, &want); err != nil {
		t.Fatalf("Error unmarshalling example text: %s", err)
	}
	labels := make([]string, len(fullLabelsSpecial))
	copy(labels, fullLabelsSpecial)
	got := Revert(labels)
	if diff := typed.Got(got).Want(&want).Diff(); diff != "" {
		t.Errorf("labels differ -want +got, %s", diff)
	}
}
