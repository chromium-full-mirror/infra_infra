// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executions

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"cloud.google.com/go/civil"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	build_api "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	artifactpb "go.chromium.org/chromiumos/config/go/test/artifact"
	api_common "go.chromium.org/chromiumos/infra/proto/go/test_platform/common"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner/steps"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/buildbucket/protoutil"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/lucictx"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/common_builders"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/cmd/cros_test_runner/analytics"
	"infra/cros/cmd/cros_test_runner/data"
	"infra/cros/cmd/cros_test_runner/internal/configs"
	"infra/cros/cmd/cros_test_runner/protos"
)

var ioProps = build.RegisterSplitProperty[*steps.RunTestsRequest, *steps.RunTestsResponse]("")
var ctrInputProp = build.RegisterInputProperty[*protos.CipdVersionInfo](common.HwTestCtrInputPropertyName)
var trInputProp = build.RegisterInputProperty[*protos.CipdVersionInfo](common.HwTestTrInputPropertyName)

// TODO : Re-structure different execution flow properly later.
// HwExecution represents hw executions.
func HwExecution() {
	build.Main(func(ctx context.Context, args []string, st *build.State) error {
		// Get current invocation name
		invocationName := ""
		rdb := lucictx.GetResultDB(ctx)
		if rdb != nil && rdb.GetCurrentInvocation() != nil {
			invocationName = rdb.GetCurrentInvocation().GetName()
		}
		input := ioProps.GetInput(ctx)
		isPartnerRun := ioProps.GetInput(ctx).GetCommonConfig().GetPartnerConfig() != nil

		log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmsgprefix)

		logging.Infof(ctx, "have input %v", input)
		ctrCipdInfo := ctrInputProp.GetInput(ctx)
		trCipdInfo := trInputProp.GetInput(ctx)
		logging.Infof(ctx, "have ctr info: %v", ctrCipdInfo)
		logging.Infof(ctx, "ctr label: %s", ctrCipdInfo.GetVersion().GetCipdLabel())
		resp := &steps.RunTestsResponse{}
		// TODO (azrahman): After stablizing in prod, move log data gs root to cft/new proto.
		var skylabResult *skylab_test_runner.Result
		var crosTestRunnerRequest *api.CrosTestRunnerDynamicRequest
		var err error
		startTime := time.Now()
		metrics := &analytics.TrInstanceBqData{
			BBID:      fmt.Sprint(st.Build().GetId()),
			CIPDLabel: trCipdInfo.GetVersion().GetCipdLabel(),
			Bucket:    st.Build().GetBuilder().GetBucket(),
			IsLed:     st.Build().GetInfra().GetLed() != nil,
		}
		defer handleAnalytics(ctx, st, &metrics, startTime, &err, &skylabResult)

		if input.CrosTestRunnerDynamicRequest != nil {
			metrics.IsDynamic = true
			// If the request is a CrosTestRunner dynamic request...
			skylabResult, err = executeHwTestsV2(ctx, nil, input.CrosTestRunnerDynamicRequest, input.CommonConfig, ctrCipdInfo.GetVersion().GetCipdLabel(), input.GetConfig().GetOutput().GetLogDataGsRoot(), invocationName, st, input.IsAlRun)
		} else if input.CftTestRequest.TranslateTrv2Request || shouldRunDynamic(input.CftTestRequest) {
			metrics.IsDynamic = true
			// If the request is a CrosTestRunner non-dynamic request with translation flag...
			crosTestRunnerRequest, err = common_builders.NewDynamicTrv2FromCftBuilder(input.CftTestRequest).BuildRequest(ctx, input.IsAlRun, isPartnerRun)
			if err == nil {
				skylabResult, err = executeHwTestsV2(ctx, input.CftTestRequest, crosTestRunnerRequest, input.CommonConfig, ctrCipdInfo.GetVersion().GetCipdLabel(), input.GetConfig().GetOutput().GetLogDataGsRoot(), invocationName, st, input.IsAlRun)
			}
		} else {
			// If the request is a CrosTestRunner non-dynamic request...
			skylabResult, err = executeHwTests(ctx, input.CftTestRequest, input.CommonConfig, ctrCipdInfo.GetVersion().GetCipdLabel(), input.GetConfig().GetOutput().GetLogDataGsRoot(), invocationName, st)
		}
		if err != nil {
			errString := err.Error()
			if common.GlobalNonInfraError != nil {
				err = common.GlobalNonInfraError
			} else {
				err = build.AttachStatus(err, buildbucketpb.Status_INFRA_FAILURE, nil)
			}
			logging.Infof(ctx, "error found: %s", err)
			st.SetSummaryMarkdown(errString)
			resp.ErrorSummaryMarkdown = errString

			// Parse the test_runner error type and add it to the skylab result if any
			// was found.
			errType := skylab_test_runner.TestRunnerErrorType_OTHER
			var tre *common.TestRunnerError
			if ok := errors.As(err, &tre); ok {
				errType = tre.Type
			}
			if skylabResult == nil {
				skylabResult = &skylab_test_runner.Result{}
			}
			skylabResult.ErrorType = errType
			skylabResult.ErrorString = errString
		}
		if skylabResult != nil {
			setMarkdown(skylabResult, st, resp)

			m, _ := proto.Marshal(skylabResult)
			var b bytes.Buffer
			w := zlib.NewWriter(&b)
			_, _ = w.Write(m)
			_ = w.Close()
			resp.CompressedResult = base64.StdEncoding.EncodeToString(b.Bytes())
		}

		ioProps.SetOutput(ctx, resp)
		return err
	})
}

func shouldRunDynamic(cftTestRequest *skylab_test_runner.CFTTestRequest) bool {
	if cftTestRequest.GetCompanionDuts() != nil && len(cftTestRequest.GetCompanionDuts()) > 0 {
		return true
	}

	return false
}

// executeHwTests executes hw tests
func executeHwTests(
	ctx context.Context,
	req *skylab_test_runner.CFTTestRequest,
	commonConfig *skylab_test_runner.CommonConfig,
	ctrCipdVersion string,
	gsRoot string,
	invocationName string,
	buildState *build.State) (*skylab_test_runner.Result, error) {

	// Validation
	if err := validateDeadline(ctx, req.GetDeadline()); err != nil {
		return nil, err
	}
	err := validateHwExecution(ctrCipdVersion, gsRoot)
	if err != nil {
		return nil, err
	}

	// Create ctr
	ctr := setupCtr(ctrCipdVersion)

	// Create configs
	metadataContainers := req.GetContainerMetadata().GetContainers()
	metadataKey := req.GetPrimaryDut().GetContainerMetadataKey()
	metadataMap, ok := metadataContainers[metadataKey]
	if !ok {
		// Loop through the map to get the first value
		for _, firstValue := range metadataContainers {
			metadataMap = firstValue
			break
		}
		if metadataMap == nil {
			return nil, fmt.Errorf("container metadata is empty")
		}
	}
	dockerKeyFile, err := common.LocateFile([]string{common.LabDockerKeyFileLocation, common.VmLabDockerKeyFileLocation})
	if err != nil {
		return nil, fmt.Errorf("unable to locate dockerKeyFile during initialization: %w", err)
	}
	cqRun := common.IsCqRun(req.TestSuites)
	containerImagesMap := metadataMap.GetImages()
	common.PatchContainerMetadata(containerImagesMap, req.GetAutotestKeyvals()["build"])
	containerCfg := configs.NewContainerConfig(ctr, containerImagesMap, cqRun)
	executorCfg := configs.NewExecutorConfig(ctr, containerCfg)
	cmdCfg := configs.NewCommandConfig(executorCfg)

	// Create state keeper
	gcsurl := common.GetGcsURL(gsRoot)
	sk := data.NewHwTestStateKeeper()
	sk.BuildState = buildState
	sk.CftTestRequest = req
	sk.CommonConfig = commonConfig
	sk.Ctr = ctr
	sk.DockerKeyFileLocation = dockerKeyFile
	sk.GcsPublishSrcDir = os.Getenv("TEMPDIR")
	sk.CpconPublishSrcDir = os.Getenv("TEMPDIR")
	sk.RdbPublishSrcDir = os.Getenv("TEMPDIR")
	sk.GcsURL = gcsurl
	sk.TesthausURL = common.GetTesthausURL(invocationName, gcsurl)
	sk.ContainerImages = containerImagesMap

	// Cros-provision uses servo-nexus as a container now.
	// Hack the command/executor into non-dynamic.
	if common.GetBotProvider() != common.BotProviderGce {
		if _, ok := containerImagesMap[common.ServoNexus]; ok {
			sk.ContainerQueue.PushBack(common_builders.BuildServoNexusContainerRequest(common.NewPrimaryDeviceIdentifier()))
		}
	}
	// Post process was only included in the dynamic format.
	// Hack the command/executor into non-dynamic.
	sk.ContainerQueue.PushBack(common_builders.BuildPostProcessContainerRequest(common.PostProcess))
	sk.PostTestQueue.PushBack(common_builders.BuildPostProcessRequest(common.PostProcess))

	if sk.CftTestRequest.GetPrimaryDut() != nil {
		sk.PrimaryDutModel = sk.CftTestRequest.GetPrimaryDut().GetDutModel()
	}
	for _, companion := range sk.CftTestRequest.GetCompanionDuts() {
		sk.CompanionDutModels = append(sk.CompanionDutModels, companion.GetDutModel())
	}

	// For demonstration/logging purposes.
	common.LogWarningIfErr(ctx, sk.Injectables.Set("req", req))
	common.LogWarningIfErr(ctx, sk.Injectables.Set("botDims", protoutil.MustBotDimensions(buildState.Build())))
	common.LogWarningIfErr(ctx, sk.Injectables.Set("gcs-url", gcsurl))
	common.LogWarningIfErr(ctx, sk.Injectables.Set("testhaus-url", common.GetTesthausURL(invocationName, gcsurl)))

	// Generate config
	hwTestConfig := configs.NewTrv2ExecutionConfig(configs.HwTestExecutionConfigType, cmdCfg, sk, req.GetStepsConfig())
	err = hwTestConfig.GenerateConfig(ctx)
	if err != nil {
		return sk.SkylabResult, errors.Annotate(err, "error during generating hw test configs: ").Err()
	}

	// Execute config
	err = hwTestConfig.Execute(ctx)
	// For demonstration/logging purposes.
	sk.Injectables.LogStorageToBuild(ctx, buildState)
	if err != nil {
		return sk.SkylabResult, errors.Annotate(err, "error during executing hw test configs: ").Err()
	}
	return sk.SkylabResult, nil
}

// getATPIDs gets the invocation and parent work unit IDs from either the build
// tags or the execution metadata.
func getATPIDs(b *buildbucketpb.Build, input *steps.RunTestsRequest) (invocationID, workUnitID string) {
	if b == nil && input == nil {
		// This returns with "", ""
		return
	}

	// First fetch the values from the execution metadata. This will serve as
	// the "default" values in case we weren't able to add them to the build
	// tags.
	for _, suite := range input.GetCrosTestRunnerDynamicRequest().GetParams().GetTestSuites() {
		for _, args := range suite.ExecutionMetadata.GetArgs() {
			switch args.GetFlag() {
			case "ants_invocation_id":
				invocationID = args.GetValue()
			case "ants_work_unit_id":
				workUnitID = args.GetValue()

			}
		}
	}

	// Search through the build tags to see if we were provided with the atp
	// information. If provided we will use these values as they indicate a
	// successful ATTEMPT work unit creation.
	for _, tag := range b.GetTags() {
		switch tag.GetKey() {
		case "ants_invocation_id":
			invocationID = tag.GetValue()
		case "attempt_wu_id":
			workUnitID = tag.GetValue()
		}
	}

	// Golang naked return. This will return the current value of the named
	// return values.
	//
	// NOTE: This can be "", "" if no values were set.
	return
}

// executeHwTestsV2 uses the dynamic CrosTestRunner request to construct
// a hardware test execution environment.
func executeHwTestsV2(
	ctx context.Context,
	cft *skylab_test_runner.CFTTestRequest,
	req *api.CrosTestRunnerDynamicRequest,
	commonConfig *skylab_test_runner.CommonConfig,
	ctrCipdVersion string,
	gsRoot string,
	invocationName string,
	buildState *build.State,
	isAlRun bool) (*skylab_test_runner.Result, error) {

	// Validation
	if err := validateDeadline(ctx, req.GetParams().GetDeadline()); err != nil {
		return nil, &common.TestRunnerError{
			Type: skylab_test_runner.TestRunnerErrorType_INPUT_VALIDATION,
			Err:  err,
		}
	}
	err := validateHwExecution(ctrCipdVersion, gsRoot)
	if err != nil {
		return nil, &common.TestRunnerError{
			Type: skylab_test_runner.TestRunnerErrorType_INPUT_VALIDATION,
			Err:  err,
		}
	}

	// Create ctr
	ctr := setupCtr(ctrCipdVersion)

	var containerImagesMap map[string]*build_api.ContainerImageInfo
	// Create configs
	// No metadata container configs required for AL run
	if !isAlRun {
		metadataContainers := req.GetParams().GetContainerMetadata().GetContainers()
		metadataKey := req.GetParams().GetContainerMetadataKey()
		metadataMap, ok := metadataContainers[metadataKey]
		if !ok {
			// Loop through the map to get the first value
			for _, firstValue := range metadataContainers {
				metadataMap = firstValue
				break
			}
			if metadataMap == nil {
				return nil, &common.TestRunnerError{
					Type: skylab_test_runner.TestRunnerErrorType_OTHER,
					Err:  fmt.Errorf("container metadata is empty"),
				}
			}
		}

		containerImagesMap = metadataMap.GetImages()
		common.PatchContainerMetadata(containerImagesMap, req.GetParams().GetKeyvals()["build"])
	}

	dockerKeyFile, err := common.LocateFile([]string{common.LabDockerKeyFileLocation, common.VmLabDockerKeyFileLocation})
	if err != nil {
		return nil, &common.TestRunnerError{
			Type: skylab_test_runner.TestRunnerErrorType_OTHER,
			Err:  fmt.Errorf("unable to locate dockerKeyFile during initialization: %w", err),
		}
	}
	// containerCfg only exists to support VM flow.
	// If we containerize the DutTopology fetching/parsing
	// then VM could use its own logic for fetching DutTopology
	// and this can go away.
	containerCfg := configs.NewContainerConfig(ctr, containerImagesMap, false)
	executorCfg := configs.NewExecutorConfig(ctr, containerCfg)
	cmdCfg := configs.NewCommandConfig(executorCfg)

	// Create state keeper
	gcsurl := common.GetGcsURL(gsRoot)
	sk := data.NewHwTestStateKeeper()
	sk.BuildState = buildState
	sk.CrosTestRunnerRequest = req
	sk.CommonConfig = commonConfig
	sk.CftTestRequest = cft
	sk.Ctr = ctr
	sk.DockerKeyFileLocation = dockerKeyFile
	sk.GcsPublishSrcDir = os.Getenv("TEMPDIR")
	sk.CpconPublishSrcDir = os.Getenv("TEMPDIR")
	sk.RdbPublishSrcDir = os.Getenv("TEMPDIR")
	sk.GcsURL = gcsurl
	sk.TesthausURL = common.GetTesthausURL(invocationName, gcsurl)
	sk.ContainerImages = containerImagesMap
	sk.PrimaryDutModel = req.GetParams().GetPrimaryDut()
	sk.CompanionDutModels = req.GetParams().GetCompanionDuts()
	sk.HostIp, _ = common.GetHostIp()
	sk.IsAlRun = isAlRun

	common.LogWarningIfErr(ctx, sk.Injectables.Set("req", req))
	common.LogWarningIfErr(ctx, sk.Injectables.Set("botDims", buildState.Build().GetInfra().GetSwarming().GetBotDimensions()))
	common.LogWarningIfErr(ctx, sk.Injectables.Set("gcs-url", gcsurl))
	common.LogWarningIfErr(ctx, sk.Injectables.Set("testhaus-url", common.GetTesthausURL(invocationName, gcsurl)))
	common.LogWarningIfErr(ctx, sk.Injectables.Set("host-ip", sk.HostIp))
	if partnerInfo := commonConfig.GetPartnerConfig(); partnerInfo != nil {
		if accountId := partnerInfo.GetAccountId(); accountId > 0 {
			common.LogWarningIfErr(ctx, sk.Injectables.Set("account-id", fmt.Sprint(accountId)))
		}
	}
	parentBBID, err := getParentBBID(buildState.Build())
	if err != nil {
		logging.Infof(ctx, fmt.Sprintf("Warning: %s", err))
	}
	common.LogWarningIfErr(ctx, sk.Injectables.Set("parentBBID", parentBBID))
	common.LogWarningIfErr(ctx, sk.Injectables.Set("schedulingMetadata", getSchedulingMetadata(buildState.Build())))

	// Set the ATP information if provided in the build tags
	invocationID, attemptWUID := getATPIDs(buildState.Build(), ioProps.GetInput(ctx))
	if invocationID != "" {
		common.LogWarningIfErr(ctx, sk.Injectables.Set("ants_invocation_id", invocationID))
	}
	if attemptWUID != "" {
		common.LogWarningIfErr(ctx, sk.Injectables.Set("parent_work_unit_id", attemptWUID))
	}

	env := common.GetCTPEnvironment(buildState.Build().GetBuilder()).String()
	if env != "" {
		common.LogWarningIfErr(ctx, sk.Injectables.Set("env", "ENV_"+env))
	}

	populateRequestQueues(sk, req)

	// Generate config
	hwTestConfig := configs.NewTrv2ExecutionConfig(configs.HwTestExecutionConfigType, cmdCfg, sk, &api_common.CftStepsConfig{})
	err = hwTestConfig.GenerateConfig(ctx)
	if err != nil {
		return sk.SkylabResult, &common.TestRunnerError{
			Type: skylab_test_runner.TestRunnerErrorType_TEST_HARNESS,
			Err:  errors.Annotate(err, "error during generating hw test configs").Err(),
		}
	}

	// Execute config
	err = hwTestConfig.Execute(ctx)
	// For debugging purposes, logs the final state of the Injectables
	// Storage to the top level of the buildState.
	sk.Injectables.LogStorageToBuild(ctx, buildState)
	if err != nil {
		return sk.SkylabResult, errors.Annotate(err, "error during executing hw test configs: ").Err()
	}
	return sk.SkylabResult, nil
}

// populateRequestQueues parses through the OrderedTasks of a CrosTestRunnerRequest
// to populate corresponding queues of requests.
func populateRequestQueues(sk *data.HwTestStateKeeper, req *api.CrosTestRunnerDynamicRequest) {
	if req != nil {
		for _, taskRequest := range req.OrderedTasks {
			for _, containerRequest := range taskRequest.OrderedContainerRequests {
				sk.ContainerQueue.PushBack(containerRequest)
			}

			switch typedRequest := taskRequest.Task.(type) {
			case *api.CrosTestRunnerDynamicRequest_Task_Provision:
				sk.ProvisionQueue.PushBack(typedRequest.Provision)
			case *api.CrosTestRunnerDynamicRequest_Task_PreTest:
				sk.PreTestQueue.PushBack(typedRequest.PreTest)
			case *api.CrosTestRunnerDynamicRequest_Task_Test:
				sk.TestQueue.PushBack(typedRequest.Test)
			case *api.CrosTestRunnerDynamicRequest_Task_PostTest:
				sk.PostTestQueue.PushBack(typedRequest.PostTest)
			case *api.CrosTestRunnerDynamicRequest_Task_Publish:
				sk.PublishQueue.PushBack(typedRequest.Publish)
			default:
			}
		}
	}
}

// validateHwExecution ensures values are set for HW.
func validateHwExecution(ctrCipdVersion, gsRoot string) error {
	if ctrCipdVersion == "" {
		return fmt.Errorf("Cros-tool-runner cipd version cannot be empty for hw test execution.")
	}
	if gsRoot == "" {
		return fmt.Errorf("GS root cannot be empty for hw test execution.")
	}

	return nil
}

// setupCtr creates an instance for CrosToolRunner.
func setupCtr(ctrCipdVersion string) *crostoolrunner.CrosToolRunner {
	ctrCipdInfo := crostoolrunner.CtrCipdInfo{
		Version:        ctrCipdVersion,
		CtrCipdPackage: common.CtrCipdPackage,
	}

	return &crostoolrunner.CrosToolRunner{
		CtrCipdInfo:       ctrCipdInfo,
		EnvVarsToPreserve: common.DockerEnvVarsToPreserve(),
	}
}

// validateDeadline ensures the request's deadline has not passed
// before beginning execution. If it is past, mark as a failed step
// and report the error.
func validateDeadline(ctx context.Context, deadline *timestamppb.Timestamp) error {
	if deadline == nil || deadline.AsTime().After(timestamppb.Now().AsTime()) {
		return nil
	}

	timeSinceDeadline := timestamppb.Now().AsTime().Sub(deadline.AsTime())
	err := fmt.Errorf("deadline exceeded: %s passed since deadline", timeSinceDeadline.String())
	step, _ := build.StartStep(ctx, "Deadline Exceeded")
	defer func() { step.End(err) }()

	common.GlobalNonInfraError = err

	return err
}

// getSchedulingMetadata gets all the build scheduling metadata from the Buildbucket build tags
func getSchedulingMetadata(b *buildbucketpb.Build) *artifactpb.SchedulingMetadata {
	buildTags := map[string]string{}
	for _, tag := range b.GetTags() {
		buildTags[tag.GetKey()] = tag.Value
	}
	schedulingMetadata := &artifactpb.SchedulingMetadata{
		SchedulingArgs: buildTags,
	}
	return schedulingMetadata
}

// getParentBBID gets the parent build ID from the given Buildbucket build.
func getParentBBID(b *buildbucketpb.Build) (int64, error) {
	ts := b.GetTags()
	for _, t := range ts {
		if t.GetKey() != "parent_buildbucket_id" {
			continue
		}
		parentBBID, err := strconv.ParseInt(t.GetValue(), 10, 64)
		if err != nil {
			return 0, errors.Annotate(err, "converting parent_buildbucket_id %s to int64", t.GetValue()).Err()
		}
		return parentBBID, nil
	}
	return 0, fmt.Errorf("no parent BBID found in build tags: %v", ts)
}

func setMarkdown(skylabResult *skylab_test_runner.Result, st *build.State, resp *steps.RunTestsResponse) {
	if skylabResult.GetAutotestResult() != nil {
		// Currently test_runner.py handles this. Don't break it for now.
		return
	} else if skylabResult.GetAndroidGenericResult() != nil {
		if len(skylabResult.GetPrejob().GetStep()) > 0 {
			if skylabResult.GetPrejob().GetStep()[0].GetVerdict() != skylab_test_runner.Result_Prejob_Step_VERDICT_PASS {
				err := fmt.Errorf("prejob failed")
				st.SetSummaryMarkdown(err.Error())
				resp.ErrorSummaryMarkdown = err.Error()
				return
			}
		}

		// If prejob goes well, check the tests.
		tc := skylabResult.GetAndroidGenericResult().GetGivenTestCases()
		for _, t := range tc {
			children := t.GetChildTestCases()
			for _, child := range children {
				v := child.GetVerdict()
				// for now, if a test is undefined, or passed, we won't blow up the builder. Otherwise, do.
				if v != skylab_test_runner.Result_Autotest_TestCase_VERDICT_UNDEFINED && v != skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS {
					err := fmt.Errorf("test(s) failed")
					st.SetSummaryMarkdown(err.Error())
					resp.ErrorSummaryMarkdown = err.Error()
					return
				}
			}

		}
	}
}

func handleAnalytics(ctx context.Context, st *build.State, metrics **analytics.TrInstanceBqData, startTime time.Time, err *error, skylabResult **skylab_test_runner.Result) {
	for _, tag := range st.Build().GetTags() {
		switch tag.GetKey() {
		case "suite", "label-suite":
			(*metrics).SuiteName = tag.GetValue()
		case "analytics_name":
			(*metrics).AnalyticsName = tag.GetValue()
		case "label-pool", "pool":
			(*metrics).Pool = tag.GetValue()
		}
	}
	(*metrics).Date = civil.DateTimeOf(startTime)
	(*metrics).Duration = time.Since(startTime).Seconds()
	(*metrics).Status, (*metrics).Summary = determineStatusAndSummary(*skylabResult, *err)

	bqClient := analytics.TrAnalyticsBQClient(ctx)
	analytics.SoftInsertTrInstanceMetrics(ctx, bqClient, (*metrics))
}

func determineStatusAndSummary(skylabResult *skylab_test_runner.Result, err error) (status, summary string) {
	status = "SUCCESS"

	if skylabResult != nil {
		for _, prejobStep := range skylabResult.GetPrejob().GetStep() {
			if prejobStep.GetVerdict() != skylab_test_runner.Result_Prejob_Step_VERDICT_PASS {
				status = "FAILURE"
				summary = fmt.Sprintf("prejob %s failed: %s", prejobStep.Name, prejobStep.HumanReadableSummary)
				return
			}
		}

		for _, testCase := range skylabResult.GetAutotestResult().GetTestCases() {
			if testCase.GetVerdict() == skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS {
				continue
			}
			status = "FAILURE"
			summary = "test failed"
		}
	}

	if err != nil {
		status = "FAILURE"
		summary = err.Error()
		if common.GlobalNonInfraError == nil {
			status = "INFRA_ERROR"
		}
		return
	}

	return
}
