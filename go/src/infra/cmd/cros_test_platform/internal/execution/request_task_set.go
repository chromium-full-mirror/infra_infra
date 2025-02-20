// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package execution

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/config"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/steps"
	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	luci_retry "go.chromium.org/luci/common/retry"
	"go.chromium.org/luci/common/retry/transient"
	"go.chromium.org/luci/grpc/grpcutil"

	"go.chromium.org/infra/cmd/cros_test_platform/internal/execution/args"
	"go.chromium.org/infra/cmd/cros_test_platform/internal/execution/build"
	"go.chromium.org/infra/cmd/cros_test_platform/internal/execution/response"
	"go.chromium.org/infra/cmd/cros_test_platform/internal/execution/retry"
	"go.chromium.org/infra/cmd/cros_test_platform/internal/execution/testrunner"
	trservice "go.chromium.org/infra/cmd/cros_test_platform/internal/execution/testrunner/service"
	"go.chromium.org/infra/cmd/cros_test_platform/internal/execution/types"
)

// Retry count on transient errors
const (
	RetryCountOnTransientError = 5

	completed = true
	running   = false
)

// suiteTestExecutionTrackerEntry will allow for easy access to the total
// tracking time per suite.
type suiteTestExecutionTrackerEntry struct {
	allDone                bool
	totalSuiteTrackingTime time.Duration
	lastSeenMap            map[types.InvocationID]time.Time
}

// Map recording the last time each task was seen running.
var lastSeenRuntimePerTask = map[string]*suiteTestExecutionTrackerEntry{}

// RequestTaskSet encapsulates the running state of the set of tasks for one
// cros_test_platform request.
type RequestTaskSet struct {
	// Unique names for invocations preserving the order of incoming arguments.
	// This is used to preserve the order in the response.
	invocationIDs []types.InvocationID

	argsGenerators      map[types.InvocationID]*args.Generator
	invocationResponses map[types.InvocationID]*response.Invocation
	activeTasks         map[types.InvocationID]*testrunner.Build
	retryCounter        retry.Counter

	step            *build.RequestStepUpdater
	invocationSteps map[types.InvocationID]*build.InvocationStepUpdater

	launched                   bool
	SuiteLimitExceptionGranted bool
}

// TaskSetConfig is a wrapper for the parameters common to the testTaskSets.
type TaskSetConfig struct {
	ParentTaskID        string
	ParentBuildID       int64
	RequestUID          string
	Deadline            time.Time
	StatusUpdateChannel *config.Config_PubSub
}

// NewRequestTaskSet creates a new RequestTaskSet.
func NewRequestTaskSet(
	name string,
	buildInstance *bbpb.Build,
	workerConfig *config.Config_SkylabWorker,
	tc *TaskSetConfig,
	params *test_platform.Request_Params,
	tests []*steps.EnumerationResponse_AutotestInvocation,
	pool string,
) (*RequestTaskSet, error) {

	step := build.NewRequestStep(name, buildInstance)

	invocationIDs := make([]types.InvocationID, len(tests))
	invocationResponses := make(map[types.InvocationID]*response.Invocation)
	argsGenerators := make(map[types.InvocationID]*args.Generator)
	invocationSteps := make(map[types.InvocationID]*build.InvocationStepUpdater)
	tm := make(map[types.InvocationID]*steps.EnumerationResponse_AutotestInvocation)
	for i, test := range tests {
		iid := types.NewInvocationID(i, test)
		invocationIDs[i] = iid
		argsGenerators[iid] = &args.Generator{
			Invocation:          test,
			Params:              params,
			WorkerConfig:        workerConfig,
			ParentTaskID:        tc.ParentTaskID,
			ParentBuildID:       tc.ParentBuildID,
			ParentRequestUID:    tc.RequestUID,
			Deadline:            tc.Deadline,
			StatusUpdateChannel: tc.StatusUpdateChannel,
			Experiments:         buildInstance.GetInput().GetExperiments(),
			GerritChanges:       buildInstance.GetInput().GetGerritChanges(),
			SwarmingPool:        pool,
			BuildInstance:       buildInstance,
		}
		// test, params, workerConfig, tc.ParentTaskID, tc.RequestUID, tc.Deadline)
		invocationResponses[iid] = response.NewInvocation(test.GetTest().GetName())
		invocationSteps[iid] = step.NewInvocationStep(test.GetTest().GetName())
		tm[iid] = test
	}
	return &RequestTaskSet{
		argsGenerators:      argsGenerators,
		invocationIDs:       invocationIDs,
		invocationResponses: invocationResponses,
		activeTasks:         make(map[types.InvocationID]*testrunner.Build),
		retryCounter:        retry.NewCounter(params, tm),
		invocationSteps:     invocationSteps,
		step:                step,
	}, nil
}

var MilestoneNotFoundError = errors.New("RequestTaskSet: No milestone found")
var MilestoneFormatError = errors.New("RequestTaskSet: image filed incofrectly formed, milestone could not be determined.")

// GetMilestone returns the milestone requirement of the invocation run.
func (r *RequestTaskSet) GetMilestone(iid types.InvocationID) (int64, error) {
	image := ""
	software_deps := r.argsGenerators[iid].Params.GetSoftwareDependencies()
	for _, dep := range software_deps {
		switch d := dep.Dep.(type) {
		case *test_platform.Request_Params_SoftwareDependency_ChromeosBuild:
			image = d.ChromeosBuild
		default:
			break
		}

		// Image value found, stop iterating.
		if image != "" {
			break
		}
	}

	if image == "" {
		return 0, MilestoneNotFoundError
	}

	re := regexp.MustCompile(`\/R(?P<milestone>\d{2,3})-\d*`)
	matches := re.FindStringSubmatch(image)

	// Nil means no matches found.
	// The first match is the full text captured and the second is the capture group defined.
	if len(matches) == 2 {
		milestone, err := strconv.ParseInt(matches[1], 10, 0)
		if err != nil {
			return 0, err
		}
		return milestone, nil
	}

	return 0, MilestoneFormatError
}

// GetSuiteName returns the testing suite in the of the given request.
func (r *RequestTaskSet) GetSuiteName(iid types.InvocationID) (string, error) {
	for _, tag := range r.argsGenerators[iid].Params.Decorations.Tags {
		if strings.HasPrefix(tag, "label-suite:") {
			return strings.TrimPrefix(tag, "label-suite:"), nil
		} else if strings.HasPrefix(tag, "suite:") {
			return strings.TrimPrefix(tag, "suite:"), nil
		}
	}
	return "", fmt.Errorf("suite tag not found.")
}

// GetSuiteName returns the HW pool testrunner will use for testing.
func (r *RequestTaskSet) GetTestRunnerPool(iid types.InvocationID) (string, error) {
	for _, tag := range r.argsGenerators[iid].Params.Decorations.Tags {
		if strings.HasPrefix(tag, "label-pool:") {
			return strings.TrimPrefix(tag, "label-pool:"), nil
		} else if strings.HasPrefix(tag, "pool:") {
			return strings.TrimPrefix(tag, "pool:"), nil
		}
	}
	return "", fmt.Errorf("Pool tag not found.")
}

// completed returns true if all tasks for this request have completed.
func (r *RequestTaskSet) completed() bool {
	return r.launched && len(r.activeTasks) == 0
}

// LaunchTasks launches initial tasks for all the tests in this request.
func (r *RequestTaskSet) LaunchTasks(ctx context.Context, c trservice.Client) error {
	r.launched = true
	for _, iid := range r.invocationIDs {
		ts := r.getInvocationResponse(iid)
		ag := r.getArgsGenerator(iid)

		if rejected, err := testrunner.ValidateDependencies(ctx, c, ag); err != nil {
			if !testrunner.InvalidDependencies.In(err) {
				return err
			}
			logging.Warningf(ctx, "Dependency validation failed for %s: %s.", ts.Name, err)
			ts.MarkNotRunnable(rejected)
			continue
		}

		task, err := r.createNewBuildWithRetry(ctx, &c, ag, ts.Name)
		if err != nil {
			return errors.Annotate(err, "Error during new test_runner build creation for %s", ts.Name).Err()
		}
		ts.NotifyTask(task)
		r.getInvocationStep(iid).NotifyNewTask(task)
		r.activeTasks[iid] = task
	}
	return nil
}

// retryParams defines retry strategy for handling transient errors
func (r *RequestTaskSet) retryParams() luci_retry.Iterator {
	return &luci_retry.ExponentialBackoff{
		Limited: luci_retry.Limited{
			Delay:    10 * time.Second,
			Retries:  RetryCountOnTransientError,
			MaxTotal: 2 * time.Minute,
		},
		Multiplier: 2,
	}
}

// createNewBuildWithRetry attempts to create new test_runner build. It retries if transient error occurs.
func (r *RequestTaskSet) createNewBuildWithRetry(ctx context.Context, c *trservice.Client, ag *args.Generator, taskName string) (task *testrunner.Build, err error) {
	err = luci_retry.Retry(ctx, transient.Only(r.retryParams), func() error {
		task, err = testrunner.NewBuild(ctx, *c, ag, nil)
		if err != nil {
			if r.isTransientError(ctx, err) {
				logging.Infof(ctx, "Transient error occured for %s: %s", taskName, err.Error())
				return transient.Tag.Apply(err)
			} else {
				logging.Infof(ctx, "Found a non-transient error for %s: %s", taskName, err.Error())
			}
		}
		return err
	}, luci_retry.LogCallback(ctx, "create-new-test_runner-build"))
	return
}

// isTransientError returns if provided error is transient or not
func (r *RequestTaskSet) isTransientError(ctx context.Context, err error) bool {
	if s, ok := status.FromError(err); ok {
		//return grpcutil.IsTransientCode(s.Code()) || s.Code() == codes.DeadlineExceeded
		if grpcutil.IsTransientCode(s.Code()) {
			logging.Infof(ctx, "Found status in grpc transient errors")
			return true
		}
		if s.Code() == codes.DeadlineExceeded {
			logging.Infof(ctx, "Found DeadlineExceeded")
		}
	} else {
		logging.Infof(ctx, "Failed to find error status")
	}
	return false
}

func (r *RequestTaskSet) getInvocationResponse(iid types.InvocationID) *response.Invocation {
	ir, ok := r.invocationResponses[iid]
	if !ok {
		panic(fmt.Sprintf("No test task set for invocation %s", iid))
	}
	return ir
}

func (r *RequestTaskSet) getArgsGenerator(iid types.InvocationID) *args.Generator {
	ag, ok := r.argsGenerators[iid]
	if !ok {
		panic(fmt.Sprintf("No args.Generator for invocation %s", iid))
	}
	return ag
}

func (r *RequestTaskSet) getInvocationStep(iid types.InvocationID) *build.InvocationStepUpdater {
	s, ok := r.invocationSteps[iid]
	if !ok {
		panic(fmt.Sprintf("No step for invocation %s", iid))
	}
	return s
}

// CheckTasksAndRetry checks the status of currently running tasks for this
// request and retries failed tasks when allowed.
//
// Returns whether all tasks are complete (so future calls to this function are
// unnecessary)
func (r *RequestTaskSet) CheckTasksAndRetry(ctx context.Context, c trservice.Client, taskSetName string, logChan chan trackingMetric) (bool, error) {
	completedTests := make(map[types.InvocationID]bool, len(r.activeTasks))
	newTasks := make(map[types.InvocationID]*testrunner.Build)

	for iid, task := range r.activeTasks {
		rerr := task.Refresh(ctx, c)
		tr := task.Result()
		if rerr != nil {
			return false, errors.Annotate(rerr, "tick for task %s", tr.LogUrl).Err()
		}

		// If the task is running then update out execution limit tracking log.
		if task.Running() {
			// Grab suite entry that was pre-made in outer execution loop.
			entry := lastSeenRuntimePerTask[taskSetName]

			// If the current iid(task) is being tracked update it's log, otherwise
			// create a log.
			if lastSeen, iidTracked := entry.lastSeenMap[iid]; iidTracked {
				rerr = updateTestExecutionTracking(ctx, iid, lastSeen, taskSetName, r, running, logChan)
				// If we've exceeded the limit, fail the run.
				if rerr != nil {
					return false, rerr
				}
			} else {
				logging.Infof(ctx, "Suite tracking: task %s started, adding to task set %s\n", iid, taskSetName)
				entry.lastSeenMap[iid] = time.Now()
			}
		}

		if !task.Completed() {
			continue
		}

		// If the task completed then we need to add time to the tracker one last
		// time then remove it from the tracking map. If it needs to retry then that
		// will be re added to the activeTasks set and then we'll begin the
		// tracking loop once more.
		if task.Completed() {
			// Update the runtime one last time with
			if lastSeen, ok := lastSeenRuntimePerTask[taskSetName].lastSeenMap[iid]; ok {
				rerr = updateTestExecutionTracking(ctx, iid, lastSeen, taskSetName, r, completed, logChan)

				// If we've exceeded the limit, fail the run.
				if rerr != nil {
					return false, rerr
				}

				logging.Infof(ctx, "Suite tracking: task completed removing task %s from suite %s entry\n", iid, taskSetName)
				delete(lastSeenRuntimePerTask[taskSetName].lastSeenMap, iid)
			}
		}

		ts := r.getInvocationResponse(iid)
		logging.Infof(ctx, "Task %s (%s) completed with verdict %s", tr.LogUrl, ts.Name, tr.GetState().GetVerdict())

		// At this point, we've determined that latestTask finished, and we've
		// updated the testTaskSet with its result. We can remove it from our
		// attention set... as long as we don't have to retry.
		shouldRetry := retry.IsNeeded(task.Result()) && r.retryCounter.CanRetry(ctx, iid)
		if !shouldRetry {
			completedTests[iid] = true
			r.getInvocationStep(iid).MarkCompleted()
			delete(r.activeTasks, iid)
			continue
		}

		logging.Infof(ctx, "Retrying %s", ts.Name)
		for _, tc := range tr.TestCases {
			if tc.Verdict != test_platform.TaskState_VERDICT_PASSED {
				logging.Infof(ctx, "Test (%s) did not pass with Verdict %s: %s", tc.Name, tc.Verdict, tc.HumanReadableSummary)
			}
		}
		nt, err := task.Retry(ctx, c, int32(r.retryCounter.RetryCount(iid)+1))
		if err != nil {
			return false, errors.Annotate(err, "tick for task %s: retry test", tr.LogUrl).Err()
		}
		newTasks[iid] = nt
		ts.NotifyTask(nt)
		r.getInvocationStep(iid).NotifyNewTask(nt)
		r.retryCounter.NotifyRetry(iid)
	}

	for iid, task := range newTasks {
		r.activeTasks[iid] = task
	}
	return r.completed(), nil
}

// Close notifies that all execution for this request has completed.
//
// Finalize must be called exactly once to clean up state.
// It is an error to call any methods except Response() on a Close()ed instance.
func (r *RequestTaskSet) Close() {
	r.step.Close(bbpb.Status_SUCCESS, "")
}

// Response returns the current response for this request.
func (r *RequestTaskSet) Response() *steps.ExecuteResponse {
	tss := make([]*response.Invocation, len(r.invocationIDs))
	for i, iid := range r.invocationIDs {
		tss[i] = r.invocationResponses[iid]
	}
	return response.Summarize(tss)
}
