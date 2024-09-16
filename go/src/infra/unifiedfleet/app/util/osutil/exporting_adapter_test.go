// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package osutil

import (
	"testing"

	"github.com/golang/protobuf/proto"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	deviceconfig "go.chromium.org/chromiumos/infra/proto/go/device"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/libs/skylab/inventory"
	ufspb "infra/unifiedfleet/api/v1/models"
	chromeosLab "infra/unifiedfleet/api/v1/models/chromeos/lab"
	manufacturing "infra/unifiedfleet/api/v1/models/chromeos/manufacturing"
)

var servoInV2 = chromeosLab.Servo{
	ServoHostname:       "test_servo",
	ServoPort:           int32(9999),
	ServoSerial:         "test_servo_serial",
	ServoType:           "v3",
	ServoSetup:          chromeosLab.ServoSetupType_SERVO_SETUP_DUAL_V4,
	ServoFwChannel:      chromeosLab.ServoFwChannel_SERVO_FW_ALPHA,
	DockerContainerName: "test_servod_docker",
	ServoTopology: &chromeosLab.ServoTopology{
		Main: &chromeosLab.ServoTopologyItem{
			Type:         "servo_v4",
			SysfsProduct: "Servo V4",
			Serial:       "C1903145591",
			UsbHubPort:   "6.4.1",
			FwVersion:    "test_firmware_v1",
		},
		Children: []*chromeosLab.ServoTopologyItem{
			{
				Type:         "ccd_cr50",
				SysfsProduct: "Cr50",
				Serial:       "0681D03A-92DCCD64",
				UsbHubPort:   "6.4.2",
				FwVersion:    "test_firmware_v1",
			},
			{
				Type:         "c2d2",
				SysfsProduct: "C2D2",
				Serial:       "0681D03A-YYYYYYYY",
				UsbHubPort:   "6.4.3",
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
						Dut: &chromeosLab.DeviceUnderTest{
							Hostname: "test_host",
							Pools:    []string{"DUT_POOL_QUOTA", "hotrod"},
							Peripherals: &chromeosLab.Peripherals{
								Servo: &servoInV2,
								Chameleon: &chromeosLab.Chameleon{
									ChameleonPeripherals: []chromeosLab.ChameleonType{
										chromeosLab.ChameleonType_CHAMELEON_TYPE_V2,
									},
									ChameleonConnectionTypes: []chromeosLab.ChameleonConnectionType{
										chromeosLab.ChameleonConnectionType_CHAMELEON_CONNECTION_TYPE_DP,
										chromeosLab.ChameleonConnectionType_CHAMELEON_CONNECTION_TYPE_HDMI,
									},
									Hostname:   "test-chameleon",
									AudioBoard: true,
								},
								Rpm: &chromeosLab.OSRPM{
									PowerunitName:   "test_power_unit_name",
									PowerunitOutlet: "test_power_unit_outlet",
									PowerunitType:   chromeosLab.OSRPM_TYPE_SENTRY,
								},
								ConnectedCamera: []*chromeosLab.Camera{
									{
										CameraType: chromeosLab.CameraType_CAMERA_HUDDLY,
									},
									{
										CameraType: chromeosLab.CameraType_CAMERA_PTZPRO2,
									},
								},
								Audio: &chromeosLab.Audio{
									AudioBox:   true,
									AudioCable: true,
									Atrus:      true,
								},
								Wifi: &chromeosLab.Wifi{
									Wificell:    true,
									AntennaConn: chromeosLab.Wifi_CONN_CONDUCTIVE,
									Router:      chromeosLab.Wifi_ROUTER_802_11AX,
									WifiRouterFeatures: []labapi.WifiRouterFeature{
										labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
										labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
									},
									WifiRouters: []*chromeosLab.WifiRouter{
										{
											Model: "gale",
											SupportedFeatures: []labapi.WifiRouterFeature{
												labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
												labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
											},
										},
										{
											Model: "OPENWRT[Ubiquiti_Unifi_6_Lite]",
											SupportedFeatures: []labapi.WifiRouterFeature{
												labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
												labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
												labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX,
											},
										},
									},
								},
								Touch: &chromeosLab.Touch{
									Mimo: true,
								},
								Carrier:           "att",
								SupportedCarriers: []string{"att", "tmobile", "verizon"},
								Camerabox:         true,
								CameraboxInfo: &chromeosLab.Camerabox{
									Facing: chromeosLab.Camerabox_FACING_BACK,
									Light:  chromeosLab.Camerabox_LIGHT_LED,
								},
								Chaos: true,
								Cable: []*chromeosLab.Cable{
									{
										Type: chromeosLab.CableType_CABLE_AUDIOJACK,
									},
									{
										Type: chromeosLab.CableType_CABLE_USBAUDIO,
									},
									{
										Type: chromeosLab.CableType_CABLE_USBPRINTING,
									},
									{
										Type: chromeosLab.CableType_CABLE_HDMIAUDIO,
									},
								},
								SmartUsbhub:         true,
								StarfishSlotMapping: "test-map-key:test-value",
								PasitHost2: &labapi.PasitHost{
									Devices: []*labapi.PasitHost_Device{
										{
											Type: labapi.PasitHost_Device_CAMERA,
										},
										{
											Type: labapi.PasitHost_Device_UNKNOWN,
										},
										{
											Type: labapi.PasitHost_Device_DUT,
										},
										{
											Type: labapi.PasitHost_Device_MONITOR,
										},
										{
											Type: labapi.PasitHost_Device_MONITOR,
										},
										{
											Type: labapi.PasitHost_Device_SWITCH_FIXTURE,
										},
										{
											Type: labapi.PasitHost_Device_DOCKING_STATION,
										},
									},
								},
							},
							Licenses: []*chromeosLab.License{
								{
									Type:       chromeosLab.LicenseType_LICENSE_TYPE_WINDOWS_10_PRO,
									Identifier: "my-windows-identifier-A001",
								},
								{
									Type:       chromeosLab.LicenseType_LICENSE_TYPE_MS_OFFICE_STANDARD,
									Identifier: "my-office-identifier-B002",
								},
							},
							Modeminfo: &chromeosLab.ModemInfo{
								Type:           chromeosLab.ModemType_MODEM_TYPE_RW101,
								Imei:           "imei",
								SupportedBands: "bands",
								SimCount:       1,
								ModelVariant:   "test_variant",
							},
							Siminfo: []*chromeosLab.SIMInfo{
								{
									Type:     chromeosLab.SIMType_SIM_DIGITAL,
									SlotId:   1,
									Eid:      "eid",
									TestEsim: true,
									ProfileInfo: []*chromeosLab.SIMProfileInfo{
										{
											Iccid:       "iccid1",
											SimPin:      "pin1",
											SimPuk:      "puk1",
											CarrierName: chromeosLab.NetworkProvider_NETWORK_ATT,
											OwnNumber:   "123456789",
											State:       chromeosLab.SIMProfileInfo_UNSPECIFIED,
										},
										{
											Iccid:       "iccid2",
											SimPin:      "pin2",
											SimPuk:      "puk2",
											CarrierName: chromeosLab.NetworkProvider_NETWORK_TEST,
											OwnNumber:   "234567890",
											State:       chromeosLab.SIMProfileInfo_BROKEN,
										},
										{
											Iccid:       "iccid3",
											SimPin:      "pin3",
											SimPuk:      "puk3",
											CarrierName: chromeosLab.NetworkProvider_NETWORK_SPRINT,
											OwnNumber:   "345678912",
											State:       chromeosLab.SIMProfileInfo_NO_NETWORK,
										},
										{
											Iccid:       "iccid4",
											SimPin:      "pin4",
											SimPuk:      "puk4",
											CarrierName: chromeosLab.NetworkProvider_NETWORK_FI,
											OwnNumber:   "456789123",
											State:       chromeosLab.SIMProfileInfo_UNSPECIFIED,
										},
										{
											Iccid:       "iccid5",
											SimPin:      "pin5",
											SimPuk:      "puk5",
											CarrierName: chromeosLab.NetworkProvider_NETWORK_CBRS,
											OwnNumber:   "567891234",
											State:       chromeosLab.SIMProfileInfo_UNSPECIFIED,
										},
										{
											Iccid:       "iccid6",
											SimPin:      "pin6",
											SimPuk:      "puk6",
											CarrierName: chromeosLab.NetworkProvider_NETWORK_POVO,
											OwnNumber:   "678912345",
											State:       chromeosLab.SIMProfileInfo_UNSPECIFIED,
										},
										{
											Iccid:       "iccid7",
											SimPin:      "pin7",
											SimPuk:      "puk7",
											CarrierName: chromeosLab.NetworkProvider_NETWORK_HANSHIN,
											OwnNumber:   "789123457",
											State:       chromeosLab.SIMProfileInfo_UNSPECIFIED,
										},
									},
								},
								{
									Type:   chromeosLab.SIMType_SIM_PHYSICAL,
									SlotId: 2,
									ProfileInfo: []*chromeosLab.SIMProfileInfo{
										{
											Iccid:       "iccid1",
											SimPin:      "pin1",
											SimPuk:      "puk1",
											CarrierName: chromeosLab.NetworkProvider_NETWORK_ATT,
											OwnNumber:   "123456789",
											State:       chromeosLab.SIMProfileInfo_WORKING,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	},
}

var devUFSState = chromeosLab.DutState{
	Id: &chromeosLab.ChromeOSDeviceID{
		Value: "test_dut",
	},
	Servo:                    chromeosLab.PeripheralState_BROKEN,
	Chameleon:                chromeosLab.PeripheralState_WORKING,
	AudioLoopbackDongle:      chromeosLab.PeripheralState_NOT_CONNECTED,
	WorkingBluetoothBtpeer:   3,
	Cr50Phase:                chromeosLab.DutState_CR50_PHASE_PVT,
	Cr50KeyEnv:               chromeosLab.DutState_CR50_KEYENV_PROD,
	StorageState:             chromeosLab.HardwareState_HARDWARE_NORMAL,
	ServoUsbState:            chromeosLab.HardwareState_HARDWARE_NEED_REPLACEMENT,
	BatteryState:             chromeosLab.HardwareState_HARDWARE_UNKNOWN,
	WifiState:                chromeosLab.HardwareState_HARDWARE_ACCEPTABLE,
	BluetoothState:           chromeosLab.HardwareState_HARDWARE_NORMAL,
	CellularModemState:       chromeosLab.HardwareState_HARDWARE_NORMAL,
	StarfishState:            chromeosLab.PeripheralState_BROKEN,
	RpmState:                 chromeosLab.PeripheralState_WORKING,
	WifiPeripheralState:      chromeosLab.PeripheralState_WORKING,
	PeripheralBtpeerState:    chromeosLab.PeripheralState_WORKING,
	AudioLatencyToolkitState: chromeosLab.PeripheralState_WORKING,
	GpuId:                    "test_gpu_id",
	AudioBeamforming:         "intelligo",
	CameraState:              chromeosLab.HardwareState_HARDWARE_NORMAL,
	FingerprintMcu:           "test_fingerprint_mcu",
}

var labstationMachine = ufspb.Machine{
	Name:         "test_labstation",
	SerialNumber: "labstation_serial",
	Device: &ufspb.Machine_ChromeosMachine{
		ChromeosMachine: &ufspb.ChromeOSMachine{
			Hwid:        "labstation_hwid",
			BuildTarget: "guado",
			Model:       "test_model",
			Sku:         "",
		},
	},
}

var labstationLSE = ufspb.MachineLSE{
	Name:     "test_labstation_host",
	Hostname: "test_labstation_host",
	Machines: []string{"test_labstation"},
	Lse: &ufspb.MachineLSE_ChromeosMachineLse{
		ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
			ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
				DeviceLse: &ufspb.ChromeOSDeviceLSE{
					Device: &ufspb.ChromeOSDeviceLSE_Labstation{
						Labstation: &chromeosLab.Labstation{
							Hostname: "test_labstation_host",
							Pools:    []string{"labstation_main"},
							Servos:   []*chromeosLab.Servo{},
							Rpm: &chromeosLab.OSRPM{
								PowerunitName:   "test_power_unit_name",
								PowerunitOutlet: "test_power_unit_outlet2",
								PowerunitType:   chromeosLab.OSRPM_TYPE_SENTRY,
							},
						},
					},
				},
			},
		},
	},
}

var devboardMachine = ufspb.Machine{
	Name:         "test_devboard",
	SerialNumber: "devboard_serial",
	Device: &ufspb.Machine_Devboard{
		Devboard: &ufspb.Devboard{
			Board: &ufspb.Devboard_Andreiboard{
				Andreiboard: &ufspb.Andreiboard{
					UltradebugSerial: "test_serial",
				},
			},
		},
	},
}

var devboardLSE = ufspb.MachineLSE{
	Name:     "test_devboard_host",
	Hostname: "test_devboard_host",
	Machines: []string{"test_devboard"},
	Lse: &ufspb.MachineLSE_ChromeosMachineLse{
		ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
			ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
				DeviceLse: &ufspb.ChromeOSDeviceLSE{
					Device: &ufspb.ChromeOSDeviceLSE_Devboard{
						Devboard: &chromeosLab.Devboard{
							Pools: []string{"devboard_main"},
							Servo: &chromeosLab.Servo{
								ServoHostname: "test_servo",
								ServoPort:     9999,
								ServoSerial:   "test_servo_serial",
							},
						},
					},
				},
			},
		},
	},
}

var labstationDevConfig = deviceconfig.Config{
	Id: &deviceconfig.ConfigId{
		PlatformId: &deviceconfig.PlatformId{
			Value: "guado",
		},
		ModelId: &deviceconfig.ModelId{
			Value: "test_model",
		},
		VariantId: &deviceconfig.VariantId{
			Value: "",
		},
	},
}

var labstationManufacturingconfig = manufacturing.ManufacturingConfig{
	ManufacturingId: &manufacturing.ConfigID{
		Value: "labstation_hwid",
	},
}

var DeviceConfig = &deviceconfig.Config{
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
	FormFactor: deviceconfig.Config_FORM_FACTOR_CHROMEBASE,
	GpuFamily:  "test_gpu",
	Graphics:   deviceconfig.Config_GRAPHICS_GLE,
	HardwareFeatures: []deviceconfig.Config_HardwareFeature{
		deviceconfig.Config_HARDWARE_FEATURE_DETACHABLE_KEYBOARD,
		deviceconfig.Config_HARDWARE_FEATURE_FINGERPRINT,
	},
	Power:   deviceconfig.Config_POWER_SUPPLY_AC_ONLY,
	Storage: deviceconfig.Config_STORAGE_SSD,
	VideoAccelerationSupports: []deviceconfig.Config_VideoAcceleration{
		deviceconfig.Config_VIDEO_ACCELERATION_ENC_H264,
		deviceconfig.Config_VIDEO_ACCELERATION_ENC_VP8,
		deviceconfig.Config_VIDEO_ACCELERATION_ENC_VP9,
	},
	Cpu: deviceconfig.Config_ARM64,
}

var data = ufspb.ChromeOSDeviceData{
	LabConfig: &lse,
	DutState:  &devUFSState,
	HwidData: &ufspb.HwidData{
		Sku:      "test_sku",
		Variant:  "test_variant",
		Stylus:   true,
		Touchpad: true,
	},
	ManufacturingConfig: &manufacturing.ManufacturingConfig{
		ManufacturingId: &manufacturing.ConfigID{
			Value: "test_hwid",
		},
		DevicePhase: manufacturing.ManufacturingConfig_PHASE_DVT,
	},
	Machine: &machine,
}

const dutTextProto = `
common {
	attributes {
		key: "HWID",
		value: "test_hwid",
	}
	attributes {
		key: "powerunit_hostname",
		value: "test_power_unit_name",
	}
	attributes {
		key: "powerunit_outlet",
		value: "test_power_unit_outlet",
	}
	attributes {
		key: "powerunit_type",
		value: "SENTRY",
	}
	attributes {
		key: "serial_number"
		value: "test_serial"
	}
	attributes {
		key: "servo_host"
		value: "test_servo"
	}
	attributes {
		key: "servod_docker"
		value: "test_servod_docker"
	}
	attributes {
		key: "servo_port"
		value: "9999"
	}
	attributes {
		key: "servo_serial",
		value: "test_servo_serial",
	}
	attributes {
		key: "servo_type",
		value: "v3",
	}
	attributes {
		key: "servo_setup",
		value: "DUAL_V4",
	}
	attributes {
		key: "servo_fw_channel",
		value: "ALPHA",
	}
	hostname: "test_host"
	hwid: "test_hwid"
	id: "test_dut"
	serial_number: "test_serial"
	labels {
		arc: true
		board: "coral"
		bot_size: BOT_SIZE_LARGE
		brand: ""
		capabilities {
			atrus: true
			bluetooth: true
			carrier: CARRIER_ATT
			supported_carriers: CARRIER_ATT
			supported_carriers: CARRIER_TMOBILE
			supported_carriers: CARRIER_VERIZON
			cbx: CBX_STATE_FALSE
			cbx_branding: CBX_BRANDING_UNSPECIFIED
			detachablebase: true
			fingerprint: true
			fingerprint_mcu: "test_fingerprint_mcu"
			form_factor: FORM_FACTOR_CHROMEBASE
			gpu_family: "test_gpu"
			gpu_id: "test_gpu_id"
			graphics: "gles"
			power: "AC_only"
			starfish_slot_mapping: "test-map-key:test-value"
			storage: "ssd"
			touchpad: true
			video_acceleration: VIDEO_ACCELERATION_ENC_H264
			video_acceleration: VIDEO_ACCELERATION_ENC_VP8
			video_acceleration: VIDEO_ACCELERATION_ENC_VP9
		}
		cr50_phase: CR50_PHASE_PVT
		cts_abi: CTS_ABI_ARM
		cts_cpu: CTS_CPU_ARM
		cr50_ro_keyid: "prod"
		ec_type: EC_TYPE_CHROME_OS
		hwid_sku: "test_sku"
		dlm_sku_id: "12345"
		licenses: {
			type: 1
			identifier: "my-windows-identifier-A001"
		}
		licenses: {
			type: 2
			identifier: "my-office-identifier-B002"
		}
		modeminfo {
			type: MODEM_TYPE_RW101
			imei: "imei"
			supported_bands: "bands"
			sim_count: 1
			model_variant: "test_variant"
		}
		siminfo {
			slot_id: 1
			type: SIM_DIGITAL
			eid: "eid"
			test_esim: true
			profile_info {
				iccid: "iccid1"
				sim_pin: "pin1"
				sim_puk: "puk1"
				carrier_name: NETWORK_ATT
				own_number: "123456789"
			}
			profile_info {
				iccid: "iccid2"
				sim_pin: "pin2"
				sim_puk: "puk2"
				carrier_name: NETWORK_TEST
				own_number: "234567890"
			}
			profile_info {
				iccid: "iccid3"
				sim_pin: "pin3"
				sim_puk: "puk3"
				carrier_name: NETWORK_SPRINT
				own_number: "345678912"
			}
			profile_info {
				iccid: "iccid4"
				sim_pin: "pin4"
				sim_puk: "puk4"
				carrier_name: NETWORK_FI
				own_number: "456789123"
			}
			profile_info {
				iccid: "iccid5"
				sim_pin: "pin5"
				sim_puk: "puk5"
				carrier_name: NETWORK_CBRS
				own_number: "567891234"
			}
			profile_info {
				iccid: "iccid6"
				sim_pin: "pin6"
				sim_puk: "puk6"
				carrier_name: NETWORK_POVO
					own_number: "678912345"
			}
			profile_info {
				iccid: "iccid7"
				sim_pin: "pin7"
				sim_puk: "puk7"
				carrier_name: NETWORK_HANSHIN
				own_number: "789123457"
			}
		}
		siminfo {
			slot_id: 2
			type: SIM_PHYSICAL
			eid: ""
			test_esim: false
			profile_info {
				iccid: "iccid1"
				sim_pin: "pin1"
				sim_puk: "puk1"
				carrier_name: NETWORK_ATT
				own_number: "123456789"
			}
		}
		model: "test_model"
		os_type: OS_TYPE_CROS
		sku: "test_variant"
		peripherals {
			audio_board: true
			audio_box: true
			audio_cable: true
			audio_loopback_dongle: false
			chameleon: true
			chameleon_type: CHAMELEON_TYPE_V2
			chameleon_connection_types: CHAMELEON_CONNECTION_TYPE_DP
			chameleon_connection_types: CHAMELEON_CONNECTION_TYPE_HDMI
			audiobox_jackplugger_state: AUDIOBOX_JACKPLUGGER_UNSPECIFIED
			trrs_type: TRRS_TYPE_UNSPECIFIED
			conductive: true
			huddly: true
			mimo: true
			ptzpro2: true
			camerabox: true
			camerabox_facing: CAMERABOX_FACING_BACK
			camerabox_light: CAMERABOX_LIGHT_LED
			servo: true
			servo_component: "servo_v4"
			servo_component: "servo_micro"
			servo_topology: {
				main: {
					usb_hub_port: "6.4.1"
					serial: "C1903145591"
					type: "servo_v4"
					sysfs_product: "Servo V4"
					fw_version: "test_firmware_v1"
				}
				children: {
					usb_hub_port: "6.4.2"
					serial: "0681D03A-92DCCD64"
					type: "ccd_cr50"
					sysfs_product: "Cr50"
					fw_version: "test_firmware_v1"
				}
				children: {
					usb_hub_port: "6.4.3"
					serial: "0681D03A-YYYYYYYY"
					type: "c2d2"
					sysfs_product: "C2D2"
					fw_version: "test_firmware_v1"
				}
			}
			servo_state: BROKEN
			servo_type: "v3"
			rpm_state: WORKING
			peripheral_btpeer_state: WORKING
			peripheral_wifi_state: WORKING
			wifi_router_features: WIFI_ROUTER_FEATURE_IEEE_802_11_N
			wifi_router_features: WIFI_ROUTER_FEATURE_IEEE_802_11_AC
			wifi_router_models: "gale"
			wifi_router_models: "OPENWRT[Ubiquiti_Unifi_6_Lite]"
			chameleon_state: WORKING
			smart_usbhub: true
			storage_state: HARDWARE_NORMAL,
			servo_usb_state: HARDWARE_NEED_REPLACEMENT,
			battery_state: HARDWARE_UNKNOWN,
			wifi_state: HARDWARE_ACCEPTABLE,
			bluetooth_state: HARDWARE_NORMAL,
			cellular_modem_state: HARDWARE_NORMAL
			working_sims: 1
			sim_state: BROKEN
			starfish_state: BROKEN
			wificell: true
			router_802_11ax: true
			working_bluetooth_btpeer: 3
			hmr_state: UNKNOWN
			audio_latency_toolkit_state: WORKING
			stylus: true
			pasit_components: "CAMERA-1"
			pasit_components: "MONITOR-1"
			pasit_components: "MONITOR-2"
			pasit_components: "DOCKING_STATION-1"
			amt_manager_state: UNKNOWN
			audio_beamforming: "intelligo"
			camera_state: HARDWARE_NORMAL
		}
		phase: PHASE_DVT
		platform: "coral"
		test_coverage_hints {
			chaos_dut: true
			hangout_app: true
			meet_app: true
			test_audiojack: true
			test_hdmiaudio: true
			test_usbaudio: true
			test_usbprinting: true
		}
		variant: "test_variant"
		critical_pools: DUT_POOL_QUOTA
		self_serve_pools: "hotrod"
		wifi_chip: "",
		wifi_on_site: true
	}
}
`

const labstationProtoFromV2WithDutState = `
common {
	attributes {
		key: "HWID",
		value: "labstation_hwid",
	}
	attributes {
		key: "powerunit_hostname",
		value: "test_power_unit_name",
	}
	attributes {
		key: "powerunit_outlet",
		value: "test_power_unit_outlet2",
	}
	attributes {
		key: "powerunit_type",
		value: "SENTRY",
	}
	attributes {
		key: "serial_number"
		value: "labstation_serial"
	}
	id: "test_labstation"
	hostname: "test_labstation_host"
	serial_number: "labstation_serial"
	labels {
		arc: false
		os_type: OS_TYPE_LABSTATION
		self_serve_pools: "labstation_main"
		model: "test_model"
		board: "guado"
		brand: ""
		sku: ""
		capabilities {
			atrus: false
			bluetooth: false
			carrier: CARRIER_INVALID
			detachablebase: false
			fingerprint: false
			fingerprint_mcu: "test_fingerprint_mcu"
			flashrom: false
			gpu_family: ""
			gpu_id: "test_gpu_id"
			graphics: ""
			hotwording: false
			internal_display: false
			lucidsleep: false
			modem: ""
			power: "AC_only"
			starfish_slot_mapping: ""
			storage: ""
			webcam: false
			telephony: ""
			touchpad: false
			touchscreen: false
		}
		cr50_phase: CR50_PHASE_PVT
		cr50_ro_keyid: "prod"
		cr50_ro_version: ""
		cr50_rw_keyid: ""
		cr50_rw_version: ""
		ec_type: EC_TYPE_INVALID
		hwid_sku: ""
		peripherals {
			audio_board: false
			audio_box: false
			audio_loopback_dongle: false
			chameleon: false
			chameleon_type: CHAMELEON_TYPE_INVALID
			conductive: false
			huddly: false
			mimo: false
			servo: true
			servo_state: BROKEN
			smart_usbhub: false
			stylus: false
			camerabox: false
			wificell: false
			router_802_11ax: false
			working_bluetooth_btpeer: 3
			storage_state: HARDWARE_NORMAL
			servo_usb_state: HARDWARE_NEED_REPLACEMENT
			battery_state: HARDWARE_UNKNOWN
			wifi_state: HARDWARE_ACCEPTABLE
			bluetooth_state: HARDWARE_NORMAL
			cellular_modem_state: HARDWARE_NORMAL
			starfish_state: BROKEN
			rpm_state: WORKING
			peripheral_btpeer_state: WORKING
			peripheral_wifi_state: WORKING
			chameleon_state: WORKING
			hmr_state: UNKNOWN
			audio_latency_toolkit_state: WORKING
			amt_manager_state: UNKNOWN
			audio_beamforming: "intelligo"
			camera_state: HARDWARE_NORMAL
		}
		platform:""
		test_coverage_hints {
				chaos_dut: false
				chaos_nightly: false
				chromesign: false
				hangout_app: false
				meet_app: false
				recovery_test: false
				test_audiojack: false
				test_hdmiaudio: false
				test_usbaudio: false
				test_usbprinting: false
				usb_detect: false
				use_lid: false
		}
		wifi_chip: "",
		wifi_on_site: true
	}
}
`

func TestAdaptToV1DutSpec(t *testing.T) {
	t.Parallel()

	ftt.Run("Verify V2 => V1", t, func(t *ftt.Test) {
		var d1 inventory.DeviceUnderTest
		err := proto.UnmarshalText(dutTextProto, &d1)
		assert.Loosely(t, err, should.BeNil)
		s1, err := inventory.WriteDUTToString(&d1)
		assert.Loosely(t, err, should.BeNil)
		dataCopy := proto.Clone(&data).(*ufspb.ChromeOSDeviceData)

		t.Run("empty input", func(t *ftt.Test) {
			_, err := AdaptToV1DutSpec(&ufspb.ChromeOSDeviceData{}, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("chromeosdevicedata is nil to adapt"))
		})
		t.Run("empty hwid data", func(t *ftt.Test) {
			dataCopy.HwidData = nil
			d, err := AdaptToV1DutSpec(dataCopy, DeviceConfig)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, d.GetCommon().GetHostname(), should.Equal("test_host"))
		})
		t.Run("empty device config", func(t *ftt.Test) {
			dataCopy.DeviceConfig = nil
			d, err := AdaptToV1DutSpec(dataCopy, DeviceConfig)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, d.GetCommon().GetHostname(), should.Equal("test_host"))
		})
		t.Run("empty manufacturing config", func(t *ftt.Test) {
			dataCopy.ManufacturingConfig = nil
			d, err := AdaptToV1DutSpec(dataCopy, DeviceConfig)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, d.GetCommon().GetHostname(), should.Equal("test_host"))
		})
		t.Run("servo_state is UNKNOWN/false by default", func(t *ftt.Test) {
			dataCopy.DutState = &chromeosLab.DutState{}
			d, err := AdaptToV1DutSpec(dataCopy, DeviceConfig)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, *d.GetCommon().GetLabels().GetPeripherals().ServoState, should.Equal(inventory.PeripheralState_UNKNOWN))
			assert.Loosely(t, *d.GetCommon().GetLabels().GetPeripherals().Servo, should.BeFalse)
		})
		t.Run("servo_state is broken", func(t *ftt.Test) {
			dataCopy.DutState = &chromeosLab.DutState{}
			dataCopy.DutState.Servo = chromeosLab.PeripheralState_BROKEN
			d, err := AdaptToV1DutSpec(dataCopy, DeviceConfig)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, *d.GetCommon().GetLabels().GetPeripherals().ServoState,
				should.Equal(
					inventory.PeripheralState_BROKEN))
			assert.Loosely(t, *d.GetCommon().GetLabels().GetPeripherals().Servo, should.Equal(true))
		})
		t.Run("servo_state is wrong_config", func(t *ftt.Test) {
			dataCopy.DutState = &chromeosLab.DutState{}
			dataCopy.DutState.Servo = chromeosLab.PeripheralState_WRONG_CONFIG
			d, err := AdaptToV1DutSpec(dataCopy, DeviceConfig)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, *d.GetCommon().GetLabels().GetPeripherals().ServoState,
				should.Equal(
					inventory.PeripheralState_WRONG_CONFIG))
			assert.Loosely(t, *d.GetCommon().GetLabels().GetPeripherals().Servo, should.Equal(true))
		})
		t.Run("servo_state is working", func(t *ftt.Test) {
			dataCopy.DutState = &chromeosLab.DutState{}
			dataCopy.DutState.Servo = chromeosLab.PeripheralState_WORKING
			d, err := AdaptToV1DutSpec(dataCopy, DeviceConfig)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, *d.GetCommon().GetLabels().GetPeripherals().ServoState,
				should.Equal(
					inventory.PeripheralState_WORKING))
			assert.Loosely(t, *d.GetCommon().GetLabels().GetPeripherals().Servo, should.Equal(true))
		})
		t.Run("servo_state is not_connected", func(t *ftt.Test) {
			dataCopy.DutState = &chromeosLab.DutState{}
			dataCopy.DutState.Servo = chromeosLab.PeripheralState_NOT_CONNECTED
			d, err := AdaptToV1DutSpec(dataCopy, DeviceConfig)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, *d.GetCommon().GetLabels().GetPeripherals().ServoState,
				should.Equal(
					inventory.PeripheralState_NOT_CONNECTED))
			assert.Loosely(t, *d.GetCommon().GetLabels().GetPeripherals().Servo, should.Equal(false))
		})
		t.Run("happy path", func(t *ftt.Test) {
			d, err := AdaptToV1DutSpec(&data, DeviceConfig)
			assert.Loosely(t, err, should.BeNil)
			s, err := inventory.WriteDUTToString(d)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, proto.Equal(&d1, d), should.BeTrue)
			assert.Loosely(t, s1, should.Equal(s))
		})
	})

	ftt.Run("Verify labstation v2 => v1", t, func(t *ftt.Test) {
		t.Run("DutState is not set", func(t *ftt.Test) {
			extLabstaion := ufspb.ChromeOSDeviceData{
				LabConfig:           &labstationLSE,
				Machine:             &labstationMachine,
				ManufacturingConfig: &labstationManufacturingconfig,
				DutState:            nil,
			}
			d, err := AdaptToV1DutSpec(&extLabstaion, &labstationDevConfig)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, d.GetCommon().GetLabels().GetPeripherals().GetServo(), should.Equal(false))
			assert.Loosely(t, d.GetCommon().GetLabels().GetPeripherals().GetServoState(), should.Equal(invServoStateUnknown))
			assert.Loosely(t, d.GetCommon().GetLabels().GetPeripherals().GetWifiState(), should.Equal(inventory.HardwareState_HARDWARE_UNKNOWN))
			assert.Loosely(t, d.GetCommon().GetLabels().GetCr50Phase(), should.Equal(inventory.SchedulableLabels_CR50_PHASE_INVALID))
		})

		t.Run("happy path", func(t *ftt.Test) {
			var labstation inventory.DeviceUnderTest
			err := proto.UnmarshalText(labstationProtoFromV2WithDutState, &labstation)
			assert.Loosely(t, err, should.BeNil)

			extLabstaion := ufspb.ChromeOSDeviceData{
				LabConfig:           &labstationLSE,
				Machine:             &labstationMachine,
				ManufacturingConfig: &labstationManufacturingconfig,
				DutState:            &devUFSState,
			}
			d, err := AdaptToV1DutSpec(&extLabstaion, &labstationDevConfig)
			assert.Loosely(t, err, should.BeNil)

			s, err := inventory.WriteDUTToString(d)
			assert.Loosely(t, err, should.BeNil)
			strLabstation, err := inventory.WriteDUTToString(&labstation)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s, should.Equal(strLabstation))
		})
	})

	ftt.Run("Verify devboard v2 => v1", t, func(t *ftt.Test) {
		t.Run("Pool is set", func(t *ftt.Test) {
			extDevboard := ufspb.ChromeOSDeviceData{
				LabConfig:           &devboardLSE,
				Machine:             &devboardMachine,
				ManufacturingConfig: &labstationManufacturingconfig,
				DutState:            nil,
			}
			d, err := AdaptToV1DutSpec(&extDevboard, &labstationDevConfig)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, d.GetCommon().GetLabels().GetSelfServePools(), should.Resemble([]string{"devboard_main"}))
			assert.Loosely(t, d.GetCommon().GetLabels().GetBoard(), should.Equal("andreiboard-devboard"))
			assert.Loosely(t, d.GetCommon().GetLabels().GetModel(), should.Equal("andreiboard-devboard"))
		})

	})
}
