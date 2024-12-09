// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang/protobuf/ptypes/duration"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	configpb "go.chromium.org/chromiumos/config/go"
	apipb "go.chromium.org/chromiumos/config/go/test/api"
	artifactpb "go.chromium.org/chromiumos/config/go/test/artifact"
	labpb "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/resultdb/pbutil"
	pb "go.chromium.org/luci/resultdb/proto/v1"
	sinkpb "go.chromium.org/luci/resultdb/sink/proto/v1"
)

const (
	// Test result JSON file with a subset of common information.
	simpleTestResultFile = "test_data/cros_test_result/simple_test_result.json"

	// Test result JSON file with full information.
	fullTestResultFile = "test_data/cros_test_result/full_test_result.json"

	// Test result JSON file with multi-DUT testing information.
	multiDUTTestResultFile = "test_data/cros_test_result/multi_dut_result.json"

	// Test result JSON file with skipped test results.
	skippedTestResultFile = "test_data/cros_test_result/skipped_test_result.json"

	// Test result JSON file with passed (with warning) test results.
	warnTestResultFile = "test_data/cros_test_result/warn_test_result.json"

	// Test result JSON file with failing test results.
	failedTestResultFile = "test_data/cros_test_result/failed_test_result.json"

	// Test result JSON file with missing test id.
	missingTestIdFile = "test_data/cros_test_result/missing_test_id.json"

	// Test result JSON file with warning test results and a long reason.
	warnTestResultWithLongReasonFile = "test_data/cros_test_result/warn_test_result_with_long_reason.json"

	// Test result JSON file with test result directory path.
	testResultDirPathFile = "test_data/cros_test_result/simple_test_result_with_result_dir.json"

	// The testhaus base url flag.
	testhausBaseURL = "https://tests.chromeos.goog/p/chromeos/logs/unified/build-12345"
)

func TestCrosTestResultConversions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	testResultsJSON := ReadJSONFileToString(simpleTestResultFile)

	testResult := &artifactpb.TestResult{
		Version: 1,
		TestInvocation: &artifactpb.TestInvocation{
			DutTopology: &labpb.DutTopology{
				Id: &labpb.DutTopology_Id{
					Value: "chromeos15-row4-rack5-host1",
				},
			},
			PrimaryExecutionInfo: &artifactpb.ExecutionInfo{
				BuildInfo: &artifactpb.BuildInfo{
					Name:            "hatch-cq/R106-15048.0.0",
					Milestone:       106,
					ChromeOsVersion: "15048.0.0",
					Source:          "hatch-cq",
					Board:           "hatch",
				},
				DutInfo: &artifactpb.DutInfo{
					Dut: &labpb.Dut{
						Id: &labpb.Dut_Id{
							Value: "chromeos15-row4-rack5-host1",
						},
						DutType: &labpb.Dut_Chromeos{
							Chromeos: &labpb.Dut_ChromeOS{
								Name: "chromeos15-row4-rack5-host1",
								DutModel: &labpb.DutModel{
									ModelName: "nipperkin",
								},
								Phase: labpb.Phase_DVT_2,
							},
						},
					},
				},
			},
		},
		TestRuns: []*artifactpb.TestRun{
			{
				TestCaseInfo: &artifactpb.TestCaseInfo{
					TestCaseMetadata: &apipb.TestCaseMetadata{
						TestCase: &apipb.TestCase{
							Id: &apipb.TestCase_Id{
								Value: "rlz_CheckPing",
							},
							Name: "rlz_CheckPing",
						},
					},
					TestCaseResult: &apipb.TestCaseResult{
						TestCaseId: &apipb.TestCase_Id{
							Value: "rlz_CheckPing",
						},
						Verdict:   &apipb.TestCaseResult_Pass_{},
						StartTime: timestamppb.New(parseTime("2022-09-07T18:53:33.983328614Z")),
						Duration:  &duration.Duration{Seconds: 60},
						TestHarness: &apipb.TestHarness{
							TestHarnessType: &apipb.TestHarness_Tradefed_{
								Tradefed: &apipb.TestHarness_Tradefed{},
							},
						},
					},
				},
				LogsInfo: []*configpb.StoragePath{
					{
						HostType: configpb.StoragePath_GS,
						Path:     "gs://chromeos-test-logs/test-runner/prod/2022-09-07/98098abe-da4f-4bfa-bef5-9cbc4936da03",
					},
				},
			},
			{
				TestCaseInfo: &artifactpb.TestCaseInfo{
					TestCaseMetadata: &apipb.TestCaseMetadata{
						TestCase: &apipb.TestCase{
							Id: &apipb.TestCase_Id{
								Value: "power_Resume",
							},
							Name: "power_Resume",
						},
					},
					TestCaseResult: &apipb.TestCaseResult{
						TestCaseId: &apipb.TestCase_Id{
							Value: "power_Resume",
						},
						Verdict:   &apipb.TestCaseResult_Fail_{},
						Reason:    "Test failed",
						StartTime: timestamppb.New(parseTime("2022-09-07T18:53:34.983328614Z")),
						Duration:  &duration.Duration{Seconds: 120, Nanos: 100000000},
					},
				},
				LogsInfo: []*configpb.StoragePath{
					{
						HostType: configpb.StoragePath_GS,
						Path:     "gs://chromeos-test-logs/test-runner/prod/2022-09-07/98098abe-da4f-4bfa-bef5-9cbc4936da04",
					},
				},
			},
		},
	}

	ftt.Run(`From JSON works`, t, func(t *ftt.Test) {
		results := &CrosTestResult{}
		err := results.ConvertFromJSON(strings.NewReader(testResultsJSON))
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, results.TestResult, should.Match(testResult))
	})

	ftt.Run(`ToProtos works`, t, func(t *ftt.Test) {
		t.Run("Basic", func(t *ftt.Test) {
			results := &CrosTestResult{}
			err := results.ConvertFromJSON(strings.NewReader(testResultsJSON))
			assert.Loosely(t, err, should.BeNil)
			testResults, err := results.ToProtos(ctx)
			assert.Loosely(t, err, should.BeNil)

			expected := []*sinkpb.TestResult{
				{
					TestId:    "rlz_CheckPing",
					Expected:  true,
					Status:    pb.TestStatus_PASS,
					StartTime: timestamppb.New(parseTime("2022-09-07T18:53:33.983328614Z")),
					Duration:  &duration.Duration{Seconds: 60},
					Tags: SortTags([]*pb.StringPair{
						pbutil.StringPair("account_id", "1"),
						pbutil.StringPair("board", "hatch"),
						pbutil.StringPair("build", "R106-15048.0.0"),
						pbutil.StringPair("builder_name", "hatch-cq"),
						pbutil.StringPair("cbx", "false"),
						pbutil.StringPair("hostname", "chromeos15-row4-rack5-host1"),
						pbutil.StringPair("image", "hatch-cq/R106-15048.0.0"),
						pbutil.StringPair("is_cft_run", "false"),
						pbutil.StringPair("is_trv2_run", "false"),
						pbutil.StringPair("is_3d_run", "false"),
						pbutil.StringPair("logs_url", "gs://chromeos-test-logs/test-runner/prod/2022-09-07/98098abe-da4f-4bfa-bef5-9cbc4936da03"),
						pbutil.StringPair("model", "nipperkin"),
						pbutil.StringPair("multiduts", "False"),
						pbutil.StringPair("phase", "DVT_2"),
					}),
					TestMetadata: &pb.TestMetadata{Name: "rlz_CheckPing"},
				},
				{
					TestId:   "power_Resume",
					Expected: false,
					Status:   pb.TestStatus_FAIL,
					FailureReason: &pb.FailureReason{
						PrimaryErrorMessage: "Test failed",
						Errors: []*pb.FailureReason_Error{
							{Message: "Test failed"},
						},
					},
					StartTime: timestamppb.New(parseTime("2022-09-07T18:53:34.983328614Z")),
					Duration:  &duration.Duration{Seconds: 120, Nanos: 100000000},
					Tags: SortTags([]*pb.StringPair{
						pbutil.StringPair("account_id", "1"),
						pbutil.StringPair("board", "hatch"),
						pbutil.StringPair("build", "R106-15048.0.0"),
						pbutil.StringPair("builder_name", "hatch-cq"),
						pbutil.StringPair("cbx", "false"),
						pbutil.StringPair("hostname", "chromeos15-row4-rack5-host1"),
						pbutil.StringPair("image", "hatch-cq/R106-15048.0.0"),
						pbutil.StringPair("is_cft_run", "false"),
						pbutil.StringPair("is_trv2_run", "false"),
						pbutil.StringPair("is_3d_run", "false"),
						pbutil.StringPair("logs_url", "gs://chromeos-test-logs/test-runner/prod/2022-09-07/98098abe-da4f-4bfa-bef5-9cbc4936da04"),
						pbutil.StringPair("model", "nipperkin"),
						pbutil.StringPair("multiduts", "False"),
						pbutil.StringPair("phase", "DVT_2"),
					}),
					TestMetadata: &pb.TestMetadata{Name: "power_Resume"},
				},
			}

			for i, tr := range expected {
				err := PopulateProperties(tr, results.TestResult.TestRuns[i])
				assert.Loosely(t, err, should.BeNil)
			}

			assert.Loosely(t, testResults, should.HaveLength(2))
			assert.Loosely(t, testResults, should.Match(expected))
			for _, tr := range testResults {
				assert.Loosely(t, tr.GetProperties().GetFields(), should.NotBeEmpty)
			}
		})

		t.Run("Uploads test artifacts when result dir is provided", func(t *ftt.Test) {
			artBaseDir := filepath.Join("test_data", "cros_test_result", "artifacts")
			artName1 := "test_artifact_1.txt"
			artName2 := "test_artifact_2.txt"
			testCount := 2
			wantArtifacts := make([]map[string]*sinkpb.Artifact, 0, testCount)
			for _, testID := range []string{"rlz_CheckPing", "power_Resume"} {
				wantArtifacts = append(wantArtifacts, map[string]*sinkpb.Artifact{
					artName1: {
						Body: &sinkpb.Artifact_FilePath{FilePath: filepath.Join(artBaseDir, artName1)},
					},
					artName2: {
						Body: &sinkpb.Artifact_FilePath{FilePath: filepath.Join(artBaseDir, artName2)},
					},
					"testhaus_logs": {
						Body:        &sinkpb.Artifact_Contents{Contents: []byte(fmt.Sprintf("%s?treeQuery=%s&test=%s", testhausBaseURL, testID, testID))},
						ContentType: "text/x-uri",
					},
				})
			}
			testResultsJSON := ReadJSONFileToString(testResultDirPathFile)
			results := &CrosTestResult{testhausBaseURL: testhausBaseURL}
			err := results.ConvertFromJSON(strings.NewReader(testResultsJSON))
			assert.Loosely(t, err, should.BeNil)

			gotTestResults, err := results.ToProtos(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, gotTestResults, should.HaveLength(testCount))

			gotArtifacts := make([]map[string]*sinkpb.Artifact, 0, testCount)
			for _, tr := range gotTestResults {
				gotArtifacts = append(gotArtifacts, tr.GetArtifacts())
			}
			assert.Loosely(t, gotArtifacts, should.Match(wantArtifacts))
		})

		t.Run("Skips test artifacts upload when result dir is invalid", func(t *ftt.Test) {
			testResultsJSON := ReadJSONFileToString(testResultDirPathFile)
			results := &CrosTestResult{testhausBaseURL: testhausBaseURL}
			err := results.ConvertFromJSON(strings.NewReader(testResultsJSON))
			assert.Loosely(t, err, should.BeNil)

			// Update the test case result dir to an invalid path
			for _, tr := range results.TestResult.TestRuns {
				tr.TestCaseInfo.TestCaseResult.ResultDirPath.Path = "invalid_dir"
			}

			gotTestResults, err := results.ToProtos(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, gotTestResults, should.HaveLength(2))
			for _, tr := range gotTestResults {
				assert.Loosely(t, tr.GetArtifacts(), should.BeEmpty)
			}
		})

		t.Run(`Check expected skip and unexpected skip tests`, func(t *ftt.Test) {
			testResultsJSON := ReadJSONFileToString(skippedTestResultFile)
			results := &CrosTestResult{}
			err := results.ConvertFromJSON(strings.NewReader(testResultsJSON))
			assert.Loosely(t, err, should.BeNil)
			testResults, err := results.ToProtos(ctx)
			assert.Loosely(t, err, should.BeNil)

			expected := []*sinkpb.TestResult{
				{
					TestId:   "rlz_CheckPing",
					Expected: true,
					Status:   pb.TestStatus_SKIP,
					FailureReason: &pb.FailureReason{
						PrimaryErrorMessage: "Test was skipped expectedly",
						Errors: []*pb.FailureReason_Error{
							{Message: "Test was skipped expectedly"},
						},
					},
					StartTime:    timestamppb.New(parseTime("2022-09-07T18:53:33.983328614Z")),
					Duration:     &duration.Duration{Seconds: 60},
					TestMetadata: &pb.TestMetadata{Name: "rlz_CheckPing"},
				},
				{
					TestId:   "power_Resume",
					Expected: false,
					Status:   pb.TestStatus_SKIP,
					FailureReason: &pb.FailureReason{
						PrimaryErrorMessage: "Test has not run yet",
						Errors: []*pb.FailureReason_Error{
							{Message: "Test has not run yet"},
						},
					},
					StartTime:    timestamppb.New(parseTime("2022-09-07T18:53:34.983328614Z")),
					Duration:     &duration.Duration{Seconds: 120, Nanos: 100000000},
					TestMetadata: &pb.TestMetadata{Name: "power_Resume"},
				},
			}

			for i, tr := range expected {
				err := PopulateProperties(tr, results.TestResult.TestRuns[i])
				assert.Loosely(t, err, should.BeNil)
			}

			assert.Loosely(t, testResults, should.HaveLength(2))
			assert.Loosely(t, testResults, should.Match(expected))
			for _, tr := range testResults {
				assert.Loosely(t, tr.GetProperties().GetFields(), should.NotBeEmpty)
			}
		})
		t.Run(`Warning results`, func(t *ftt.Test) {
			testResultsJSON := ReadJSONFileToString(warnTestResultFile)
			results := &CrosTestResult{}
			err := results.ConvertFromJSON(strings.NewReader(testResultsJSON))
			assert.Loosely(t, err, should.BeNil)
			testResults, err := results.ToProtos(ctx)
			assert.Loosely(t, err, should.BeNil)

			expected := []*sinkpb.TestResult{
				{
					TestId:   "rlz_CheckPing",
					Expected: true,
					// Warning results are reported as pass, and without
					// the failure reason set. Warning messages are included
					// in the test result properties.
					Status:       pb.TestStatus_PASS,
					StartTime:    timestamppb.New(parseTime("2022-09-07T18:53:33.983328614Z")),
					Duration:     &duration.Duration{Seconds: 60},
					TestMetadata: &pb.TestMetadata{Name: "rlz_CheckPing"},
				},
			}

			for i, tr := range expected {
				err := PopulateProperties(tr, results.TestResult.TestRuns[i])
				assert.Loosely(t, err, should.BeNil)
			}

			assert.Loosely(t, testResults, should.Match(expected))
			for _, tr := range testResults {
				assert.Loosely(t, tr.GetProperties().GetFields(), should.NotBeEmpty)
			}
		})
		t.Run(`Failed results`, func(t *ftt.Test) {
			testResultsJSON := ReadJSONFileToString(failedTestResultFile)
			results := &CrosTestResult{}
			err := results.ConvertFromJSON(strings.NewReader(testResultsJSON))
			assert.Loosely(t, err, should.BeNil)
			testResults, err := results.ToProtos(ctx)
			assert.Loosely(t, err, should.BeNil)

			expected := []*sinkpb.TestResult{
				{
					TestId:   "rlz_CheckPing",
					Expected: false,
					Status:   pb.TestStatus_ABORT,
					FailureReason: &pb.FailureReason{
						PrimaryErrorMessage: "Failed to start Chrome: login failed: context timeout",
						Errors: []*pb.FailureReason_Error{
							{Message: "Failed to start Chrome: login failed: context timeout"},
						},
					},
					StartTime:    timestamppb.New(parseTime("2022-09-07T18:53:33.983328614Z")),
					Duration:     &duration.Duration{Seconds: 60},
					TestMetadata: &pb.TestMetadata{Name: "rlz_CheckPing"},
				},
				{
					TestId:   "power_Resume",
					Expected: false,
					Status:   pb.TestStatus_FAIL,
					FailureReason: &pb.FailureReason{
						PrimaryErrorMessage: "Failed to start Chrome: login failed: OOBE not dismissed, it is on screen \"signin-fatal-error\"",
						Errors: []*pb.FailureReason_Error{
							{Message: "Failed to start Chrome: login failed: OOBE not dismissed, it is on screen \"signin-fatal-error\""},
							{Message: "Failed to clean-up Chrome: some error"},
							{Message: "Error three"},
						},
					},
					StartTime:    timestamppb.New(parseTime("2022-09-07T18:53:34.983328614Z")),
					Duration:     &duration.Duration{Seconds: 120, Nanos: 100000000},
					TestMetadata: &pb.TestMetadata{Name: "power_Resume"},
				},
			}

			for i, tr := range expected {
				err := PopulateProperties(tr, results.TestResult.TestRuns[i])
				assert.Loosely(t, err, should.BeNil)
			}

			assert.Loosely(t, testResults, should.Match(expected))
			for _, tr := range testResults {
				assert.Loosely(t, tr.GetProperties().GetFields(), should.NotBeEmpty)
			}
		})

		t.Run(`Check the full list of tags`, func(t *ftt.Test) {
			testResultsJSON := ReadJSONFileToString(fullTestResultFile)
			results := &CrosTestResult{}
			err := results.ConvertFromJSON(strings.NewReader(testResultsJSON))
			assert.Loosely(t, err, should.BeNil)
			testResults, err := results.ToProtos(ctx)
			assert.Loosely(t, err, should.BeNil)

			expected := []*sinkpb.TestResult{
				{
					TestId:    "rlz_CheckPing",
					Expected:  true,
					Status:    pb.TestStatus_PASS,
					StartTime: timestamppb.New(parseTime("2022-09-07T18:53:33.983328614Z")),
					Duration:  &duration.Duration{Seconds: 60},
					Tags: SortTags([]*pb.StringPair{
						pbutil.StringPair("account_id", "4"),
						pbutil.StringPair("analytics_name", "Bluetooth_Sa_Perbuild"),
						pbutil.StringPair("ancestor_buildbucket_ids", "8814950840874708945,8814951792758733697"),
						pbutil.StringPair("ap_ro_fwid", "Google_Voema.13672.224.0"),
						pbutil.StringPair("ap_rw_fwid", "Google_Voema.13672.224.0"),
						pbutil.StringPair("ash_version", "109.0.5391.0"),
						pbutil.StringPair("avl_part_model", "0x0000f5 MMC32G"),
						pbutil.StringPair("avl_part_firmware", "0xa200000000000000"),
						pbutil.StringPair("avl_component_type", "storage"),
						pbutil.StringPair("board", "hatch"),
						pbutil.StringPair("board_type", "HW"),
						pbutil.StringPair("bot_config", "cloudbots_config.py"),
						pbutil.StringPair("bot_id", "cloudbots-prod-1715342009263-7kz6"),
						pbutil.StringPair("branch", "main"),
						pbutil.StringPair("bug_component", "b:1234"),
						pbutil.StringPair("build", "R106-15048.0.0"),
						pbutil.StringPair("buildbucket_builder", "test_runner-dev"),
						pbutil.StringPair("builder_name", "hatch-cq"),
						pbutil.StringPair("carrier", "CARRIER_ESIM"),
						pbutil.StringPair("cbx", "true"),
						pbutil.StringPair("chameleon_type", "CHAMELEON_TYPE_V2,CHAMELEON_TYPE_V3"),
						pbutil.StringPair("chameleon_connection_types", "CHAMELEON_CONNECTION_TYPE_HDMI,CHAMELEON_CONNECTION_TYPE_USB"),
						pbutil.StringPair("channel", "DEV"),
						pbutil.StringPair("contacts", "user@google.com"),
						pbutil.StringPair("criteria", "Tauto wrapper for specified tast tests"),
						pbutil.StringPair("ctp_fwd_task_name", "Bluetooth_Sa_Perbuild"),
						pbutil.StringPair("declared_name", "hatch-cq/R102-14632.0.0-62834-8818718496810023809/wificell-cq/tast.wificell-cq"),
						pbutil.StringPair("display_panel_name", "CMN 4679"),
						pbutil.StringPair("display_present_hdr", "hdr unsupported"),
						pbutil.StringPair("display_present_psr", "psr unsupported"),
						pbutil.StringPair("display_present_vrr", "vrr unsupported"),
						pbutil.StringPair("display_refresh_rate", "60.00"),
						pbutil.StringPair("display_resolution", "1366x912"),
						pbutil.StringPair("dlm_sku_id", "16968"),
						pbutil.StringPair("suite", "arc-cts-vm"),
						pbutil.StringPair("drone", "skylab-drone-deployment-prod-6dc79d4f9-czjlj"),
						pbutil.StringPair("drone_server", "chromeos4-row4-rack1-drone8"),
						pbutil.StringPair("gpu_family", "mali-g57"),
						pbutil.StringPair("gpu_id", "mediatek:mali-g57"),
						pbutil.StringPair("gpu_open_gles_version", "3.2"),
						pbutil.StringPair("gpu_vendor", "mediatek"),
						pbutil.StringPair("gpu_vulkan_version", "1.3.283"),
						pbutil.StringPair("gsc_buildurl", "latest-tot"),
						pbutil.StringPair("gsc_ccd_serial", "1482903f-4c2ac261"),
						pbutil.StringPair("gsc_devboardservice_version", "1.30"),
						pbutil.StringPair("gsc_hyperdebug_serial", "206636793452"),
						pbutil.StringPair("gsc_hyperdebug_version", "hyperdebug_20230517_01"),
						pbutil.StringPair("gsc_opentitantool_version", "123"),
						pbutil.StringPair("gsc_ro_version", "0.0.59"),
						pbutil.StringPair("gsc_rw_branch", "tot:v0.0"),
						pbutil.StringPair("gsc_rw_rev", "1495"),
						pbutil.StringPair("gsc_rw_sha", "-2fdf6034"),
						pbutil.StringPair("gsc_rw_version", "0.26.112"),
						pbutil.StringPair("gsc_tast_version", "R125-15837.0.0"),
						pbutil.StringPair("gsc_testbed_serial", "123882-0008"),
						pbutil.StringPair("gsc_testbed_type", "gsc_dt_shield"),
						pbutil.StringPair("hostname", "chromeos15-row4-rack5-host1"),
						pbutil.StringPair("hwid_sku", "katsu_MT8183_0B"),
						pbutil.StringPair("image", "hatch-cq/R106-15048.0.0"),
						pbutil.StringPair("is_cft_run", "true"),
						pbutil.StringPair("is_trv2_run", "true"),
						pbutil.StringPair("is_3d_run", "true"),
						pbutil.StringPair("job_name", "bb-8818737803155059937-chromeos/general/Full"),
						pbutil.StringPair("kernel_version", "5.4.151-16902-g93699f4e73de"),
						pbutil.StringPair("label_pool", "DUT_POOL_QUOTA"),
						pbutil.StringPair("lacros_version", "109.0.5391.0"),
						pbutil.StringPair("logs_url", "gs://chromeos-test-logs/test-runner/prod/2022-09-07/98098abe-da4f-4bfa-bef5-9cbc4936da03"),
						pbutil.StringPair("main_builder_name", "main-release"),
						pbutil.StringPair("model", "nipperkin"),
						pbutil.StringPair("modem_type", "MODEM_TYPE_FIBOCOMM_L850GL"),
						pbutil.StringPair("multiduts", "False"),
						pbutil.StringPair("owners", "owner1@test.com,owner2@test.com"),
						pbutil.StringPair("phase", "DVT_2_MPS_LTE"),
						pbutil.StringPair("platform_cpu_vendor", "mediatek"),
						pbutil.StringPair("platform_disk_size", "62"),
						pbutil.StringPair("platform_memory_size", "4"),
						pbutil.StringPair("pool", "ChromeOSSkylab"),
						pbutil.StringPair("qs_account", "unmanaged_p2"),
						pbutil.StringPair("qual_bug_id", "1234"),
						pbutil.StringPair("queued_time", "2022-06-03 18:53:33.983328614 +0000 UTC"),
						pbutil.StringPair("requester", "ldap@google.com"),
						pbutil.StringPair("requirements", "requirement 1,requirement 2"),
						pbutil.StringPair("ro_fwid", "Google_Voema.13672.224.0"),
						pbutil.StringPair("rw_fwid", "Google_Voema.13672.224.0"),
						pbutil.StringPair("suite_task_id", "59ef5e9532bbd611"),
						pbutil.StringPair("task_id", "59f0e13fe7af0710"),
						pbutil.StringPair("test_args", "bug_id=12345 qual_run_id=1712172839652"),
						pbutil.StringPair("test_harness", "Tast"),
						pbutil.StringPair("ufs_zone", "ZONE_SFO36_OS"),
						pbutil.StringPair("wifi_chip", "marvell"),
						pbutil.StringPair("wifi_router_models", "gale"),
					}),
					TestMetadata: &pb.TestMetadata{Name: "rlz_CheckPing"},
				},
			}
			err = PopulateProperties(expected[0], results.TestResult.TestRuns[0])
			assert.Loosely(t, err, should.BeNil)

			assert.Loosely(t, testResults, should.HaveLength(1))
			assert.Loosely(t, testResults, should.Match(expected))
			assert.Loosely(t, testResults[0].GetProperties().GetFields(), should.NotBeEmpty)
		})

		t.Run(`Check multi DUT testing`, func(t *ftt.Test) {
			testResultsJSON := ReadJSONFileToString(multiDUTTestResultFile)
			results := &CrosTestResult{}
			err := results.ConvertFromJSON(strings.NewReader(testResultsJSON))
			assert.Loosely(t, err, should.BeNil)
			testResults, err := results.ToProtos(ctx)
			assert.Loosely(t, err, should.BeNil)

			expected := []*sinkpb.TestResult{
				{
					TestId:    "rlz_CheckPing",
					Expected:  true,
					Status:    pb.TestStatus_PASS,
					StartTime: timestamppb.New(parseTime("2022-09-07T18:53:33.983328614Z")),
					Duration:  &duration.Duration{Seconds: 60},
					Tags: SortTags([]*pb.StringPair{
						pbutil.StringPair("account_id", "1"),
						pbutil.StringPair("board", "hatch"),
						pbutil.StringPair("build", "R106-15048.0.0"),
						pbutil.StringPair("builder_name", "hatch-cq"),
						pbutil.StringPair("cbx", "false"),
						pbutil.StringPair("hostname", "chromeos1-sinclair-callbox2-unit1"),
						pbutil.StringPair("image", "hatch-cq/R106-15048.0.0"),
						pbutil.StringPair("is_cft_run", "false"),
						pbutil.StringPair("is_trv2_run", "false"),
						pbutil.StringPair("is_3d_run", "false"),
						pbutil.StringPair("logs_url", "gs://chromeos-test-logs/test-runner/prod/2022-09-07/98098abe-da4f-4bfa-bef5-9cbc4936da03"),
						pbutil.StringPair("model", "nipperkin"),
						pbutil.StringPair("multiduts", "True"),
						pbutil.StringPair("phase", "DVT_2"),
						pbutil.StringPair("primary_board", "hatch"),
						pbutil.StringPair("primary_model", "nipperkin"),
						pbutil.StringPair("primary_phase", "DVT_2"),
						pbutil.StringPair("secondary_boards", "brya"),
						pbutil.StringPair("secondary_models", "gimble"),
						pbutil.StringPair("secondary_phases", "DVT_2"),
					}),
					TestMetadata: &pb.TestMetadata{Name: "rlz_CheckPing"},
				},
			}

			err = PopulateProperties(expected[0], results.TestResult.TestRuns[0])
			assert.Loosely(t, err, should.BeNil)

			assert.Loosely(t, testResults, should.HaveLength(1))
			assert.Loosely(t, testResults, should.Match(expected))
			assert.Loosely(t, testResults[0].GetProperties().GetFields(), should.NotBeEmpty)
		})

		t.Run(`Check with missing test id`, func(t *ftt.Test) {
			// There are 3 test cases in the test file. The first test case has
			// both id and name while the second one only has the name. The
			// third one doesn't have id and name, so it would throw an error
			// to surface the problem explicitly and the first two would be
			// skipped.
			testResultsJSON := ReadJSONFileToString(missingTestIdFile)
			results := &CrosTestResult{}
			err := results.ConvertFromJSON(strings.NewReader(testResultsJSON))
			assert.Loosely(t, err, should.BeNil)
			_, err = results.ToProtos(ctx)
			assert.Loosely(t, err, should.ErrLike("testId is unspecified due to the missing id in test case"))
		})

		t.Run(`Truncate reason field when stored in the properties of test result`, func(t *ftt.Test) {
			testResultsJSON := ReadJSONFileToString(warnTestResultWithLongReasonFile)
			results := &CrosTestResult{}
			err := results.ConvertFromJSON(strings.NewReader(testResultsJSON))
			assert.Loosely(t, err, should.BeNil)
			testResults, err := results.ToProtos(ctx)
			assert.Loosely(t, err, should.BeNil)

			expected := []*sinkpb.TestResult{
				{
					TestId:   "rlz_CheckPing",
					Expected: true,
					// Warning results are reported as pass, and without
					// the failure reason set. Warning messages are included
					// in the test result properties.
					Status:       pb.TestStatus_PASS,
					StartTime:    timestamppb.New(parseTime("2022-09-07T18:53:33.983328614Z")),
					Duration:     &duration.Duration{Seconds: 60},
					TestMetadata: &pb.TestMetadata{Name: "rlz_CheckPing"},
				},
			}

			for i, tr := range expected {
				err := PopulateProperties(tr, results.TestResult.TestRuns[i])
				assert.Loosely(t, err, should.BeNil)
			}

			assert.Loosely(t, testResults, should.Match(expected))
			for _, tr := range testResults {
				assert.Loosely(t, tr.GetProperties().
					GetFields()["testCaseInfo"].GetStructValue().
					GetFields()["testCaseResult"].GetStructValue().
					GetFields()["reason"].GetStringValue(),
					should.HaveLength(2048))
			}
		})
	})
}

func TestPopulateProperties(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		input          *sinkpb.TestResult
		reason         string
		errors         []*apipb.TestCaseResult_Error
		expectedReason *structpb.Value
		expectedErrors *structpb.Value
	}{
		{
			name: "Basic",
			input: &sinkpb.TestResult{
				TestId: "rlz_CheckPing",
			},
			reason: "Test failed",
			errors: []*apipb.TestCaseResult_Error{
				{
					Message: strings.Repeat("b", 1024),
				},
			},
			expectedReason: &structpb.Value{
				Kind: &structpb.Value_StringValue{
					StringValue: "Test failed",
				},
			},
			expectedErrors: &structpb.Value{
				Kind: &structpb.Value_ListValue{
					ListValue: &structpb.ListValue{
						Values: []*structpb.Value{
							{
								Kind: &structpb.Value_StructValue{
									StructValue: &structpb.Struct{
										Fields: map[string]*structpb.Value{
											"message": {
												Kind: &structpb.Value_StringValue{StringValue: strings.Repeat("b", 1024)}},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Long reason",
			input: &sinkpb.TestResult{
				TestId: "rlz_CheckPing",
			},
			reason: strings.Repeat("a", 3072), // Exceeds the limit
			errors: []*apipb.TestCaseResult_Error{},
			expectedReason: &structpb.Value{
				Kind: &structpb.Value_StringValue{
					StringValue: strings.Repeat("a", maxPropErrorMessageBytes-3) + "...", // Truncated
				},
			},
			expectedErrors: &structpb.Value{},
		},
		{
			name: "Long errors",
			input: &sinkpb.TestResult{
				TestId: "rlz_CheckPing",
			},
			reason: "",
			errors: []*apipb.TestCaseResult_Error{
				{
					Message: strings.Repeat("b", 3072), // Exceeds the limit
				},
				{
					Message: strings.Repeat("c", 3072), // Exceeds the limit
				},
			},
			expectedReason: &structpb.Value{
				Kind: &structpb.Value_StringValue{},
			},
			expectedErrors: &structpb.Value{
				Kind: &structpb.Value_ListValue{
					ListValue: &structpb.ListValue{
						Values: []*structpb.Value{
							{
								Kind: &structpb.Value_StructValue{
									StructValue: &structpb.Struct{
										Fields: map[string]*structpb.Value{
											"message": {
												Kind: &structpb.Value_StringValue{StringValue: strings.Repeat("b", maxPropErrorMessageBytes-3) + "..."}}, // Truncated
										},
									},
								},
							},
							{
								Kind: &structpb.Value_StructValue{
									StructValue: &structpb.Struct{
										Fields: map[string]*structpb.Value{
											"message": {
												Kind: &structpb.Value_StringValue{StringValue: strings.Repeat("c", maxPropErrorMessageBytes-3) + "..."}}, // Truncated
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

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testRun := &artifactpb.TestRun{
				TestCaseInfo: &artifactpb.TestCaseInfo{
					TestCaseResult: &apipb.TestCaseResult{
						Reason: tc.reason,
						Errors: tc.errors,
					},
				},
			}
			wantTestResult := &sinkpb.TestResult{
				TestId: "rlz_CheckPing",
				Properties: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"testCaseInfo": {
							Kind: &structpb.Value_StructValue{
								StructValue: &structpb.Struct{
									Fields: map[string]*structpb.Value{
										"testCaseResult": {
											Kind: &structpb.Value_StructValue{
												StructValue: &structpb.Struct{
													Fields: map[string]*structpb.Value{},
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
			if tc.reason != "" {
				wantTestResult.Properties.Fields["testCaseInfo"].GetStructValue().Fields["testCaseResult"].GetStructValue().Fields["reason"] = tc.expectedReason
			}
			if len(tc.errors) != 0 {
				wantTestResult.Properties.Fields["testCaseInfo"].GetStructValue().Fields["testCaseResult"].GetStructValue().Fields["errors"] = tc.expectedErrors
			}

			err := PopulateProperties(tc.input, testRun)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, tc.input, should.Match(wantTestResult))
		})
	}
}
