// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"go.chromium.org/chromiumos/config/go/test/api"
)

func TestGetTesthausURL(t *testing.T) {
	t.Parallel()

	Convey("Test get Testhaus URL", t, func() {
		tests := []struct {
			invocationName      string
			gcsURL              string
			wantTesthausPostfix string
		}{
			{
				invocationName:      "invocations/inv-123",
				gcsURL:              "gs://test_bucket/test_prefix",
				wantTesthausPostfix: "invocations/inv-123",
			},
			{
				invocationName:      "invocations/inv-123",
				gcsURL:              "",
				wantTesthausPostfix: "invocations/inv-123",
			},
			{
				invocationName:      "",
				gcsURL:              "gs://test_bucket/test_prefix",
				wantTesthausPostfix: "test_bucket/test_prefix",
			},
			{
				invocationName:      "",
				gcsURL:              "gs://",
				wantTesthausPostfix: "",
			},
			{
				invocationName:      "",
				gcsURL:              "",
				wantTesthausPostfix: "",
			},
		}
		for _, tc := range tests {

			Convey(fmt.Sprintf("When the invocation is: %q and gcsURL is: %q should get Testhaus URL postfix: %q", tc.invocationName, tc.gcsURL, tc.wantTesthausPostfix), func() {
				wantTesthausURL := fmt.Sprintf("%s%s", TesthausURLPrefix, tc.wantTesthausPostfix)
				gotTesthausURL := GetTesthausURL(tc.invocationName, tc.gcsURL)
				So(gotTesthausURL, ShouldEqual, wantTesthausURL)
			})
		}
	})
}

func Test_UpdateGivenTestResults_TradeFed(t *testing.T) {
	crosTestResponse := &api.CrosTestResponse{
		TestCaseResults: []*api.TestCaseResult{
			{
				TestCaseId: &api.TestCase_Id{
					Value: "tradefed.cts.CtsAppSecurityHostTestCases#android.appsecurity.cts.PkgInstallSignatureVerificationTest#testInstallV2TwoSignersRejectsWhenOneBroken",
				},
				Verdict: &api.TestCaseResult_Pass_{},
			},
			{
				TestCaseId: &api.TestCase_Id{
					Value: "tradefed.cts.CtsAppSecurityHostTestCases#android.appsecurity.cts.PkgInstallSignatureVerificationTest#testInstallV1OneSignerSHA384withRSA",
				},
				Verdict: &api.TestCaseResult_Pass_{},
			},
			{
				TestCaseId: &api.TestCase_Id{
					Value: "tradefed.cts.CtsBluetoothTestCases#android.bluetooth.cts.AdvertiseCallbackTest#advertiseFailure",
				},
				Verdict: &api.TestCaseResult_Pass_{},
			},
			{
				TestCaseId: &api.TestCase_Id{
					Value: "tradefed.cts.CtsBluetoothTestCases#android.bluetooth.cts.AdvertiseCallbackTest#advertiseSuccess",
				},
				Verdict: &api.TestCaseResult_Pass_{},
			},
		},
	}

	testMap := GenerateGivenTestResultsMap(crosTestResponse)
	if len(testMap) != 2 {
		t.Errorf("Expected len : 2, found %d", len(crosTestResponse.GetGivenTestResults()))
	}

	for parentTest, childTestResults := range testMap {
		if len(childTestResults) != 2 {
			t.Errorf("Expected len : 2, found %d", len(crosTestResponse.GetGivenTestResults()))
		}
		if parentTest != "tradefed.cts.CtsBluetoothTestCases" && parentTest != "tradefed.cts.CtsAppSecurityHostTestCases" {
			t.Errorf("Expected name found different %s", parentTest)
		}
	}

}

func Test_UpdateGivenTestResults_NonTradeFed(t *testing.T) {
	crosTestResponse := &api.CrosTestResponse{
		TestCaseResults: []*api.TestCaseResult{
			{
				TestCaseId: &api.TestCase_Id{
					Value: "crosier.QuickSettingsIntegrationTest.OpenOsSettings",
				},
				Verdict: &api.TestCaseResult_Pass_{},
			},
			{
				TestCaseId: &api.TestCase_Id{
					Value: "crosier.QuickSettingsIntegrationTest.ManagedDeviceInfo",
				},
				Verdict: &api.TestCaseResult_Pass_{},
			},
		},
	}

	testMap := GenerateGivenTestResultsMap(crosTestResponse)

	if len(testMap) != 0 {
		t.Errorf("Expected len : 0, found %d", len(crosTestResponse.GetGivenTestResults()))
	}
}
