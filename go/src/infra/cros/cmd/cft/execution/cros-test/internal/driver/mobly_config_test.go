// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package driver

import (
	"log"
	"testing"

	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v2"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

var fullConfig = MoblyTestConfig{
	TestBeds: []*TestBed{
		{
			Name: "LocalTestBed",
			Controllers: &Controllers{
				OpenWrtDevices: []*OpenWrtDevice{
					{
						Hostname: "SERIAL_1-router",
						Params: paramMap{
							"skip_init_reboot": "True",
						},
					},
					{
						Hostname: "SERIAL_1-foo",
						Params: paramMap{
							"skip_init_reboot": "True",
						},
					},
					{
						Hostname: "SERIAL_1-pcap",
						Params: paramMap{
							"skip_init_reboot": "True",
						},
					},
				},
				AndroidDevices: []*AndroidDevice{
					{
						Serial: "SERIAL_1",
						Role:   "source_device",
						Params: paramMap{},
					},
					{
						Serial: "SERIAL_2",
						Role:   "target_device",
						Params: paramMap{},
					},
				},
				BtReferenceDevice: []*BtReferenceDevice{
					{
						Hostname: "SERIAL_1-btpeer1",
						Username: "root",
						Password: "test0000",
						Params:   paramMap{"fake_global_param": "True", "fake_peer1_param": "False"},
					},
					{
						Hostname: "SERIAL_1-btpeer2",
						Username: "root",
						Password: "test0000",
						Params:   paramMap{"fake_global_param": "True"},
					},
					{
						Hostname: "SERIAL_1-btpeer5",
						Username: "root",
						Password: "test0000",
						Params:   paramMap{"fake_global_param": "True"},
					},
					{
						Hostname: "SERIAL_1-btpeer3",
						Username: "root",
						Password: "test0000",
						Params:   paramMap{"fake_global_param": "True"},
					},
					{
						Hostname: "SERIAL_1-btpeer4",
						Username: "root",
						Password: "test0000",
						Params:   paramMap{"fake_global_param": "True"},
					},
				},
				PassportHost: []*PassportHost{
					{
						Params: paramMap{
							"foo": "bar", "foo2": "bar2",
						},
						HostTopology: &labapi.PasitHost{
							Devices: []*labapi.PasitHost_Device{
								{
									Id:   "SERIAL_1",
									Type: labapi.PasitHost_Device_DUT,
								},
								{
									Id:   "dock_switch",
									Type: labapi.PasitHost_Device_SWITCH_FIXTURE,
								},
								{
									Id:   "dock_1",
									Type: labapi.PasitHost_Device_DOCKING_STATION,
								},
								{
									Id:   "hdmi_switch",
									Type: labapi.PasitHost_Device_SWITCH_FIXTURE,
								},
								{
									Id:   "monitor_1",
									Type: labapi.PasitHost_Device_MONITOR,
								},
								{
									Id:   "dp_switch",
									Type: labapi.PasitHost_Device_SWITCH_FIXTURE,
								},
								{
									Id:   "monitor_2",
									Type: labapi.PasitHost_Device_MONITOR,
								},
								{
									Id:   "eth_switch",
									Type: labapi.PasitHost_Device_SWITCH_FIXTURE,
								},
								{
									Id:   "network_1",
									Type: labapi.PasitHost_Device_NETWORK,
								},
							},
							Connections: []*labapi.PasitHost_Connection{
								{
									Type:     "USBC",
									ParentId: "SERIAL_1",
									ChildId:  "dock_switch",
								},
								{
									Type:     "USBC",
									ParentId: "dock_Switch",
									ChildId:  "dock_1",
								},
								{
									Type:     "HDMI",
									ParentId: "dock_1",
									ChildId:  "dp_switch",
								},
								{
									Type:     "HDMI",
									ParentId: "dp_switch",
									ChildId:  "monitor_2",
								},
								{
									Type:     "HDMI",
									ParentId: "hdmi_switch",
									ChildId:  "monitor_1",
								},
								{
									Type:     "HDMI",
									ParentId: "dock_1",
									ChildId:  "hdmi_switch",
								},
								{
									Type:     "ETHERNET",
									ParentId: "dock_1",
									ChildId:  "eth_switch",
								},
								{
									Type:     "ETHERNET",
									ParentId: "eth_switch",
									ChildId:  "network_1",
								},
							},
						},
					},
				},
				ChameleonDevice: []*ChameleonDevice{
					{
						ChameleonIP:         "dut-chameleon",
						ChameleonXMLRPCPort: "9992",
						Params: paramMap{
							"foo":  "bar",
							"foo2": "bar2",
						},
					},
				},
				StarfishDevice: []*StarfishDevice{
					{
						Carrier: "VERIZON",
						ModemInfo: &labapi.ModemInfo{
							Type:           labapi.ModemType_MODEM_TYPE_LCUK54,
							Imei:           "123456789",
							SupportedBands: "1,2,3,4,5",
							SimCount:       2,
							ModelVariant:   "test_variant",
						},
						SIMInfos: []*labapi.SIMInfo{
							{
								SlotId: 1,
								Type:   *labapi.SIMType_SIM_UNKNOWN.Enum(),
								ProfileInfo: []*labapi.SIMProfileInfo{
									{
										Iccid:       "1",
										SimPin:      "1111",
										SimPuk:      "49830420",
										CarrierName: *labapi.NetworkProvider_NETWORK_TMOBILE.Enum(),
										OwnNumber:   "123456789",
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
										OwnNumber:   "987654321",
									},
								},
							},
						},
						Params: paramMap{
							"foo": "bar", "foo2": "bar2",
							"starfish.carrier": "VERIZON",
						},
					},
				},
			},
			TestParams: &TestParams{
				Params: paramMap{
					"foo": "bar", "foo2": "bar2",
					"passport_switch_service_host": "10.0.0.2",
					"passport_switch_service_port": "12345",
				},
			},
		},
	},
}

var fullConfigYaml = `TestBeds:
- Name: LocalTestBed
  TestParams:
    foo: bar
    foo2: bar2
    passport_switch_service_host: 10.0.0.2
    passport_switch_service_port: "12345"
  Controllers:
    OpenWrtDevice:
    - hostname: SERIAL_1-router
      skip_init_reboot: "True"
    - hostname: SERIAL_1-foo
      skip_init_reboot: "True"
    - hostname: SERIAL_1-pcap
      skip_init_reboot: "True"
    AndroidDevice:
    - serial: SERIAL_1
      role: source_device
    - serial: SERIAL_2
      role: target_device
    BtReferenceDevice:
    - hostname: SERIAL_1-btpeer1
      username: root
      password: test0000
      fake_global_param: "True"
      fake_peer1_param: "False"
    - hostname: SERIAL_1-btpeer2
      username: root
      password: test0000
      fake_global_param: "True"
    - hostname: SERIAL_1-btpeer5
      username: root
      password: test0000
      fake_global_param: "True"
    - hostname: SERIAL_1-btpeer3
      username: root
      password: test0000
      fake_global_param: "True"
    - hostname: SERIAL_1-btpeer4
      username: root
      password: test0000
      fake_global_param: "True"
    PassportHost:
    - host_topology:
        connections:
        - child_id: dock_switch
          parent_id: SERIAL_1
          type: USBC
        - child_id: dock_1
          parent_id: dock_Switch
          type: USBC
        - child_id: dp_switch
          parent_id: dock_1
          type: HDMI
        - child_id: monitor_2
          parent_id: dp_switch
          type: HDMI
        - child_id: monitor_1
          parent_id: hdmi_switch
          type: HDMI
        - child_id: hdmi_switch
          parent_id: dock_1
          type: HDMI
        - child_id: eth_switch
          parent_id: dock_1
          type: ETHERNET
        - child_id: network_1
          parent_id: eth_switch
          type: ETHERNET
        devices:
        - id: SERIAL_1
          type: DUT
        - id: dock_switch
          type: SWITCH_FIXTURE
        - id: dock_1
          type: DOCKING_STATION
        - id: hdmi_switch
          type: SWITCH_FIXTURE
        - id: monitor_1
          type: MONITOR
        - id: dp_switch
          type: SWITCH_FIXTURE
        - id: monitor_2
          type: MONITOR
        - id: eth_switch
          type: SWITCH_FIXTURE
        - id: network_1
          type: NETWORK
      foo: bar
      foo2: bar2
    ChameleonDevice:
    - chameleon_ip: dut-chameleon
      chameleon_xmlrpc_port: "9992"
      foo: bar
      foo2: bar2
    StarfishDevice:
    - carrier: VERIZON
      modem_info:
        imei: "123456789"
        model_variant: test_variant
        sim_count: 2
        supported_bands: 1,2,3,4,5
        type: MODEM_TYPE_LCUK54
      sim_infos:
      - profile_info:
        - carrier_name: NETWORK_TMOBILE
          iccid: "1"
          own_number: "123456789"
          sim_pin: "1111"
          sim_puk: "49830420"
        slot_id: 1
      - profile_info:
        - carrier_name: NETWORK_ATT
          iccid: "2"
          own_number: "987654321"
          sim_pin: "2222"
          sim_puk: "49830420"
        slot_id: 2
        type: SIM_PHYSICAL
      foo: bar
      foo2: bar2
      starfish.carrier: VERIZON
`

// TestYamlConversion verifies that a config is converted into the expected yaml representation.
func TestYamlConversion(t *testing.T) {
	yamlGot, _ := yaml.Marshal(&fullConfig)
	t.Log(string(yamlGot))
	if diff := cmp.Diff(string(yamlGot), fullConfigYaml); diff != "" {
		t.Errorf("Got unexpected argument from NewMoblyConfig (-got +want):\n%s\n%v\n--\n%v\n", string(yamlGot), fullConfigYaml, diff)
	}
}

// TestGenerateConfigs verifies the Mobly config generation.
func TestGenerateConfigs(t *testing.T) {
	tests := []struct {
		name     string
		expected MoblyTestConfig
		serials  []string
		devices  []*labapi.Dut
		metadata []*api.Arg
	}{
		{
			name:    "full_config",
			serials: []string{"SERIAL_1", "SERIAL_2"},
			metadata: []*api.Arg{
				{
					Flag:  "test-params",
					Value: "foo:bar,foo2:bar2",
				},
				{
					Flag:  "openwrt-params",
					Value: "skip_init_reboot:True,primary-router,primary-pcap",
				},
				{
					Flag:  "btreference-params",
					Value: "fake_global_param:True,primary-btpeer1,fake_peer1_param:False,primary-btpeer2,primary-btpeer3,primary-btpeer4",
				},
				{
					Flag:  "extra-test-params",
					Value: "passport_switch_service_host:10.0.0.2,passport_switch_service_port:12345",
				},
				{
					Flag:  "passport-params",
					Value: "foo:bar,foo2:bar2",
				},
				{
					Flag:  "chameleon-params",
					Value: "foo:bar,foo2:bar2",
				},
				{
					Flag:  "starfish-params",
					Value: "foo:bar,foo2:bar2",
				},
				{
					Flag:  "starfish.carrier",
					Value: "VERIZON",
				},
			},
			devices: []*labapi.Dut{
				{
					DutType: &labapi.Dut_Chromeos{
						Chromeos: &labapi.Dut_ChromeOS{
							BluetoothPeers: []*labapi.BluetoothPeer{
								{
									Hostname: "SERIAL_1-btpeer1",
								},
								{
									Hostname: "SERIAL_1-btpeer2",
								},
								{
									Hostname: "SERIAL_1-btpeer5",
								},
							},
							Wifi: &labapi.Wifi{
								WifiRouters: []*labapi.WifiRouter{
									{
										Hostname: "SERIAL_1-router",
									},
									{
										Hostname: "SERIAL_1-foo",
									},
								},
							},
							Cellular: &labapi.Cellular{
								Carrier: "VERIZON",
							},
							ModemInfo: &labapi.ModemInfo{
								Type:           labapi.ModemType_MODEM_TYPE_LCUK54,
								Imei:           "123456789",
								SupportedBands: "1,2,3,4,5",
								SimCount:       2,
								ModelVariant:   "test_variant",
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
											OwnNumber:   "123456789",
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
											OwnNumber:   "987654321",
										},
									},
								},
							},
							PasitHost: &labapi.PasitHost{
								Devices: []*labapi.PasitHost_Device{
									{
										Id:   "SERIAL_1",
										Type: labapi.PasitHost_Device_DUT,
									},
									{
										Id:   "dock_switch",
										Type: labapi.PasitHost_Device_SWITCH_FIXTURE,
									},
									{
										Id:   "dock_1",
										Type: labapi.PasitHost_Device_DOCKING_STATION,
									},
									{
										Id:   "hdmi_switch",
										Type: labapi.PasitHost_Device_SWITCH_FIXTURE,
									},
									{
										Id:   "monitor_1",
										Type: labapi.PasitHost_Device_MONITOR,
									},
									{
										Id:   "dp_switch",
										Type: labapi.PasitHost_Device_SWITCH_FIXTURE,
									},
									{
										Id:   "monitor_2",
										Type: labapi.PasitHost_Device_MONITOR,
									},
									{
										Id:   "eth_switch",
										Type: labapi.PasitHost_Device_SWITCH_FIXTURE,
									},
									{
										Id:   "network_1",
										Type: labapi.PasitHost_Device_NETWORK,
									},
								},
								Connections: []*labapi.PasitHost_Connection{
									{
										Type:     "USBC",
										ParentId: "SERIAL_1",
										ChildId:  "dock_switch",
									},
									{
										Type:     "USBC",
										ParentId: "dock_Switch",
										ChildId:  "dock_1",
									},
									{
										Type:     "HDMI",
										ParentId: "dock_1",
										ChildId:  "dp_switch",
									},
									{
										Type:     "HDMI",
										ParentId: "dp_switch",
										ChildId:  "monitor_2",
									},
									{
										Type:     "HDMI",
										ParentId: "hdmi_switch",
										ChildId:  "monitor_1",
									},
									{
										Type:     "HDMI",
										ParentId: "dock_1",
										ChildId:  "hdmi_switch",
									},
									{
										Type:     "ETHERNET",
										ParentId: "dock_1",
										ChildId:  "eth_switch",
									},
									{
										Type:     "ETHERNET",
										ParentId: "eth_switch",
										ChildId:  "network_1",
									},
								},
							},
							Chameleon: &labapi.Chameleon{
								Hostname: "dut-chameleon",
							},
						},
					},
				},
			},
			expected: fullConfig,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := NewMoblyConfig(log.Default(), test.serials, test.metadata, test.devices)
			// Convert to yaml to make it easier to compare.
			yamlWant, err := yaml.Marshal(&test.expected)
			if err != nil {
				panic(err)
			}
			yamlGot, err := yaml.Marshal(config)
			if err != nil {
				panic(err)
			}
			if diff := cmp.Diff(string(yamlGot), string(yamlWant)); diff != "" {
				t.Errorf("Got unexpected argument from NewMoblyConfig (-got +want):\n%s\n%v\n--\n%v\n", string(yamlGot), string(yamlWant), diff)
			}
		})
	}
}
