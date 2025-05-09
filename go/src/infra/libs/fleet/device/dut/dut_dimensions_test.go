// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	deviceconfig "go.chromium.org/chromiumos/infra/proto/go/device"

	"go.chromium.org/infra/cros/dutstate"
	"go.chromium.org/infra/libs/skylab/inventory/swarming"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	lab "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
	manufacturing "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/manufacturing"
	"go.chromium.org/infra/unifiedfleet/app/util/osutil"
)

var servo = lab.Servo{
	ServoHostname:       "test_servo",
	ServoPort:           int32(9999),
	ServoSerial:         "test_servo_serial",
	ServoType:           "v3",
	ServoSetup:          lab.ServoSetupType_SERVO_SETUP_DUAL_V4,
	ServoFwChannel:      lab.ServoFwChannel_SERVO_FW_ALPHA,
	DockerContainerName: "test_servod_docker",
	ServoTopology: &lab.ServoTopology{
		Main: &lab.ServoTopologyItem{
			Type:         "servo_v4",
			SysfsProduct: "Servo V4",
			Serial:       "C1903145591",
			UsbHubPort:   "6.4.1",
			FwVersion:    "test_firmware_v1",
		},
		Children: []*lab.ServoTopologyItem{
			{
				Type:         "ccd_cr50",
				SysfsProduct: "Cr50",
				Serial:       "0681D03A-92DCCD64",
				UsbHubPort:   "6.4.2",
				FwVersion:    "test_firmware_v1",
			},
		},
	},
	ServoComponent: []string{"servo_v4", "servo_micro"},
}

var machine = ufspb.Machine{
	Name:         "test_dut",
	SerialNumber: "test_serial",
	Device: &ufspb.Machine_ChromeosMachine{
		ChromeosMachine: &ufspb.ChromeOSMachine{
			Hwid:        "test_hwid",
			BuildTarget: "coral",
			Model:       "test_model",
			Sku:         "test_variant",
			DlmSkuId:    "12345",
			HasWifiBt:   true,
		},
	},
}

var lse = ufspb.MachineLSE{
	Name:     "test_host",
	Hostname: "test_host",
	Machines: []string{"test_dut"},
	Lse: &ufspb.MachineLSE_ChromeosMachineLse{
		ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
			ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
				DeviceLse: &ufspb.ChromeOSDeviceLSE{
					Device: &ufspb.ChromeOSDeviceLSE_Dut{
						Dut: &lab.DeviceUnderTest{
							Hostname: "test_host",
							Pools:    []string{"DUT_POOL_QUOTA", "hotrod"},
							Peripherals: &lab.Peripherals{
								Servo: &servo,
								Chameleon: &lab.Chameleon{
									ChameleonPeripherals: []lab.ChameleonType{
										lab.ChameleonType_CHAMELEON_TYPE_V2,
									},
									ChameleonConnectionTypes: []lab.ChameleonConnectionType{
										lab.ChameleonConnectionType_CHAMELEON_CONNECTION_TYPE_DP,
									},
									Hostname: "test-chameleon",
								},
								Rpm: &lab.OSRPM{
									PowerunitName:   "test_power_unit_name",
									PowerunitOutlet: "test_power_unit_outlet",
								},
								ConnectedCamera: []*lab.Camera{
									{
										CameraType: lab.CameraType_CAMERA_HUDDLY,
									},
								},
								Wifi: &lab.Wifi{
									Wificell:    true,
									AntennaConn: lab.Wifi_CONN_CONDUCTIVE,
									Router:      lab.Wifi_ROUTER_802_11AX,
									WifiRouterFeatures: []labapi.WifiRouterFeature{
										labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
									},
									WifiRouters: []*lab.WifiRouter{
										{
											Model: "gale",
											SupportedFeatures: []labapi.WifiRouterFeature{
												labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
											},
										},
									},
								},
								Touch: &lab.Touch{
									Mimo: true,
								},
								Carrier: "att",
								Chaos:   true,
								Cable: []*lab.Cable{
									{
										Type: lab.CableType_CABLE_AUDIOJACK,
									},
								},
								SmartUsbhub:         true,
								StarfishSlotMapping: "test-map-key:test-value",
							},
							Modeminfo: &lab.ModemInfo{
								Type:           lab.ModemType_MODEM_TYPE_QUALCOMM_SC7180,
								Imei:           "imei",
								SupportedBands: "bands",
								SimCount:       1,
								ModelVariant:   "test_variant",
							},
							Siminfo: []*lab.SIMInfo{
								{
									Type:     lab.SIMType_SIM_DIGITAL,
									SlotId:   1,
									Eid:      "eid",
									TestEsim: true,
									ProfileInfo: []*lab.SIMProfileInfo{
										{
											Iccid:       "iccid1",
											SimPin:      "pin1",
											SimPuk:      "puk1",
											CarrierName: lab.NetworkProvider_NETWORK_ATT,
											OwnNumber:   "123456789",
										},
									},
								},
							},
							Hive: "e",
						},
					},
				},
			},
		},
	},
	Zone: "UFS_TEST_ZONE",
}

var devUFSState = lab.DutState{
	Id: &lab.ChromeOSDeviceID{
		Value: "test_dut",
	},
	Servo:                  lab.PeripheralState_BROKEN,
	WorkingBluetoothBtpeer: 3,
	WifiState:              lab.HardwareState_HARDWARE_ACCEPTABLE,
	RpmState:               lab.PeripheralState_WORKING,
	DolosState:             lab.PeripheralState_WORKING,
	VersionInfo: &lab.VersionInfo{
		OsType:     lab.VersionInfo_CHROMEOS,
		Os:         "coral-release/R100.12000.00",
		RwFirmware: "Google_Coral.10068.101.0",
		RoFirmware: "Google_Coral.10068.101.0",
	},
}

var deviceConfig = &deviceconfig.Config{
	Id: &deviceconfig.ConfigId{
		PlatformId: &deviceconfig.PlatformId{
			Value: "coral",
		},
		ModelId: &deviceconfig.ModelId{
			Value: "test_model",
		},
		VariantId: &deviceconfig.VariantId{
			Value: "test_variant",
		},
	},
	Power:   deviceconfig.Config_POWER_SUPPLY_AC_ONLY,
	Storage: deviceconfig.Config_STORAGE_SSD,
	VideoAccelerationSupports: []deviceconfig.Config_VideoAcceleration{
		deviceconfig.Config_VIDEO_ACCELERATION_ENC_H264,
	},
	Cpu: deviceconfig.Config_ARM64,
}

var osDeviceData = ufspb.ChromeOSDeviceData{
	LabConfig: &lse,
	DutState:  &devUFSState,
	HwidData: &ufspb.HwidData{
		Sku:     "test_sku",
		Variant: "test_variant",
	},
	ManufacturingConfig: &manufacturing.ManufacturingConfig{
		ManufacturingId: &manufacturing.ConfigID{
			Value: "test_hwid",
		},
		DevicePhase: manufacturing.ManufacturingConfig_PHASE_DVT,
	},
	Machine: &machine,
}

var baseDUTDims = swarming.Dimensions{
	"dut_id":                           {"test_dut"},
	"dut_name":                         {"test_host"},
	"hwid":                             {"test_hwid"},
	"label-arc":                        {"True"},
	"label-bluetooth":                  {"True"},
	"label-board":                      {"coral"},
	"label-bot_size":                   {"BOT_SIZE_LARGE"},
	"label-carrier":                    {"CARRIER_ATT"},
	"label-cbx":                        {"False"},
	"label-cellular_variant":           {"test_variant"},
	"label-chameleon":                  {"True"},
	"label-chameleon_connection_types": {"CHAMELEON_CONNECTION_TYPE_DP"},
	"label-chameleon_type":             {"CHAMELEON_TYPE_V2"},
	"label-chaos_dut":                  {"True"},
	"label-conductive":                 {"True"},
	"label-cts_abi":                    {"CTS_ABI_ARM"},
	"label-cts_cpu":                    {"CTS_CPU_ARM"},
	"label-dlm_sku_id":                 {"12345"},
	"label-ec_type":                    {"EC_TYPE_CHROME_OS"},
	"label-hangout_app":                {"True"},
	"label-huddly":                     {"True"},
	"label-hwid_sku":                   {"test_sku"},
	"label-meet_app":                   {"True"},
	"label-mimo":                       {"True"},
	"label-multiduts":                  {"False"},
	"label-model":                      {"test_model"},
	"label-modem_imei":                 {"imei"},
	"label-modem_sim_count":            {"1"},
	"label-modem_supported_bands":      {"bands"},
	"label-modem_type":                 {"MODEM_TYPE_QUALCOMM_SC7180"},
	"label-os_type":                    {"OS_TYPE_CROS"},
	"label-phase":                      {"PHASE_DVT"},
	"label-platform":                   {"coral"},
	"label-pool":                       {"DUT_POOL_QUOTA", "hotrod"},
	"label-power":                      {"AC_only"},
	"label-router_802_11ax":            {"True"},
	"label-servo":                      {"True"},
	"label-servo_component":            {"servo_v4", "servo_micro"},
	"label-servo_state":                {"BROKEN"},
	"label-sim_1_0_carrier_name":       {"NETWORK_ATT"},
	"label-sim_1_0_iccid":              {"iccid1"},
	"label-sim_1_0_own_number":         {"123456789"},
	"label-sim_1_0_pin":                {"pin1"},
	"label-sim_1_0_puk":                {"puk1"},
	"label-sim_1_eid":                  {"eid"},
	"label-sim_1_num_profiles":         {"1"},
	"label-sim_1_test_esim":            {"True"},
	"label-sim_1_type":                 {"SIM_DIGITAL"},
	"label-sim_slot_id":                {"1"},
	"label-sku":                        {"test_variant"},
	"label-starfish_slot_mapping":      {"test-map-key:test-value"},
	"label-storage":                    {"ssd"},
	"label-test_audiojack":             {"True"},
	"label-variant":                    {"test_variant"},
	"label-video_acceleration":         {"VIDEO_ACCELERATION_ENC_H264"},
	"label-wifi_router_features":       {"WIFI_ROUTER_FEATURE_IEEE_802_11_N"},
	"label-wifi_router_models":         {"gale"},
	"label-wifi_state":                 {"ACCEPTABLE"},
	"label-wificell":                   {"True"},
	"label-wifi_on_site":               {"True"},
	"label-working_bluetooth_btpeer":   {"1", "2", "3"},
	"serial_number":                    {"test_serial"},
	"ufs_zone":                         {"UFS_TEST_ZONE"},
	"version_info_os_type":             {"CHROMEOS"},
	"version_info_os":                  {"coral-release/R100.12000.00"},
	"version_info_rw_firmware":         {"Google_Coral.10068.101.0"},
	"version_info_ro_firmware":         {"Google_Coral.10068.101.0"},
}

var fullDUTDims = swarming.Dimensions{
	"dut_id":                           {"test_dut"},
	"dut_name":                         {"test_host"},
	"dut_state":                        {"ready"},
	"hwid":                             {"test_hwid"},
	"label-arc":                        {"True"},
	"label-bluetooth":                  {"True"},
	"label-board":                      {"coral"},
	"label-bot_size":                   {"BOT_SIZE_LARGE"},
	"label-carrier":                    {"CARRIER_ATT"},
	"label-cbx":                        {"False"},
	"label-cellular_variant":           {"test_variant"},
	"label-chameleon":                  {"True"},
	"label-chameleon_connection_types": {"CHAMELEON_CONNECTION_TYPE_DP"},
	"label-chameleon_type":             {"CHAMELEON_TYPE_V2"},
	"label-chaos_dut":                  {"True"},
	"label-conductive":                 {"True"},
	"label-cts_abi":                    {"CTS_ABI_ARM"},
	"label-cts_cpu":                    {"CTS_CPU_ARM"},
	"label-dlm_sku_id":                 {"12345"},
	"label-ec_type":                    {"EC_TYPE_CHROME_OS"},
	"label-hangout_app":                {"True"},
	"label-huddly":                     {"True"},
	"label-hwid_sku":                   {"test_sku"},
	"label-meet_app":                   {"True"},
	"label-mimo":                       {"True"},
	"label-multiduts":                  {"False"},
	"label-model":                      {"test_model"},
	"label-modem_imei":                 {"imei"},
	"label-modem_sim_count":            {"1"},
	"label-modem_supported_bands":      {"bands"},
	"label-modem_type":                 {"MODEM_TYPE_QUALCOMM_SC7180"},
	"label-os_type":                    {"OS_TYPE_CROS"},
	"label-phase":                      {"PHASE_DVT"},
	"label-platform":                   {"coral"},
	"label-pool":                       {"DUT_POOL_QUOTA", "hotrod"},
	"label-power":                      {"AC_only"},
	"label-router_802_11ax":            {"True"},
	"label-servo":                      {"True"},
	"label-servo_component":            {"servo_v4", "servo_micro"},
	"label-servo_state":                {"BROKEN"},
	"label-sim_1_0_carrier_name":       {"NETWORK_ATT"},
	"label-sim_1_0_iccid":              {"iccid1"},
	"label-sim_1_0_own_number":         {"123456789"},
	"label-sim_1_0_pin":                {"pin1"},
	"label-sim_1_0_puk":                {"puk1"},
	"label-sim_1_eid":                  {"eid"},
	"label-sim_1_num_profiles":         {"1"},
	"label-sim_1_test_esim":            {"True"},
	"label-sim_1_type":                 {"SIM_DIGITAL"},
	"label-sim_slot_id":                {"1"},
	"label-sku":                        {"test_variant"},
	"label-starfish_slot_mapping":      {"test-map-key:test-value"},
	"label-storage":                    {"ssd"},
	"label-test_audiojack":             {"True"},
	"label-variant":                    {"test_variant"},
	"label-video_acceleration":         {"VIDEO_ACCELERATION_ENC_H264"},
	"label-wifi_router_features":       {"WIFI_ROUTER_FEATURE_IEEE_802_11_N"},
	"label-wifi_router_models":         {"gale"},
	"label-wifi_state":                 {"ACCEPTABLE"},
	"label-wificell":                   {"True"},
	"label-wifi_on_site":               {"True"},
	"label-working_bluetooth_btpeer":   {"1", "2", "3"},
	"serial_number":                    {"test_serial"},
	"ufs_zone":                         {"UFS_TEST_ZONE"},
	"version_info_os_type":             {"CHROMEOS"},
	"version_info_os":                  {"coral-release/R100.12000.00"},
	"version_info_rw_firmware":         {"Google_Coral.10068.101.0"},
	"version_info_ro_firmware":         {"Google_Coral.10068.101.0"},
}

func getMockDUTDeviceData(data *ufspb.ChromeOSDeviceData, dc *deviceconfig.Config) *ufspb.ChromeOSDeviceData {
	dutV1, err := osutil.AdaptToV1DutSpec(data, dc)
	if err != nil {
		return nil
	}
	data.DutV1 = dutV1
	return data
}

func TestGetDUTBotDimensions(t *testing.T) {
	ctx := context.Background()
	r := func(e error) { fmt.Printf("sanitize dimensions: %s\n", e) }
	tests := []struct {
		name         string
		dutState     dutstate.Info
		ufsData      *ufspb.ChromeOSDeviceData
		expectedDims swarming.Dimensions
	}{
		{
			name:         "empty DUT state",
			dutState:     dutstate.Info{},
			ufsData:      getMockDUTDeviceData(&osDeviceData, deviceConfig),
			expectedDims: baseDUTDims,
		},
		{
			name: "full DUT data",
			dutState: dutstate.Info{
				State: dutstate.Ready,
			},
			ufsData:      getMockDUTDeviceData(&osDeviceData, deviceConfig),
			expectedDims: fullDUTDims,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dims := GetDUTBotDims(ctx, r, tt.dutState, tt.ufsData)
			if !reflect.DeepEqual(tt.expectedDims, dims) {
				for k, v := range tt.expectedDims {
					if !reflect.DeepEqual(v, dims[k]) {
						t.Errorf("Diff in dim %s; expected %v, got %v", k, v, dims[k])
					}
					if _, ok := dims[k]; !ok {
						t.Errorf("Missing dim %s; expected %v", k, v)
					}
				}
				for k, v := range dims {
					if !reflect.DeepEqual(v, tt.expectedDims[k]) {
						t.Errorf("Extra dim %s; expected none, got %v", k, v)
					}
				}
			}
		})
	}
}
