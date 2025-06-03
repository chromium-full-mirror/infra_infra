// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package executions stores the infrastructure to run the CTPv2 builder.
package executions

import (
	"bytes"
	"compress/zlib"
	"container/list"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/pubsub"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/config"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/steps"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/analytics"
	androidapi "go.chromium.org/infra/cros/cmd/common_lib/android_api"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/commonbuilders"
	"go.chromium.org/infra/cros/cmd/common_lib/commontypes"
	"go.chromium.org/infra/cros/cmd/common_lib/tools/crostoolrunner"
	"go.chromium.org/infra/cros/cmd/common_lib/tools/outputprops"
	"go.chromium.org/infra/cros/cmd/common_lib/tools/suitelimits"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/protos"
	"go.chromium.org/infra/cros/cmd/ctpv2/data"
	"go.chromium.org/infra/cros/cmd/ctpv2/internal/configs"
)

var inputProps = build.RegisterInputProperty[*steps.CTPv2BinaryBuildInput]("")
var outputProps = build.RegisterOutputProperty[map[string]string]("ctpv2/sub-build")

var ctrInputVersion = build.RegisterInputProperty[*protos.CipdVersionInfo](common.HwTestCtrInputPropertyName)
var ctpv2InputVersion = build.RegisterInputProperty[*protos.CipdVersionInfo](common.HwTestCtpv2InputPropertyName)
var ctpv2WithFifoProps = build.RegisterInputProperty[map[string][]string](common.Ctpv2WithFifoPropertyName)

// LuciBuildExecution represents build executions.
//
// TODO : Re-structure different execution flow properly later.
func LuciBuildExecution() {
	build.Main(
		func(ctx context.Context, args []string, st *build.State) error {
			input := inputProps.GetInput(ctx)

			log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmsgprefix)
			logging.Infof(ctx, "have input %v", input)
			ctrCipdInfo := ctrInputVersion.GetInput(ctx)
			ctpv2CipdInfo := ctpv2InputVersion.GetInput(ctx)
			outputprops.CTPv2AtpUpdate.SetOutput(ctx, nil)
			logging.Infof(ctx, "ctpv2 label: %s", ctpv2CipdInfo.GetVersion().GetCipdLabel())
			logging.Infof(ctx, "partner_config: %v", input.PartnerConfig)
			bqClient := analytics.CtpAnalyticsBQClient(ctx)
			if bqClient != nil {
				defer bqClient.Close()
			}

			// Fetch the scheduke disallow list and place it in the "read only"
			// variable in the common package in common_lib.
			Ctpv2WithFifoList := ctpv2WithFifoProps.GetInput(ctx)

			// Ensure that the property has the correct key.
			list, ok := Ctpv2WithFifoList[common.Ctpv2WithFifoMapKey]
			if !ok {
				return fmt.Errorf("no Ctpv2WithFifo list was provided")
			}

			err := common.SetCtpv2WithFifoList(list)
			if err != nil {
				return err
			}
			logging.Infof(ctx, "schedukeDisallowList: %+v", common.GetCtpv2WithFifoList())

			logging.Infof(ctx, "have ctr info: %v", ctrCipdInfo)
			logging.Infof(ctx, "ctr label: %s", ctrCipdInfo.GetVersion().GetCipdLabel())
			resp, err := executeRequests(ctx, input, ctrCipdInfo.GetVersion().GetCipdLabel(), st, bqClient, ctpv2CipdInfo.GetVersion().GetCipdLabel(), input.GetPartnerConfig().GetValue())
			if err != nil {
				logging.Infof(ctx, "error found: %s", err)
				st.SetSummaryMarkdown(err.Error())
				resp.ErrorSummaryMarkdown = err.Error()
			}

			outputProps.SetOutput(ctx, map[string]string{"compressed_responses": resp.CompressedResponses})
			return err
		},
	)
}

func executeRequests(
	ctx context.Context,
	input *steps.CTPv2BinaryBuildInput,
	ctrCipdVersion string,
	buildState *build.State,
	BQClient *bigquery.Client,
	ctpv2CipdVersion string,
	isPartnerRun bool) (*steps.CTPv2BinaryBuildOutput, error) {
	buildOutput := &steps.CTPv2BinaryBuildOutput{}

	// Validation
	if ctrCipdVersion == "" {
		return nil, fmt.Errorf("cros-tool-runner cipd version cannot be empty for hw test execution")
	}
	// Create ctr
	ctrCipdInfo := crostoolrunner.CtrCipdInfo{
		Version:        ctrCipdVersion,
		CtrCipdPackage: common.CtrCipdPackage,
	}

	ctr := &crostoolrunner.CrosToolRunner{
		CtrCipdInfo:       ctrCipdInfo,
		EnvVarsToPreserve: []string{},
	}

	dockerKeyFile, err := common.LocateFile([]string{common.LabDockerKeyFileLocation, common.VMLabDockerKeyFileLocation})
	if err != nil {
		return nil, fmt.Errorf("unable to locate dockerKeyFile during initialization: %w", err)
	}

	executorCfg := configs.NewExecutorConfig(ctr, nil)
	cmdCfg := configs.NewCommandConfig(executorCfg)

	sk := &data.PrePostFilterStateKeeper{
		DockerKeyFileLocation:   dockerKeyFile,
		Ctr:                     ctr,
		CtpV1Requests:           input.GetRequests(),
		CtpV2Request:            input.GetCtpv2Request(),
		BQClient:                BQClient,
		BuildState:              buildState,
		IsPartnerRun:            isPartnerRun,
		ContainerRequestChannel: make(chan commontypes.ContainerManagementRequest),
	}

	ctpv2PreConfig := configs.NewCtpv2ExecutionConfig(0, configs.Ctpv2PreExecutionConfigType, cmdCfg, sk)
	err = ctpv2PreConfig.GenerateConfig(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "error during generating pre execution configs: ").Err()
	}

	ctpv2PostConfig := configs.NewCtpv2ExecutionConfig(0, configs.Ctpv2PostExecutionConfigType, cmdCfg, sk)
	err = ctpv2PostConfig.GenerateConfig(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "error during generating post execution configs: ").Err()
	}

	// Execute pre configs
	err = ctpv2PreConfig.Execute(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "error during executing pre execution configs: ").Err()
	}

	// Execute Ctpv2 Reqs
	// Check if direct ctpv2Request was provided and translate it into a map
	keyReqMap := map[string]*api.CTPRequest{}
	if sk.CtpV2Request != nil {
		for i, req := range sk.CtpV2Request.GetRequests() {
			keyReqMap[string(rune(i))] = req
		}
	} else {
		keyReqMap = sk.V1KeyToCTPv2Req
	}

	// var workUnitTrees map[string]*androidapi.WorkUnitTree
	var top *androidapi.WorkUnitNode
	var generateInvocation, workUnitsOnly bool
	if sk.AlStateInfo != nil {
		if sk.AlStateInfo.Top != nil {
			top = sk.AlStateInfo.Top
		}

		generateInvocation = sk.AlStateInfo.GenerateInvocation
		workUnitsOnly = sk.AlStateInfo.WorkUnitsOnly
	}

	resultsMap := executeCtpv2Reqs(ctx, keyReqMap, input.Config, buildState, ctr, BQClient, ctpv2CipdVersion, top, isPartnerRun, generateInvocation, workUnitsOnly, sk.ContainerRequestChannel)
	sk.AllTestResults = resultsMap

	// Execute post configs
	err = ctpv2PostConfig.Execute(ctx)
	if err != nil {
		return buildOutput, errors.Annotate(err, "error during executing post execution configs: ").Err()
	}

	if sk.ExecuteResponses != nil {
		m, _ := proto.Marshal(sk.ExecuteResponses)
		var b bytes.Buffer
		w := zlib.NewWriter(&b)
		_, _ = w.Write(m)
		_ = w.Close()

		buildOutput.CompressedResponses = base64.StdEncoding.EncodeToString(b.Bytes())
	}

	return buildOutput, nil
}

func executeCtpv2Reqs(ctx context.Context,
	keyRequestMap map[string]*api.CTPRequest, config *config.Config, buildState *build.State, ctr *crostoolrunner.CrosToolRunner, BQClient *bigquery.Client, ctpVersion string, top *androidapi.WorkUnitNode, isPartnerRun, generateInvocation, workUnitsOnly bool, containerRequestChannel chan commontypes.ContainerManagementRequest) map[string][]*data.TestResults {
	resultsMap := map[string][]*data.TestResults{}
	var err error
	step, ctx := build.StartStep(ctx, "Suite Executions (async)")

	defer func() { step.End(err) }()

	resultsChan := make(chan map[string][]*data.TestResults)
	suiteExecutionSummaryChan := make(chan map[string]string, len(keyRequestMap))
	wg := &sync.WaitGroup{}
	suiteCounter := map[string]int{}

	// Begin the metrics logging collection system for SuiteLimits.
	go suitelimits.LogMetrics(ctx)

	for key, ctpReq := range keyRequestMap {
		suiteName := ctpReq.GetSuiteRequest().GetTestSuite().GetName()
		suiteDisplayName := suiteName
		if _, ok := suiteCounter[suiteName]; !ok {
			suiteCounter[suiteName] = 0
		} else {
			// we have seen this suite before
			suiteCounter[suiteName]++
			suiteNum := suiteCounter[suiteName]
			suiteDisplayName = fmt.Sprintf("%s_%d", suiteName, suiteNum)
		}
		wg.Add(1)
		go executeFiltersInLuciBuild(ctx, ctpReq, config, buildState, wg, ctr, resultsChan, suiteDisplayName, BQClient, key, ctpVersion, top, isPartnerRun, generateInvocation, workUnitsOnly, suiteExecutionSummaryChan, containerRequestChannel)
	}
	go func() {
		wg.Wait()
		close(resultsChan) // Close the channel when all workers are done
		close(suiteExecutionSummaryChan)

		// Flush the SL metrics results.
		suitelimits.CloseMetricChan()
	}()

	// Read results
	for suiteResultsMap := range resultsChan {
		if len(suiteResultsMap) == 0 {
			continue
		}
		for k, v := range suiteResultsMap {
			resultsMap[k] = v
		}
	}
	// safely collect AI failure(if any) summary for all suites
	aiSummaryMap := make(map[string]string)
	for suiteExecutionSummaryMap := range suiteExecutionSummaryChan {
		if len(suiteExecutionSummaryMap) == 0 {
			continue
		}
		for k, v := range suiteExecutionSummaryMap {
			aiSummaryMap[k] = v
		}
	}

	common.WriteAnyObjectToStepLog(ctx, step, resultsMap, "consolidated suite results")
	common.WriteAnyObjectToStepLog(ctx, step, aiSummaryMap, "AI suites summary")
	return resultsMap
}

func executeFiltersInLuciBuild(
	ctx context.Context,
	req *api.CTPRequest,
	config *config.Config,
	buildState *build.State,
	wg *sync.WaitGroup,
	ctr *crostoolrunner.CrosToolRunner,
	results chan<- map[string][]*data.TestResults,
	suiteDisplayName string,
	BQClient *bigquery.Client,
	reqKey,
	ctpVersion string,
	top *androidapi.WorkUnitNode,
	isPartnerRun,
	generateInvocation,
	workUnitsOnly bool,
	suiteExecutionSummary chan<- map[string]string,
	containerRequestChannel chan commontypes.ContainerManagementRequest) {
	defer wg.Done()
	var err error
	step, ctx := build.StartStep(ctx, suiteDisplayName)
	defer func() { step.End(err) }()

	dockerKeyFile, err := common.LocateFile([]string{common.LabDockerKeyFileLocation, common.VMLabDockerKeyFileLocation})
	if err != nil {
		err = fmt.Errorf("unable to locate dockerKeyFile during initialization: %w", err)
		logging.Errorf(ctx, "executeFiltersInLuciBuild: %w", err)
		suiteExecutionSummary <- map[string]string{suiteDisplayName: "unable to locate dockerKeyFile during initialization"}
		return
	}

	executorCfg := configs.NewExecutorConfig(ctr, nil)
	cmdCfg := configs.NewCommandConfig(executorCfg)

	var alStateInfo *data.AlStateInfo

	workUnitTrees := make(map[string]*androidapi.WorkUnitTree)
	if top != nil {
		workUnitTrees["test"] = &androidapi.WorkUnitTree{
			Head:        top,
			ShardsByKey: map[string]*androidapi.WorkUnitNode{},
		}
	} else if generateInvocation {
		workUnitTrees["test"] = &androidapi.WorkUnitTree{}
	}

	if isReqFromATP(req) {
		buildIDStr := strconv.FormatInt(buildState.Build().Id, 10)
		// TODO (azrahman:atp): infer this from the new test job field; create a deep copy for current state
		inputTestJobMsg, err := common.DecodeTestJobMsg(ctx, req.GetEncodedAtpTestJobMsg())
		if err != nil {
			inputTestJobMsg = &common.TestJobMessage{Id: buildIDStr, Runner: "CTP", TestJobState: "QUEUED", StartTimestamp: buildState.Build().CreateTime.AsTime().Format(common.ATPSupportedTimeFormat), ResultLinks: []string{common.ShortBBUrl(buildState.Build().Id)}}
		} else {
			inputTestJobMsg.TestJobState = "QUEUED"
			inputTestJobMsg.StartTimestamp = buildState.Build().CreateTime.AsTime().Format(common.ATPSupportedTimeFormat)
			inputTestJobMsg.ResultLinks = []string{common.ShortBBUrl(buildState.Build().Id)}
		}

		testJobEventState := &common.TestJobEventMessage{TestJobId: buildIDStr, TestJob: inputTestJobMsg, State: "QUEUED", Type: "STATE_CHANGED"}
		// create pubsub client
		atpProjectID := common.GetAtpProjectID(buildState.Build().GetBuilder())
		logging.Infof(ctx, "ATP project id that will be used for the run: %s", atpProjectID)
		client, err := pubsub.NewClient(ctx, atpProjectID)
		if err != nil {
			err = fmt.Errorf("failed to create client for %s: %w", common.ATPSwitcherProjectIDProd, err)
			logging.Errorf(ctx, "executeFiltersInLuciBuild: %w", err)
			suiteExecutionSummary <- map[string]string{suiteDisplayName: "AL flow - unable to create pubsub client"}
			return
		}
		defer client.Close()
		alStateInfo = &data.AlStateInfo{InputTestJob: inputTestJobMsg, CurrentTestJob: inputTestJobMsg, CurrentTestJobEvent: testJobEventState, TestJobEventPubSubClient: client, WorkUnitTrees: workUnitTrees, GenerateInvocation: generateInvocation}
	} else if req.IsAlRun {
		// This gets hit if we are in an AL run not started by atp. e.g. Kron,
		// crosfleet, LED.
		alStateInfo = &data.AlStateInfo{GenerateInvocation: generateInvocation, WorkUnitsOnly: workUnitsOnly, WorkUnitTrees: workUnitTrees}
	} else {
		alStateInfo = nil
	}

	hasAshChrome := false
	for _, scheduleTarget := range req.GetScheduleTargets() {
		for _, target := range scheduleTarget.GetTargets() {
			for _, kvs := range target.GetSwTarget().GetLegacySw().GetKeyValues() {
				if kvs.Key == commonbuilders.AshChromeGcsPath {
					hasAshChrome = true
				}
			}
		}
	}
	sk := &data.FilterStateKeeper{
		CtpReq:                  req,
		Ctr:                     ctr,
		FiltersQueue:            list.New(),
		BuildState:              buildState,
		Scheduler:               req.GetSchedulerInfo().GetScheduler(),
		BQClient:                BQClient,
		Config:                  config,
		RequestKey:              reqKey,
		DockerKeyFile:           dockerKeyFile,
		CTPversion:              ctpVersion,
		Environment:             common.GetCTPEnvironment(buildState.Build().GetBuilder()).String(),
		AlStateInfo:             alStateInfo,
		IsAlRun:                 req.IsAlRun,
		IsPartnerRun:            isPartnerRun,
		HasAshChrome:            hasAshChrome,
		SuiteTestResults:        map[string]*data.TestResults{},
		ExecutionAIContext:      fmt.Sprintf("%s \n Suite request: %s", common.CTPContext, req),
		ContainerRequestChannel: containerRequestChannel,
		ContainerLogsChannel:    make(chan *commontypes.ContainerLogInfo),
	}

	nFilters := getTotalFilters(ctx, req, common.MakeDefaultFilters(ctx, req.GetSuiteRequest(), buildState.Build().Input.Experiments, isPartnerRun, req.IsAlRun, hasAshChrome))
	logging.Infof(ctx, "nfilters: %s", nFilters)
	// Generate config
	ctpv2Config := configs.NewCtpv2ExecutionConfig(nFilters, configs.LuciBuildFilterExecutionConfigType, cmdCfg, sk)
	err = ctpv2Config.GenerateConfig(ctx)
	if err != nil {
		// Do not return err as we want to collect the testResults
		err = errors.Annotate(err, "error during generating filter configs: ").Err()
		logging.Infof(ctx, err.Error())
	}

	// Execute config
	err = ctpv2Config.Execute(ctx)
	if err != nil {
		// Do not return err as we want to collect the testResults
		err = errors.Annotate(err, "error during executing hw test configs: ").Err()
		logging.Infof(ctx, err.Error())
	}

	resultsList := []*data.TestResults{}
	for _, v := range sk.SuiteTestResults {
		resultsList = append(resultsList, v)
	}

	// Send the result via channel
	results <- map[string][]*data.TestResults{suiteDisplayName: resultsList}

	if err != nil {
		logging.Errorf(ctx, "executeFiltersInLuciBuild: %w", err)
	}
	aiSummary := AISummarizeSuiteExecution(ctx, sk, step, err)
	// Send the result via channel
	suiteExecutionSummary <- map[string]string{suiteDisplayName: aiSummary}
}

func AISummarizeSuiteExecution(ctx context.Context, sk *data.FilterStateKeeper, step *build.Step, err error) string {
	// configsLog := step.Log("AI failure Summary")
	// logging.Infof(ctx, sk.ExecutionAIContext)
	// apiKey, keyErr := fetchGeminiAPIKey(ctx)
	// var aiSummary string
	// var aiErr error
	// if keyErr != nil {
	// 	logging.Infof(ctx, "error during fetching Gemini API key: %s", keyErr)
	// 	return "error during fetching Gemini API key"
	// }
	// if err != nil {
	// 	aiSummary, aiErr = common.AISummarize(ctx, sk.ExecutionAIContext, common.SuiteFailureSummaryPrompt, apiKey, true)
	// 	if aiErr != nil {
	// 		logging.Infof(ctx, "error during AI failure summarization: %s", aiErr)
	// 		return "error during AI failure summarization"
	// 	}
	// 	_, logErr := configsLog.Write([]byte(aiSummary))
	// 	if logErr != nil {
	// 		logging.Infof(ctx, "error during writing aiSummary: %s", logErr)
	// 	}
	// } else {
	// 	aiSummary = "No Infra failures detected."
	// 	_, logErr := configsLog.Write([]byte(aiSummary))
	// 	if logErr != nil {
	// 		logging.Infof(ctx, "error during writing aiSummary: %s", logErr)
	// 	}
	// }
	return "aiSummary disabled"
}

func fetchGeminiAPIKey(ctx context.Context) (string, error) {
	cmd := exec.Command("gcloud", "secrets", "versions", "access", "latest", "--secret="+common.GeminiApiKey, "--project="+common.GeminiApiKeyProject)
	logging.Infof(ctx, "Running command: %s", cmd.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.Infof(ctx, "Error retrieving secret: %v\nOutput: %s\n", err, string(output))
		return "", err
	}

	return string(output), nil
}

func getTotalFilters(ctx context.Context, req *api.CTPRequest, defaultFilters []*api.CTPFilter) int {
	filterSet := map[string]bool{}
	logging.Infof(ctx, "n defaultKarbonFilterNames: %s", len(defaultFilters))

	for _, defaultFilter := range defaultFilters {
		filterName := defaultFilter.GetContainerInfo().GetContainer().GetName()
		filterSet[filterName] = true
	}

	for _, filter := range req.GetKarbonFilters() {
		filterName := filter.GetContainerInfo().GetContainer().GetName()
		if filterName == common.ALTestFinderName {
			filterName = common.TestFinderContainerName
		}
		filterSet[filterName] = true
	}

	return len(filterSet)
}

func isReqFromATP(req *api.CTPRequest) bool {
	return req.IsAlRun && req.EncodedAtpTestJobMsg != ""
}

// hasValidDigest ensures the digest starts with `sha256:` and
// the SHA value itself is 64 characters in length.
func hasValidDigest(digest string) bool {
	return strings.HasPrefix(digest, "sha256:") && len(digest) == len("sha256:")+64
}
