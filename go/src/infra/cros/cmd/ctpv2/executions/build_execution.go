// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executions

import (
	"bytes"
	"compress/zlib"
	"container/list"
	"context"
	"encoding/base64"
	"fmt"
	"log"
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

	"infra/cros/cmd/common_lib/analytics"
	androidapi "infra/cros/cmd/common_lib/android_api"
	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/cmd/common_lib/tools/outputprops"
	"infra/cros/cmd/common_lib/tools/suitelimits"
	"infra/cros/cmd/cros_test_runner/protos"
	"infra/cros/cmd/ctpv2/data"
	"infra/cros/cmd/ctpv2/internal/configs"
)

var inputProps = build.RegisterInputProperty[*steps.CTPv2BinaryBuildInput]("")
var outputProps = build.RegisterOutputProperty[map[string]string]("ctpv2/sub-build")

var ctrInputVersion = build.RegisterInputProperty[*protos.CipdVersionInfo](common.HwTestCtrInputPropertyName)
var ctpv2InputVersion = build.RegisterInputProperty[*protos.CipdVersionInfo](common.HwTestCtpv2InputPropertyName)

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
		return nil, fmt.Errorf("Cros-tool-runner cipd version cannot be empty for hw test execution.")
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

	dockerKeyFile, err := common.LocateFile([]string{common.LabDockerKeyFileLocation, common.VmLabDockerKeyFileLocation})
	if err != nil {
		return nil, fmt.Errorf("unable to locate dockerKeyFile during initialization: %w", err)
	}

	executorCfg := configs.NewExecutorConfig(ctr, nil)
	cmdCfg := configs.NewCommandConfig(executorCfg)

	sk := &data.PrePostFilterStateKeeper{
		DockerKeyFileLocation: dockerKeyFile,
		Ctr:                   ctr,
		CtpV1Requests:         input.GetRequests(),
		CtpV2Request:          input.GetCtpv2Request(),
		BQClient:              BQClient,
		BuildState:            buildState,
		IsPartnerRun:          isPartnerRun,
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

	var workUnitTrees map[string]*androidapi.WorkUnitTree
	var generateInvocation, workUnitsOnly bool
	if sk.AlStateInfo != nil {
		if sk.AlStateInfo.WorkUnitTrees != nil {
			workUnitTrees = sk.AlStateInfo.WorkUnitTrees
		}

		generateInvocation = sk.AlStateInfo.GenerateInvocation
		workUnitsOnly = sk.AlStateInfo.WorkUnitsOnly
	}

	resultsMap := executeCtpv2Reqs(ctx, keyReqMap, input.Config, buildState, ctr, BQClient, ctpv2CipdVersion, workUnitTrees, isPartnerRun, generateInvocation, workUnitsOnly)
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
	keyRequestMap map[string]*api.CTPRequest, config *config.Config, buildState *build.State, ctr *crostoolrunner.CrosToolRunner, BQClient *bigquery.Client, ctpVersion string, workUnitTrees map[string]*androidapi.WorkUnitTree, isPartnerRun, generateInvocation, workUnitsOnly bool) map[string][]*data.TestResults {
	resultsMap := map[string][]*data.TestResults{}
	var err error
	step, ctx := build.StartStep(ctx, "Suite Executions (async)")
	defer func() { step.End(err) }()

	resultsChan := make(chan map[string][]*data.TestResults)
	wg := &sync.WaitGroup{}
	contInfoMap := data.NewContainerInfoMap()
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
		go executeFiltersInLuciBuild(ctx, ctpReq, config, buildState, wg, ctr, contInfoMap, resultsChan, suiteDisplayName, BQClient, key, ctpVersion, workUnitTrees, isPartnerRun, generateInvocation, workUnitsOnly)
	}
	go func() {
		wg.Wait()
		close(resultsChan) // Close the channel when all workers are done

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
	common.WriteAnyObjectToStepLog(ctx, step, resultsMap, "consolidated suite results")
	return resultsMap
}

func executeFiltersInLuciBuild(
	ctx context.Context,
	req *api.CTPRequest,
	config *config.Config,
	buildState *build.State,
	wg *sync.WaitGroup, ctr *crostoolrunner.CrosToolRunner, contInfoMap *data.ContainerInfoMap, results chan<- map[string][]*data.TestResults, suiteDisplayName string, BQClient *bigquery.Client, reqKey, ctpVersion string, workUnitTrees map[string]*androidapi.WorkUnitTree, isPartnerRun, generateInvocation, workUnitsOnly bool) error {
	defer wg.Done()
	var err error
	step, ctx := build.StartStep(ctx, suiteDisplayName)
	defer func() { step.End(err) }()

	dockerKeyFile, err := common.LocateFile([]string{common.LabDockerKeyFileLocation, common.VmLabDockerKeyFileLocation})
	if err != nil {
		err = fmt.Errorf("unable to locate dockerKeyFile during initialization: %w", err)
		return err
	}

	executorCfg := configs.NewExecutorConfig(ctr, nil)
	cmdCfg := configs.NewCommandConfig(executorCfg)

	alStateInfo := &data.AlStateInfo{}

	if isReqFromATP(req) {
		buildIdStr := strconv.FormatInt(buildState.Build().Id, 10)
		// TODO (azrahman:atp): infer this from the new test job field; create a deep copy for current state
		inputTestJobMsg, err := common.DecodeTestJobMsg(ctx, req.GetEncodedAtpTestJobMsg())
		if err != nil {
			inputTestJobMsg = &common.TestJobMessage{Id: buildIdStr, Runner: "CTP", TestJobState: "QUEUED", StartTimestamp: buildState.Build().CreateTime.AsTime().Format(common.ATPSupportedTimeFormat)}
		} else {
			inputTestJobMsg.TestJobState = "QUEUED"
			inputTestJobMsg.StartTimestamp = buildState.Build().CreateTime.AsTime().Format(common.ATPSupportedTimeFormat)
		}

		testJobEventState := &common.TestJobEventMessage{TestJobId: buildIdStr, TestJob: inputTestJobMsg, State: "QUEUED", Type: "STATE_CHANGED"}
		// create pubsub client
		atpProjectId := common.GetAtpProjectID(buildState.Build().GetBuilder())
		logging.Infof(ctx, "ATP project id that will be used for the run: %s", atpProjectId)
		client, err := pubsub.NewClient(ctx, atpProjectId)
		if err != nil {
			return fmt.Errorf("Failed to create client for %s: %v", common.ATPSwitcherProjectIDProd, err)
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

	sk := &data.FilterStateKeeper{
		CtpReq:             req,
		Ctr:                ctr,
		ContainerInfoQueue: list.New(),
		BuildState:         buildState,
		Scheduler:          req.GetSchedulerInfo().GetScheduler(),
		ContainerInfoMap:   contInfoMap,
		BQClient:           BQClient,
		Config:             config,
		RequestKey:         reqKey,
		DockerKeyFile:      dockerKeyFile,
		CTPversion:         ctpVersion,
		Environment:        common.GetCTPEnvironment(buildState.Build().GetBuilder()).String(),
		AlStateInfo:        alStateInfo,
		IsAlRun:            req.IsAlRun,
		IsPartnerRun:       isPartnerRun,
	}

	fillInUserDefinedFilters(ctx, req, dockerKeyFile, ctpVersion, isPartnerRun)
	nFilters := getTotalFilters(ctx, req, common.MakeDefaultFilters(ctx, req.GetSuiteRequest(), buildState.Build().Input.Experiments), common.DefaultKoffeeFilterNames)
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

	return err
}

func getTotalFilters(ctx context.Context, req *api.CTPRequest, defaultKarbonFilterNames []string, defaultKoffeeFilterNames []string) int {
	filterSet := map[string]bool{}
	logging.Infof(ctx, "n defaultKarbonFilterNames: %s", len(defaultKarbonFilterNames))
	logging.Infof(ctx, "Given Karbon: %s And Koffee: %s", defaultKarbonFilterNames, defaultKoffeeFilterNames)

	for _, filterName := range defaultKarbonFilterNames {
		filterSet[filterName] = true
	}

	for _, filterName := range defaultKoffeeFilterNames {
		filterSet[filterName] = true
	}

	for _, filter := range req.GetKarbonFilters() {
		filterSet[filter.GetContainerInfo().GetContainer().GetName()] = true
	}

	for _, filter := range req.GetKoffeeFilters() {
		filterSet[filter.GetContainerInfo().GetContainer().GetName()] = true
	}

	return len(filterSet)
}

func isReqFromATP(req *api.CTPRequest) bool {
	return req.IsAlRun && req.EncodedAtpTestJobMsg != ""
}

func fillInUserDefinedFilters(ctx context.Context, req *api.CTPRequest, creds, ctpVersion string, isPartnerRun bool) {
	updatedFilters := []*api.CTPFilter{}
	for _, filter := range req.GetKarbonFilters() {
		container := filter.GetContainerInfo().GetContainer()
		filterName := container.GetName()
		// Overwrite container image info if present within filter input.
		// Else, check if map already contains image info for the filter.
		if hasValidDigest(container.Digest) || len(container.Tags) > 0 {
			updatedFilters = append(updatedFilters, filter)
			continue
		}

		fireStoreDB := common.TestPlatformFireStore
		if isPartnerRun {
			fireStoreDB = common.PartnerTestPlatformFireStore
		}
		// Fetch the filter from the firestore.
		if containerInfo, err := common.FetchContainerInfoFromFirestore(ctx, fireStoreDB, creds, ctpVersion, filterName); err == nil && containerInfo != nil {
			logging.Infof(ctx, "Found filter inside the firestore for %s", filterName)
			if containerInfo.GetContainer().GetName() == "" {
				containerInfo.Container.Name = filterName
			}
			containerInfo.BinaryName = filter.GetContainerInfo().GetBinaryName()
			containerInfo.BinaryArgs = filter.GetContainerInfo().GetBinaryArgs()
			updatedFilters = append(updatedFilters, &api.CTPFilter{
				ContainerInfo: containerInfo,
			})
			continue
		}
	}

	req.KarbonFilters = updatedFilters
}

// hasValidDigest ensures the digest starts with `sha256:` and
// the SHA value itself is 64 characters in length.
func hasValidDigest(digest string) bool {
	return strings.HasPrefix(digest, "sha256:") && len(digest) == len("sha256:")+64
}
