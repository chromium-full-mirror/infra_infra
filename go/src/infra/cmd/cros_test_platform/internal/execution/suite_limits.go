// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package execution

import (
	"context"
	"errors"
	"strings"
	"time"

	buildbucket "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cmd/cros_test_platform/internal/execution/build"
	trservice "go.chromium.org/infra/cmd/cros_test_platform/internal/execution/testrunner/service"
	"go.chromium.org/infra/cmd/cros_test_platform/internal/execution/types"
)

const (
	hour                             = 60 * 60
	day                              = 24 * hour
	suiteTestExecutionMaximumSeconds = 3 * hour
	dutPoolQuota                     = "DUT_POOL_QUOTA"
	managedPoolQuota                 = "MANAGED_POOL_QUOTA"
	quota                            = "quota"
	suiteLimitMinimumMilestone       = 117
)

var errSuiteLimit = errors.New("TestExecutionLimit: Maximum suite execution runtime exceeded.")

// cancelExceededTests sends a buildbucket cancellation request to all active testrunners for the given suite(taskSet).
func cancelExceededTests(ctx context.Context, client trservice.Client, taskSetName string, taskSet *RequestTaskSet) error {
	// Aggregate all the references of tasks to be cancelled.
	taskIds := []trservice.TaskReference{}
	for _, task := range taskSet.activeTasks {
		taskIds = append(taskIds, task.TaskReference)
	}
	err := client.CancelTasks(ctx, taskIds, "SUITE EXECUTION TIME LIMIT EXCEEDED: go/suitelimits-faqs")
	if err != nil {
		return err
	}

	// Remove all tasks from the active task set
	for iid := range taskSet.activeTasks {
		taskSet.step.Close(buildbucket.Status_FAILURE, build.ExceededExecutionTimeText)
		taskSet.invocationSteps[iid].AddCancelledSummary()

		delete(taskSet.activeTasks, iid)
	}

	return err
}

// checkForException iterates through the exceptions list to see if we should
// allow the suite to continue running.
func checkForException(ctx context.Context, suiteName string, request *RequestTaskSet) bool {
	// Iterate through the exceptions list.
	for _, exception := range exceptions {
		// Continue early if current exception doesn't apply.
		if !strings.HasSuffix(suiteName, exception.suiteName) {
			continue
		}

		// Exception has expired but has not been removed from the list. Explain
		// the failure in the summary markdown.
		if exception.expiration.Before(time.Now()) {
			request.step.DisplayExceptionExpiredSummary(exception.expiration)
			return false
		}

		// Show exception information in the summary markdown.
		request.step.DisplayExceptionSummary(exception.expiration)

		// Active exception found.
		return true
	}

	// No Exception Found
	return false
}

// isQuotaPool checks to see if the given pool is within one of the shared HW pools.
func isQuotaPool(pool string) bool {
	return (strings.Contains(pool, dutPoolQuota) || strings.Contains(pool, managedPoolQuota) || strings.Contains(pool, quota))
}

// isEligibleForSuiteLimits cross checks the requested run against qualifications for being limited.
// To qualify for suite runtime limitation:
//  1. The suite must be in the shared pool(DUT_POOL_QUOTA, MANAGED_POOL_QUOTA)
//  2. The milestone must be >= R117
//  3. The suite must not be granted an exception.
func isEligibleForSuiteLimits(ctx context.Context, iid types.InvocationID, taskSetName string, request *RequestTaskSet) bool {
	if request.SuiteLimitExceptionGranted {
		return false
	}
	// In some cases single suite runs are listed as default. This will extract the real SuiteName from the requirement.
	suiteName, err := request.GetSuiteName(iid)

	// TODO(b/293153310): Single tests will temporarily exempt from SuiteLimits because CTS needs to run individual tests manually to avoid re-running a full suite.
	// Some runs do not launch suites and run individual tests. In that case we will default to the task set name chosen by CTP.
	if err != nil {
		logging.Infof(ctx, "No Suite found for %s\n", taskSetName)
		request.SuiteLimitExceptionGranted = true
		return false
	}

	// Assume in shared pool, if label-pool isn't found then the pool will be treated as if it is in the shared pools.
	inSharedPool := true
	pool, err := request.GetTestRunnerPool(iid)
	if err != nil {
		logging.Infof(ctx, "%s\n", err.Error())
	} else {
		inSharedPool = isQuotaPool(pool)
	}

	// If its in a private pool then SuiteLimits do no apply to the run.
	if !inSharedPool {
		logging.Infof(ctx, "Suite Limits: Running in private pool.\n")
		request.SuiteLimitExceptionGranted = true
		return false
	}

	// If the milestone is less than 117 then do not apply suite limits. If we cannot determine the milestone then consider the milestone < 117.
	if milestone, err := request.GetMilestone(iid); err != nil || milestone < suiteLimitMinimumMilestone {
		if err != nil {
			logging.Infof(ctx, "SuiteLimits Milestone error: %s\n", err.Error())
		}
		logging.Infof(ctx, "Suite Limits: Milestone exception milestone(%d)\n", milestone)
		request.SuiteLimitExceptionGranted = true
		return false

	}

	// If an exception was found for the suite then do not check if the execution limit was exceeded.
	if checkForException(ctx, suiteName, request) {
		logging.Infof(ctx, "Suite Limits: SuiteLimits exemption found for suite %s in task set %s\n", suiteName, taskSetName)
		request.SuiteLimitExceptionGranted = true
		return false
	}

	logging.Infof(ctx, "Suite Limits: No SuiteLimits exemption found for suite %s\n", suiteName)
	return true
}

// updateTestExecutionTracking calculates the amount of time it's been since
// this task was last seen. It then increase the global value tracking test
// execution and returns the updated map with a new timestamp for the current iid.
func updateTestExecutionTracking(ctx context.Context, iid types.InvocationID, lastSeen time.Time, taskSetName string, request *RequestTaskSet, completed bool, logChan chan trackingMetric) error {

	// Mark the current time for calculation of the duration since last seen.
	currentlySeenAt := time.Now()
	lastSeenRuntimePerTask[taskSetName].lastSeenMap[iid] = currentlySeenAt

	// Calculate the duration of time it has been since the last time we've seen
	// this iid running.
	delta := lastSeenRuntimePerTask[taskSetName].lastSeenMap[iid].Sub(lastSeen)

	// Increase the total test execution time for the suite.
	lastSeenRuntimePerTask[taskSetName].totalSuiteTrackingTime += delta

	// Add update to the log.
	logChan <- trackingMetric{
		suiteName:       taskSetName,
		taskName:        string(iid),
		lastSeen:        lastSeen,
		currentlySeenAt: currentlySeenAt,
		delta:           delta,
		completed:       completed,
	}

	// Check the run for eligibility according to the SuiteLimit rules.
	if !isEligibleForSuiteLimits(ctx, iid, taskSetName, request) {
		return nil
	}

	// Finally, check if we've exceeded the maximum time allowed for test execution.
	if lastSeenRuntimePerTask[taskSetName].totalSuiteTrackingTime.Seconds() > suiteTestExecutionMaximumSeconds {
		suiteName, err := request.GetSuiteName(iid)
		if err != nil {
			suiteName = taskSetName
		}
		logging.Infof(ctx, "No exemption for suite %s found. TaskSet %s exceeded execution runtime limit. %d seconds allowed, %d seconds used.", suiteName, taskSetName, suiteTestExecutionMaximumSeconds, int(lastSeenRuntimePerTask[taskSetName].totalSuiteTrackingTime.Seconds()))
		return errSuiteLimit
	}

	return nil
}
