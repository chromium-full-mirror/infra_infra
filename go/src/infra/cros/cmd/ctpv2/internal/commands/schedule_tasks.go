// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/config"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/analytics"
	androidapi "go.chromium.org/infra/cros/cmd/common_lib/android_api"
	"go.chromium.org/infra/cros/cmd/common_lib/ants/androidbuildinternal/v3"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/common_lib/schedulers"
	"go.chromium.org/infra/cros/cmd/common_lib/tools/rdb"
	"go.chromium.org/infra/cros/cmd/common_lib/tools/suitelimits"
	"go.chromium.org/infra/cros/cmd/ctpv2/data"
	dm "go.chromium.org/infra/device_manager/client"
)

const (
	testRunnerGceBuilderName = "test_runner_gce"
	autoVMRun                = "auto_vm_run"
	scheduler                = "scheduler"
)

// getBuildFieldMask is the list of buildbucket fields that are needed.
var getBuildFieldMask = []string{
	"id",
	"infra.backend.task.id.id",
	"infra.swarming.task_id",
	// Build details are parsed from the build's output properties.
	"output.properties",
	// Build status is used to determine whether the build is complete.
	"status",
}

// ScheduleTasksCmd represents scheduling task(s) cmd.
type ScheduleTasksCmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor

	// Deps
	BuildState        *build.State
	Scheduler         interfaces.SchedulerInterface
	directBBScheduler interfaces.SchedulerInterface
	DynamicRun        bool
	CredentialsFile   string
	// EnvVersion denotes whether the environment
	// is prod or something else.
	EnvVersion      string
	FirestoreDBName string

	// Deps
	InternalTestPlan *api.InternalTestplan
	BuildsMap        map[string]*data.BuildRequest
	RequestKey       string
	IsAlRun          bool

	// Updates
	TestResults map[string]*data.TestResults
	AlStateInfo *data.AlStateInfo // will be used as dep as well

	// For logging
	BQClient     *bigquery.Client
	StartCmdTime time.Time
	Config       *config.Config

	ExecutionError error
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *ScheduleTasksCmd) ExtractDependencies(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.FilterStateKeeper:
		err = cmd.extractDepsFromFilterStateKeeper(ctx, sk)

	default:
		return fmt.Errorf("stateKeeper '%T' is not supported by cmd type %s", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *ScheduleTasksCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.FilterStateKeeper:
		err = cmd.updateScheduleStateKeeper(ctx, sk)
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *ScheduleTasksCmd) extractDepsFromFilterStateKeeper(
	ctx context.Context,
	sk *data.FilterStateKeeper) error {

	if sk.MiddledOutResp == nil {
		return fmt.Errorf("cmd %q missing dependency: MiddleOutResponse", cmd.GetCommandType())
	}

	if len(sk.BuildsMap) == 0 {
		return fmt.Errorf("cmd %q missing dependency: BuildsMap", cmd.GetCommandType())
	}

	if sk.BuildState == nil {
		return fmt.Errorf("cmd %q missing dependency: BuildState", cmd.GetCommandType())
	}

	if sk.Config == nil {
		logging.Warningf(ctx, "cmd %q missing Config", cmd.GetCommandType())
	}

	if sk.Scheduler == api.SchedulerInfo_UNSPECIFIED {
		return fmt.Errorf("cmd %q missing dependency: Scheduler", cmd.GetCommandType())
	}

	if sk.AlStateInfo == nil {
		logging.Warningf(ctx, "cmd %q missing optional dependency: AlStateInfo", cmd.GetCommandType())
	}

	if sk.BQClient != nil {
		cmd.BQClient = sk.BQClient
	}
	cmd.InternalTestPlan = proto.Clone(sk.TestPlanStates[len(sk.TestPlanStates)-1]).(*api.InternalTestplan)

	if sk.CtpReq == nil {
		return fmt.Errorf("cmd %q missing dependency: CtpReq", cmd.GetCommandType())
	}

	if sk.RequestKey != "" {
		cmd.RequestKey = sk.RequestKey
	}

	fireStoreDB := common.TestPlatformFireStore
	if sk.IsPartnerRun && sk.IsAlRun {
		fireStoreDB = common.PartnerTestPlatformFireStore
	}
	dynamicExperiment := slices.Contains(sk.BuildState.Build().GetInput().GetExperiments(), common.DynamicExperiment)
	cmd.FirestoreDBName = fireStoreDB
	cmd.CredentialsFile = sk.DockerKeyFile
	cmd.EnvVersion = sk.CTPversion
	cmd.DynamicRun = sk.CtpReq.RunDynamic || dynamicExperiment
	cmd.BuildsMap = sk.BuildsMap
	cmd.BuildState = sk.BuildState
	cmd.Config = sk.Config
	cmd.AlStateInfo = sk.AlStateInfo
	cmd.directBBScheduler = schedulers.NewDirectBBScheduler()
	// Assign scheduler
	switch s := sk.Scheduler; s {
	case api.SchedulerInfo_QSCHEDULER:
		cmd.Scheduler = schedulers.NewDirectBBScheduler()
	case api.SchedulerInfo_PRINT_REQUEST_ONLY:
		cmd.Scheduler = schedulers.NewDryRunScheduler()
	case api.SchedulerInfo_SCHEDUKE:
		cmd.Scheduler = schedulers.NewSchedukeScheduler()
	default:
		return fmt.Errorf("cmd %q specified invalid scheduler type %s", cmd.GetCommandType(), s)
	}

	cmd.ExecutionError = sk.ExecutionError
	cmd.IsAlRun = sk.IsAlRun
	return nil
}

func (cmd *ScheduleTasksCmd) updateScheduleStateKeeper(ctx context.Context, sk *data.FilterStateKeeper) error {
	if len(cmd.TestResults) != 0 {
		for k, v := range cmd.TestResults {
			sk.SuiteTestResults[k] = v
		}
	}
	cmd.InternalTestPlan = proto.Clone(sk.TestPlanStates[len(sk.TestPlanStates)-1]).(*api.InternalTestplan)

	sk.AlStateInfo = cmd.AlStateInfo
	sk.ExecutionError = cmd.ExecutionError

	// Update current test job event
	cmd.updateCurrentTestJobEvent()

	return nil
}

func (cmd *ScheduleTasksCmd) updateCurrentTestJobEvent() {
	if cmd.AlStateInfo == nil || cmd.AlStateInfo.CurrentTestJobEvent == nil {
		return
	}
	currTestJobEvent := cmd.AlStateInfo.CurrentTestJobEvent
	testJobEventState := common.TaskCompletedState

	// Get test counts
	totalTestCount := 0
	totalFailedTestCount := 0
	totalFailedTestRunCount := 0

	tasks := []*common.TestTaskMessage{}
	testRunnerLinks := []string{}
	// TODO (azrahman): handle rety logic when ctp level retries are enabled
	for _, results := range cmd.TestResults {
		testRunnerLinks = append(testRunnerLinks, results.BuildURL)
		totalShards := cmd.findTotalShards(results.Key)
		currTestCount, currFailedTestCount, currSkippedTestCount, currFailedTestRunCount := results.GetTestCounts()
		totalTestCount = totalTestCount + currTestCount
		totalFailedTestCount = totalFailedTestCount + currFailedTestCount
		totalFailedTestRunCount = totalFailedTestRunCount + currFailedTestRunCount
		summary := fmt.Sprintf("passed: %d, failed: %d, skipped: %d, module_failed: %d", (currTestCount - currFailedTestCount - currSkippedTestCount), currFailedTestCount, currSkippedTestCount, currFailedTestRunCount)
		taskState := common.TaskCompletedState
		// Intentionally casting wider net to capture all kinds of downstream errors.
		if err := results.GetTestRunnerErr(); err != nil {
			summary = err.Error()
			taskState = common.TaskErrorState
			testJobEventState = common.TaskErrorState
		} else if err := results.GetProvisionErrIfAny(); err != nil {
			// TODO: GetTestRunnerErr should cover provisioning error as well. but keeping this as a safety net till
			// we gain more confidence on the new error pipeline. Remove this block when makes sense.
			summary = err.Error()
			taskState = common.TaskErrorState
			testJobEventState = common.TaskErrorState
		}

		// As no rety is enabled now, different results means different shards/requests
		task := &common.TestTaskMessage{
			Id:                results.Key,
			TestTaskState:     taskState,
			Shards:            totalShards,
			ShardIndex:        int64(results.ShardIndex),
			CreationTimestamp: results.CreationTimestamp.UTC().Format(common.ATPSupportedTimeFormat),
			StartTimestamp:    results.StartTimestamp.UTC().Format(common.ATPSupportedTimeFormat),
			EndTimestamp:      results.EndTimestamp.UTC().Format(common.ATPSupportedTimeFormat),
			UpdateTimestamp:   results.EndTimestamp.UTC().Format(common.ATPSupportedTimeFormat),
			Attempts: []*common.TestTaskAttemptMessage{
				{
					Id:                   fmt.Sprintf("%s_%d", results.Key, results.BuildID),
					TestTaskAttemptState: taskState,
					TotalTestCount:       int64(currTestCount),
					FailedTestCount:      int64(currFailedTestCount),
					FailedTestRunCount:   int64(currFailedTestRunCount),
					CreationTimestamp:    results.CreationTimestamp.UTC().Format(common.ATPSupportedTimeFormat),
					StartTimestamp:       results.StartTimestamp.UTC().Format(common.ATPSupportedTimeFormat),
					EndTimestamp:         results.EndTimestamp.UTC().Format(common.ATPSupportedTimeFormat),
					UpdateTimestamp:      results.EndTimestamp.UTC().Format(common.ATPSupportedTimeFormat),
					AttemptInfo:          []*common.KeyValuesMessage{{Key: "summary", Values: []string{summary}}},
				},
			},
		}

		tasks = append(tasks, task)
	}

	// update test job event
	if currTestJobEvent.TestJob.ResultLinks == nil {
		currTestJobEvent.TestJob.ResultLinks = []string{}
	}
	currTestJobEvent.State = testJobEventState
	currTestJobEvent.TotalTestCount = int64(totalTestCount)
	currTestJobEvent.FailedTestCount = int64(totalFailedTestCount)
	currTestJobEvent.FailedTestRunCount = int64(totalFailedTestRunCount)
	currTestJobEvent.TestJob.EndTimestamp = time.Now().UTC().Format(common.ATPSupportedTimeFormat)
	currTestJobEvent.TestJob.TestJobState = testJobEventState
	currTestJobEvent.TestJob.Tasks = tasks
	currTestJobEvent.TestJob.Id = strconv.FormatInt(cmd.BuildState.Build().Id, 10)
	currTestJobEvent.TestJob.ResultLinks = append(currTestJobEvent.TestJob.ResultLinks, testRunnerLinks...)
}

func (cmd *ScheduleTasksCmd) findTotalShards(key string) int64 {
	prefixedKey := common.GetPrefixBasedOnDelim(key, "-shard")
	if prefixedKey == "" {
		return 1 // shouldn't happen but if there is no "-shard",that means it's a one off request
	}
	shardCount := 0
	for _, results := range cmd.TestResults {
		if strings.HasPrefix(results.Key, prefixedKey) {
			shardCount++
		}
	}
	return int64(shardCount)
}

// Execute executes the command.
func (cmd *ScheduleTasksCmd) Execute(ctx context.Context) error {
	var err error
	step, ctx := build.StartStep(ctx, "Schedule tasks")
	defer func() { step.End(err) }()

	cmd.ObserveCmdStart(ctx)
	cmd.TestResults = map[string]*data.TestResults{}

	scheduler := cmd.Scheduler
	pool := getPool(cmd.InternalTestPlan.SuiteInfo)
	err = scheduler.Setup(pool)
	if err != nil {
		errmsg := "error while setting up scheduler"
		cmd.ObserveSchedulerSetupFailure(ctx, errmsg)
		logging.Infof(ctx, "%s: %s", errmsg, err)
		return errors.Annotate(err, "%s", errmsg).Err()
	}
	cmd.directBBScheduler.Setup(pool)
	dmc, err := dm.NewClient(ctx, pool)
	if err != nil {
		return errors.Annotate(err, "error while connecting to Device Manager").Err()
	}
	cmd.ObserveSchedulerSetupSuccess(ctx)

	// Todo: batch call
	resultsChan := make(chan *data.TestResults)
	wg := &sync.WaitGroup{}

	// b/377196624 - coolOffLimit defines number of test runners scheduled after which execution should stop for coolOffDuration time.
	// this is to stop causing oom while multiple ScheduleAndMonitor goroutines are invoked
	const coolOffLimit = 50
	const coolOffDuration = time.Second * 2

	// configure bbClient
	bbClient, err := newBBClient(ctx)
	if err != nil {
		return err
	}
	counter := 0

	// Determine and endtime for the suites based on the input requests
	endTime := time.Time{}
	if cmd.BuildsMap != nil {
		for _, req := range cmd.BuildsMap {
			if !endTime.IsZero() {
				break
			}

			if req == nil {
				continue
			}

			endTime = time.Now().Add(req.SuiteInfo.GetSuiteRequest().MaximumDuration.AsDuration())
		}
	}

	// If there is no given in the request then grant a day for a time out. This
	// will  likely not get hit as the builder has a shorter timeout.
	if endTime.IsZero() {
		endTime = time.Now().Add(Day)
	}

	logging.Infof(ctx, "now is %s", time.Now().Local().String())
	logging.Infof(ctx, "end time is %s", endTime.Local().String())

	// Create a separate context for the semaphore so that we can force a
	// timeout of the semaphore.
	ctx, cancel := context.WithDeadline(ctx, endTime)
	defer cancel()

	for k, v := range cmd.BuildsMap {
		wg.Add(1)
		go cmd.ScheduleAndMonitor(ctx, k, v, wg, resultsChan, 0, dmc, len(cmd.BuildsMap), bbClient)

		counter++
		if counter >= coolOffLimit {
			// start cool off period
			time.Sleep(coolOffDuration)
			// reset the counter
			counter = 0
		}
	}

	go func() {
		wg.Wait()
		close(resultsChan) // Close the channel when all workers are done
	}()

	// Read results
	for result := range resultsChan {
		mapKey := result.Key
		if result.Attempt != 0 {
			mapKey = fmt.Sprintf("%s-retry-%d", result.Key, result.Attempt)
		}
		cmd.TestResults[mapKey] = result
	}

	cmd.ObserveCmdEndSuccess(ctx)
	common.WriteAnyObjectToStepLog(ctx, step, cmd.TestResults, "consolidated results")

	if cmd.AlStateInfo != nil {
		cmd.AlStateInfo.DoneTesting = true
	}

	return nil

}

// formatBotListURL generates a swarming URL for all the request dimensions
// provided. This will show all of the available bots that swarming has matched
// with the request.
func formatBotListURL(dims []*buildbucketpb.RequestedDimension) (string, error) {
	baseURL := "https://chromeos-swarming.appspot.com/botlist?c=id&c=task&c=os&c=status&d=asc&k=pool&s=id"

	for _, dim := range dims {
		// Replace spaces with the URL safe unicode value.
		value := strings.ReplaceAll(dim.GetValue(), " ", "%20")

		baseURL += "&f=" + dim.GetKey() + ":" + value + "&k=" + dim.GetKey()
	}

	swarmingURL, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	return swarmingURL.String(), nil
}

func (cmd *ScheduleTasksCmd) getATPShardFromCMDState(key string) *androidapi.WorkUnitNode {
	if cmd.AlStateInfo != nil && cmd.AlStateInfo.WorkUnitTrees != nil {
		if tree, ok := cmd.AlStateInfo.WorkUnitTrees["test"]; !ok {
			return nil
		} else {
			return tree.ShardsByKey[key]
		}
	}

	return nil
}

func cancelRunningTask(ctx context.Context, scheduledBuild *buildbucketpb.Build, bbClient buildbucketpb.BuildsClient, cancelError error) error {
	if scheduledBuild == nil {
		return nil
	}

	logging.Infof(ctx, "Cancelling running test_runner %d", scheduledBuild.Id)

	// Using a new context so that the RPC doesn't fail if we are past the
	// parent's context deadline
	bbClientCtx := context.Background()

	req := &buildbucketpb.GetBuildRequest{
		Id: int64(scheduledBuild.Id),
	}
	build, err := bbClient.GetBuild(bbClientCtx, req)
	if err != nil {
		return err
	}

	// Do a bitmask check to see if the build is in a terminal state. If not
	// then cancel the build.
	if build.Status&buildbucketpb.Status_ENDED_MASK != buildbucketpb.Status_ENDED_MASK {
		_, err := bbClient.CancelBuild(bbClientCtx, &buildbucketpb.CancelBuildRequest{
			Id:              build.Id,
			SummaryMarkdown: cancelError.Error(),
		})
		if err != nil {
			return err
		}

	}

	return nil
}

func (cmd *ScheduleTasksCmd) ScheduleAndMonitor(rootCtx context.Context, key string, buildReq *data.BuildRequest, wg *sync.WaitGroup, resultsChan chan<- *data.TestResults, retryNum int, dmc *dm.Client, buildsMapLen int, bbClient buildbucketpb.BuildsClient) {
	defer wg.Done()
	var err error

	// Assign through mainErrPtr to unambiguously set the main error.
	// This variable is a major hack, see the patchset below (it is the first patchset) for more information.
	// https://chromium-review.googlesource.com/c/infra/infra/+/6581399/1
	// That patchset is more principled in some ways, but it has a bug on line 524 with errors not propagating properly.
	// This suggests to me that the right thing to do is to have a light-touch way to unambiguously assign to the main error
	// until this function can be refactored.
	mainErrPtr := &err

	defer func(e error) {
		cmd.ExecutionError = e
	}(err)

	suiteName := suiteName(buildReq.SuiteInfo)
	stepName := key
	if retryNum > 0 {
		stepName = fmt.Sprintf("%s-retry-%d", key, retryNum)
	}
	step, ctx := build.StartStep(rootCtx, stepName)
	defer func() { step.End(err) }()

	// Construct test results
	result := &data.TestResults{Key: key, Suite: suiteName, Attempt: retryNum, RequestKey: cmd.RequestKey, Name: fmt.Sprintf("%s_%s", suiteName, key), ShardIndex: buildReq.ShardNum, CreationTimestamp: time.Now(), TestCases: buildReq.OriginalTrReq.Tcs, IsALRun: cmd.IsAlRun}

	// If we are inside of an AL run that has built a WU tree then generate and
	// insert an attempt node.
	var attemptNode *androidapi.WorkUnitNode
	if shardNode := cmd.getATPShardFromCMDState(key); shardNode != nil {
		tree := cmd.AlStateInfo.WorkUnitTrees["test"]
		keys := []string{}
		for key := range tree.ShardsByKey {
			keys = append(keys, key)
		}

		logging.Infof(ctx, "Keys seen in shards map; %+v", keys)

		logging.Infof(ctx, "SHARD Node Parent %s-%s: %+v found for key %s\n", shardNode.GetWorkUnit().Id, shardNode.GetWorkUnit().Name, shardNode.GetIndex(), key)

		attemptNode, err = androidapi.NewWorkUnitNode(ctx, shardNode.GetWorkUnit().Id, shardNode.GetWorkUnit().InvocationId, androidapi.WULayerAttempt, shardNode, common.GetCTPEnvironment(cmd.BuildState.Build().GetBuilder()))
		if err != nil {
			setTopLevelError(ctx, step, result, resultsChan, err, attemptNode, nil, bbClient)
			return
		}

		logging.Infof(ctx, "ATTEMPT Node %s-%s: %+v\n", attemptNode.GetWorkUnit().Id, attemptNode.GetWorkUnit().Name, attemptNode.GetIndex())

		head, err := attemptNode.FetchHead()
		if err != nil {
			setTopLevelError(ctx, step, result, resultsChan, err, attemptNode, nil, bbClient)
			return
		}

		invocationID := attemptNode.GetWorkUnit().InvocationId
		attemptWUID := attemptNode.GetWorkUnit().Id
		// The head of the tree points directly to the ATP WU which kicked off
		// testing.
		atpWUID := head.GetWorkUnit().ParentId

		alTags := []*buildbucketpb.StringPair{
			{
				Key:   "ants_invocation_id",
				Value: invocationID,
			},
			{
				Key:   "atp_work_unit_id",
				Value: atpWUID,
			},
			{
				Key:   "attempt_wu_id",
				Value: attemptWUID,
			},
		}

		atpConfigTag := cmd.buildTag("atp_config_name")
		if atpConfigTag != nil {
			alTags = append(alTags, atpConfigTag)
		}

		atpTriggerTag := cmd.buildTag("atp_trigger_type")
		if atpTriggerTag != nil {
			alTags = append(alTags, atpTriggerTag)
		}

		// If we have a proper build request then add the atp tags to the req.
		if attemptNode.GetWorkUnit() != nil {
			if scheduleBuild := buildReq.ScheduleBuildRequest; scheduleBuild != nil {
				if scheduleBuild.Tags != nil {
					// Append
					scheduleBuild.Tags = append(scheduleBuild.Tags, alTags...)
				} else {
					scheduleBuild.Tags = alTags
				}
			}
		}
	}

	if buildReq.Err != nil {
		err = buildReq.Err
		setTopLevelError(ctx, step, result, resultsChan, buildReq.Err, attemptNode, nil, bbClient)
		return
	}
	req := buildReq.ScheduleBuildRequest

	// b/377196624,b/388180876 - enable log streaming for test runner requests only if there are fewer requests than number defined by logsAndAnalyticsLimit.
	// exceeding this limit may lead to an oom issue. (This metric is when executed on a bot with 8 GB memory)
	logsAndAnalyticsLimit := 50
	if buildsMapLen < logsAndAnalyticsLimit {
		// Spit out the request
		requestData, err := json.MarshalIndent(req, "", "  ")
		if err != nil {
			logging.Infof(
				ctx,
				"error during writing request data to log: %s",
				err.Error())
		}
		_, err = step.Log("BB Request").Write(requestData)
		if err != nil {
			logging.Errorf(ctx, "ScheduleAndMonitor: %w", err)
			return
		}

		// Spit out requested dims since scheduke doesn't pass this info to swarming
		common.WriteAnyObjectToStepLog(ctx, step, req.GetDimensions(), "requested dimensions")

		botListURL, err := formatBotListURL(req.GetDimensions())
		if err != nil {
			logging.Errorf(ctx, "ScheduleAndMonitor: %w", err)
			return
		}
		common.WriteStringToStepLog(ctx, step, botListURL, "requested dimensions bot list")
	}

	builderID := common.TestRunnerBuilderID(cmd.Config)

	trSchedulingStart := time.Now()
	cmd.ObserveTrSchedulingStart(ctx, buildReq)

	var scheduledBuild *buildbucketpb.Build
	var leaseID string

	// if filter updated scheduling unit to vm then schedule via direct bb scheduler and update tags
	if req.GetBuilder().Builder == testRunnerGceBuilderName && cmd.Scheduler.GetSchedulerType() == schedulers.SchedukeSchedulerType {
		// update tags info for the test runner gce build
		for _, v := range req.GetTags() {
			if v.GetKey() == scheduler {
				v.Value = api.SchedulerInfo_QSCHEDULER.String()
			}
			if v.GetKey() == autoVMRun {
				v.Value = "true"
			}
		}
		scheduledBuild, leaseID, err = cmd.directBBScheduler.ScheduleRequest(ctx, req, step)

		if err != nil {
			err = fmt.Errorf("error while scheduling req: %w", err)
			cmd.ObserveTrSchedulingFail(ctx, buildReq, err.Error(), trSchedulingStart)
			setTopLevelError(ctx, step, result, resultsChan, err, attemptNode, nil, bbClient)
			return
		}
	} else {
		// BQ TODO log the request is in the scheduling tool (ie log the scheduke ID if possible?)
		scheduledBuild, leaseID, err = cmd.Scheduler.ScheduleRequest(ctx, req, step)
		if err != nil {
			err = fmt.Errorf("error while scheduling req: %w", err)
			cmd.ObserveTrSchedulingFail(ctx, buildReq, err.Error(), trSchedulingStart)
			setTopLevelError(ctx, step, result, resultsChan, err, attemptNode, nil, bbClient)
			return
		}
	}

	if leaseID != "" {
		step.Log(fmt.Sprintf("Device Manager lease ID: %s", leaseID))
	}

	// Don't poll for build status if this is a dry-run.
	dryRun := cmd.Scheduler.GetSchedulerType() == schedulers.DryRunSchedulerType
	if dryRun {
		// Return incomplete results so that we don't completely ignore this failure
		// in the recipe summary step.
		result.Results = getIncompleteRunResults(buildReq)
		err = fmt.Errorf("dry run skipped running TR")
		setTopLevelError(ctx, step, result, resultsChan, err, attemptNode, nil, bbClient)
		return
	}

	summaries := []string{}
	if scheduledBuild != nil && scheduledBuild.GetId() != 0 {
		result.StartTimestamp = time.Now()
		// Link the scheduled TestRunner to the current CTP builder so that we
		// can show test results in MILO at the CTP level.
		rdbClient, err := newRDBClient(ctx, cmd.BuildState.Build().Infra.GetResultdb().GetHostname())
		if err != nil {
			logging.Errorf(ctx, "ScheduleAndMonitor: %w", err)
			return
		}
		rdb.InheritRDBInvocation(ctx, scheduledBuild.GetId(), bbClient, rdbClient)

		result.BuildID = scheduledBuild.GetId()
		result.BuildURL = common.BBUrl(builderID, scheduledBuild.GetId())

		summaries = append(summaries, fmt.Sprintf("* [latest attempt](%s)", common.BBUrl(builderID, scheduledBuild.GetId())))
		step.SetSummaryMarkdown(strings.Join(summaries, "\n"))
	} else {
		errStr := "no bbid found from scheduler as no device found due to unavailability or priority"
		err = fmt.Errorf("%s", errStr)
		cmd.ObserveTrSchedulingFail(ctx, buildReq, err.Error(), trSchedulingStart)

		setTopLevelError(ctx, step, result, resultsChan, err, attemptNode, nil, bbClient)
		return
	}

	// Log the successful start.
	cmd.ObserveTrSchedulingSuccess(ctx, buildReq, fmt.Sprint(scheduledBuild.GetId()), trSchedulingStart)

	// Re-init the data for the run build step. Keep the previously populated
	// data.
	trBuildStart := time.Now()
	cmd.ObserveTrBuildStart(ctx, buildReq)

	// Since the requests are combined in CTPv2 and untangled later we need to
	// fetch the real request key name so that we can separate the merged
	// requests.
	target, err := suitelimits.ExtractTarget(result.Key)
	if err != nil {
		logging.Errorf(ctx, "ScheduleAndMonitor: %w", err)
		return
	}

	// Add the request to the SuiteLimits cache to begin being tracked.
	err = suitelimits.AddRequestTask(rootCtx, cmd.RequestKey, suiteName, target, getPool(cmd.InternalTestPlan.SuiteInfo), scheduledBuild.GetId(), cmd.IsAlRun)
	if err != nil {
		logging.Errorf(ctx, "ScheduleAndMonitor: %w", err)
		return
	}

	// Monitor here
	lastLeaseExtensionTime := time.Now()
	loopSleepInterval := 30 * time.Second
	statusReq := &buildbucketpb.GetBuildStatusRequest{
		Id: scheduledBuild.GetId(),
	}

	for {
		// b/377196624 - ease memory execution by adding sleep of 10 secs, there can be several goroutines doing same execution at same time, which may lead to oom.
		time.Sleep(5 * time.Second)

		// If the context has been closed then exit early.
		if err = ctx.Err(); err != nil {
			err = fmt.Errorf("user configured maximum duration exceeded. Cancelling - %s", fmt.Sprintf("* [latest attempt](%s)", common.BBUrl(builderID, scheduledBuild.GetId())))
			setTopLevelError(ctx, step, result, resultsChan, err, attemptNode, scheduledBuild, bbClient)
			return
		}

		buildInfo, err := CheckBuildInfoIfBuildEnded(ctx, statusReq, bbClient)
		if err != nil || buildInfo == nil {
			// this means the build didn't end
			if err != nil {
				logging.Infof(ctx, "error while checking build status: %s", err)
			}

			if ctx.Err() != nil {
				// A timeout while waiting for tests to complete is reported as
				// aborts when summarizing individual tests' results.
				// The execute step completes without errors.
				logging.Errorf(ctx, "ScheduleAndMonitor: %w", err)
				return
			}

			if leaseID != "" && time.Since(lastLeaseExtensionTime) >= dm.LeaseExtensionInterval {
				_, err = dmc.Extend(ctx, leaseID, dm.LeaseExtensionAmount)
				if err != nil {
					err = fmt.Errorf("error while extending lease %s with Device Manager: %w", leaseID, err)
					summaries = append(summaries, fmt.Sprintf("* %s", err))
					err = fmt.Errorf("%s", strings.Join(summaries, "\n"))
					setTopLevelError(ctx, step, result, resultsChan, err, attemptNode, scheduledBuild, bbClient)
					return
				}
				lastLeaseExtensionTime = time.Now()
			}

			overLimit, err := suitelimits.UpdateTotalTime(rootCtx, cmd.RequestKey, suiteName, target, scheduledBuild.GetId())
			if err != nil {
				logging.Errorf(ctx, "ScheduleAndMonitor: %w", err)
				return
			}

			// If the request is over the allotted SuiteLimits maximum DUT hour
			// time and it does not have an active exemption, cancel all child
			// tasks.
			if overLimit {
				err := suitelimits.CancelTasks(rootCtx, cmd.RequestKey, target, bbClient)
				if err != nil {
					logging.Errorf(ctx, "ScheduleAndMonitor: %w", err)
					return
				}
			} else {
				time.Sleep(loopSleepInterval)
			}

			// we don't wanna fail coz it could be a flake so we continue checking
			continue
		}

		if leaseID != "" {
			err = dmc.Release(ctx, leaseID)
			if err != nil {
				err = fmt.Errorf("error while releasing lease %s with Device Manager: %w", leaseID, err)
				logging.Infof(ctx, err.Error())
				summaries = append(summaries, fmt.Sprintf("* %s", err))
				step.SetSummaryMarkdown(strings.Join(summaries, "\n"))
			}
		}

		// The build ended so we extract results now
		common.WriteAnyObjectToStepLog(ctx, step, buildInfo, "final build info")
		result.EndTimestamp = time.Now()

		// Log success as we found a completed build. The status is not of the child build itself.
		cmd.ObserveTrBuildSuccess(ctx, buildReq, buildInfo, trBuildStart)

		logging.Infof(ctx, "bb status: %s", buildInfo.GetStatus())
		if buildInfo.GetStatus() != buildbucketpb.Status_SUCCESS {
			// setting this for the step to fail
			*mainErrPtr = fmt.Errorf("test_runner failed")
		}

		// If we are in an AL run then update the attempt with the results of
		// the tests.
		//
		// TODO(b/372507028): For a richer experience, attach errors to the
		// metadata of the WU. For now focus on the "success" path.
		if attemptNode != nil {
			// If the WU changed in anyway inside TestRunner then our current WU
			// is going to be outdated. This will refresh the CTP WU so that we
			// can make updates without conflict.
			refreshedAttemptWU, err := attemptNode.Service.Get(attemptNode.GetWorkUnit().Id)
			if err != nil {
				setTopLevelError(ctx, step, result, resultsChan, err, attemptNode, scheduledBuild, bbClient)
				return
			}
			attemptNode.SetWorkUnit(refreshedAttemptWU)

			switch buildInfo.GetStatus() {
			case buildbucketpb.Status_SUCCESS:
				attemptNode.GetWorkUnit().State = common.TaskCompletedState
				logging.Infof(ctx, "WU %s-%s completed testing in %s status", attemptNode.GetWorkUnit().Id, attemptNode.GetWorkUnit().Name, attemptNode.GetWorkUnit().State)
			case buildbucketpb.Status_FAILURE:
				attemptNode.GetWorkUnit().State = common.TaskErrorState
				logging.Infof(ctx, "WU %s-%s completed testing in %s status", attemptNode.GetWorkUnit().Id, attemptNode.GetWorkUnit().Name, attemptNode.GetWorkUnit().State)
				attemptNode.GetWorkUnit().DebugInfo = &androidbuildinternal.DebugInfo{
					ErrorCode: 1,
					// TODO: add a proper error message here propagated from
					// TestRunner.
					ErrorMessage: "TESTING FAILURE",
				}
			case buildbucketpb.Status_INFRA_FAILURE:
				attemptNode.GetWorkUnit().State = common.TaskErrorState
				logging.Infof(ctx, "WU %s-%s completed testing in %s status", attemptNode.GetWorkUnit().Id, attemptNode.GetWorkUnit().Name, attemptNode.GetWorkUnit().State)
				attemptNode.GetWorkUnit().DebugInfo = &androidbuildinternal.DebugInfo{
					ErrorCode:    1,
					ErrorMessage: "TESTING INFRA_FAILURE",
				}
			case buildbucketpb.Status_CANCELED:
				attemptNode.GetWorkUnit().State = common.TaskCanceledState
				logging.Infof(ctx, "WU %s-%s completed testing in %s status", attemptNode.GetWorkUnit().Id, attemptNode.GetWorkUnit().Name, attemptNode.GetWorkUnit().State)
			default:
				attemptNode.GetWorkUnit().State = common.TaskUnknownState
				logging.Infof(ctx, "WU %s-%s completed testing in %s status", attemptNode.GetWorkUnit().Id, attemptNode.GetWorkUnit().Name, attemptNode.GetWorkUnit().State)
			}
		}

		// Check the TestRunner build's step summary to see if the task was cancelled by
		// SuiteLimits.
		suiteLimited, slErr := suitelimits.ExceededLimit(cmd.RequestKey, target)
		if slErr != nil {
			logging.Infof(ctx, "suite limit exceed error: %s", slErr)
			setTopLevelError(ctx, step, result, resultsChan, slErr, attemptNode, scheduledBuild, bbClient)
			return
		}

		slExempt, slErr := suitelimits.HasExemption(cmd.RequestKey, target)
		if slErr != nil {
			logging.Infof(ctx, "suite limit exemption check error: %s", slErr)
			setTopLevelError(ctx, step, result, resultsChan, slErr, attemptNode, scheduledBuild, bbClient)
			return
		}
		if suiteLimited && !slExempt {
			totalDUTHours, slErr := suitelimits.GetTotalDUTHours(cmd.RequestKey, target)
			if slErr != nil {
				logging.Infof(ctx, "err while getting total dut hours: %s", slErr)
				setTopLevelError(ctx, step, result, resultsChan, slErr, attemptNode, scheduledBuild, bbClient)
				return
			}

			common.WriteAnyObjectToStepLog(ctx, step, fmt.Sprintf("total DUT hour runtime: %.2f", totalDUTHours.Hours()), "SUITE EXECUTION TIME LIMIT EXCEEDED")

			err = &data.SuiteLimitsError{RequestName: cmd.RequestKey, SuiteName: suiteName, TotalDUTHours: totalDUTHours}

			// Attach the failure BB status so that the step shows up as red in
			// MILO.
			//
			// TODO(b/360406127): This still is leaving the step green. This
			// needs to be fixed so that the step ends in a red failure state.
			err = build.AttachStatus(err, buildbucketpb.Status_FAILURE, nil)
			setTopLevelError(ctx, step, result, resultsChan, err, attemptNode, scheduledBuild, bbClient)
			return
		}

		trResult, err := extractResult(buildInfo, buildReq)
		// there can be incomplete result even with an error so let's set it first
		if trResult != nil {
			result.Results = trResult
		}
		if err != nil {
			err = fmt.Errorf("error while extracting results from test_runner build %d: %w", buildInfo.Id, err)
			summaries = append(summaries, fmt.Sprintf("* %s", err))
			err = fmt.Errorf("%s", strings.Join(summaries, "\n"))
			setTopLevelError(ctx, step, result, resultsChan, err, attemptNode, scheduledBuild, bbClient)
			return
		}
		common.WriteAnyObjectToStepLog(ctx, step, result, "extracted result from trv2")

		// Retry if qualifies
		if buildReq.SuiteInfo.GetSuiteRequest().GetRetryCount() > int64(retryNum) {
			newBuildReq := cmd.RetryReqIfQualifies(ctx, trResult, step, buildReq)
			if newBuildReq != nil && newBuildReq.ScheduleBuildRequest != nil {
				logging.Infof(ctx, "total retry left after current retry: %d", buildReq.SuiteInfo.GetSuiteRequest().GetRetryCount()-int64(retryNum))
				// Schedule retry
				wg.Add(1)
				go cmd.ScheduleAndMonitor(rootCtx, newBuildReq.Key, newBuildReq, wg, resultsChan, retryNum+1, dmc, buildsMapLen, bbClient)
			}
		}

		// Send the result via channel
		resultsChan <- result
		break
	}
}

func (cmd *ScheduleTasksCmd) RetryReqIfQualifies(ctx context.Context, trResult *skylab_test_runner.Result, step *build.Step, buildReq *data.BuildRequest) *data.BuildRequest {
	shouldRetry, retriableTestsMap := determineRetriablity(trResult)
	if !shouldRetry {
		logging.Infof(ctx, "no retriable tests found for: %s", buildReq.Key)
		return nil
	}

	logging.Infof(ctx, "This req qualified for retry...")
	common.WriteAnyObjectToStepLog(ctx, step, retriableTestsMap, "retriable test list")

	newBuildReq := GenerateNewBuildReqForRetry(ctx, buildReq, retriableTestsMap)
	common.WriteAnyObjectToStepLog(ctx, step, newBuildReq, "new build req for retry")

	// Generate a new req
	if newBuildReq.ScheduleBuildRequest == nil {
		req, err := cmd.GenerateReqForRetry(ctx, newBuildReq)
		if err != nil {
			newBuildReq.Err = err
			logging.Infof(ctx, "no more retry will take place for %s since trv2 req generation failed: %s", newBuildReq.Key, err)
		} else {
			newBuildReq.ScheduleBuildRequest = req
		}
	}

	return newBuildReq
}

func (cmd *ScheduleTasksCmd) GenerateReqForRetry(ctx context.Context, buildReq *data.BuildRequest) (*buildbucketpb.ScheduleBuildRequest, error) {
	// TODO (azrahman/dbeckett): add analytics metrics for this func
	var err error
	trReq := buildReq.OriginalTrReq
	key := buildReq.Key
	shardNum := buildReq.ShardNum

	var TrReqhwDef *api.SwarmingDefinition
	TrReqhwDef = nil

	var publishKeys []*api.PublishKey
	var schedUnit *api.SchedulingUnit
	schedUnit = nil
	if trReq.NewReq != nil && len(trReq.NewReq.GetSchedulingUnits()) != 0 {
		// '0'ed index because we should always have one hw here. It supports multiple
		// MO should reduce it down to 1 always. The len check is done at MO step.
		schedUnit = trReq.NewReq.GetSchedulingUnits()[0]
		publishKeys = trReq.NewReq.GetPublishKeys()
	} else {
		// '0'ed index because we should always have one hw here. It supports multiple
		// MO should reduce it down to 1 always. The len check is done at MO step.
		TrReqhwDef = trReq.Req.GetHwDefinition()[0]
	}
	testCases := trReq.Tcs

	// Input validations
	if len(testCases) == 0 {
		errStr := "no test found for retry, no retry will be attempted"
		logging.Infof(ctx, errStr)
		err = fmt.Errorf("%s", errStr)
		return nil, err

	}

	if trReq.DevicesInfo.LabDevicesCount == 0 {
		logging.Infof(ctx, "no suitable device found to run tests so, rejecting task")
		err := &data.BotParamsRejectedError{Key: key, RejectedDims: trReq.DevicesInfo.Dims}
		return nil, err
	}

	helper := &TrV2ReqHelper{
		schedUnit:            schedUnit,
		trReqHWDef:           TrReqhwDef,
		testCases:            testCases,
		build:                cmd.BuildState,
		suiteInfo:            cmd.InternalTestPlan.SuiteInfo,
		shardNum:             shardNum,
		dynamicRun:           cmd.DynamicRun,
		schedUnitMetadataMap: buildSchedUnitMap(cmd.InternalTestPlan.GetSuiteInfo()),
		config:               cmd.Config,
		credentialsFile:      cmd.CredentialsFile,
		envVersion:           cmd.EnvVersion,
		firestoreDBName:      cmd.FirestoreDBName,
		is3DRun:              cmd.InternalTestPlan.GetSuiteInfo().GetSuiteRequest().GetDddSuite(),
		alStateInfo:          cmd.AlStateInfo,
		publishKeys:          publishKeys,
	}

	req, err := GenerateTrv2Req(ctx, false, helper, common.IsLedRun(cmd.BuildState.Build().GetBuilder()), cmd.InternalTestPlan.GetSuiteInfo().GetSuiteRequest().GetTags())
	if err != nil {
		logging.Infof(ctx, "error while generating req: %s", err)
		return nil, errors.Annotate(err, "error while generating req:").Err()
	}

	return req, nil
}

func CheckBuildInfoIfBuildEnded(ctx context.Context, statusReq *buildbucketpb.GetBuildStatusRequest, bbClient buildbucketpb.BuildsClient) (*buildbucketpb.Build, error) {
	// Check Build status.
	b, err := bbClient.GetBuildStatus(ctx, statusReq)
	if err != nil {
		logging.Infof(ctx, "error while getting build status: %s", err)
		return nil, err
	}

	if b == nil || int(b.GetStatus())&int(buildbucketpb.Status_ENDED_MASK) == 0 {
		return nil, nil
	}

	// Get more build info
	req := &buildbucketpb.GetBuildRequest{
		Id:   b.Id,
		Mask: &buildbucketpb.BuildMask{Fields: &field_mask.FieldMask{Paths: getBuildFieldMask}},
	}
	buildInfo, err := bbClient.GetBuild(ctx, req)
	if err != nil {
		logging.Infof(ctx, "error while getting build info: %s", err)
		return nil, err
	}

	return buildInfo, nil
}

func GenerateNewBuildReqForRetry(ctx context.Context, buildReq *data.BuildRequest, retriableTests map[string]bool) *data.BuildRequest {
	// dereference so that we can make changes
	retryBuildReq := *buildReq
	retryBuildReq.Err = nil
	testCases := retryBuildReq.OriginalTrReq.Tcs

	// If any tauto.tast failure or incomplete result, rerun all the test cases
	if IsAnyTautoTastTestCase(testCases) || len(retriableTests) == 0 {
		logging.Infof(ctx, "Reruning all the test cases...")
		return &retryBuildReq
	}

	// Otherwise create new request with new test cases
	retryBuildReq.ScheduleBuildRequest = nil

	newTcs := []*api.TestCase_Id{}
	for _, tc := range testCases {
		// append the retriable tests and ignore others
		if _, ok := retriableTests[tc.GetValue()]; ok {
			newTcs = append(newTcs, tc)
		}
	}

	retryBuildReq.OriginalTrReq.Tcs = newTcs
	return &retryBuildReq
}

func IsAnyTautoTastTestCase(testCases []*api.TestCase_Id) bool {
	for _, tc := range testCases {
		if strings.HasPrefix(strings.ToLower(tc.GetValue()), "tauto.tast") {
			return true
		}
	}
	return false
}

func determineRetriablity(trResult *skylab_test_runner.Result) (bool, map[string]bool) {
	shouldRetry := false
	retriableTests := map[string]bool{}
	// First check if there are valid trResult
	resultsMap := trResult.GetAutotestResults()
	if len(resultsMap) > 0 {
		shouldRetry = resultsMap["original_test"].GetIncomplete()
		testCases := resultsMap["original_test"].GetTestCases()
		for _, testCase := range testCases {
			if IsTcRetriable(testCase.GetVerdict()) {
				retriableTests[testCase.GetName()] = true
				shouldRetry = true
			}
		}
	}

	return shouldRetry, retriableTests
}

// IsTcRetriable determines if a task result indicates that the test needs to
// be retried.
//
// Panics on unknown verdicts.
func IsTcRetriable(verdict skylab_test_runner.Result_Autotest_TestCase_Verdict) bool {
	switch verdict {
	case skylab_test_runner.Result_Autotest_TestCase_VERDICT_NO_VERDICT,
		skylab_test_runner.Result_Autotest_TestCase_VERDICT_FAIL,
		skylab_test_runner.Result_Autotest_TestCase_VERDICT_ERROR,
		skylab_test_runner.Result_Autotest_TestCase_VERDICT_ABORT:
		return true
	case skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS:
		return false
	default:
		panic(fmt.Sprintf("IsTcRetriable: unknown verdict %s", verdict.String()))
	}
}

func setTopLevelError(ctx context.Context, step *build.Step, result *data.TestResults, resultsChan chan<- *data.TestResults, err error, attemptNode *androidapi.WorkUnitNode, scheduledBuild *buildbucketpb.Build, bbClient buildbucketpb.BuildsClient) {
	logging.Infof(ctx, err.Error())
	step.SetSummaryMarkdown(err.Error())
	result.TopLevelError = err

	// Send the result via channel
	resultsChan <- result
	if attemptNode != nil {
		wu := attemptNode.GetWorkUnit()
		wu.State = common.TaskErrorState
		wu.DebugInfo = &androidbuildinternal.DebugInfo{
			// TODO: Populate this with a wider range of error codes.
			ErrorCode:    1,
			ErrorMessage: err.Error(),
		}
	}

	_ = cancelRunningTask(ctx, scheduledBuild, bbClient, err)

}

func extractResult(from *buildbucketpb.Build, buildReq *data.BuildRequest) (*skylab_test_runner.Result, error) {
	op := from.GetOutput().GetProperties().GetFields()
	if op == nil {
		// return incomplete results so that we don't completely ignore this failure in recipes summarize
		return getIncompleteRunResults(buildReq), fmt.Errorf("output props is empty")
	}
	cr := op["compressed_result"].GetStringValue()
	if cr == "" {
		return nil, fmt.Errorf("compressed_result is empty")
	}
	pb, err := common.Decompress(cr)
	if err != nil {
		return nil, errors.Annotate(err, "extract results from build %d", from.Id).Err()
	}
	var r skylab_test_runner.Result
	if err := proto.Unmarshal(pb, &r); err != nil {
		return nil, errors.Annotate(err, "extract results from build %d", from.Id).Err()
	}
	return &r, nil
}

func getIncompleteRunResults(buildReq *data.BuildRequest) *skylab_test_runner.Result {
	reqTestCases := buildReq.OriginalTrReq.Tcs
	testCases := []*skylab_test_runner.Result_Autotest_TestCase{}
	for _, testCase := range reqTestCases {
		testCases = append(testCases, &skylab_test_runner.Result_Autotest_TestCase{
			Name:    testCase.GetValue(),
			Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_NO_VERDICT,
		})
	}

	return &skylab_test_runner.Result{
		Harness: &skylab_test_runner.Result_AutotestResult{
			AutotestResult: &skylab_test_runner.Result_Autotest{
				TestCases:  testCases,
				Incomplete: true,
			},
		},
	}

}

// FindBuildName finds build name from suite info.
func FindBuildName(suiteInfo *api.SuiteInfo, board string) string {
	for _, target := range suiteInfo.GetSuiteMetadata().GetTargetRequirements() {
		// [0] is bc multi-dut
		hwDef := target.GetHwRequirements().GetHwDefinition()[0]
		if hwDef.GetDutInfo().GetChromeos().GetDutModel().GetBuildTarget() == board {
			return target.GetSwRequirement().GetBuild()
		}
	}

	return ""
}

// ------------ analytics funcs -------

func (cmd *ScheduleTasksCmd) ObserveCmdStart(ctx context.Context) {
	cmd.StartCmdTime = time.Now()
	bqData := &analytics.BqData{Step: string(cmd.GetCommandType()), Status: analytics.Start}
	analytics.SoftInsertStepWInternalPlan(ctx, cmd.BQClient, bqData, cmd.InternalTestPlan, cmd.BuildState)
}

func (cmd *ScheduleTasksCmd) ObserveCmdEndSuccess(ctx context.Context) {
	bqData := &analytics.BqData{Step: string(cmd.GetCommandType()), Status: analytics.Success, Duration: float32(time.Since(cmd.StartCmdTime).Seconds())}
	analytics.SoftInsertStepWInternalPlan(ctx, cmd.BQClient, bqData, cmd.InternalTestPlan, cmd.BuildState)
}

func (cmd *ScheduleTasksCmd) ObserveSchedulerSetupSuccess(ctx context.Context) {
	bqData := &analytics.BqData{Step: "SchedulerSetup", Status: analytics.Success, Duration: float32(time.Since(cmd.StartCmdTime).Seconds())}
	analytics.SoftInsertStepWInternalPlan(ctx, cmd.BQClient, bqData, cmd.InternalTestPlan, cmd.BuildState)
}

func (cmd *ScheduleTasksCmd) ObserveSchedulerSetupFailure(ctx context.Context, err string) {
	bqData := &analytics.BqData{Step: "SchedulerSetup", Status: analytics.Fail, Duration: float32(time.Since(cmd.StartCmdTime).Seconds()), Freeform: err}
	analytics.SoftInsertStepWInternalPlan(ctx, cmd.BQClient, bqData, cmd.InternalTestPlan, cmd.BuildState)
}

func (cmd *ScheduleTasksCmd) ObserveTrSchedulingStart(ctx context.Context, buildReq *data.BuildRequest) {
	data := &analytics.TaskData{
		Step:          "ScheduleBuild",
		DisplayName:   buildReq.Key,
		AnalyticsName: buildReq.SuiteInfo.GetSuiteRequest().GetAnalyticsName(),
		Status:        analytics.Start,
	}
	analytics.SoftInsertStepWTrReq(ctx, cmd.BQClient, data, buildReq.OriginalTrReq, buildReq.SuiteInfo, cmd.BuildState)
}

func (cmd *ScheduleTasksCmd) ObserveTrSchedulingSuccess(ctx context.Context, buildReq *data.BuildRequest, taskBBId string, start time.Time) {
	data := &analytics.TaskData{
		Step:          "ScheduleBuild",
		DisplayName:   buildReq.Key,
		AnalyticsName: buildReq.SuiteInfo.GetSuiteRequest().GetAnalyticsName(),
		Status:        analytics.Success,
		TrTaskID:      taskBBId,
		Duration:      float32(time.Since(start).Seconds()),
	}
	analytics.SoftInsertStepWTrReq(ctx, cmd.BQClient, data, buildReq.OriginalTrReq, buildReq.SuiteInfo, cmd.BuildState)
}

func (cmd *ScheduleTasksCmd) ObserveTrSchedulingFail(ctx context.Context, buildReq *data.BuildRequest, err string, start time.Time) {
	data := &analytics.TaskData{
		Step:          "ScheduleBuild",
		DisplayName:   buildReq.Key,
		AnalyticsName: buildReq.SuiteInfo.GetSuiteRequest().GetAnalyticsName(),
		Status:        analytics.Fail,
		Freeform:      err,
		Duration:      float32(time.Since(start).Seconds()),
	}
	analytics.SoftInsertStepWTrReq(ctx, cmd.BQClient, data, buildReq.OriginalTrReq, buildReq.SuiteInfo, cmd.BuildState)
}

func (cmd *ScheduleTasksCmd) ObserveTrBuildStart(ctx context.Context, buildReq *data.BuildRequest) {
	data := &analytics.TaskData{
		Step:          "RunBuild",
		DisplayName:   buildReq.Key,
		AnalyticsName: buildReq.SuiteInfo.GetSuiteRequest().GetAnalyticsName(),
		Status:        analytics.Start,
	}
	analytics.SoftInsertStepWTrReq(ctx, cmd.BQClient, data, buildReq.OriginalTrReq, buildReq.SuiteInfo, cmd.BuildState)
}

func (cmd *ScheduleTasksCmd) ObserveTrBuildSuccess(ctx context.Context, buildReq *data.BuildRequest, buildInfo *buildbucketpb.Build, start time.Time) {
	data := &analytics.TaskData{
		Step:          "RunBuild",
		DisplayName:   buildReq.Key,
		AnalyticsName: buildReq.SuiteInfo.GetSuiteRequest().GetAnalyticsName(),
		Status:        analytics.Success,
		TrTaskID:      fmt.Sprint(buildInfo.GetId()),
		Freeform:      fmt.Sprint("build status: ", buildInfo.GetStatus()),
		Duration:      float32(time.Since(start).Seconds()),
	}
	analytics.SoftInsertStepWTrReq(ctx, cmd.BQClient, data, buildReq.OriginalTrReq, buildReq.SuiteInfo, cmd.BuildState)
}

func (cmd *ScheduleTasksCmd) ObserveTrBuildFail(ctx context.Context, buildReq *data.BuildRequest, err string, start time.Time) {
	data := &analytics.TaskData{
		Step:          "RunBuild",
		DisplayName:   buildReq.Key,
		AnalyticsName: buildReq.SuiteInfo.GetSuiteRequest().GetAnalyticsName(),
		Status:        analytics.Fail,
		Freeform:      err,
		Duration:      float32(time.Since(start).Seconds()),
	}
	analytics.SoftInsertStepWTrReq(ctx, cmd.BQClient, data, buildReq.OriginalTrReq, buildReq.SuiteInfo, cmd.BuildState)
}

// ----------- end analytics funcs --------

func NewScheduleTasksCmd() *ScheduleTasksCmd {
	abstractCmd := interfaces.NewAbstractCmd(ScheduleTasksCmdType)
	abstractSingleCmdByNoExecutor := &interfaces.AbstractSingleCmdByNoExecutor{AbstractCmd: abstractCmd}
	return &ScheduleTasksCmd{AbstractSingleCmdByNoExecutor: abstractSingleCmdByNoExecutor}
}

// buildTag returns a build's BB tag based on the specific tag name, and a
// bool indicating whether it was found.
func (cmd *ScheduleTasksCmd) buildTag(tagName string) *buildbucketpb.StringPair {
	for _, t := range cmd.BuildState.Build().GetTags() {
		if t.GetKey() == tagName {
			return t
		}
	}
	return nil
}
