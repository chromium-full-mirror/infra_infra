// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package service

import (
	"testing"

	"google.golang.org/protobuf/types/known/anypb"

	_go "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/metadata"
	"go.chromium.org/chromiumos/config/go/test/artifact"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/cmd/cft/publish/commonutils/clients/rdbclient"
)

func TestIsChromiumTest(t *testing.T) {
	t.Parallel()

	ftt.Run("Is Chromium Test", t, func(t *ftt.Test) {
		// Create a test result proto with resultdb_settings flag in test args.
		testResult := &artifact.TestResult{
			TestRuns: []*artifact.TestRun{
				{
					ExecutionMetadata: &artifact.ExecutionMetadata{
						TestArgs: map[string]string{
							"resultdb_settings": "test",
						},
					},
				},
			},
		}

		// Call isChromiumTest.
		isChromium := isChromiumTest(testResult)

		// Verify that the function returns true.
		assert.That(t, isChromium, should.BeTrue)
	})

	ftt.Run("Is not Chromium Test", t, func(t *ftt.Test) {
		// Create a test result proto without resultdb_settings flag in test
		// args.
		testResult := &artifact.TestResult{
			TestRuns: []*artifact.TestRun{
				{
					ExecutionMetadata: &artifact.ExecutionMetadata{
						TestArgs: map[string]string{},
					},
				},
			},
		}

		// Call isChromiumTest.
		isChromium := isChromiumTest(testResult)

		// Verify that the function returns false.
		assert.That(t, isChromium, should.BeFalse)
	})
}

func TestExtractBaseChromiumRDBConfig(t *testing.T) {
	t.Parallel()

	ftt.Run("Extract Base Chromium RDB Config", t, func(t *ftt.Test) {
		// Create a test args map with resultdb_settings flag.
		testArgs := map[string]string{
			"resultdb_settings": "eyJiYXNlX3ZhcmlhbnQiOiB7ImJ1aWxkZXIiOiAiY2hyb21lb3MtYmV0dHktY2hyb21lIiwgImNyb3NfaW1nIjogImJldHR5LXJlbGVhc2UvUjEyNy0xNTkxMi4wLjAiLCAiZGV2aWNlX3R5cGUiOiAiYmV0dHkiLCAib3MiOiAiQ2hyb21lT1MiLCAidGVzdF9zdWl0ZSI6ICJtZWRpYV91bml0dGVzdHMgUkVMRUFTRV9MS0dNIn0sICJjb2VyY2VfbmVnYXRpdmVfZHVyYXRpb24iOiB0cnVlLCAiZXhvbmVyYXRlX3VuZXhwZWN0ZWRfcGFzcyI6IHRydWUsICJpbmNsdWRlIjogZmFsc2UsICJyZXN1bHRfZm9ybWF0IjogImd0ZXN0IiwgInRlc3RfaWRfcHJlZml4IjogIm5pbmphOi8vbWVkaWE6bWVkaWFfdW5pdHRlc3RzLyJ9",
		}
		wantRDBConfig := map[string]any{
			"base_variant": map[string]any{
				"builder":     "chromeos-betty-chrome",
				"cros_img":    "betty-release/R127-15912.0.0",
				"device_type": "betty",
				"os":          "ChromeOS",
				"test_suite":  "media_unittests RELEASE_LKGM",
			},
			"result_format":             "gtest",
			"coerce_negative_duration":  true,
			"exonerate_unexpected_pass": true,
			"include":                   false,
			"test_id_prefix":            "ninja://media:media_unittests/",
		}

		// Call extractBaseChromiumRDBConfig.
		gotRDBSettings, err := extractBaseChromiumRDBConfig(testArgs)

		// Verify that the function returns the correct rdb settings.
		assert.Loosely(t, err, should.BeNil)
		assert.That(t, gotRDBSettings, should.Match(wantRDBConfig))
	})

	ftt.Run("Extract Base Chromium RDB Config with invalid base64 string", t, func(t *ftt.Test) {
		// Create a test args map with invalid resultdb_settings flag.
		testArgs := map[string]string{
			"resultdb_settings": "invalid_base64_string",
		}

		// Call extractBaseChromiumRDBConfig.
		gotRDBSettings, err := extractBaseChromiumRDBConfig(testArgs)

		// Verify that the function returns an error.
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, gotRDBSettings, should.BeNil)
	})

	ftt.Run("Extract Base Chromium RDB Config with invalid JSON string", t, func(t *ftt.Test) {
		// Create a test args map with invalid resultdb_settings flag.
		testArgs := map[string]string{
			"resultdb_settings": "eyJiYXNlX3ZhcmlhbnQiOiB7ImJ1aWxkZXIiOiAiY2hyb21lb3MtYmV0dHktY2hyb21lIiwgImNyb3NfaW1nIjogImJldHR5LXJlbGVhc2UvUjEyNy0xNTkxMi4wLjAiLCAiZGV2aWNlX3R5cGUiOiAiYmV0dHkiLCAib3MiOiAiQ2hyb21lT1MiLCAidGVzdF9zdWl0ZSI6ICJtZWRpYV91bml0dGVzdHMgUkVMRUFTRV9MS0dNIn0sICJjb2VyY2VfbmVnYXRpdmVfZHVyYXRpb24iOiB0cnVlLCAiZXhvbmVyYXRlX3VuZXhwZWN0ZWRfcGFzcyI6IHRydWUsICJpbmNsdWRlIjogZmFsc2UsICJyZXN1bHRfZm9ybWF0IjogImd0ZXN0IiwgInRlc3RfaWRfcHJlZml4IjogIm5pbmphOi8vbWVkaWE6bWV9",
		}

		// Call extractBaseChromiumRDBConfig.
		rdbSettings, err := extractBaseChromiumRDBConfig(testArgs)

		// Verify that the function returns an error.
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, rdbSettings, should.BeNil)
	})
}

func TestChromiumTestRDBConfig(t *testing.T) {
	t.Parallel()

	ftt.Run("Chromium Test RDB Config for gtest result format", t, func(t *ftt.Test) {
		// Create a test result proto with resultdb_settings flag in test args.
		testResult := &artifact.TestResult{
			TestInvocation: &artifact.TestInvocation{
				PrimaryExecutionInfo: &artifact.ExecutionInfo{
					BuildInfo: &artifact.BuildInfo{
						Name: "hatch-cq/R106-15048.0.0",
					},
					EnvInfo: &artifact.ExecutionInfo_SkylabInfo{
						SkylabInfo: &artifact.SkylabInfo{
							BuildbucketInfo: &artifact.BuildbucketInfo{
								AncestorIds: []int64{
									8814950840874708945,
									8814951792758733697,
								},
							},
						},
					},
				},
			},
			TestRuns: []*artifact.TestRun{
				{
					TestCaseInfo: &artifact.TestCaseInfo{
						TestCaseResult: &api.TestCaseResult{
							ResultDirPath: &_go.StoragePath{
								Path: "test/results/dir",
							},
						},
					},
					ExecutionMetadata: &artifact.ExecutionMetadata{
						TestArgs: map[string]string{
							"resultdb_settings": "eyJiYXNlX3ZhcmlhbnQiOiB7ImJ1aWxkZXIiOiAiY2hyb21lb3MtYmV0dHktY2hyb21lIiwgImNyb3NfaW1nIjogImJldHR5LXJlbGVhc2UvUjEyNy0xNTkxMi4wLjAiLCAiZGV2aWNlX3R5cGUiOiAiYmV0dHkiLCAib3MiOiAiQ2hyb21lT1MiLCAidGVzdF9zdWl0ZSI6ICJtZWRpYV91bml0dGVzdHMgUkVMRUFTRV9MS0dNIn0sICJjb2VyY2VfbmVnYXRpdmVfZHVyYXRpb24iOiB0cnVlLCAiZXhvbmVyYXRlX3VuZXhwZWN0ZWRfcGFzcyI6IHRydWUsICJpbmNsdWRlIjogZmFsc2UsICJyZXN1bHRfZm9ybWF0IjogImd0ZXN0IiwgInRlc3RfaWRfcHJlZml4IjogIm5pbmphOi8vbWVkaWE6bWVkaWFfdW5pdHRlc3RzLyJ9",
						},
					},
				},
			},
		}
		wantRDBConfig := &rdbclient.RdbStreamConfig{
			BaseTags: map[string]string{
				"ancestor_buildbucket_ids": "8814950840874708945,8814951792758733697",
				"build":                    "R106-15048.0.0",
				"image":                    "hatch-cq/R106-15048.0.0",
			},
			BaseVariant: map[string]string{
				"builder":     "chromeos-betty-chrome",
				"cros_img":    "betty-release/R127-15912.0.0",
				"device_type": "betty",
				"os":          "ChromeOS",
				"test_suite":  "media_unittests RELEASE_LKGM",
			},
			ResultFile:              "test/results/dir/chromium/results/output.json",
			ResultFormat:            "gtest",
			TesthausBaseURL:         "testhaus_url",
			TestIdPrefix:            "ninja://media:media_unittests/",
			CoerceNegativeDuration:  true,
			ExonerateUnexpectedPass: true,
			Include:                 false,
		}

		// Call chromiumTestRDBConfig.
		gotRDBconfig, err := chromiumTestRDBConfig(testResult, map[string]string{}, map[string]string{}, "testhaus_url", "")

		// Verify that the function returns the correct rdb config.
		assert.Loosely(t, err, should.BeNil)
		assert.That(t, gotRDBconfig, should.Match(wantRDBConfig))
	})

	ftt.Run("Chromium Test RDB Config for gtest result format", t, func(t *ftt.Test) {
		// Create a test result proto with resultdb_settings flag in test args.
		testResult := &artifact.TestResult{
			TestRuns: []*artifact.TestRun{
				{
					TestCaseInfo: &artifact.TestCaseInfo{
						TestCaseResult: &api.TestCaseResult{
							ResultDirPath: &_go.StoragePath{
								Path: "test/results/dir",
							},
							TestCaseMetadata: &api.TestCaseMetadata{
								TestCase: &api.TestCase{
									Id: &api.TestCase_Id{
										Value: "tauto.chromium_Graphics",
									},
									Name: "chromium_Graphics",
								},
							},
						},
					},
					ExecutionMetadata: &artifact.ExecutionMetadata{
						TestArgs: map[string]string{
							"resultdb_settings": "eyJiYXNlX3ZhcmlhbnQiOiB7ImJ1aWxkZXIiOiAiY2hyb21lb3MtYmV0dHktY2hyb21lIiwgImNyb3NfaW1nIjogImJldHR5LXJlbGVhc2UvUjEyNy0xNTkxMi4wLjAiLCAiZGV2aWNlX3R5cGUiOiAiYmV0dHkiLCAib3MiOiAiQ2hyb21lT1MiLCAidGVzdF9zdWl0ZSI6ICJtZWRpYV91bml0dGVzdHMgUkVMRUFTRV9MS0dNIn0sICJjb2VyY2VfbmVnYXRpdmVfZHVyYXRpb24iOiB0cnVlLCAiZXhvbmVyYXRlX3VuZXhwZWN0ZWRfcGFzcyI6IHRydWUsICJpbmNsdWRlIjogZmFsc2UsICJyZXN1bHRfZm9ybWF0IjogIm5hdGl2ZSIsICJ0ZXN0X2lkX3ByZWZpeCI6ICJuaW5qYTovL21lZGlhOm1lZGlhX3VuaXR0ZXN0cy8ifQ==",
						},
					},
				},
			},
		}
		wantRDBConfig := &rdbclient.RdbStreamConfig{
			BaseTags: map[string]string{},
			BaseVariant: map[string]string{
				"builder":     "chromeos-betty-chrome",
				"cros_img":    "betty-release/R127-15912.0.0",
				"device_type": "betty",
				"os":          "ChromeOS",
				"test_suite":  "media_unittests RELEASE_LKGM",
			},
			ResultFile:              "test/results/dir/chromium_Graphics/results/native_results.jsonl",
			ResultFormat:            "native",
			TesthausBaseURL:         "testhaus_url",
			TestIdPrefix:            "ninja://media:media_unittests/",
			CoerceNegativeDuration:  true,
			ExonerateUnexpectedPass: true,
			Include:                 false,
		}

		// Call chromiumTestRDBConfig.
		gotRDBconfig, err := chromiumTestRDBConfig(testResult, map[string]string{}, map[string]string{}, "testhaus_url", "")

		// Verify that the function returns the correct rdb config.
		assert.Loosely(t, err, should.BeNil)
		assert.That(t, gotRDBconfig, should.Match(wantRDBConfig))
	})

	ftt.Run("Chromium Test RDB Config for tast result format", t, func(t *ftt.Test) {
		// Create a test result proto with resultdb_settings flag in test args.
		testResult := &artifact.TestResult{
			TestRuns: []*artifact.TestRun{
				{
					TestCaseInfo: &artifact.TestCaseInfo{
						TestCaseResult: &api.TestCaseResult{
							ResultDirPath: &_go.StoragePath{
								Path: "test/results/dir",
							},
						},
					},
					ExecutionMetadata: &artifact.ExecutionMetadata{
						TestArgs: map[string]string{
							"resultdb_settings": "eyJiYXNlX3ZhcmlhbnQiOiB7ImJ1aWxkZXIiOiAiY2hyb21lb3MtYmV0dHktY2hyb21lIiwgImNyb3NfaW1nIjogImJldHR5LXJlbGVhc2UvUjEyNy0xNTkxMi4wLjAiLCAiZGV2aWNlX3R5cGUiOiAiYmV0dHkiLCAib3MiOiAiQ2hyb21lT1MiLCAidGVzdF9zdWl0ZSI6ICJtZWRpYV91bml0dGVzdHMgUkVMRUFTRV9MS0dNIn0sICJjb2VyY2VfbmVnYXRpdmVfZHVyYXRpb24iOiB0cnVlLCAiZXhvbmVyYXRlX3VuZXhwZWN0ZWRfcGFzcyI6IHRydWUsICJpbmNsdWRlIjogZmFsc2UsICJyZXN1bHRfZm9ybWF0IjogInRhc3QiLCAidGVzdF9pZF9wcmVmaXgiOiAibmluamE6Ly9tZWRpYTptZWRpYV91bml0dGVzdHMvIn0=",
						},
					},
				},
			},
		}
		wantResultFile := "cros-test-777c42c3/cros-test/results/tauto/results-1-tast.chrome-from-gcs/tast/results/streamed_results.jsonl"
		wantRDBConfig := &rdbclient.RdbStreamConfig{
			BaseTags: map[string]string{},
			BaseVariant: map[string]string{
				"builder":     "chromeos-betty-chrome",
				"cros_img":    "betty-release/R127-15912.0.0",
				"device_type": "betty",
				"os":          "ChromeOS",
				"test_suite":  "media_unittests RELEASE_LKGM",
			},
			ResultFile:              wantResultFile,
			ResultFormat:            ResultAdapterResultFormat,
			TesthausBaseURL:         "testhaus_url",
			TestIdPrefix:            "ninja://media:media_unittests/",
			CoerceNegativeDuration:  true,
			ExonerateUnexpectedPass: true,
			Include:                 false,
		}

		// Call chromiumTestRDBConfig.
		gotRDBconfig, err := chromiumTestRDBConfig(testResult, map[string]string{}, map[string]string{}, "testhaus_url", wantResultFile)

		// Verify that the function returns the correct rdb config.
		assert.Loosely(t, err, should.BeNil)
		assert.That(t, gotRDBconfig, should.Match(wantRDBConfig))
	})

	ftt.Run("Chromium Test RDB Config with invalid resultdb_settings flag", t, func(t *ftt.Test) {
		// Create a test result proto with invalid resultdb_settings flag in test args.
		testResult := &artifact.TestResult{
			TestRuns: []*artifact.TestRun{
				{
					TestCaseInfo: &artifact.TestCaseInfo{
						TestCaseResult: &api.TestCaseResult{
							ResultDirPath: &_go.StoragePath{
								Path: "test/results/dir",
							},
						},
					},
					ExecutionMetadata: &artifact.ExecutionMetadata{
						TestArgs: map[string]string{
							"resultdb_settings": "invalid_base64_string",
						},
					},
				},
			},
		}

		// Call chromiumTestRDBConfig.
		config, err := chromiumTestRDBConfig(testResult, map[string]string{}, map[string]string{}, "testhaus_url", "")

		// Verify that the function returns an error.
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, config, should.BeNil)
	})
}

func TestIngestPostProcessResponses(t *testing.T) {
	t.Parallel()

	ftt.Run("Ingest post process responses", t, func(t *ftt.Test) {
		// Create a test result proto.
		testResult := &artifact.TestResult{
			TestInvocation: &artifact.TestInvocation{
				PrimaryExecutionInfo: &artifact.ExecutionInfo{
					BuildInfo: &artifact.BuildInfo{
						BuildMetadata: &artifact.BuildMetadata{},
					},
				},
			},
			TestRuns: []*artifact.TestRun{
				{
					TestCaseInfo: &artifact.TestCaseInfo{
						TestCaseResult: &api.TestCaseResult{
							TestCaseId: &api.TestCase_Id{
								Value: "tast.gscdevboard.GSCSysinfo",
							},
						},
					},
				},
			},
		}

		servoInfo := &artifact.BuildMetadata_ServoInfo{
			ServodVersion: "v1.0.2345-4b2de21e 2024-08-17 00:30:57",
			ServoType:     "servo_v4_with_c2d2_and_ccd_ti50",
			ServoVersions: "c2d2_v2.4.73-d771c18ba9,0.24.40/ti50_common_prepvt-15086.B:v0.0.355-15c69d7f,fizz-labstation-release/R115-15474.55.0,servo_v4_v2.4.58-c37246f9c",
		}
		servoInfoAny, _ := anypb.New(servoInfo)

		// Create a post process response proto.
		postProcessResps := &api.RunActivitiesResponse{
			Responses: []*api.RunActivityResponse{
				{
					Response: &api.RunActivityResponse_GetFwInfoResponse{
						GetFwInfoResponse: &api.GetFWInfoResponse{
							RoFwid:        "Google_Voema.13672.224.0",
							RwFwid:        "Google_Voema.13672.224.1",
							KernelVersion: "5.4.151-16902-g93699f4e73de",
						},
					},
				},
				{
					Response: &api.RunActivityResponse_GetGfxInfoResponse{
						GetGfxInfoResponse: &api.GetGfxInfoResponse{
							GfxLabels: map[string]string{
								"display_panel_name":    "AUO 10380",
								"display_present_hdr":   "hdr unsupported",
								"display_present_psr":   "psr unsupported",
								"display_present_vrr":   "vrr unsupported",
								"display_refresh_rate":  "60.06",
								"display_resolution":    "1366x768",
								"gpu_family":            "cezanne",
								"gpu_id":                "amd:15e7",
								"gpu_open_gles_version": "3.2",
								"gpu_vendor":            "amd",
								"gpu_vulkan_version":    "1.3.274",
								"platform_cpu_vendor":   "amd",
								"platform_disk_size":    "128",
								"platform_memory_size":  "32",
							},
						},
					},
				},
				{
					Response: &api.RunActivityResponse_GetAvlInfoResponse{
						GetAvlInfoResponse: &api.GetAvlInfoResponse{
							AvlInfos: map[string]*api.AvlInfo{
								"tast.gscdevboard.GSCSysinfo": {
									AvlComponentType: "storage",
									AvlPartFirmware:  "0xa200000000000000",
									AvlPartModel:     "0x0000f5 MMC32G",
								},
							},
						},
					},
				},
				{
					Response: &api.RunActivityResponse_GetServoInfoResponse{
						GetServoInfoResponse: &api.GetServoInfoResponse{
							ServoInfo: servoInfoAny,
						},
					},
				},
			},
		}

		// Call ingestPostProcessResponses.
		ingestPostProcessResponses(testResult, postProcessResps)

		// Verify that the all info are populated correctly.
		wantBuildMetadata := &artifact.BuildMetadata{
			Firmware: &artifact.BuildMetadata_Firmware{
				RoVersion: "Google_Voema.13672.224.0",
				RwVersion: "Google_Voema.13672.224.1",
			},
			Kernel: &artifact.BuildMetadata_Kernel{
				Version: "5.4.151-16902-g93699f4e73de",
			},
			GfxInfo: &artifact.BuildMetadata_GfxInfo{
				DisplayPanelName:   "AUO 10380",
				DisplayPresentHdr:  "hdr unsupported",
				DisplayPresentPsr:  "psr unsupported",
				DisplayPresentVrr:  "vrr unsupported",
				DisplayRefreshRate: "60.06",
				DisplayResolution:  "1366x768",
				GpuFamily:          "cezanne",
				GpuId:              "amd:15e7",
				GpuOpenGlesVersion: "3.2",
				GpuVendor:          "amd",
				GpuVulkanVersion:   "1.3.274",
				PlatformCpuVendor:  "amd",
				PlatformDiskSize:   128,
				PlatformMemorySize: 32,
			},
			ServoInfo: servoInfo,
		}
		wantAVL := &artifact.AvlInfo{
			AvlComponentType: "storage",
			AvlPartFirmware:  "0xa200000000000000",
			AvlPartModel:     "0x0000f5 MMC32G",
		}
		assert.That(t, testResult.TestInvocation.PrimaryExecutionInfo.BuildInfo.BuildMetadata, should.Match(wantBuildMetadata))
		assert.That(t, testResult.TestRuns[0].TestCaseInfo.AvlInfo, should.Match(wantAVL))
	})
}

func TestPopulateFirmwareInfo(t *testing.T) {
	t.Parallel()

	ftt.Run("Populates Firmware info", t, func(t *ftt.Test) {
		// Create a build metadata proto.
		buildMetadata := &artifact.BuildMetadata{}

		// Create a firmware info response proto.
		fwInfoResp := &api.GetFWInfoResponse{
			RoFwid: "Google_Voema.13672.224.0",
			RwFwid: "Google_Voema.13672.224.1",
		}

		// Call populateFirmwareInfo.
		populateFirmwareInfo(buildMetadata, fwInfoResp)

		// Verify that the firmware info is populated correctly.
		wantFwInfo := &artifact.BuildMetadata_Firmware{
			RoVersion: "Google_Voema.13672.224.0",
			RwVersion: "Google_Voema.13672.224.1",
		}
		assert.That(t, buildMetadata.Firmware, should.Match(wantFwInfo))
	})

	ftt.Run("Populates Firmware info with empty values", t, func(t *ftt.Test) {
		// Create a build metadata proto.
		buildMetadata := &artifact.BuildMetadata{}

		// Create a firmware info response proto with empty values.
		fwInfoResp := &api.GetFWInfoResponse{}

		// Call populateFirmwareInfo.
		populateFirmwareInfo(buildMetadata, fwInfoResp)

		// Verify that the firmware info is populated correctly.
		assert.That(t, buildMetadata.Firmware, should.Match(&artifact.BuildMetadata_Firmware{}))
	})

	ftt.Run("Skip if the response is nil", t, func(t *ftt.Test) {
		// Create a build metadata proto.
		buildMetadata := &artifact.BuildMetadata{}

		// Call populateFirmwareInfo.
		populateFirmwareInfo(buildMetadata, nil)

		// Verify no value is populated.
		assert.Loosely(t, buildMetadata.Firmware, should.BeNil)
	})
}

func TestPopulateKernelInfo(t *testing.T) {
	t.Parallel()

	ftt.Run("Populates Kernel info", t, func(t *ftt.Test) {
		// Create a build metadata proto.
		buildMetadata := &artifact.BuildMetadata{}

		// Create a firmware info response proto.
		fwInfoResp := &api.GetFWInfoResponse{
			KernelVersion: "5.4.151-16902-g93699f4e73de",
		}

		// Call populateKernelInfo.
		populateKernelInfo(buildMetadata, fwInfoResp)

		// Verify that the kernel info is populated correctly.
		wantKernel := &artifact.BuildMetadata_Kernel{
			Version: "5.4.151-16902-g93699f4e73de",
		}
		assert.That(t, buildMetadata.Kernel, should.Match(wantKernel))
	})

	ftt.Run("Populates Kernel info with empty values", t, func(t *ftt.Test) {
		// Create a build metadata proto.
		buildMetadata := &artifact.BuildMetadata{}

		// Create a firmware info response proto with empty values.
		fwInfoResp := &api.GetFWInfoResponse{}

		// Call populateKernelInfo.
		populateKernelInfo(buildMetadata, fwInfoResp)

		// Verify that the kernel info is populated correctly.
		assert.That(t, buildMetadata.Kernel, should.Match(&artifact.BuildMetadata_Kernel{}))
	})

	ftt.Run("Skip if the response is nil", t, func(t *ftt.Test) {
		// Create a build metadata proto.
		buildMetadata := &artifact.BuildMetadata{}

		// Call populateKernelInfo.
		populateKernelInfo(buildMetadata, nil)

		// Verify no value is populated.
		assert.Loosely(t, buildMetadata.Kernel, should.BeNil)
	})
}

func TestPopulateGSCFirmwareInfo(t *testing.T) {
	t.Parallel()

	ftt.Run("Populates GSC Firmware info", t, func(t *ftt.Test) {
		// Create a test result proto with a test run and a test case.
		testResult := &artifact.TestResult{
			TestRuns: []*artifact.TestRun{
				{
					TestCaseInfo: &artifact.TestCaseInfo{
						TestCaseResult: &api.TestCaseResult{},
					},
				},
			},
		}

		// Create a firmware info response proto.
		fwInfoResp := &api.GetFWInfoResponse{
			GscRo: "Google_Voema.13672.224.0",
			GscRw: "Google_Voema.13672.224.1",
		}

		// Call populateGSCFirmwareInfo.
		populateGSCFirmwareInfo(testResult, fwInfoResp)

		// Verify that the GSC firmware info is populated correctly.
		wantGscFwInfo := &artifact.GscInfo{
			GscRoVersion: "Google_Voema.13672.224.0",
			GscRwVersion: "Google_Voema.13672.224.1",
		}
		assert.That(t, testResult.TestRuns[0].TestCaseInfo.GscInfo, should.Match(wantGscFwInfo))
	})

	ftt.Run("Populates GSC Firmware info with empty values", t, func(t *ftt.Test) {
		// Create a test result proto with a test run and a test case.
		testResult := &artifact.TestResult{
			TestRuns: []*artifact.TestRun{
				{
					TestCaseInfo: &artifact.TestCaseInfo{
						TestCaseResult: &api.TestCaseResult{},
					},
				},
			},
		}

		// Create a firmware info response proto with empty values.
		fwInfoResp := &api.GetFWInfoResponse{}

		// Call populateGSCFirmwareInfo.
		populateGSCFirmwareInfo(testResult, fwInfoResp)

		// Verify that the GSC firmware info is populated correctly.
		assert.That(t, testResult.TestRuns[0].TestCaseInfo.GscInfo, should.Match(&artifact.GscInfo{}))
	})

	ftt.Run("Skip if the response is nil", t, func(t *ftt.Test) {
		// Create a test result proto with a test run and a test case.
		testResult := &artifact.TestResult{
			TestRuns: []*artifact.TestRun{
				{
					TestCaseInfo: &artifact.TestCaseInfo{
						TestCaseResult: &api.TestCaseResult{},
					},
				},
			},
		}

		// Call populateGSCFirmwareInfo.
		populateGSCFirmwareInfo(testResult, nil)

		// Verify no value is populated.
		assert.Loosely(t, testResult.TestRuns[0].TestCaseInfo.GscInfo, should.BeNil)
	})
}

func TestPopulateGfxInfo(t *testing.T) {
	t.Parallel()

	ftt.Run("Populates Graphics info", t, func(t *ftt.Test) {
		// Create a build metadata proto.
		buildMetadata := &artifact.BuildMetadata{}

		// Create a Graphics info response proto.
		gfxInfoResp := &api.GetGfxInfoResponse{
			GfxLabels: map[string]string{
				"display_panel_name":    "AUO 10380",
				"display_present_hdr":   "hdr unsupported",
				"display_present_psr":   "psr unsupported",
				"display_present_vrr":   "vrr unsupported",
				"display_refresh_rate":  "60.06",
				"display_resolution":    "1366x768",
				"gpu_family":            "cezanne",
				"gpu_id":                "amd:15e7",
				"gpu_open_gles_version": "3.2",
				"gpu_vendor":            "amd",
				"gpu_vulkan_version":    "1.3.274",
				"platform_cpu_vendor":   "amd",
				"platform_disk_size":    "128",
				"platform_memory_size":  "32",
			},
		}

		// Call populateGfxInfo.
		populateGfxInfo(buildMetadata, gfxInfoResp)

		// Verify that the Graphics info is populated correctly.
		wantGFXInfo := &artifact.BuildMetadata_GfxInfo{
			DisplayPanelName:   "AUO 10380",
			DisplayPresentHdr:  "hdr unsupported",
			DisplayPresentPsr:  "psr unsupported",
			DisplayPresentVrr:  "vrr unsupported",
			DisplayRefreshRate: "60.06",
			DisplayResolution:  "1366x768",
			GpuFamily:          "cezanne",
			GpuId:              "amd:15e7",
			GpuOpenGlesVersion: "3.2",
			GpuVendor:          "amd",
			GpuVulkanVersion:   "1.3.274",
			PlatformCpuVendor:  "amd",
			PlatformDiskSize:   128,
			PlatformMemorySize: 32,
		}
		assert.Loosely(t, buildMetadata.GfxInfo, should.Match(wantGFXInfo))
	})

	ftt.Run("Populates Graphics info with empty values", t, func(t *ftt.Test) {
		// Create a build metadata proto.
		buildMetadata := &artifact.BuildMetadata{}

		// Create a Graphics info response proto with empty values.
		gfxInfoResp := &api.GetGfxInfoResponse{}

		// Call populateGfxInfo.
		populateGfxInfo(buildMetadata, gfxInfoResp)

		// Verify that the Graphics info is populated correctly.
		assert.That(t, buildMetadata.GfxInfo, should.Match(&artifact.BuildMetadata_GfxInfo{}))
	})

	ftt.Run("Skip if the response is nil", t, func(t *ftt.Test) {
		// Create a build metadata proto.
		buildMetadata := &artifact.BuildMetadata{}

		// Call populateGfxInfo.
		populateGfxInfo(buildMetadata, nil)

		// Verify no value is populated.
		assert.Loosely(t, buildMetadata.GfxInfo, should.BeNil)
	})
}

func TestPopulateAVLInfo(t *testing.T) {
	t.Parallel()

	ftt.Run("Populates AVL info", t, func(t *ftt.Test) {
		// Create a test result proto with a test run and a test case.
		testResult := &artifact.TestResult{
			TestRuns: []*artifact.TestRun{
				{
					TestCaseInfo: &artifact.TestCaseInfo{
						TestCaseResult: &api.TestCaseResult{
							TestCaseId: &api.TestCase_Id{
								Value: "tast.gscdevboard.GSCSysinfo",
							},
						},
					},
				},
			},
		}

		// Create an AVL info response proto with AVL info for the test case.
		avlInfoResp := &api.GetAvlInfoResponse{
			AvlInfos: map[string]*api.AvlInfo{
				"tast.gscdevboard.GSCSysinfo": {
					AvlComponentType: "storage",
					AvlPartFirmware:  "0xa200000000000000",
					AvlPartModel:     "0x0000f5 MMC32G",
				},
			},
		}

		// Call populateAVLInfo.
		populateAVLInfo(testResult, avlInfoResp)

		wantAVL := &artifact.AvlInfo{
			AvlComponentType: "storage",
			AvlPartFirmware:  "0xa200000000000000",
			AvlPartModel:     "0x0000f5 MMC32G",
		}
		assert.Loosely(t, testResult.TestRuns[0].TestCaseInfo.AvlInfo, should.Match(wantAVL))
	})

	ftt.Run("Populates AVL info with missing test case", t, func(t *ftt.Test) {
		// Create a test result proto with a test run without any test case.
		testResult := &artifact.TestResult{
			TestRuns: []*artifact.TestRun{
				{
					TestCaseInfo: &artifact.TestCaseInfo{},
				},
			},
		}

		// Create an AVL info response proto with AVL info for the test case.
		avlInfoResp := &api.GetAvlInfoResponse{
			AvlInfos: map[string]*api.AvlInfo{
				"tast.gscdevboard.GSCSysinfo": {
					AvlComponentType: "storage",
					AvlPartFirmware:  "0xa200000000000000",
					AvlPartModel:     "0x0000f5 MMC32G",
				},
			},
		}

		// Call populateAVLInfo.
		populateAVLInfo(testResult, avlInfoResp)

		assert.Loosely(t, testResult.TestRuns[0].TestCaseInfo.AvlInfo, should.BeNil)
	})

	ftt.Run("Populates AVL info with missing AVL info", t, func(t *ftt.Test) {
		// Create a test result proto with a test run and a test case.
		testResult := &artifact.TestResult{
			TestRuns: []*artifact.TestRun{
				{
					TestCaseInfo: &artifact.TestCaseInfo{
						TestCaseResult: &api.TestCaseResult{
							TestCaseId: &api.TestCase_Id{
								Value: "tast.gscdevboard.GSCSysinfo",
							},
						},
					},
				},
			},
		}

		// Create an AVL info response proto with AVL info for the test case.
		avlInfoResp := &api.GetAvlInfoResponse{
			AvlInfos: map[string]*api.AvlInfo{},
		}

		// Call populateAVLInfo.
		populateAVLInfo(testResult, avlInfoResp)

		assert.Loosely(t, testResult.TestRuns[0].TestCaseInfo.AvlInfo, should.BeNil)
	})

	ftt.Run("Skip if the response is nil", t, func(t *ftt.Test) {
		// Create a test result proto with a test run and a test case.
		testResult := &artifact.TestResult{
			TestRuns: []*artifact.TestRun{
				{
					TestCaseInfo: &artifact.TestCaseInfo{
						TestCaseResult: &api.TestCaseResult{
							TestCaseId: &api.TestCase_Id{
								Value: "tast.gscdevboard.GSCSysinfo",
							},
						},
					},
				},
			},
		}

		// Call populateAVLInfo.
		populateAVLInfo(testResult, nil)

		assert.Loosely(t, testResult.TestRuns[0].TestCaseInfo.AvlInfo, should.BeNil)
	})
}

func TestPopulateGSCInfo(t *testing.T) {
	t.Parallel()

	// Create a GSC devboard info response proto with GSC devboard info for the test case.
	gscInfo := &artifact.GscInfo{
		GscRoVersion:   "0.0.59",
		GscRwVersion:   "0.26.112",
		GscRwBranch:    "tot:v0.0",
		GscRwRev:       "1479",
		GscRwSha:       "-38cb7844",
		GscBuildurl:    "gs://chromeos-image-dummy/firmware-ti50-postsubmit/R123-12345.0.0",
		GscTestbedType: "gsc_dt_shield",
		GscCcdSerial:   "1482101a-4c2ac261",
	}
	gscInfoAny, _ := anypb.New(gscInfo)
	testName := "tast.gscdevboard.GSCSysinfo"
	gscInfoResp := &api.GetGscInfoResponse{
		// Generate the &artifact.GscInfo proto based on the follow data
		GscInfos: map[string]*anypb.Any{
			testName: gscInfoAny,
		},
	}

	ftt.Run("Populates GSC devboard info", t, func(t *ftt.Test) {
		// Create a test result proto with a test run and a test case.
		testResult := &artifact.TestResult{
			TestRuns: []*artifact.TestRun{
				{
					TestCaseInfo: &artifact.TestCaseInfo{
						TestCaseResult: &api.TestCaseResult{
							TestCaseId: &api.TestCase_Id{
								Value: testName,
							},
						},
					},
				},
			},
		}

		// Call populateGSCInfo.
		populateGSCInfo(testResult, gscInfoResp)

		assert.That(t, testResult.TestRuns[0].TestCaseInfo.GscInfo, should.Match(gscInfo))
	})

	ftt.Run("Populates GSC devboard info with missing test case", t, func(t *ftt.Test) {
		// Create a test result proto with a test run without any test case.
		testResult := &artifact.TestResult{
			TestRuns: []*artifact.TestRun{
				{
					TestCaseInfo: &artifact.TestCaseInfo{},
				},
			},
		}

		// Call populateGSCInfo.
		populateGSCInfo(testResult, gscInfoResp)

		// Verify that the GSC devboard info is not populated.
		assert.Loosely(t, testResult.TestRuns[0].TestCaseInfo.GscInfo, should.BeNil)
	})

	ftt.Run("Populates GSC devboard info with missing GSC devboard info", t, func(t *ftt.Test) {
		// Create a test result proto with a test run and a test case.
		testResult := &artifact.TestResult{
			TestRuns: []*artifact.TestRun{
				{
					TestCaseInfo: &artifact.TestCaseInfo{
						TestCaseResult: &api.TestCaseResult{
							TestCaseId: &api.TestCase_Id{
								Value: testName,
							},
						},
					},
				},
			},
		}

		// Create a GSC devboard info response proto without GSC devboard info for the test case.
		gscInfoResp := &api.GetGscInfoResponse{
			GscInfos: map[string]*anypb.Any{},
		}

		// Call populateGSCInfo.
		populateGSCInfo(testResult, gscInfoResp)

		// Verify that the GSC devboard info is not populated.
		assert.Loosely(t, testResult.TestRuns[0].TestCaseInfo.GscInfo, should.BeNil)
	})

	ftt.Run("Populates GSC devboard info with empty GSC devboard info", t, func(t *ftt.Test) {
		// Create a test result proto with a test run and a test case.
		testResult := &artifact.TestResult{
			TestRuns: []*artifact.TestRun{
				{
					TestCaseInfo: &artifact.TestCaseInfo{
						TestCaseResult: &api.TestCaseResult{
							TestCaseId: &api.TestCase_Id{
								Value: testName,
							},
						},
					},
				},
			},
		}

		// Create a GSC devboard info response proto an empty GSC devboard info for the test case.
		emptyGSCInfoAny, _ := anypb.New(&artifact.GscInfo{})
		gscInfoResp := &api.GetGscInfoResponse{
			GscInfos: map[string]*anypb.Any{
				testName: emptyGSCInfoAny,
			},
		}

		// Call populateGSCInfo.
		populateGSCInfo(testResult, gscInfoResp)

		// Verify that the GSC devboard info is not populated.
		assert.Loosely(t, testResult.TestRuns[0].TestCaseInfo.GscInfo, should.BeNil)
	})

	ftt.Run("Skip if the response is nil", t, func(t *ftt.Test) {
		// Create a test result proto with a test run and a test case.
		testResult := &artifact.TestResult{
			TestRuns: []*artifact.TestRun{
				{
					TestCaseInfo: &artifact.TestCaseInfo{
						TestCaseResult: &api.TestCaseResult{
							TestCaseId: &api.TestCase_Id{
								Value: testName,
							},
						},
					},
				},
			},
		}

		// Call populateGSCInfo.
		populateGSCInfo(testResult, nil)

		// Verify that the GSC devboard info is not populated.
		assert.Loosely(t, testResult.TestRuns[0].TestCaseInfo.GscInfo, should.BeNil)
	})

	ftt.Run("Handles invalid GSC devboard info", t, func(t *ftt.Test) {
		// Create a test result proto with a test run and a test case.
		testResult := &artifact.TestResult{
			TestRuns: []*artifact.TestRun{
				{
					TestCaseInfo: &artifact.TestCaseInfo{
						TestCaseResult: &api.TestCaseResult{
							TestCaseId: &api.TestCase_Id{
								Value: testName,
							},
						},
					},
				},
			},
		}

		// Create a GSC devboard info response proto with invalid GSC devboard info for the test case.
		invalidGSCInfoResp := &api.GetGscInfoResponse{
			GscInfos: map[string]*anypb.Any{
				testName: {
					TypeUrl: "type.googleapis.com/go.chromium.org/chromiumos/config/go/test/artifact.GscInfo",
					Value:   []byte(`invalid json`),
				},
			},
		}

		// Call populateGSCInfo.
		populateGSCInfo(testResult, invalidGSCInfoResp)

		// Verify that the GSC devboard info is not populated.
		assert.Loosely(t, testResult.TestRuns[0].TestCaseInfo.GscInfo, should.BeNil)
	})
}

func TestPopulateServoInfo(t *testing.T) {
	t.Parallel()

	servoInfo := &artifact.BuildMetadata_ServoInfo{
		ServodVersion: "v1.0.2345-4b2de21e 2024-08-17 00:30:57",
		ServoType:     "servo_v4_with_c2d2_and_ccd_ti50",
		ServoVersions: "c2d2_v2.4.73-d771c18ba9,0.24.40/ti50_common_prepvt-15086.B:v0.0.355-15c69d7f,fizz-labstation-release/R115-15474.55.0,servo_v4_v2.4.58-c37246f9c",
	}
	servoInfoAny, _ := anypb.New(servoInfo)

	ftt.Run("Populates servo info", t, func(t *ftt.Test) {
		// Create a build metadata proto.
		buildMetadata := &artifact.BuildMetadata{}

		// Create a servo info response proto.
		servoInfoResp := &api.GetServoInfoResponse{
			ServoInfo: servoInfoAny,
		}

		// Call populateServoInfo.
		populateServoInfo(buildMetadata, servoInfoResp)

		// Verify that the servo info is populated correctly.
		wantServoInfo := &artifact.BuildMetadata_ServoInfo{
			ServodVersion: "v1.0.2345-4b2de21e 2024-08-17 00:30:57",
			ServoType:     "servo_v4_with_c2d2_and_ccd_ti50",
			ServoVersions: "c2d2_v2.4.73-d771c18ba9,0.24.40/ti50_common_prepvt-15086.B:v0.0.355-15c69d7f,fizz-labstation-release/R115-15474.55.0,servo_v4_v2.4.58-c37246f9c",
		}
		assert.That(t, buildMetadata.ServoInfo, should.Match(wantServoInfo))
	})

	ftt.Run("Skip if the servo info has empty values", t, func(t *ftt.Test) {
		// Create a build metadata proto.
		buildMetadata := &artifact.BuildMetadata{}

		// Create a servo info response proto with empty values.
		servoInfoResp := &api.GetServoInfoResponse{}

		// Call populateServoInfo.
		populateServoInfo(buildMetadata, servoInfoResp)

		// Verify no value is populated.
		assert.Loosely(t, buildMetadata.GfxInfo, should.BeNil)
	})

	ftt.Run("Skip if the response is nil", t, func(t *ftt.Test) {
		// Create a build metadata proto.
		buildMetadata := &artifact.BuildMetadata{}

		// Call populateServoInfo.
		populateServoInfo(buildMetadata, nil)

		// Verify no value is populated.
		assert.Loosely(t, buildMetadata.GfxInfo, should.BeNil)
	})
}

func TestNewRdbPublishService(t *testing.T) {
	t.Parallel()

	testResult := &artifact.TestResult{
		TestInvocation: &artifact.TestInvocation{
			PrimaryExecutionInfo: &artifact.ExecutionInfo{
				BuildInfo: &artifact.BuildInfo{
					BuildMetadata: &artifact.BuildMetadata{},
				},
			},
		},
		TestRuns: []*artifact.TestRun{
			{
				TestCaseInfo: &artifact.TestCaseInfo{
					TestCaseResult: &api.TestCaseResult{
						TestCaseId: &api.TestCase_Id{
							Value: "tast.gscdevboard.GSCSysinfo",
						},
					},
				},
			},
		},
	}

	// In this unit test, why the sizeCaches of the wantService and the goService are different?
	ftt.Run("Valid request", t, func(t *ftt.Test) {
		req := &api.PublishRequest{
			Is_3DRun: true,
			ArtifactDirPath: &_go.StoragePath{
				HostType: _go.StoragePath_LOCAL,
				Path:     "test/results/dir",
			},
		}
		testResult.TestInvocation.Is_3DRun = req.GetIs_3DRun()
		testResult.TestRuns[0].EqcInfo = &artifact.EqcInfo{
			EqcHash: "9073744604696850342",
		}
		eqcInfoMap := map[string]string{
			"eqcCategoryExpression": "WifiBtChipset_assert.Thatc_Kernel_Intel",
			"eqcDimensions":         "{\"dlm:soc\":\"Cometlake-U\",\"image:_kernel_version\":\"5.15\",\"wireless_field\":\"INTEL_HRP2_AX201\"}",
			"eqcHash":               "9073744604696850342",
			"eqcName":               "Cometlake-U__INTEL_HRP2_AX201__5.15",
			"eqcTests":              "[\"tast.gscdevboard.GSCSysinfo\"]",
		}
		metadata := &metadata.PublishRdbMetadata{
			CurrentInvocationId: "inv_id",
			TestResult:          testResult,
			TesthausUrl:         "https://tests.chromeos.goog/",
			Sources: &metadata.PublishRdbMetadata_Sources{
				GsPath: "gs://bucket/dir/metadata/sources.jsonpb",
			},
			BaseVariant: map[string]string{},
			PublishKeys: []*api.PublishKey{
				{
					Subject:   EQCSubjectKey,
					KeyValues: eqcInfoMap,
				},
			},
		}
		req.Metadata, _ = anypb.New(metadata)
		m, _ := UnpackMetadata(req)
		wantService := &RdbPublishService{
			CurrentInvocationId:       m.GetCurrentInvocationId(),
			TestResultProto:           m.GetTestResult(),
			TesthausURL:               m.GetTesthausUrl(),
			Sources:                   m.GetSources(),
			BaseVariant:               m.GetBaseVariant(),
			PostProcessResponses:      m.GetPostProcessResponses(),
			FirmwareProvisionResponse: m.GetFirmwareProvisionResponse(),
			Is3DRun:                   req.GetIs_3DRun(),
		}

		gotService, err := NewRdbPublishService(req)

		assert.Loosely(t, err, should.BeNil)
		assert.That(t, gotService, should.Match(wantService))
	})

	ftt.Run("Valid request with retry count", t, func(t *ftt.Test) {
		req := &api.PublishRequest{
			RetryCount: 2,
			ArtifactDirPath: &_go.StoragePath{
				HostType: _go.StoragePath_LOCAL,
				Path:     "test/results/dir",
			},
		}
		metadata := &metadata.PublishRdbMetadata{
			CurrentInvocationId: "inv_id",
			TestResult:          testResult,
		}
		req.Metadata, _ = anypb.New(metadata)

		service, err := NewRdbPublishService(req)

		assert.Loosely(t, err, should.BeNil)
		assert.That(t, service.RetryCount, should.Equal(2))
	})

	ftt.Run("Invalid metadata", t, func(t *ftt.Test) {
		req := &api.PublishRequest{}
		req.Metadata, _ = anypb.New(&artifact.TestResult{}) // Incorrect type

		service, err := NewRdbPublishService(req)

		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, service, should.BeNil)
	})

	ftt.Run("Missing invocation id", t, func(t *ftt.Test) {
		req := &api.PublishRequest{}
		metadata := &metadata.PublishRdbMetadata{
			TestResult: testResult,
		}
		req.Metadata, _ = anypb.New(metadata)

		service, err := NewRdbPublishService(req)

		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, service, should.BeNil)
	})

	ftt.Run("Missing test result", t, func(t *ftt.Test) {
		req := &api.PublishRequest{}
		metadata := &metadata.PublishRdbMetadata{
			CurrentInvocationId: "inv_id",
		}
		req.Metadata, _ = anypb.New(metadata)
		service, err := NewRdbPublishService(req)

		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, service, should.BeNil)
	})
}

func TestIngestFirmwareProvisionResponse(t *testing.T) {
	t.Parallel()

	ftt.Run("Ingest firmware provision response", t, func(t *ftt.Test) {
		buildMetadata := &artifact.BuildMetadata{}

		// Create a firmware provision response proto.
		fwProvisionResponse := &api.FirmwareProvisionResponse{
			ApRoVersion: "Google_Voema.13672.224.0",
			ApRwVersion: "Google_Voema.13672.224.1",
			EcRoVersion: "vilboz_v2.0.5705-a8a7681f94",
			EcRwVersion: "vilboz_v2.0.5705-a8a7681f95",
		}

		// Call ingestFirmwareProvisionResponse.
		ingestFirmwareProvisionResponse(buildMetadata, fwProvisionResponse)

		// Verify that the firmware info is populated correctly.
		wantFirmware := &artifact.BuildMetadata_Firmware{
			ApRoVersion: "Google_Voema.13672.224.0",
			ApRwVersion: "Google_Voema.13672.224.1",
			EcRoVersion: "vilboz_v2.0.5705-a8a7681f94",
			EcRwVersion: "vilboz_v2.0.5705-a8a7681f95",
		}
		assert.That(t, buildMetadata.Firmware, should.Match(wantFirmware))
	})

	ftt.Run("Ingest firmware provision response with empty values", t, func(t *ftt.Test) {
		buildMetadata := &artifact.BuildMetadata{}

		// Create a firmware provision response proto with empty values.
		fwProvisionResponse := &api.FirmwareProvisionResponse{}

		// Call ingestFirmwareProvisionResponse.
		ingestFirmwareProvisionResponse(buildMetadata, fwProvisionResponse)

		// Verify that the firmware info is populated correctly.
		assert.That(t, buildMetadata.Firmware, should.Match(&artifact.BuildMetadata_Firmware{}))
	})

	ftt.Run("Skip if the response is nil", t, func(t *ftt.Test) {
		buildMetadata := &artifact.BuildMetadata{}

		// Call ingestFirmwareProvisionResponse.
		ingestFirmwareProvisionResponse(buildMetadata, nil)

		// Verify no value is populated.
		assert.Loosely(t, buildMetadata.Firmware, should.BeNil)
	})
}
