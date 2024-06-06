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
	"sync"

	"cloud.google.com/go/bigquery"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/steps"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/analytics"
	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/cmd/cros_test_runner/protos"
	"infra/cros/cmd/ctpv2/data"
	"infra/cros/cmd/ctpv2/internal/configs"

	"go.chromium.org/chromiumos/infra/proto/go/test_platform/config"
)

// TODO : Re-structure different execution flow properly later.
// LuciBuildExecution represents build executions.
func LuciBuildExecution() {
	// Set input property reader functions
	var ctrCipdInfoReader func(context.Context) *protos.CipdVersionInfo
	build.MakePropertyReader(common.HwTestCtrInputPropertyName, &ctrCipdInfoReader)
	input := &steps.CTPv2BinaryBuildInput{}

	// Set output props writer functions
	// TODO: add the fields to the response that is responsible for
	// feeding the test results to upstream.
	var writeOutputProps func(*steps.CTPv2BinaryBuildOutput)
	var mergeOutputProps func(*steps.CTPv2BinaryBuildOutput)

	build.Main(input, &writeOutputProps, &mergeOutputProps,
		func(ctx context.Context, args []string, st *build.State) error {
			log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmsgprefix)
			logging.Infof(ctx, "have input %v", input)
			ctrCipdInfo := ctrCipdInfoReader(ctx)
			bqClient := analytics.CtpAnalyticsBQClient(ctx)
			if bqClient != nil {
				defer bqClient.Close()
			}
			logging.Infof(ctx, "have ctr info: %v", ctrCipdInfo)
			logging.Infof(ctx, "ctr label: %s", ctrCipdInfo.GetVersion().GetCipdLabel())
			resp, err := executeRequests(ctx, input, ctrCipdInfo.GetVersion().GetCipdLabel(), st, bqClient)
			if err != nil {
				logging.Infof(ctx, "error found: %s", err)
				st.SetSummaryMarkdown(err.Error())
				resp.ErrorSummaryMarkdown = err.Error()
			}

			writeOutputProps(resp)
			return err
		},
	)
}

func executeRequests(
	ctx context.Context,
	input *steps.CTPv2BinaryBuildInput,
	ctrCipdVersion string,
	buildState *build.State,
	BQClient *bigquery.Client) (*steps.CTPv2BinaryBuildOutput, error) {
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
	resultsMap := executeCtpv2Reqs(ctx, keyReqMap, input.Config, buildState, ctr, BQClient)
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
	keyRequestMap map[string]*api.CTPRequest, config *config.Config, buildState *build.State, ctr *crostoolrunner.CrosToolRunner, BQClient *bigquery.Client) map[string][]*data.TestResults {
	resultsMap := map[string][]*data.TestResults{}
	var err error
	step, ctx := build.StartStep(ctx, "Suite Executions (async)")
	defer func() { step.End(err) }()

	resultsChan := make(chan map[string][]*data.TestResults)
	wg := &sync.WaitGroup{}
	contInfoMap := data.NewContainerInfoMap()
	suiteCounter := map[string]int{}
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
		go executeFiltersInLuciBuild(ctx, ctpReq, config, buildState, wg, ctr, contInfoMap, resultsChan, suiteDisplayName, BQClient, key)
	}
	go func() {
		wg.Wait()
		close(resultsChan) // Close the channel when all workers are done
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
	wg *sync.WaitGroup, ctr *crostoolrunner.CrosToolRunner, contInfoMap *data.ContainerInfoMap, results chan<- map[string][]*data.TestResults, suiteDisplayName string, BQClient *bigquery.Client, reqKey string) error {
	defer wg.Done()
	var err error
	step, ctx := build.StartStep(ctx, suiteDisplayName)
	defer func() { step.End(err) }()

	executorCfg := configs.NewExecutorConfig(ctr, nil)
	cmdCfg := configs.NewCommandConfig(executorCfg)

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
	}

	nFilters := getTotalFilters(ctx, req, common.MakeDefaultFilters(ctx, req.GetSuiteRequest()), common.DefaultKoffeeFilterNames)
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
