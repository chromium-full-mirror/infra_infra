// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hostinfo

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	grpc "google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/chromiumos/infra/proto/go/lab_platform"

	fleet "go.chromium.org/infra/appengine/crosskylabadmin/api/fleet/v1"
	"go.chromium.org/infra/cros/stableversion/keys"
	"go.chromium.org/infra/libs/skylab/inventory"
	models "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

const fullResponse = `{
	"labels": [
		"arc",
		"board:FAKE-BOARD",
		"model:FAKE-MODEL",
		"device-sku:FAKE-DEVICE-SKU",
		"sku:FAKE-SKU",
		"platform:FAKE-PLATFORM",
		"wifi_chip:FAKE-CHIP",
		"ec:cros",
		"os:cros",
		"phase:DVT",
		"variant:FAKE-VARIANT",
		"bluetooth",
		"fingerprint",
		"internal_display",
		"touchpad",
		"power:battery",
		"storage:nvme",
		"hw_video_acc_h264",
		"cr50:prepvt",
		"cr50-ro-keyid:prod",
		"cts_abi_x86",
		"cts_cpu_x86",
		"hwid_component:FAKE-BATTERY",
		"hwid_component:FAKE-DISPLAY",
		"audio_loopback_dongle",
		"conductive:False",
		"servo_deployed",
		"servo_state:WORKING",
		"servo_type:servo_v4_with_ccd_cr50",
		"servo_topology:eyJtYWluIjp7InR5cGUiOiJzZXJ2b192NCIsInN5c2ZzX3Byb2R1Y3QiOiJTZXJ2byBWNCIsInNlcmlhbCI6IkZBS0UtU0VSVk8tU0VSSUFMIiwidXNiX2h1Yl9wb3J0IjoiRkFLRS1TRVJWTy1VU0ItSFVCLVBPUlQifSwiY2hpbGRyZW4iOlt7InR5cGUiOiJjY2RfY3I1MCIsInN5c2ZzX3Byb2R1Y3QiOiJDcjUwIiwic2VyaWFsIjoiRkFLRS1UT1BPTE9HWS1JVEVNIiwidXNiX2h1Yl9wb3J0IjoiRkFLRS1VU0ItSFVCLVBPUlQifV19",
		"servo_component:servo_v4",
		"servo_component:ccd_cr50",
		"servo_usb_state:NORMAL",
		"storage_state:NORMAL",
		"pool:faft-test"
	],
	"attributes": {
		"HWID": "FAKE-HWID",
		"powerunit_hostname": "FAKE-POWERUNIT-HOSTNAME",
		"powerunit_outlet": "AA6",
		"serial_number": "FAKE-SERIAL-NUMBER",
		"servo_host": "FAKE-SERVO-HOST",
		"servo_port": "FAKE-SERVO-PORT",
		"servo_serial": "FAKE-SERVO-SERIAL",
		"servo_setup": "REGULAR",
		"servo_type": "servo_v4_with_ccd_cr50"
	},
	"stable_versions": {
		"FwRoImagePath": "FAKE-FAFT-VERSION",
		"FwRoVersion": "FAKE-FIRMWARE-VERSION",
		"OsImagePath": "FAKE-CROS-PATH",
		"OsVersion": "FAKE-CROS-VERSION"
	},
	"serializer_version": 1
}`

type FakeGetDutInfo struct {
	hostname string
	response *models.ChromeOSDeviceData
}

// GetChromeOSDeviceData retrieves requested Chrome OS device data from the UFS and inventoryV2.
func (f *FakeGetDutInfo) GetChromeOSDeviceData(ctx context.Context, req *ufsAPI.GetChromeOSDeviceDataRequest, opts ...grpc.CallOption) (*models.ChromeOSDeviceData, error) {
	if req.Hostname == "" {
		return nil, fmt.Errorf("by hostname not provided")
	}
	return f.response, nil
}

type FakeGetStableVersion struct {
	version map[string]*lab_platform.StableVersion
}

func (f *FakeGetStableVersion) GetRecoveryVersion(ctx context.Context, in *fleet.GetRecoveryVersionRequest, opts ...grpc.CallOption) (*fleet.GetRecoveryVersionResponse, error) {
	resp := &fleet.GetRecoveryVersionResponse{}
	pools := append([]string{}, in.Pools...)
	// Add empty pool for keys when pool was not specified.
	pools = append(pools, "")
	for _, pool := range pools {
		key := keys.New(in.DeviceType, in.Board, in.Model, pool).String()
		if v, ok := f.version[key]; ok {
			resp.Version = &lab_platform.StableVersion{
				Target: &lab_platform.StableVersionTarget{
					DeviceType: in.DeviceType,
					Board:      in.Board,
					Model:      in.Model,
					Pool:       pool,
				},
				OsVersion:           v.OsVersion,
				OsImagePath:         v.OsImagePath,
				FirmwareRoVersion:   v.FirmwareRoVersion,
				FirmwareRoImagePath: v.FirmwareRoImagePath,
			}
			return resp, nil
		}
	}
	// if that is request per hostname.
	if in.DeviceName != "" && in.Board == "" && in.Model == "" && len(in.Pools) == 0 {
		key := "|hostname:" + in.DeviceName
		if v, ok := f.version[key]; ok {
			resp.Version = &lab_platform.StableVersion{
				Target:              &lab_platform.StableVersionTarget{},
				OsVersion:           v.OsVersion,
				OsImagePath:         v.OsImagePath,
				FirmwareRoVersion:   v.FirmwareRoVersion,
				FirmwareRoImagePath: v.FirmwareRoImagePath,
			}
		}
		return resp, nil
	}
	return resp, nil
}

func TestGetContentsForHostname(t *testing.T) {
	bg := context.Background()

	const hostname = "FAKE-HOSTNAME"
	const expected = fullResponse
	const expectedErr = ""

	g := NewGetter(
		&FakeGetDutInfo{
			hostname: hostname,
			response: &models.ChromeOSDeviceData{
				DutV1: &inventory.DeviceUnderTest{
					Common: &inventory.CommonDeviceSpecs{
						Attributes: []*inventory.KeyValue{
							{
								Key:   s("HWID"),
								Value: s("FAKE-HWID"),
							},
							{
								Key:   s("powerunit_hostname"),
								Value: s("FAKE-POWERUNIT-HOSTNAME"),
							},
							{
								Key:   s("powerunit_outlet"),
								Value: s("AA6"),
							},
							{
								Key:   s("serial_number"),
								Value: s("FAKE-SERIAL-NUMBER"),
							},
							{
								Key:   s("servo_host"),
								Value: s("FAKE-SERVO-HOST"),
							},
							{
								Key:   s("servo_port"),
								Value: s("FAKE-SERVO-PORT"),
							},
							{
								Key:   s("servo_setup"),
								Value: s("REGULAR"),
							},
							{
								Key:   s("servo_serial"),
								Value: s("FAKE-SERVO-SERIAL"),
							},
							{
								Key:   s("servo_type"),
								Value: s("servo_v4_with_ccd_cr50"),
							},
						},
						Labels: &inventory.SchedulableLabels{
							Arc:   b(true),
							Board: s("FAKE-BOARD"),
							Brand: nil,
							Capabilities: &inventory.HardwareCapabilities{
								Atrus:           nil,
								Bluetooth:       b(true),
								Carrier:         nil,
								Fingerprint:     b(true),
								GpuFamily:       nil,
								Graphics:        nil,
								InternalDisplay: b(true),
								Power:           s("battery"),
								Storage:         s("nvme"),
								Touchpad:        b(true),
								VideoAcceleration: []inventory.HardwareCapabilities_VideoAcceleration{
									inventory.HardwareCapabilities_VIDEO_ACCELERATION_H264,
								},
							},
							Cr50Phase:   cr50(inventory.SchedulableLabels_CR50_PHASE_PREPVT),
							Cr50RoKeyid: s("prod"),
							CtsAbi: []inventory.SchedulableLabels_CTSABI{
								inventory.SchedulableLabels_CTS_ABI_X86,
							},
							CtsCpu: []inventory.SchedulableLabels_CTSCPU{
								inventory.SchedulableLabels_CTS_CPU_X86,
							},
							EcType: ectype(inventory.SchedulableLabels_EC_TYPE_CHROME_OS),
							HwidComponent: []string{
								"FAKE-BATTERY",
								"FAKE-DISPLAY",
							},
							HwidSku: s("FAKE-SKU"),
							Model:   s("FAKE-MODEL"),
							Sku:     s("FAKE-DEVICE-SKU"),
							OsType:  ostype(inventory.SchedulableLabels_OS_TYPE_CROS),
							Peripherals: &inventory.Peripherals{
								AudioBoard:          nil,
								AudioBox:            nil,
								AudioCable:          nil,
								AudioLoopbackDongle: b(true),
								Chameleon:           nil,
								Conductive:          nil,
								Huddly:              nil,
								Mimo:                nil,
								Servo:               b(true),
								ServoComponent:      []string{"servo_v4", "ccd_cr50"},
								ServoState:          peripheralState(inventory.PeripheralState_WORKING),
								ServoType:           s("servo_v4_with_ccd_cr50"),
								ServoTopology: &inventory.ServoTopology{
									Main: &inventory.ServoTopologyItem{
										Type:         s("servo_v4"),
										SysfsProduct: s("Servo V4"),
										Serial:       s("FAKE-SERVO-SERIAL"),
										UsbHubPort:   s("FAKE-SERVO-USB-HUB-PORT"),
									},
									Children: []*inventory.ServoTopologyItem{
										{
											Type:         s("ccd_cr50"),
											SysfsProduct: s("Cr50"),
											Serial:       s("FAKE-TOPOLOGY-ITEM"),
											UsbHubPort:   s("FAKE-USB-HUB-PORT"),
										},
									},
								},
								SmartUsbhub:     nil,
								Camerabox:       nil,
								Wificell:        nil,
								Router_802_11Ax: nil,
								StorageState:    hardwarestate(inventory.HardwareState_HARDWARE_NORMAL),
								ServoUsbState:   hardwarestate(inventory.HardwareState_HARDWARE_NORMAL),
							},
							Phase:    phase(inventory.SchedulableLabels_PHASE_DVT),
							Platform: s("FAKE-PLATFORM"),
							SelfServePools: []string{
								"faft-test",
							},
							TestCoverageHints: &inventory.TestCoverageHints{
								ChaosDut: nil,
							},
							Variant: []string{
								"FAKE-VARIANT",
							},
							WifiChip: s("FAKE-CHIP"),
						},
					},
				},
			},
		},
		&FakeGetStableVersion{
			version: map[string]*lab_platform.StableVersion{
				"|hostname:FAKE-HOSTNAME": {
					OsVersion:           "FAKE-CROS-VERSION",
					OsImagePath:         "FAKE-CROS-PATH",
					FirmwareRoImagePath: "FAKE-FAFT-VERSION",
					FirmwareRoVersion:   "FAKE-FIRMWARE-VERSION",
				},
			},
		},
	)

	out, e := g.GetContentsForHostname(bg, hostname)
	eMsg := errToString(e)

	if diff := cmp.Diff(expected, out); diff != "" {
		t.Errorf("wanted: (%s) got: (%s)\n(%s)", expected, out, diff)
	}

	if diff := cmp.Diff(expectedErr, eMsg); diff != "" {
		t.Errorf("wanted: (%s) got: (%s)\n(%s)", expectedErr, eMsg, diff)
	}
}

func TestGetStableVersionForHostname(t *testing.T) {
	bg := context.Background()

	const hostname = "FAKE-HOSTNAME"
	const deviceType = "cros"
	const expectedErr = ""
	expected := &lab_platform.StableVersion{
		OsVersion:           "FAKE-CROS-VERSION",
		OsImagePath:         "FAKE-CROS-PATH",
		FirmwareRoImagePath: "FAKE-FAFT-VERSION",
		FirmwareRoVersion:   "FAKE-FIRMWARE-VERSION",
	}

	g := NewGetter(
		nil,
		&FakeGetStableVersion{
			version: map[string]*lab_platform.StableVersion{
				"|hostname:FAKE-HOSTNAME": {
					OsVersion:           "FAKE-CROS-VERSION",
					OsImagePath:         "FAKE-CROS-PATH",
					FirmwareRoImagePath: "FAKE-FAFT-VERSION",
					FirmwareRoVersion:   "FAKE-FIRMWARE-VERSION",
				},
				"|board:fake-board|model:fake-model": {
					OsVersion:           "FAKE1-board-mode-CROS-VERSION",
					OsImagePath:         "FAKE1-board-mode-CROS-PATH",
					FirmwareRoImagePath: "FAKE1-board-mode-FAFT-VERSION",
					FirmwareRoVersion:   "FAKE1-board-mode-FIRMWARE-VERSION",
				},
				"|hostname:FAKE1": {
					OsVersion:           "FAKE2-CROS-VERSION",
					OsImagePath:         "FAKE2-CROS-PATH",
					FirmwareRoImagePath: "FAKE2-FAFT-VERSION",
					FirmwareRoVersion:   "FAKE2-FIRMWARE-VERSION",
				},
			},
		},
	)

	out, e := g.GetStableVersion(bg, deviceType, hostname, "", "", nil)
	eMsg := errToString(e)
	// Reset target as not important!
	out.Target = nil

	if diff := cmp.Diff(protoToString(expected), protoToString(out)); diff != "" {
		t.Errorf("wanted: (%v) \ngot: (%v)\n(%s)", expected, out, diff)
	}

	if diff := cmp.Diff(expectedErr, eMsg); diff != "" {
		t.Errorf("wanted: (%s) \ngot: (%s)\n(%s)", expectedErr, eMsg, diff)
	}
}

func TestGetStableVersionForModel(t *testing.T) {
	bg := context.Background()

	const expectedErr = ""
	const deviceType = "cros"
	expected := &lab_platform.StableVersion{
		FirmwareRoImagePath: "FAKE2-board-mode-FAFT-VERSION",
		FirmwareRoVersion:   "FAKE2-board-mode-FIRMWARE-VERSION",
		OsImagePath:         "FAKE2-board-mode-CROS-PATH",
		OsVersion:           "FAKE2-board-mode-CROS-VERSION",
	}

	g := NewGetter(
		nil,
		&FakeGetStableVersion{
			version: map[string]*lab_platform.StableVersion{
				keys.New("cros", "fake-mode", "fake-model", "").String(): {
					OsVersion:           "FAKE1-mode-mode-CROS-VERSION",
					OsImagePath:         "FAKE1-mode-mode-CROS-PATH",
					FirmwareRoImagePath: "FAKE1-mode-mode-FAFT-VERSION",
					FirmwareRoVersion:   "FAKE1-mode-mode-FIRMWARE-VERSION",
				},
				keys.New("cros", "fake-board", "fake-model", "").String(): {
					OsVersion:           "FAKE2-board-mode-CROS-VERSION",
					OsImagePath:         "FAKE2-board-mode-CROS-PATH",
					FirmwareRoImagePath: "FAKE2-board-mode-FAFT-VERSION",
					FirmwareRoVersion:   "FAKE2-board-mode-FIRMWARE-VERSION",
				},
				keys.New("", "fake-board", "fake-model", "").String(): {
					OsVersion:           "FAKE6-board-mode-CROS-VERSION",
					OsImagePath:         "FAKE6-board-mode-CROS-PATH",
					FirmwareRoImagePath: "FAKE6-board-mode-FAFT-VERSION",
					FirmwareRoVersion:   "FAKE6-board-mode-FIRMWARE-VERSION",
				},
				"|hostname:FAKE-HOSTNAME": {
					OsVersion:           "FAKE3-hostname-CROS-VERSION",
					OsImagePath:         "FAKE3-hostname-CROS-PATH",
					FirmwareRoImagePath: "FAKE3-hostname-FAFT-VERSION",
					FirmwareRoVersion:   "FAKE3-hostname-FIRMWARE-VERSION",
				},
			},
		},
	)

	out, e := g.GetStableVersion(bg, deviceType, "", "fake-board", "fake-model", nil)
	eMsg := errToString(e)
	if e != nil {
		t.Errorf("TestGetStableVersionForModel: unexpected error: %s", e)
	} else {
		out.Target = nil
		if diff := cmp.Diff(protoToString(expected), protoToString(out)); diff != "" {
			t.Errorf("TestGetStableVersionForModel: wanted: (%s) got: (%s)\n(%s)", expected, out, diff)
		}
		if diff := cmp.Diff(expectedErr, eMsg); diff != "" {
			t.Errorf("TestGetStableVersionForModel: wanted: (%s) got: (%s)\n(%s)", expectedErr, eMsg, diff)
		}
	}
}

func TestGetStableVersionForModelAndPool(t *testing.T) {
	bg := context.Background()

	const expectedErr = ""
	const deviceType = "cros"
	expected := &lab_platform.StableVersion{
		OsVersion:           "FAKE4-board-mode-CROS-VERSION",
		OsImagePath:         "FAKE4-board-mode-CROS-PATH",
		FirmwareRoImagePath: "FAKE4-board-mode-FAFT-VERSION",
		FirmwareRoVersion:   "FAKE4-board-mode-FIRMWARE-VERSION",
	}

	g := NewGetter(
		nil,
		&FakeGetStableVersion{
			version: map[string]*lab_platform.StableVersion{
				keys.New("cros", "fake-mode", "fake-model", "").String(): {
					OsVersion:           "FAKE1-mode-mode-CROS-VERSION",
					OsImagePath:         "FAKE1-mode-mode-CROS-PATH",
					FirmwareRoImagePath: "FAKE1-mode-mode-FAFT-VERSION",
					FirmwareRoVersion:   "FAKE1-mode-mode-FIRMWARE-VERSION",
				},
				keys.New("cros", "fake-board", "fake-model", "").String(): {
					OsVersion:           "FAKE2-board-mode-CROS-VERSION",
					OsImagePath:         "FAKE2-board-mode-CROS-PATH",
					FirmwareRoImagePath: "FAKE2-board-mode-FAFT-VERSION",
					FirmwareRoVersion:   "FAKE2-board-mode-FIRMWARE-VERSION",
				},
				keys.New("cros", "fake-board", "fake-model", "fake-pool").String(): {
					OsVersion:           "FAKE4-board-mode-CROS-VERSION",
					OsImagePath:         "FAKE4-board-mode-CROS-PATH",
					FirmwareRoImagePath: "FAKE4-board-mode-FAFT-VERSION",
					FirmwareRoVersion:   "FAKE4-board-mode-FIRMWARE-VERSION",
				},
				keys.New("android", "fake-board", "fake-model", "fake-pool").String(): {
					OsVersion:           "FAKE5-board-mode-CROS-VERSION",
					OsImagePath:         "FAKE5-board-mode-CROS-PATH",
					FirmwareRoImagePath: "FAKE5-board-mode-FAFT-VERSION",
					FirmwareRoVersion:   "FAKE5-board-mode-FIRMWARE-VERSION",
				},
				keys.New("", "fake-board", "fake-model", "fake-pool").String(): {
					OsVersion:           "FAKE6-board-mode-CROS-VERSION",
					OsImagePath:         "FAKE6-board-mode-CROS-PATH",
					FirmwareRoImagePath: "FAKE6-board-mode-FAFT-VERSION",
					FirmwareRoVersion:   "FAKE6-board-mode-FIRMWARE-VERSION",
				},
				"|hostname:FAKE-HOSTNAME": {
					OsVersion:           "FAKE3-hostname-CROS-VERSION",
					OsImagePath:         "FAKE3-hostname-CROS-PATH",
					FirmwareRoImagePath: "FAKE3-hostname-FAFT-VERSION",
					FirmwareRoVersion:   "FAKE3-hostname-FIRMWARE-VERSION",
				},
			},
		},
	)

	out, e := g.GetStableVersion(bg, deviceType, "", "fake-board", "fake-model", []string{"fake-pool"})
	eMsg := errToString(e)
	if e != nil {
		t.Errorf("TestGetStableVersionForModelAndPool: unexpected error %s", e)
	} else {
		out.Target = nil
		if diff := cmp.Diff(protoToString(expected), protoToString(out)); diff != "" {
			t.Errorf("TestGetStableVersionForModelAndPool: wanted: (%s) got: (%s)\n(%s)", expected, out, diff)
		}
		if diff := cmp.Diff(expectedErr, eMsg); diff != "" {
			t.Errorf("TestGetStableVersionForModelAndPool: wanted: (%s) got: (%s)\n(%s)", expectedErr, eMsg, diff)
		}
	}
}

func errToString(e error) string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("<%s>", e.Error())
}

// Pointer to string, for building protos.
func s(s string) *string {
	return &s
}

// Pointer to bool, for building protos.
func b(b bool) *bool {
	return &b
}

// Pointer to Cr50 Phase, for building protos.
func cr50(cr50 inventory.SchedulableLabels_CR50_Phase) *inventory.SchedulableLabels_CR50_Phase {
	return &cr50
}

// Pointer to ECType, for building protos.
func ectype(ectype inventory.SchedulableLabels_ECType) *inventory.SchedulableLabels_ECType {
	return &ectype
}

// Pointer to OSType, for building protos.
func ostype(ostype inventory.SchedulableLabels_OSType) *inventory.SchedulableLabels_OSType {
	return &ostype
}

// Pointer to peripheral state, for building protos.
func peripheralState(peripheralState inventory.PeripheralState) *inventory.PeripheralState {
	return &peripheralState
}

// Pointer to hardware state, for building protos.
func hardwarestate(hardwareState inventory.HardwareState) *inventory.HardwareState {
	return &hardwareState
}

// Pointer to phase, for building protos.
func phase(phase inventory.SchedulableLabels_Phase) *inventory.SchedulableLabels_Phase {
	return &phase
}

func protoToString(msq *lab_platform.StableVersion) string {
	b, _ := (&protojson.MarshalOptions{}).Marshal(msq)
	return string(b)
}
