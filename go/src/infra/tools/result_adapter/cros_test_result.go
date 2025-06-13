// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/gogo/protobuf/proto"
	protojson "google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	apipb "go.chromium.org/chromiumos/config/go/test/api"
	artifactpb "go.chromium.org/chromiumos/config/go/test/artifact"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/resultdb/pbutil"
	pb "go.chromium.org/luci/resultdb/proto/v1"
	sinkpb "go.chromium.org/luci/resultdb/sink/proto/v1"
)

// Follows ChromeOS test result convention. Use TestResult to represent the
// ChromeOS test result contract proto defined in
// https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/config/proto/chromiumos/test/artifact/test_result.proto
type CrosTestResult struct {
	TestResult *artifactpb.TestResult `json:"test_result"`

	// testhausBaseURL links to the logs for the test result.
	testhausBaseURL string
}

// ConvertFromJSON reads the provided reader into the receiver.
// The TestResult is cleared and overwritten.
func (r *CrosTestResult) ConvertFromJSON(ctx context.Context, reader io.Reader) error {
	r.TestResult = &artifactpb.TestResult{}
	var rawMessage json.RawMessage
	if err := json.NewDecoder(reader).Decode(&rawMessage); err != nil {
		return err
	}

	if err := protojson.Unmarshal(rawMessage, r.TestResult); err != nil {
		logging.Warningf(ctx, "Warning: found unknown field(s) in test result proto: %+v", rawMessage, err)

		unmarshalOpts := protojson.UnmarshalOptions{
			DiscardUnknown: true,
		}
		if err := unmarshalOpts.Unmarshal(rawMessage, r.TestResult); err != nil {
			return err
		}
	}

	return nil
}

// ToProtos converts ChromeOS test results in r to []*sinkpb.TestResult.
func (r *CrosTestResult) ToProtos(ctx context.Context) ([]*sinkpb.TestResult, error) {
	var ret []*sinkpb.TestResult
	for _, testRun := range r.TestResult.GetTestRuns() {
		testCaseInfo := testRun.GetTestCaseInfo()
		testCaseResult := testCaseInfo.GetTestCaseResult()
		status, expected := genTestResultStatus(testCaseResult)
		testId := getTestId(testCaseResult)
		if testId == "" {
			return nil, errors.Reason("testId is unspecified due to the missing id in test case result: %v",
				testCaseResult).Err()
		}

		tr := &sinkpb.TestResult{
			TestId:   testId,
			Status:   status,
			Expected: expected,
			// TODO(b/251357069): Move the invocation-level info and
			// result-level info to the new JSON type columns accordingly when
			// the new JSON type columns are ready in place.
			Tags:         genTestResultTags(ctx, testRun, r.TestResult.GetTestInvocation()),
			TestMetadata: &pb.TestMetadata{Name: testId},
		}

		if len(testCaseResult.Errors) > 0 &&
			(status == pb.TestStatus_FAIL || status == pb.TestStatus_ABORT || status == pb.TestStatus_CRASH) {
			var rdbErrors []*pb.FailureReason_Error
			var errorsSize int
			var primaryErrorMessage string
			for i, e := range testCaseResult.Errors {
				errorMessage := e.Message
				if len(e.StackTrace) > 0 {
					errorMessage += "\n" + e.StackTrace
				}
				if i == 0 {
					primaryErrorMessage = errorMessage
				}

				rdbError := &pb.FailureReason_Error{
					Message: truncateString(errorMessage, maxErrorMessageBytes),
				}
				errorSize := proto.Size(rdbError)
				if errorsSize+errorSize > maxErrorsBytes {
					// No more errors fit.
					break
				}
				rdbErrors = append(rdbErrors, rdbError)
				errorsSize += errorSize
			}

			tr.FailureReason = &pb.FailureReason{
				PrimaryErrorMessage:  truncateString(primaryErrorMessage, maxErrorMessageBytes),
				Errors:               rdbErrors,
				TruncatedErrorsCount: int32(len(testCaseResult.Errors) - len(rdbErrors)),
			}
		} else if testCaseResult.GetReason() != "" && status != pb.TestStatus_PASS {
			// This path exists to support legacy results until Testhaus UI
			// migration is complete.
			// In future, failure reason should only be set for results considered
			// failed/crashed/aborted by ResultDB; not skipped or passed results.
			// See go/resultdb-failure-reason-integrity-proposal.
			reason := truncateString(
				testCaseResult.GetReason(), maxErrorMessageBytes)
			tr.FailureReason = &pb.FailureReason{
				PrimaryErrorMessage: reason,
				Errors: []*pb.FailureReason_Error{
					{Message: reason},
				},
			}
		}

		if testCaseResult.GetStartTime().CheckValid() == nil {
			tr.StartTime = testCaseResult.GetStartTime()
			if testCaseResult.GetDuration().CheckValid() == nil {
				tr.Duration = testCaseResult.GetDuration()
			}
		}

		if err := PopulateProperties(tr, testRun); err != nil {
			return nil, errors.Annotate(err,
				"failed to unmarshal properties for test result").Err()
		}

		// Add test level artifacts.
		testCaseDir := testCaseResult.GetResultDirPath().GetPath()
		testID := testCaseResult.GetTestCaseId().GetValue()
		arts, err := testResultArtifacts(r.testhausBaseURL, testCaseDir, testCaseResult.GetTestCaseMetadata().GetTestCase().GetName(), testID)
		if err != nil {
			logging.Warningf(ctx, "Warning: failed to prepare test level artifacts from dir: %q for test: %q, err: %v", testCaseDir, testID, err)
		} else {
			logging.Infof(ctx, "Info: Uploading %d test level artifacts to resultdb from dir: %q for test: %q", len(arts), testCaseDir, testID)
			tr.Artifacts = arts
		}
		ret = append(ret, tr)
	}
	return ret, nil
}

// getTestId gets the test id based on the test case result.
func getTestId(testCaseResult *apipb.TestCaseResult) string {
	if testCaseResult == nil || testCaseResult.GetTestCaseId() == nil {
		return ""
	}
	return testCaseResult.GetTestCaseId().GetValue()
}

// genTestResultStatus converts a TestCase Verdict into a ResultSink Status and
// determines the expected field.
func genTestResultStatus(result *apipb.TestCaseResult) (status pb.TestStatus, expected bool) {
	switch result.GetVerdict().(type) {
	case *apipb.TestCaseResult_Pass_:
		return pb.TestStatus_PASS, true
	case *apipb.TestCaseResult_Fail_:
		return pb.TestStatus_FAIL, false
	case *apipb.TestCaseResult_Crash_:
		return pb.TestStatus_CRASH, false
	case *apipb.TestCaseResult_Abort_:
		return pb.TestStatus_ABORT, false
	// Expectedly skipped (TEST_NA in Testhaus).
	case *apipb.TestCaseResult_Skip_:
		return pb.TestStatus_SKIP, true
	// Unexpectedly skipped (NOSTATUS in Testhaus).
	case *apipb.TestCaseResult_NotRun_:
		return pb.TestStatus_SKIP, false
	default:
		return pb.TestStatus_STATUS_UNSPECIFIED, false
	}
}

// PopulateProperties populates the properties of the test result.
func PopulateProperties(testResult *sinkpb.TestResult, testRun *artifactpb.TestRun) error {
	if testRun == nil {
		return nil
	}

	if testResult == nil {
		return errors.Reason("the input test result is nil").Err()
	}

	// Truncates the errors and reason field in advance to reduce the amount of
	// bytes stored in the properties field of test result.
	testCaseResult := testRun.GetTestCaseInfo().GetTestCaseResult()
	if testCaseResult.GetReason() != "" {
		testCaseResult.Reason = truncateString(
			testCaseResult.GetReason(), maxPropErrorMessageBytes)
	}

	errorsSize := len(testCaseResult.Errors)
	if errorsSize > 0 {
		// Truncates the error messages if needed before uploading to the
		// properties field.
		propErrors := make([]*apipb.TestCaseResult_Error, 0, errorsSize)
		curErrorsSize := 0
		for _, e := range testCaseResult.Errors {
			errorMessage := e.Message
			if len(e.StackTrace) > 0 {
				errorMessage += "\n" + e.StackTrace
			}

			propMessage := truncateString(errorMessage, maxPropErrorMessageBytes)
			errorSize := len(propMessage)
			if curErrorsSize+errorSize > maxPropErrorsBytes {
				// No more errors fit.
				break
			}

			propError := &apipb.TestCaseResult_Error{
				Message: propMessage,
			}
			propErrors = append(propErrors, propError)
			curErrorsSize += errorSize
		}
		testCaseResult.Errors = propErrors
	}

	data, err := protojson.Marshal(testRun)
	if err != nil {
		return err
	}

	testResult.Properties = &structpb.Struct{}
	return protojson.Unmarshal(data, testResult.Properties)
}

// TODO(b/240897202): Remove the tags when a JSON type field is supported in
// ResultDB schema.
// genTestResultTags generates test result tags based on the ChromeOS test
// result contract proto and returns the sorted tags.
func genTestResultTags(ctx context.Context, testRun *artifactpb.TestRun, testInvocation *artifactpb.TestInvocation) []*pb.StringPair {
	tags := []*pb.StringPair{}

	if testInvocation != nil {
		tags = AppendTags(tags, "is_cft_run", strconv.FormatBool(testInvocation.IsCftRun))
		tags = AppendTags(tags, "is_trv2_run", strconv.FormatBool(testInvocation.IsTrv2Run))

		is3DRun := strconv.FormatBool(testInvocation.Is_3DRun)
		tags = AppendTags(tags, "is_3d_run", is3DRun)
		if is3DRun == "true" {
			eqcInfo := testRun.GetEqcInfo()
			if eqcInfo != nil {
				tags = AppendTags(tags, "eqc_hash", eqcInfo.GetEqcHash())
			}
		}

		dutTopology := testInvocation.GetDutTopology()
		if dutTopology != nil {
			// Populates the hostname by the DUT topology ID which captures
			// the correct ID of the scheduling unit for multi-dut testing.
			// For single-dut testing, the DUT topology ID is the same as the
			// primary DUT ID.
			tags = AppendTags(tags, "hostname", dutTopology.GetId().GetValue())
		}

		// For Testhaus MVP parity.
		// Refer to `_generate_resultdb_base_tags` in test_runner recipe:
		// https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:infra/recipes/recipes/test_platform/test_runner.py;l=472?q=test_platform%2Ftest_runner.py
		primaryExecInfo := testInvocation.GetPrimaryExecutionInfo()
		if primaryExecInfo != nil {
			buildInfo := primaryExecInfo.GetBuildInfo()
			if buildInfo != nil {
				buildName := buildInfo.Name
				tags = AppendTags(tags, "image", buildName)
				tags = AppendTags(tags, "builder_name", strings.Split(buildName, "/")[0])
				tags = AppendTags(tags, "build", strings.Split(buildName, "/")[1])
				tags = AppendTags(tags, "board", buildInfo.Board)
				tags = AppendTags(tags, "board_type", buildInfo.BoardType)

				tags = configBuildMetaDataTags(tags, buildInfo.BuildMetadata)
			}

			dutInfo := primaryExecInfo.GetDutInfo()
			if dutInfo != nil {
				if dutInfo.GetDut() != nil {
					dut := dutInfo.GetDut()
					chromeOSInfo := dut.GetChromeos()
					if chromeOSInfo != nil {
						tags = AppendTags(tags, "model", chromeOSInfo.GetDutModel().GetModelName())
						tags = AppendTags(tags, "phase", chromeOSInfo.GetPhase().String())
					}

					tags = AppendTags(tags, "cbx", strconv.FormatBool(dutInfo.GetCbx()))
				}
				usbInfo := dutInfo.GetUsbInfo()
				if usbInfo != nil {
					tags = AppendTags(tags, "power_delivery_port_servo", usbInfo.GetPowerDeliveryPortServo())
					tags = AppendTags(tags, "power_delivery_port_count", strconv.FormatUint(uint64(usbInfo.GetPowerDeliveryPortCount()), 10))
				}
			}

			inventoryInfo := primaryExecInfo.GetInventoryInfo()
			if inventoryInfo != nil {
				ufsZone := inventoryInfo.GetUfsZone()
				if len(ufsZone) != 0 {
					tags = AppendTags(tags, "ufs_zone", ufsZone)
				}
			}

			tags = configEnvInfoTags(tags, primaryExecInfo)

			// For Multi-DUT testing info.
			tags = configMultiDUTTags(tags, primaryExecInfo, testInvocation.GetSecondaryExecutionsInfo())
		}

		schedulingMetadata := testInvocation.GetSchedulingMetadata()
		if schedulingMetadata != nil {
			schedulingArgs := schedulingMetadata.GetSchedulingArgs()
			requestedArgs := map[string]bool{
				"analytics_name":    true,
				"ctp-fwd-task-name": true,
				"qs_account":        true,
			}
			for k, v := range schedulingArgs {
				if _, ok := requestedArgs[k]; ok {
					// Converts tag key properly since some might contain
					// hyphens.
					tags = AppendTags(tags, convertTagKey(k), v)
				}
			}
		}

		// Set default account ID as Google ID from Testhaus
		// because Account ID in PartnerInfo is set only for Partner accounts.
		accountIDTag := "1"
		partnerInfo := testInvocation.GetPartnerInfo()
		if partnerInfo != nil {
			accountID := partnerInfo.GetAccountId()
			if accountID != 0 {
				accountIDTag = strconv.FormatInt(accountID, 10)
			}
		}
		tags = AppendTags(tags, "account_id", accountIDTag)

		tags = configProjectTrackerMetadataTags(tags, testInvocation)
	}

	if testRun != nil {
		// For Testhaus MVP parity.
		// Refer to `_generate_resultdb_base_tags` in test_runner recipe:
		// https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:infra/recipes/recipes/test_platform/test_runner.py;l=472?q=test_platform%2Ftest_runner.py
		for _, logInfo := range testRun.LogsInfo {
			tags = AppendTags(tags, "logs_url", logInfo.Path)
		}

		testCaseInfo := testRun.TestCaseInfo
		if testCaseInfo != nil {
			tags = AppendTags(tags, "declared_name", testCaseInfo.DisplayName)
			tags = AppendTags(tags, "branch", testCaseInfo.Branch)
			tags = AppendTags(tags, "main_builder_name", testCaseInfo.MainBuilderName)
			tags = AppendTags(tags, "contacts", strings.Join(testCaseInfo.Contacts, ","))
			tags = AppendTags(tags, "suite", testCaseInfo.Suite)
			tags = AppendTags(tags, "channel", testCaseInfo.Channel)
			tags = AppendTags(tags, "requester", testCaseInfo.Requester)

			tags = configTestMetadataTags(ctx, tags, testCaseInfo.GetTestCaseResult().GetTestCaseMetadata())
			tags = configAVLInfoTags(tags, testCaseInfo.GetAvlInfo())
			tags = configGSCInfoTags(tags, testCaseInfo.GetGscInfo())
			tags = configStressTestInfoTags(tags, testCaseInfo.GetStressTestInfo())
		}

		timeInfo := testRun.TimeInfo
		if timeInfo != nil {
			if timeInfo.GetQueuedTime().CheckValid() == nil {
				tags = AppendTags(tags, "queued_time", timeInfo.GetQueuedTime().AsTime().UTC().String())
			}
		}

		execMetadata := testRun.ExecutionMetadata
		if execMetadata != nil {
			testArgs := convertTestArgsTag(execMetadata.TestArgs)
			if testArgs != "" {
				tags = AppendTags(tags, "test_args", testArgs)
			}
		}
	}

	pbutil.SortStringPairs(tags)
	return tags
}

// convertTestArgsTag convert the test args map into a tag.
func convertTestArgsTag(testArgs map[string]string) string {
	testArgsLen := len(testArgs)
	if testArgsLen == 0 {
		return ""
	}

	testArgsSlice := make([]string, 0, testArgsLen)
	for key, val := range testArgs {
		testArgsSlice = append(testArgsSlice, key+"="+val)
	}

	sort.Strings(testArgsSlice)
	return truncateString(strings.Join(testArgsSlice, " "), maxTagValueBytes)
}

// configBuildMetaDataTags configures test result tags based on the build metadata.
func configBuildMetaDataTags(tags []*pb.StringPair, buildMetadata *artifactpb.BuildMetadata) []*pb.StringPair {
	if buildMetadata == nil {
		return tags
	}

	newTags := make([]*pb.StringPair, 0, len(tags))
	newTags = append(newTags, tags...)

	firmware := buildMetadata.GetFirmware()
	if firmware != nil {
		newTags = AppendTags(newTags, "ro_fwid", firmware.GetRoVersion())
		newTags = AppendTags(newTags, "rw_fwid", firmware.GetRwVersion())
		newTags = AppendTags(newTags, "ap_ro_fwid", firmware.GetApRoVersion())
		newTags = AppendTags(newTags, "ap_rw_fwid", firmware.GetApRwVersion())
		newTags = AppendTags(newTags, "ec_ro_fwid", firmware.GetEcRoVersion())
		newTags = AppendTags(newTags, "ec_rw_fwid", firmware.GetEcRwVersion())
	}

	chipset := buildMetadata.GetChipset()
	if chipset != nil {
		newTags = AppendTags(newTags, "wifi_chip", chipset.GetWifiChip())
		newTags = AppendTags(newTags, "wifi_router_models", chipset.GetWifiRouterModels())
	}

	kernel := buildMetadata.GetKernel()
	if kernel != nil {
		newTags = AppendTags(newTags, "kernel_version", kernel.GetVersion())
	}

	sku := buildMetadata.GetSku()
	if sku != nil {
		newTags = AppendTags(newTags, "hwid", sku.GetHwid())
		newTags = AppendTags(newTags, "hwid_sku", sku.GetHwidSku())
		newTags = AppendTags(newTags, "dlm_sku_id", sku.GetDlmSkuId())
	}

	cellular := buildMetadata.GetCellular()
	if cellular != nil {
		newTags = AppendTags(newTags, "carrier", cellular.GetCarrier())
	}

	lacros := buildMetadata.GetLacros()
	if lacros != nil {
		newTags = AppendTags(newTags, "ash_version", lacros.GetAshVersion())
		newTags = AppendTags(newTags, "lacros_version", lacros.GetLacrosVersion())
	}

	chameleonInfo := buildMetadata.GetChameleonInfo()
	if chameleonInfo != nil {
		chameleonTypes := make([]string, 0, len(chameleonInfo.ChameleonType))
		for _, t := range chameleonInfo.ChameleonType {
			chameleonTypes = append(chameleonTypes, t.String())
		}
		if len(chameleonTypes) != 0 {
			newTags = AppendTags(newTags, "chameleon_type", strings.Join(chameleonTypes, ","))
		}

		chameleonConnectionTypes := make([]string, 0, len(chameleonInfo.ChameleonConnectionTypes))
		for _, t := range chameleonInfo.ChameleonConnectionTypes {
			chameleonConnectionTypes = append(chameleonConnectionTypes, t.String())
		}
		if len(chameleonConnectionTypes) != 0 {
			newTags = AppendTags(newTags, "chameleon_connection_types", strings.Join(chameleonConnectionTypes, ","))
		}
	}

	modemInfo := buildMetadata.GetModemInfo()
	if modemInfo != nil {
		newTags = AppendTags(newTags, "modem_type", modemInfo.GetType().String())
	}

	gfxInfo := buildMetadata.GetGfxInfo()
	if gfxInfo != nil {
		newTags = AppendTags(newTags, "display_panel_name", gfxInfo.DisplayPanelName)
		newTags = AppendTags(newTags, "display_present_hdr", gfxInfo.DisplayPresentHdr)
		newTags = AppendTags(newTags, "display_present_psr", gfxInfo.DisplayPresentPsr)
		newTags = AppendTags(newTags, "display_present_vrr", gfxInfo.DisplayPresentVrr)
		newTags = AppendTags(newTags, "display_refresh_rate", gfxInfo.DisplayRefreshRate)
		newTags = AppendTags(newTags, "display_resolution", gfxInfo.DisplayResolution)
		newTags = AppendTags(newTags, "gpu_family", gfxInfo.GpuFamily)
		newTags = AppendTags(newTags, "gpu_id", gfxInfo.GpuId)
		newTags = AppendTags(newTags, "gpu_open_gles_version", gfxInfo.GpuOpenGlesVersion)
		newTags = AppendTags(newTags, "gpu_vendor", gfxInfo.GpuVendor)
		newTags = AppendTags(newTags, "gpu_vulkan_version", gfxInfo.GpuVulkanVersion)
		newTags = AppendTags(newTags, "platform_cpu_vendor", gfxInfo.PlatformCpuVendor)
		newTags = AppendTags(newTags, "platform_disk_size", strconv.FormatUint(gfxInfo.PlatformDiskSize, 10))
		newTags = AppendTags(newTags, "platform_memory_size", strconv.FormatUint(gfxInfo.PlatformMemorySize, 10))
	}

	servoInfo := buildMetadata.GetServoInfo()
	if servoInfo != nil {
		newTags = AppendTags(newTags, "servod_version", servoInfo.ServodVersion)
		newTags = AppendTags(newTags, "servo_type", servoInfo.ServoType)
		newTags = AppendTags(newTags, "servo_versions", servoInfo.ServoVersions)
	}

	return newTags
}

// configEnvInfoTags configs test result tags based on the test environment
// information.
func configEnvInfoTags(tags []*pb.StringPair, execInfo *artifactpb.ExecutionInfo) []*pb.StringPair {
	envInfo := execInfo.GetEnvInfo()
	if envInfo == nil {
		return tags
	}

	newTags := make([]*pb.StringPair, 0, len(tags))
	newTags = append(newTags, tags...)

	switch envInfo.(type) {
	case *artifactpb.ExecutionInfo_SkylabInfo:
		skylabInfo := execInfo.GetSkylabInfo()
		if skylabInfo != nil {
			newTags = configDroneTags(newTags, skylabInfo.GetDroneInfo())
			newTags = configSwarmingTags(newTags, skylabInfo.GetSwarmingInfo())
			newTags = configBuildbucketTags(newTags, skylabInfo.GetBuildbucketInfo())
		}
	case *artifactpb.ExecutionInfo_SatlabInfo:
		satlabInfo := execInfo.GetSatlabInfo()
		if satlabInfo != nil {
			newTags = configSwarmingTags(newTags, satlabInfo.GetSwarmingInfo())
			newTags = configBuildbucketTags(newTags, satlabInfo.GetBuildbucketInfo())
			newTags = configDroneTags(newTags, satlabInfo.GetDroneInfo())
		}
	}
	return newTags
}

// configDroneTags configs test result tags based on the Drone information.
func configDroneTags(tags []*pb.StringPair, droneInfo *artifactpb.DroneInfo) []*pb.StringPair {
	if droneInfo == nil {
		return tags
	}

	newTags := make([]*pb.StringPair, 0, len(tags))
	newTags = append(newTags, tags...)

	newTags = AppendTags(newTags, "drone", droneInfo.GetDrone())
	newTags = AppendTags(newTags, "drone_server", droneInfo.GetDroneServer())
	return newTags
}

// configSwarmingTags configs test result tags based on the swarming
// information.
func configSwarmingTags(tags []*pb.StringPair, swarmingInfo *artifactpb.SwarmingInfo) []*pb.StringPair {
	if swarmingInfo == nil {
		return tags
	}

	newTags := make([]*pb.StringPair, 0, len(tags))
	newTags = append(newTags, tags...)

	newTags = AppendTags(newTags, "task_id", swarmingInfo.GetTaskId())
	newTags = AppendTags(newTags, "suite_task_id", swarmingInfo.GetSuiteTaskId())
	newTags = AppendTags(newTags, "job_name", swarmingInfo.GetTaskName())
	newTags = AppendTags(newTags, "pool", swarmingInfo.GetPool())
	newTags = AppendTags(newTags, "label_pool", swarmingInfo.GetLabelPool())
	newTags = AppendTags(newTags, "bot_id", swarmingInfo.GetBotId())
	newTags = AppendTags(newTags, "bot_config", swarmingInfo.GetBotConfig())
	return newTags
}

// configBuildbucketTags configs test result tags based on the buildbucket
// information.
func configBuildbucketTags(tags []*pb.StringPair, buildbucketInfo *artifactpb.BuildbucketInfo) []*pb.StringPair {
	if buildbucketInfo == nil {
		return tags
	}

	newTags := make([]*pb.StringPair, 0, len(tags))
	newTags = append(newTags, tags...)

	buildbucketBuilder := buildbucketInfo.GetBuilder()
	if buildbucketBuilder != nil {
		newTags = AppendTags(
			newTags, "buildbucket_builder", buildbucketBuilder.GetBuilder())
	}

	return AppendTags(
		newTags,
		"ancestor_buildbucket_ids",
		strings.Trim(
			strings.Join(strings.Fields(
				fmt.Sprint(buildbucketInfo.GetAncestorIds())), ","),
			"[]"))
}

// configMultiDUTTags configs test result tags based on the multi-DUT testing.
func configMultiDUTTags(tags []*pb.StringPair, primaryExecInfo *artifactpb.ExecutionInfo, secondaryExecInfos []*artifactpb.ExecutionInfo) []*pb.StringPair {
	// PrimaryExecInfo must be set for Multi-DUT testing.
	if primaryExecInfo == nil {
		return tags
	}

	newTags := make([]*pb.StringPair, 0, len(tags))
	newTags = append(newTags, tags...)

	if len(secondaryExecInfos) == 0 {
		return AppendTags(newTags, "multiduts", "False")
	}

	newTags = AppendTags(newTags, "multiduts", "True")
	newTags = AppendTags(newTags, "primary_board", primaryExecInfo.GetBuildInfo().GetBoard())
	newTags = AppendTags(newTags, "primary_model", primaryExecInfo.GetDutInfo().GetDut().GetChromeos().GetDutModel().GetModelName())
	newTags = AppendTags(newTags, "primary_phase", primaryExecInfo.GetDutInfo().GetDut().GetChromeos().GetPhase().String())

	secordaryDUTSize := len(secondaryExecInfos)
	secondaryBoards := make([]string, 0, secordaryDUTSize)
	secondaryModels := make([]string, 0, secordaryDUTSize)
	secondaryPhases := make([]string, 0, secordaryDUTSize)
	for _, execInfo := range secondaryExecInfos {
		buildInfo := execInfo.GetBuildInfo()
		dutInfo := execInfo.GetDutInfo()
		if buildInfo != nil && dutInfo != nil {
			secondaryBoards = append(secondaryBoards, buildInfo.GetBoard())
			secondaryModels = append(secondaryModels, dutInfo.GetDut().GetChromeos().GetDutModel().GetModelName())
			secondaryPhases = append(secondaryPhases, dutInfo.GetDut().GetChromeos().GetPhase().String())
		}
	}

	// Concatenates board names and model names separately.
	newTags = AppendTags(newTags, "secondary_boards", strings.Join(secondaryBoards, " | "))
	newTags = AppendTags(newTags, "secondary_models", strings.Join(secondaryModels, " | "))
	newTags = AppendTags(newTags, "secondary_phases", strings.Join(secondaryPhases, " | "))
	return newTags
}

// configProjectTrackerMetadataTags configs test result tags based on the project tracker metadata.
func configProjectTrackerMetadataTags(tags []*pb.StringPair, testInvocation *artifactpb.TestInvocation) []*pb.StringPair {
	projectTrackerMetadata := testInvocation.GetProjectTrackerMetadata()
	if projectTrackerMetadata == nil {
		return tags
	}

	newTags := make([]*pb.StringPair, 0, len(tags))
	newTags = append(newTags, tags...)

	newTags = AppendTags(newTags, "qual_bug_id", projectTrackerMetadata.GetBugId())
	return newTags
}

// configTestMetadataTags configs test result tags based on the test case metadata.
func configTestMetadataTags(ctx context.Context, tags []*pb.StringPair, testMetadata *apipb.TestCaseMetadata) []*pb.StringPair {
	metadataTags := metadataToTags(ctx, testMetadata)
	if len(metadataTags) == 0 {
		return tags
	}

	newTags := make([]*pb.StringPair, 0, len(tags)+len(metadataTags))
	newTags = append(newTags, tags...)
	newTags = append(newTags, metadataTags...)
	return newTags
}

// configAVLInfoTags configs test result tags based on the AVL info.
func configAVLInfoTags(tags []*pb.StringPair, avlInfo *artifactpb.AvlInfo) []*pb.StringPair {
	if avlInfo == nil {
		return tags
	}

	newTags := make([]*pb.StringPair, 0, len(tags)+avlInfo.ProtoReflect().Descriptor().Fields().Len())
	newTags = append(newTags, tags...)

	newTags = AppendTags(newTags, "avl_part_model", avlInfo.GetAvlPartModel())
	newTags = AppendTags(newTags, "avl_part_firmware", avlInfo.GetAvlPartFirmware())
	newTags = AppendTags(newTags, "avl_component_type", avlInfo.GetAvlComponentType())

	return newTags
}

// configGSCInfoTags configs test result tags based on the GSC devboard info.
func configGSCInfoTags(tags []*pb.StringPair, gscInfo *artifactpb.GscInfo) []*pb.StringPair {
	if gscInfo == nil {
		return tags
	}

	newTags := make([]*pb.StringPair, 0, len(tags)+gscInfo.ProtoReflect().Descriptor().Fields().Len())
	newTags = append(newTags, tags...)

	newTags = AppendTags(newTags, "gsc_buildurl", gscInfo.GetGscBuildurl())
	newTags = AppendTags(newTags, "gsc_ccd_serial", gscInfo.GetGscCcdSerial())
	newTags = AppendTags(newTags, "gsc_devboardservice_version", gscInfo.GetGscDevboardserviceVersion())
	newTags = AppendTags(newTags, "gsc_hyperdebug_serial", gscInfo.GetGscHyperdebugSerial())
	newTags = AppendTags(newTags, "gsc_hyperdebug_version", gscInfo.GetGscHyperdebugVersion())
	newTags = AppendTags(newTags, "gsc_opentitantool_version", gscInfo.GetGscOpentitantoolVersion())
	newTags = AppendTags(newTags, "gsc_ro_version", gscInfo.GetGscRoVersion())
	newTags = AppendTags(newTags, "gsc_rw_branch", gscInfo.GetGscRwBranch())
	newTags = AppendTags(newTags, "gsc_rw_rev", gscInfo.GetGscRwRev())
	newTags = AppendTags(newTags, "gsc_rw_sha", gscInfo.GetGscRwSha())
	newTags = AppendTags(newTags, "gsc_rw_version", gscInfo.GetGscRwVersion())
	newTags = AppendTags(newTags, "gsc_tast_version", gscInfo.GetGscTastVersion())
	newTags = AppendTags(newTags, "gsc_testbed_serial", gscInfo.GetGscTestbedSerial())
	newTags = AppendTags(newTags, "gsc_testbed_type", gscInfo.GetGscTestbedType())

	return newTags
}

// configStressTestInfoTags configs test result tags based on the stress test info.
func configStressTestInfoTags(tags []*pb.StringPair, stressTestInfo *artifactpb.StressTestInfo) []*pb.StringPair {
	if stressTestInfo == nil {
		return tags
	}

	newTags := make([]*pb.StringPair, 0, len(tags)+stressTestInfo.ProtoReflect().Descriptor().Fields().Len())
	newTags = append(newTags, tags...)

	newTags = AppendTags(newTags, "stress_test_iterations", strconv.FormatUint(uint64(stressTestInfo.GetIterations()), 10))

	return newTags
}

// testResultArtifacts returns the map of relative filepaths to the result sink artifacts by walking the resultDir.
// It also adds the test level testhaus logs link based on the testhausBaseURL, testName and testID.
// testhausBaseURL is the base URL of the testhaus logs link.
// e.g. https://tests.chromeos.goog/p/chromeos/logs/unified/invocation/build-12345
// Note that testName is the testID without the test harness name.
func testResultArtifacts(testhausBaseURL, resultDir, testName, testID string) (map[string]*sinkpb.Artifact, error) {
	artifacts := map[string]*sinkpb.Artifact{}
	// Map normal relative paths to full paths.
	normPathToFullPaths, err := processArtifacts(resultDir)
	if err != nil {
		return nil, err
	}
	for normPath, fullPath := range normPathToFullPaths {
		artifacts[normPath] = &sinkpb.Artifact{
			Body: &sinkpb.Artifact_FilePath{FilePath: fullPath},
		}
	}

	if testhausBaseURL != "" {
		artifacts["testhaus_logs"] = &sinkpb.Artifact{
			Body: &sinkpb.Artifact_Contents{
				Contents: []byte(fmt.Sprintf("%s?treeQuery=%s&test=%s", strings.TrimSuffix(testhausBaseURL, "/"), url.QueryEscape(testName), url.QueryEscape(testID))),
			},
			ContentType: "text/x-uri",
		}
	}

	return artifacts, nil
}
