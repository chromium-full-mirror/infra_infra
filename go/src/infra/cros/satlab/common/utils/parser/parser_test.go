// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package parser

import "testing"

func Test_ParseDeployURL(t *testing.T) {
	t.Parallel()

	s := `
{
	"name": "dedede-sasukette-137",
	"type": "DUT",
	"model": "sasukette",
	"location": {
		"aisle": "",
		"row": "",
		"rack": "satlab-0wgtfqin1846803b-rack",
		"rackNumber": "",
		"shelf": "",
		"position": "",
		"barcodeName": "",
		"zone": "ZONE_SFP_6",
		"rackId": 0,
		"labId": 0,
		"fullLocationName": ""
	},
	"info": {
		"assetTag": "",
		"serialNumber": "",
		"costCenter": "",
		"googleCodeName": "",
		"model": "sasukette",
		"buildTarget": "dedede",
		"referenceBoard": "",
		"ethernetMacAddress": "",
		"sku": "",
		"phase": "",
		"hwid": "",
		"gpn": "",
		"referenceDesign": "",
		"productStatus": "",
		"fingerprintSensor": false,
		"hwXComplianceVersion": 0,
		"touchScreen": false,
		"isCbx": false,
		"cbxFeatureType": "UNKNOWN",
		"isMixedX": false
	},
	"updateTime": "2023-10-25T06:36:01.533562333Z",
	"realm": "chromeos:ufs/sfp_6",
	"tags": []
}
Successfully added the asset:  dedede-sasukette-137

Warning: Could not verify zone from DUT name "satlab-0wgtfqin1846803b-dedede-sasukette-137". Continuing.
{
	"name": "satlab-0wgtfqin1846803b-dedede-sasukette-137",
	"machineLsePrototype": "",
	"hostname": "satlab-0wgtfqin1846803b-dedede-sasukette-137",
	"chromeosMachineLse": {
		"deviceLse": {
			"config": null,
			"rpmInterface": null,
			"networkDeviceInterface": null,
			"dut": {
				"hostname": "satlab-0wgtfqin1846803b-dedede-sasukette-137",
				"peripherals": {
					"servo": {
						"servoHostname": "",
						"servoPort": 0,
						"servoSerial": "",
						"servoType": "",
						"servoSetup": "SERVO_SETUP_REGULAR",
						"servoTopology": null,
						"servoFwChannel": "SERVO_FW_STABLE",
						"servoComponent": [],
						"dockerContainerName": "",
						"usbDrive": null
					},
					"chameleon": {
						"chameleonPeripherals": [],
						"audioBoard": false,
						"hostname": "",
						"rpm": null,
						"audioboxJackplugger": "AUDIOBOX_JACKPLUGGER_UNSPECIFIED",
						"trrsType": "TRRS_TYPE_UNSPECIFIED"
					},
					"rpm": {
						"powerunitName": "",
						"powerunitOutlet": ""
					},
					"connectedCamera": [],
					"audio": {
						"audioBox": false,
						"atrus": false,
						"audioCable": false
					},
					"wifi": {
						"wificell": false,
						"antennaConn": "CONN_UNKNOWN",
						"router": "ROUTER_UNSPECIFIED",
						"wifiRouters": [],
						"features": [],
						"wifiRouterFeatures": []
					},
					"touch": {
						"mimo": false
					},
					"carrier": "",
					"starfishSlotMapping": "",
					"camerabox": false,
					"chaos": false,
					"cable": [],
					"cameraboxInfo": {
						"facing": "FACING_UNKNOWN",
						"light": "LIGHT_UNKNOWN"
					},
					"smartUsbhub": false,
					"cameraRoiBack": false,
					"cameraRoiFront": false,
					"bluetoothPeers": [],
					"humanMotionRobot": null
				},
				"criticalPools": [],
				"pools": [
					"love-satlab"
				],
				"licenses": [],
				"modeminfo": null,
				"siminfo": [],
				"roVpdMap": {},
				"cbi": null,
				"cbx": false
			}
		}
	},
	"machines": [
		"dedede-sasukette-137"
	],
	"updateTime": "2023-10-25T06:36:02.664403501Z",
	"nic": "",
	"vlan": "",
	"ip": "",
	"rack": "satlab-0wgtfqin1846803b-rack",
	"manufacturer": "",
	"tags": [],
	"zone": "ZONE_SFP_6",
	"deploymentTicket": "",
	"description": "",
	"resourceState": "STATE_REGISTERED",
	"schedulable": false,
	"ownership": null,
	"logicalZone": "LOGICAL_ZONE_UNSPECIFIED",
	"realm": "chromeos:ufs/sfp_6"
}
Successfully added DUT to UFS: satlab-0wgtfqin1846803b-dedede-sasukette-137
Deployment URL: https://ci.chromium.org/p/chromeos/builders/external-cienet/deploy/b8766285423942274257
`

	// Act
	res, err := ParseDeployURL(s)

	// Asset
	if err != nil {
		t.Errorf("unexpected error: %v\n", err)
	}

	expected := "https://ci.chromium.org/p/chromeos/builders/external-cienet/deploy/b8766285423942274257"
	if expected != res {
		t.Errorf("unexpected res, expected: %v, got %v\n", expected, res)
	}
}
