// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package device_test tests the exported APIs of the package device.
package device

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

type expectEndpointResult struct {
	ip       *labapi.IpEndpoint
	expected string
}

// TestAddress the function Address to return the addresses of DUTs.
func TestAddress(t *testing.T) {
	data := []expectEndpointResult{
		{
			ip:       &labapi.IpEndpoint{Address: "127.0.0.1", Port: 0},
			expected: "127.0.0.1",
		},
		{
			ip:       &labapi.IpEndpoint{Address: "0:0:0:0:0:ffff:7f00:1", Port: 2},
			expected: "[0:0:0:0:0:ffff:7f00:1]:2",
		},
		{
			ip:       &labapi.IpEndpoint{Address: "0:0:0:0:0:ffff:7f00:1", Port: 0},
			expected: "0:0:0:0:0:ffff:7f00:1",
		},
		{
			ip:       &labapi.IpEndpoint{Address: "chromeos6-row17-rack5-host15.cros.corp.google.com", Port: 0},
			expected: "chromeos6-row17-rack5-host15.cros.corp.google.com",
		},
		{
			ip:       &labapi.IpEndpoint{Address: "chromeos6-row17-rack5-host15", Port: 2555},
			expected: "chromeos6-row17-rack5-host15:2555",
		},
	}

	for _, d := range data {
		device := api.CrosTestRequest_Device{
			Dut: &labapi.Dut{
				Id:      &labapi.Dut_Id{Value: "AnyId"},
				DutType: &labapi.Dut_Chromeos{Chromeos: &labapi.Dut_ChromeOS{Ssh: d.ip}},
			},
		}
		got, err := Address(&device)
		if err != nil {
			t.Errorf("Cannot get address for dut %v: %v", &device, err)
		}
		if got != d.expected {
			t.Errorf("Got %q from Address; wanted:%q", got, d.expected)
		}
	}
}

// TestFillDUTInfo makes sure FillDUTInfo behaved as expected.
func TestFillDUTInfo(t *testing.T) {
	input := []*api.CrosTestRequest_Device{
		{
			Dut: &labapi.Dut{
				Id: &labapi.Dut_Id{Value: "AnyId"},
				DutType: &labapi.Dut_Chromeos{
					Chromeos: &labapi.Dut_ChromeOS{
						Ssh: &labapi.IpEndpoint{Address: "127.0.0.1", Port: 2222},
						Servo: &labapi.Servo{
							Present: true,
							ServodAddress: &labapi.IpEndpoint{
								Address: "c6-r9-r7-labstation",
								Port:    9996,
							},
						},
						Cellular: &labapi.Cellular{
							Carrier: "VERIZON",
						},
						SimInfos: []*labapi.SIMInfo{
							{
								SlotId: 1,
								Type:   *labapi.SIMType_SIM_UNKNOWN.Enum(),
								ProfileInfo: []*labapi.SIMProfileInfo{
									{
										Iccid:       "1",
										SimPin:      "1111",
										SimPuk:      "49830420",
										CarrierName: *labapi.NetworkProvider_NETWORK_TMOBILE.Enum(),
										OwnNumber:   "6507891159",
									},
								},
							},
							{
								SlotId: 2,
								Type:   *labapi.SIMType_SIM_PHYSICAL.Enum(),
								ProfileInfo: []*labapi.SIMProfileInfo{
									{
										Iccid:       "2",
										SimPin:      "2222",
										SimPuk:      "49830420",
										CarrierName: *labapi.NetworkProvider_NETWORK_ATT.Enum(),
										OwnNumber:   "6507891159",
									},
								},
							},
						},
					},
				},
				WifiSecret: &labapi.WifiSecret{
					Ssid:     "thelabssid",
					Security: "wpa",
					Password: "the_p@ssw0rd",
				},
			},
			DutServer:       &labapi.IpEndpoint{Address: "cros-dut0", Port: 80},
			ProvisionServer: &labapi.IpEndpoint{Address: "cros-provision0", Port: 80},
		},
		{
			Dut: &labapi.Dut{
				Id: &labapi.Dut_Id{Value: "AnyId"},
				DutType: &labapi.Dut_Chromeos{
					Chromeos: &labapi.Dut_ChromeOS{
						Ssh: &labapi.IpEndpoint{Address: "0:0:0:0:0:ffff:7f00:1", Port: 2},
						Servo: &labapi.Servo{
							Present: true,
							ServodAddress: &labapi.IpEndpoint{
								Address: "c6-r8-r7-labstation",
								Port:    9999,
							},
						},
					},
				},
			},
			DutServer:       &labapi.IpEndpoint{Address: "cros-dut1", Port: 80},
			ProvisionServer: &labapi.IpEndpoint{Address: "cros-provision1", Port: 80},
		},
		{
			Dut: &labapi.Dut{
				Id: &labapi.Dut_Id{Value: "AnyId"},
				DutType: &labapi.Dut_Chromeos{
					Chromeos: &labapi.Dut_ChromeOS{
						Ssh: &labapi.IpEndpoint{Address: "c6-r8-rack7-host7", Port: 0},
						Servo: &labapi.Servo{
							Present: true,
							ServodAddress: &labapi.IpEndpoint{
								Address: "c6-r7-r7-labstation",
								Port:    9999,
							},
						},
					},
				},
			},
			DutServer:       &labapi.IpEndpoint{Address: "cros-dut2", Port: 80},
			ProvisionServer: &labapi.IpEndpoint{Address: "cros-provision2", Port: 80},
		},
		{
			Dut: &labapi.Dut{
				Id: &labapi.Dut_Id{Value: "AnyId"},
				DutType: &labapi.Dut_Chromeos{
					Chromeos: &labapi.Dut_ChromeOS{
						Ssh: &labapi.IpEndpoint{Address: "0:0:0:0:0:ffff:7f00:1", Port: 0},
					},
				},
			},
		}, {
			Dut: &labapi.Dut{
				Id: &labapi.Dut_Id{Value: "AnyId"},
				DutType: &labapi.Dut_Devboard_{
					Devboard: &labapi.Dut_Devboard{
						BoardType:           "AnyBoardType",
						UltradebugSerial:    "AnySerical",
						FingerprintModuleId: "AnyFPID",
					},
				},
			},
			DevboardServer: &labapi.IpEndpoint{Address: "cros-board", Port: 80},
		},
	}
	expected := []*DutInfo{
		{
			Addr:            "127.0.0.1:2222",
			Role:            "",
			Servo:           "c6-r9-r7-labstation:9996",
			DutServer:       "cros-dut0:80",
			ProvisionServer: "cros-provision0:80",
			ServoHostname:   "c6-r9-r7-labstation",
			ServoPort:       "9996",
			Phase:           "PHASE_UNSPECIFIED",
			CarrierList:     []string{"carrier:verizon"},
			SimInfos: []string{
				"sim_slot_id:1", "sim_1_num_profiles:1", "sim_1_0_iccid:1", "sim_1_0_pin:1111",
				"sim_1_0_puk:49830420", "sim_1_0_carrier_name:NETWORK_TMOBILE",
				"sim_1_0_own_number:6507891159", "sim_slot_id:2", "sim_2_type:SIM_PHYSICAL",
				"sim_2_num_profiles:1", "sim_2_0_iccid:2", "sim_2_0_pin:2222",
				"sim_2_0_puk:49830420", "sim_2_0_carrier_name:NETWORK_ATT",
				"sim_2_0_own_number:6507891159",
			},
			WifiSecretSsid:     "thelabssid",
			WifiSecretSecurity: "wpa",
			WifiSecretPassword: "the_p@ssw0rd",
			DUT:                input[0].Dut,
			IP:                 "127.0.0.1",
		},
		{
			Addr:            "[0:0:0:0:0:ffff:7f00:1]:2",
			Role:            "cd1",
			Servo:           "c6-r8-r7-labstation:9999",
			DutServer:       "cros-dut1:80",
			ProvisionServer: "cros-provision1:80",
			ServoHostname:   "c6-r8-r7-labstation",
			ServoPort:       "9999",
			Phase:           "PHASE_UNSPECIFIED",
			DUT:             input[1].Dut,
			IP:              "0:0:0:0:0:ffff:7f00:1",
		},
		{
			Addr:            "c6-r8-rack7-host7",
			Role:            "cd2",
			Servo:           "c6-r7-r7-labstation:9999",
			DutServer:       "cros-dut2:80",
			ProvisionServer: "cros-provision2:80",
			ServoHostname:   "c6-r7-r7-labstation",
			ServoPort:       "9999",
			Phase:           "PHASE_UNSPECIFIED",
			DUT:             input[2].Dut,
			IP:              "c6-r8-rack7-host7",
		},
		{
			Addr:            "0:0:0:0:0:ffff:7f00:1",
			Role:            "cd3",
			Servo:           "",
			DutServer:       "",
			ProvisionServer: "",
			DevboardServer:  "",
			Phase:           "PHASE_UNSPECIFIED",
			DUT:             input[3].Dut,
			IP:              "0:0:0:0:0:ffff:7f00:1",
		},
		{
			Addr:            "",
			Role:            "",
			Servo:           "",
			DutServer:       "",
			ProvisionServer: "",
			DevboardServer:  "cros-board:80",
			Phase:           "PHASE_UNSPECIFIED",
			DUT:             input[4].Dut,
		},
	}

	for i, wanted := range expected {
		dut := input[i]
		got, err := FillDUTInfo(dut, wanted.Role)
		if err != nil {
			t.Errorf("Cannot get address for dut %v: %v", dut, err)
		}
		if diff := cmp.Diff(got, wanted, cmp.Comparer(proto.Equal)); diff != "" {
			t.Errorf("FillDUTInfo mismatch (-got +want):\n%s", diff)
		}
	}
}

// TestFillDUTInfo makes sure FillDUTInfo behaved as expected.
func TestFillDUTInfoExtended(t *testing.T) {
	dut := &labapi.Dut{
		Id: &labapi.Dut_Id{Value: "AnyId"},
		DutType: &labapi.Dut_Chromeos{
			Chromeos: &labapi.Dut_ChromeOS{
				Ssh: &labapi.IpEndpoint{Address: "127.0.0.1", Port: 2222},
				Servo: &labapi.Servo{
					Present: true,
					ServodAddress: &labapi.IpEndpoint{
						Address: "127.123.332.121",
						Port:    1337,
					},
					Serial: "8675309",
				},
				DutModel: &labapi.DutModel{
					BuildTarget: "Fred",
					ModelName:   "Flintstone",
				},
				Chameleon: &labapi.Chameleon{
					Peripherals: []labapi.Chameleon_Peripheral{
						labapi.Chameleon_VGA,
						labapi.Chameleon_HDMI,
					},
					AudioBoard: true,
				},
				Audio: &labapi.Audio{
					Atrus: true,
				},
				Touch: &labapi.Touch{
					Mimo: true,
				},

				Camerabox: &labapi.Camerabox{
					Facing: labapi.Camerabox_FRONT,
				},
				Cables: []*labapi.Cable{
					{
						Type: labapi.Cable_USBAUDIO,
					},
				},
				Phase:         labapi.Phase_DVT,
				Sku:           "123",
				HwidComponent: []string{"hwid123"},
				Hwid:          "Atlas 123-34",
				Cellular: &labapi.Cellular{
					Carrier: "VERIZON",
				},
				SimInfos: []*labapi.SIMInfo{
					{
						SlotId: 1,
						Type:   *labapi.SIMType_SIM_UNKNOWN.Enum(),
						ProfileInfo: []*labapi.SIMProfileInfo{
							{
								Iccid:       "1",
								SimPin:      "1111",
								SimPuk:      "49830420",
								CarrierName: *labapi.NetworkProvider_NETWORK_TMOBILE.Enum(),
								OwnNumber:   "6507891159",
							},
						},
					},
					{
						SlotId: 2,
						Type:   *labapi.SIMType_SIM_PHYSICAL.Enum(),
						ProfileInfo: []*labapi.SIMProfileInfo{
							{
								Iccid:       "2",
								SimPin:      "2222",
								SimPuk:      "49830420",
								CarrierName: *labapi.NetworkProvider_NETWORK_ATT.Enum(),
								OwnNumber:   "6507891159",
							},
						},
					},
				},
			},
		},
		WifiSecret: &labapi.WifiSecret{
			Ssid:     "thelabssid",
			Security: "wpa",
			Password: "the_p@ssw0rd",
		},
		CacheServer: &labapi.CacheServer{Address: &labapi.IpEndpoint{Address: "cache1", Port: 8001}},
	}
	expected := []*DutInfo{
		{
			Addr:                "127.0.0.1:2222",
			Role:                "",
			Servo:               "127.123.332.121:1337",
			DutServer:           "cros-dut0:80",
			ProvisionServer:     "cros-provision0:80",
			Board:               "Fred",
			Model:               "Flintstone",
			ServoHostname:       "127.123.332.121",
			ServoPort:           "1337",
			ServoSerial:         "8675309",
			ChameleonAudio:      true,
			ChamelonPresent:     true,
			ChamelonPeriphsList: []string{"chameleon:vga", "chameleon:hdmi"},
			AtrusAudio:          true,
			TouchMimo:           true,
			CameraboxFacing:     "front",
			CableList:           []string{"type:usbaudio"},
			Phase:               "DVT",
			Sku:                 "123",
			HwIDList:            []string{"hwid_component:hwid123"},
			CacheServer:         "cache1:8001",
			HWID:                "Atlas 123-34",
			CarrierList:         []string{"carrier:verizon"},
			SimInfos: []string{
				"sim_slot_id:1", "sim_1_num_profiles:1", "sim_1_0_iccid:1", "sim_1_0_pin:1111",
				"sim_1_0_puk:49830420", "sim_1_0_carrier_name:NETWORK_TMOBILE",
				"sim_1_0_own_number:6507891159", "sim_slot_id:2", "sim_2_type:SIM_PHYSICAL",
				"sim_2_num_profiles:1", "sim_2_0_iccid:2", "sim_2_0_pin:2222",
				"sim_2_0_puk:49830420", "sim_2_0_carrier_name:NETWORK_ATT",
				"sim_2_0_own_number:6507891159",
			},
			WifiSecretSsid:     "thelabssid",
			WifiSecretSecurity: "wpa",
			WifiSecretPassword: "the_p@ssw0rd",
			DUT:                dut,
			IP:                 "127.0.0.1",
		},
	}
	input := []*api.CrosTestRequest_Device{
		{
			Dut:             dut,
			DutServer:       &labapi.IpEndpoint{Address: "cros-dut0", Port: 80},
			ProvisionServer: &labapi.IpEndpoint{Address: "cros-provision0", Port: 80},
		},
	}

	for i, wanted := range expected {
		dut := input[i]
		got, err := FillDUTInfo(dut, wanted.Role)
		if err != nil {
			t.Errorf("Cannot get address for dut %v: %v", dut, err)
		}
		if diff := cmp.Diff(got, wanted, cmp.Comparer(proto.Equal)); diff != "" {
			t.Errorf("FillDUTInfo mismatch (-got +want):\n%s", diff)
		}
	}
}

// TestFillAndroidInfo makes sure FillAndroidInfo behaved as expected.
func TestFillAndroidInfo(t *testing.T) {
	input := []*api.CrosTestRequest_Device{
		{
			Dut: &labapi.Dut{
				Id: &labapi.Dut_Id{Value: "AnyId"},
				DutType: &labapi.Dut_Android_{
					Android: &labapi.Dut_Android{
						AssociatedHostname: &labapi.IpEndpoint{Address: "host1"},
						SerialNumber:       "ABC123",
						DutModel: &labapi.DutModel{
							BuildTarget: "abc",
							ModelName:   "pixel6",
						},
					},
				},
			},
		},
		{
			Dut: &labapi.Dut{
				Id: &labapi.Dut_Id{Value: "AnyId"},
				DutType: &labapi.Dut_Android_{
					Android: &labapi.Dut_Android{
						AssociatedHostname: &labapi.IpEndpoint{Address: "127.0.0.1", Port: 2222},
						SerialNumber:       "DEF456",
						DutModel: &labapi.DutModel{
							BuildTarget: "abc",
							ModelName:   "pixel7",
						},
					},
				},
			},
		},
	}
	expected := []*AndroidInfo{
		{
			AssoicateAddr: "host1",
			Serial:        "ABC123",
			ModelName:     "pixel6",
			DUT:           input[0].Dut,
		},
		{
			AssoicateAddr: "127.0.0.1:2222",
			Serial:        "DEF456",
			ModelName:     "pixel7",
			DUT:           input[1].Dut,
		},
	}

	for i, wanted := range expected {
		android := input[i]
		got := FillAndroidInfo(android)
		if diff := cmp.Diff(got, wanted, cmp.Comparer(proto.Equal)); diff != "" {
			t.Errorf("FillAndroidInfo mismatch (-got +want):\n%s", diff)
		}
	}
}

func TestDeviceSerials(t *testing.T) {

	req := &api.CrosTestRequest{
		Primary: &api.CrosTestRequest_Device{
			Dut: &labapi.Dut{
				Id: &labapi.Dut_Id{Value: "Dut1"},
				DutType: &labapi.Dut_Chromeos{
					Chromeos: &labapi.Dut_ChromeOS{
						Ssh: &labapi.IpEndpoint{Address: "host1", Port: 22},
					},
				},
			},
		},
		Companions: []*api.CrosTestRequest_Device{
			{
				Dut: &labapi.Dut{
					Id: &labapi.Dut_Id{Value: "CompanionDut"},
					DutType: &labapi.Dut_Android_{
						Android: &labapi.Dut_Android{
							AssociatedHostname: &labapi.IpEndpoint{Address: "a1", Port: 22},
							DutModel:           &labapi.DutModel{BuildTarget: "XYZ", ModelName: "pixel6"},
							SerialNumber:       "ABC123",
						},
					},
				},
			},
		},
	}

	wanted := []string{"host1:5555", "ABC123"}
	got, err := DerviceSerials(req)
	if err != nil {
		t.Error("failed to call DerviceSerials mismatch", err)
	}
	if diff := cmp.Diff(got, wanted); diff != "" {
		t.Errorf("DerviceSerials mismatch (-got +want):\n%s", diff)
	}
}
