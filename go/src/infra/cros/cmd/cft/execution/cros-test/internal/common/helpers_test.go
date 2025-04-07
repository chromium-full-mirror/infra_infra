// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	_go "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/cft/execution/cros-test/internal/device"
)

func TestCompanions(t *testing.T) {
	androidDUT := &labapi.Dut{
		Id: &labapi.Dut_Id{Value: "android1"},
		DutType: &labapi.Dut_Android_{
			Android: &labapi.Dut_Android{
				AssociatedHostname: &labapi.IpEndpoint{Address: "127.0.0.1", Port: 2224},
				DutModel:           &labapi.DutModel{BuildTarget: "XYZ", ModelName: "pixel6"},
				SerialNumber:       "ABC123",
			},
		},
	}
	chromeOSDUT := &labapi.Dut{
		Id: &labapi.Dut_Id{Value: "chromeos1"},
		DutType: &labapi.Dut_Chromeos{
			Chromeos: &labapi.Dut_ChromeOS{
				Ssh: &labapi.IpEndpoint{Address: "127.0.0.1", Port: 2223},
			},
		},
	}
	devices := []*api.CrosTestRequest_Device{
		{
			Dut: androidDUT,
		},
		{
			Dut: chromeOSDUT,
		},
	}

	expectedCompanions := []*device.DutInfo{
		{
			Addr:  "127.0.0.1:2223",
			Role:  "cd1",
			DUT:   chromeOSDUT,
			Phase: "PHASE_UNSPECIFIED",
			IP:    "127.0.0.1",
		},
	}
	expectedAndroids := []*device.AndroidInfo{
		{
			AssoicateAddr: "127.0.0.1:2224",
			Serial:        "ABC123",
			ModelName:     "pixel6",
			DUT:           androidDUT,
		},
	}

	companions, androids, err := Companions(devices)
	if err != nil {
		t.Error("Failed to call Companions: ", err)
	}

	if diff := cmp.Diff(companions, expectedCompanions,
		cmp.Comparer(proto.Equal)); diff != "" {
		t.Errorf("Got unexpected compainons (-got +want):\n%s", diff)
	}
	if diff := cmp.Diff(androids, expectedAndroids,
		cmp.Comparer(proto.Equal)); diff != "" {
		t.Errorf("Got unexpected androids (-got +want):\n%s", diff)
	}

}

var moblyResultYaml string = `
---
Requested Tests:
- test_hello
- test_multidevice
Type: TestNameList
...
---
Begin Time: 1726178525229
Details: null
End Time: 1726178525573
Extra Errors: {}
Extras: null
Parent: null
Result: PASS
Retry Parent: null
Signature: test_hello-1726178525229
Stacktrace: null
Termination Signal Type: null
Test Class: MultiDeviceTest
Test Name: test_hello
Type: Record
UID: null
...
---
Begin Time: 1726178525574
Details: null
End Time: 1726178525575
Extra Errors: {}
Extras: null
Parent: null
Result: PASS
Retry Parent: null
Signature: test_multidevice-1726178525574
Stacktrace: null
Termination Signal Type: null
Test Class: MultiDeviceTest
Test Name: test_multidevice
Type: Record
UID: null
...
---
Controller Info:
-   build_info:
        build_characteristics: desktop,device
        build_fingerprint: google/cf_x86_64_al/vsoc_x86_64_only:VanillaIceCream/MAIN.46f9824a/12360107:userdebug/dev-keys
        build_id: MAIN.46f9824a
        build_product: vsoc_x86_64_only
        build_type: userdebug
        build_version_codename: VanillaIceCream
        build_version_incremental: '12360107'
        build_version_sdk: '35'
        debuggable: '1'
        hardware: cutf_cvm
        product_name: cf_x86_64_al
    model: cf_x86_64_al
    serial: 127.0.0.1:38513
    user_added_info: {}
-   build_info:
        build_characteristics: desktop,device
        build_fingerprint: google/cf_x86_64_al/vsoc_x86_64_only:VanillaIceCream/MAIN.29688522/12359532:userdebug/dev-keys
        build_id: MAIN.29688522
        build_product: vsoc_x86_64_only
        build_type: userdebug
        build_version_codename: VanillaIceCream
        build_version_incremental: '12359532'
        build_version_sdk: '35'
        debuggable: '1'
        hardware: cutf_cvm
        product_name: cf_x86_64_al
    model: cf_x86_64_al
    serial: 127.0.0.1:34511
    user_added_info: {}
Controller Name: AndroidDevice
Test Class: MultiDeviceTest
Timestamp: 1726178525.5891118
Type: ControllerInfo
...
---
Error: 0
Executed: 2
Failed: 0
Passed: 2
Requested: 2
Skipped: 0
Type: Summary
...
`

func TestTranslateMoblyResults(t *testing.T) {
	startTime1 := time.UnixMilli(1726178525229)
	endTime1 := time.UnixMilli(1726178525573)
	duration1 := endTime1.Sub(startTime1)
	startTimeProto1 := timestamppb.New(startTime1)

	startTime2 := time.UnixMilli(1726178525574)
	endTime2 := time.UnixMilli(1726178525575)
	startTimeProto2 := timestamppb.New(startTime2)
	duration2 := endTime2.Sub(startTime2)
	resultsRootDir := "resultsRootDir"
	expected := []*api.TestCaseResult{
		{
			TestCaseId: &api.TestCase_Id{Value: "MultiDeviceTest.test_hello"},
			ResultDirPath: &_go.StoragePath{
				HostType: _go.StoragePath_LOCAL,
				Path:     resultsRootDir,
			},
			Verdict: &api.TestCaseResult_Pass_{Pass: &api.TestCaseResult_Pass{}},
			TestHarness: &api.TestHarness{
				TestHarnessType: &api.TestHarness_Mobly_{
					Mobly: &api.TestHarness_Mobly{},
				},
			},
			StartTime: startTimeProto1,
			Duration:  durationpb.New(duration1),
		},
		{
			TestCaseId: &api.TestCase_Id{Value: "MultiDeviceTest.test_multidevice"},
			ResultDirPath: &_go.StoragePath{
				HostType: _go.StoragePath_LOCAL,
				Path:     resultsRootDir,
			},
			Verdict: &api.TestCaseResult_Pass_{Pass: &api.TestCaseResult_Pass{}},
			TestHarness: &api.TestHarness{
				TestHarnessType: &api.TestHarness_Mobly_{
					Mobly: &api.TestHarness_Mobly{},
				},
			},
			StartTime: startTimeProto2,
			Duration:  durationpb.New(duration2),
		},
	}

	got, err := TranslateMoblyResults([]byte(moblyResultYaml), resultsRootDir)
	if err != nil {
		t.Errorf("Cannot translate mobly result: %s", err)
	}

	if diff := cmp.Diff(got, expected,
		cmp.Comparer(proto.Equal)); diff != "" {
		t.Errorf("Got unexpected mobly result (-got +want):\n%s", diff)
	}
}
