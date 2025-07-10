// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package suitelimits implements the tooling to limit CTP requests to 3 DUT
// hours in total. This project was implemented, inline, in CTPv1 but is being
// generalized here.
package suitelimits

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	buildbucket "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/logging"
)

const (
	hour                  = 60 * 60
	DutHourMaximumSeconds = 3 * hour

	dutPoolQuota     = "DUT_POOL_QUOTA"
	managedPoolQuota = "MANAGED_POOL_QUOTA"
	quota            = "quota"
	schedukePool     = "schedukeTest"
)

type suiteLimitEntry struct {
	suiteName        string
	pool             string
	exemptionGranted bool
	totalDUTHours    time.Duration
	perTaskLastSeen  map[int64]time.Time
}

type RequestKey string

// HWTarget is a string int he form of [<board>([-]?<variant>)]
type HWTarget string

type suiteLimitCache struct {
	// cache  map[string]*suiteLimitEntry
	cache  map[RequestKey]map[HWTarget]*suiteLimitEntry
	access sync.Mutex
}

var (
	// suiteLimitsTracker is the cache that we will continually update to track each
	// request's total DUT hour usage. It's string key will be determined by the
	// caller but must represent a unique request.
	suiteLimitsTracker = suiteLimitCache{
		cache:  map[RequestKey]map[HWTarget]*suiteLimitEntry{},
		access: sync.Mutex{},
	}
)

// IsExempt determines if the entry is exempt from the SuiteLimits restrictions.
// Eligible exemptions are:
//  1. Request is running in a private pool
//  2. Explicit exemption granted in the config file
//
// NOTE: must only be called within a mutex locked outer function.
func IsExempt(ctx context.Context, entry *suiteLimitEntry) bool {
	// If exemption is already granted then skip checking eligibility again.
	if entry.exemptionGranted {
		return true
	}

	// If the pool is running in a private pool then we will not enact
	// SuiteLimit's rules on it.
	if entry.pool != dutPoolQuota && entry.pool != managedPoolQuota && entry.pool != quota && entry.pool != schedukePool {
		logging.Infof(ctx, "suite entry %s is running in a private pool %s and is exempted from SuiteLimits", entry.suiteName, entry.pool)
		entry.exemptionGranted = true
		return true
	}

	// If the request's suite has been granted an explicit exemption then do not
	// enact SuiteLimit's rules on it.
	for _, exemption := range exemptions {
		if entry.suiteName == exemption.suiteName {
			logging.Infof(ctx, "suite entry %s has an explicit exemption and is exempted from SuiteLimits", entry.suiteName)
			entry.exemptionGranted = true
			return true
		}
	}

	// If non of the approved exemptions have applied for this request then it
	// is not exempt from Suite Limits.
	logging.Infof(ctx, "suite entry %s is running in the shared pool and has no exemption given", entry.suiteName)
	return false
}

// AddRequestTask adds a new tracking bbid for the request set with the start
// time set to the current time.time value when the addition is made.
func AddRequestTask(ctx context.Context, requestKey, suiteName, hwTarget, pool string, taskBBID int64, isAL bool) error {
	suiteLimitsTracker.access.Lock()
	defer suiteLimitsTracker.access.Unlock()

	logging.Infof(ctx, "Adding BBID %d to requestKey %s and target %s tracking for pool %s. Traffic is AL: %t\n", taskBBID, requestKey, hwTarget, pool, isAL)

	// If this is the first addition of the suite into the map then create the
	// full path to the entry.
	if _, ok := suiteLimitsTracker.cache[RequestKey(requestKey)]; !ok {
		suiteLimitsTracker.cache[RequestKey(requestKey)] = map[HWTarget]*suiteLimitEntry{}
	}

	if _, ok := suiteLimitsTracker.cache[RequestKey(requestKey)][HWTarget(hwTarget)]; !ok {
		newEntry := &suiteLimitEntry{
			suiteName:       suiteName,
			totalDUTHours:   0,
			pool:            pool,
			perTaskLastSeen: map[int64]time.Time{},
		}

		// At initial map insertion, check if the entry qualifies for a
		// SuiteLimits exemption.
		//
		// NOTE: For now all AL testing is exempt from SuiteLimits.
		newEntry.exemptionGranted = isAL || IsExempt(ctx, newEntry)

		suiteLimitsTracker.cache[RequestKey(requestKey)][HWTarget(hwTarget)] = newEntry
	}

	requestEntry := suiteLimitsTracker.cache[RequestKey(requestKey)][HWTarget(hwTarget)]
	// Set the startTime for the task request to the current time.
	startTime := time.Now()
	requestEntry.perTaskLastSeen[taskBBID] = startTime

	// Send an initial entry to the metrics log.
	metricsChan <- trackingMetric{
		requestKey:      requestKey,
		suiteName:       suiteName,
		taskName:        hwTarget,
		taskBBID:        taskBBID,
		lastSeen:        startTime,
		currentlySeenAt: startTime,
		delta:           0,
	}

	return nil
}

// UpdateTotalTime updates the total request time of the passed in unique
// request set by the amount of time it's been since the passed in BBID has been
// seen.
func UpdateTotalTime(ctx context.Context, requestKey, suiteName, taskName string, taskBBID int64) (bool, error) {
	suiteLimitsTracker.access.Lock()
	defer suiteLimitsTracker.access.Unlock()

	hwMap, ok := suiteLimitsTracker.cache[RequestKey(requestKey)]
	if !ok {
		return false, fmt.Errorf("requestKey %s was not seen in the tracking cache", requestKey)
	}

	requestEntry, ok := hwMap[HWTarget(taskName)]
	if !ok {
		return false, fmt.Errorf("task %s was not seen in the tracking cache for requestKey %s", taskName, requestKey)
	}

	if _, ok := requestEntry.perTaskLastSeen[taskBBID]; !ok {
		return false, fmt.Errorf("bbid %d was not seen in the tracking cache for requestKey %s and task %s", taskBBID, requestKey, taskName)
	}

	// requestEntry := hwMap[HWTarget(taskName)]

	// Add the difference in time since the last tick to the total DUT hour time
	// for the request.
	lastSeen := requestEntry.perTaskLastSeen[taskBBID]
	timeDelta := time.Since(lastSeen)
	requestEntry.totalDUTHours += timeDelta

	// Update the last seen time to the current time.
	updateTime := time.Now()
	requestEntry.perTaskLastSeen[taskBBID] = updateTime
	item := trackingMetric{
		requestKey:      requestKey,
		suiteName:       suiteName,
		taskName:        taskName,
		taskBBID:        taskBBID,
		lastSeen:        lastSeen,
		currentlySeenAt: updateTime,
		delta:           timeDelta,
	}

	// Send a metric log of the update task.
	metricsChan <- item

	// If the suite has exceeded the limit and is non-exempt return true to
	// signal the cancel tasks start.
	if !requestEntry.exemptionGranted && int(requestEntry.totalDUTHours.Seconds()) >= DutHourMaximumSeconds {
		logging.Infof(ctx, "no exemption granted and total dut time of %f seconds exceeded %d maximum\n", requestEntry.totalDUTHours.Seconds(), DutHourMaximumSeconds)
		return true, nil
	}

	return false, nil
}

func getRequestEntry(requestKey, taskName string) (*suiteLimitEntry, error) {
	hwMap, ok := suiteLimitsTracker.cache[RequestKey(requestKey)]
	if !ok {
		return nil, fmt.Errorf("no entry in suite limits for the requestName %s", requestKey)
	}

	requestEntry, ok := hwMap[HWTarget(taskName)]
	if !ok {
		return nil, fmt.Errorf("no entry in suite limits for the requestName %s", requestKey)
	}

	return requestEntry, nil
}

// GetTotalDUTHours returns the total DUT hour usage for the given request.
func GetTotalDUTHours(requestKey, taskName string) (time.Duration, error) {
	suiteLimitsTracker.access.Lock()
	defer suiteLimitsTracker.access.Unlock()

	requestEntry, err := getRequestEntry(requestKey, taskName)
	if err != nil {
		return 0, err
	}

	return requestEntry.totalDUTHours, nil

}

// ExceededLimit returns if the provided request has exceeded the DUT hour
// limit.
func ExceededLimit(requestKey, taskName string) (bool, error) {
	suiteLimitsTracker.access.Lock()
	defer suiteLimitsTracker.access.Unlock()

	requestEntry, err := getRequestEntry(requestKey, taskName)
	if err != nil {
		return false, err
	}

	return (requestEntry.totalDUTHours.Seconds() >= DutHourMaximumSeconds), nil
}

// HasExemption returns if the provided request has an active exemption from the
// Suite Limits service.
func HasExemption(requestKey, taskName string) (bool, error) {
	suiteLimitsTracker.access.Lock()
	defer suiteLimitsTracker.access.Unlock()

	requestEntry, err := getRequestEntry(requestKey, taskName)
	if err != nil {
		return false, err
	}

	return requestEntry.exemptionGranted, nil
}

// CancelTasks sends a BuildBucket Cancel request for each bbid being tracked in
// the entry. This should only be called when the request has gone past the
// maximum allowed DUT hour time and it does not have an active exemption.
func CancelTasks(ctx context.Context, requestKey, taskName string, bbClient buildbucket.BuildsClient) error {
	suiteLimitsTracker.access.Lock()
	defer suiteLimitsTracker.access.Unlock()

	requestEntry, err := getRequestEntry(requestKey, taskName)
	if err != nil {
		return err
	}

	for bbid := range requestEntry.perTaskLastSeen {
		request := &buildbucket.CancelBuildRequest{
			Id:              bbid,
			SummaryMarkdown: "SUITE EXECUTION TIME LIMIT EXCEEDED: go/suitelimits-faqs",
		}

		_, err := bbClient.CancelBuild(ctx, request)
		if err != nil {
			return err
		}
	}

	return nil
}

// ExtractTarget extracts the target key from the shard name so that
// we can use it to fetch from requestToTargetChainMap.
func ExtractTarget(shardName string) (string, error) {
	index := strings.IndexRune(shardName, ']')

	if index == -1 {
		return "", fmt.Errorf("the shard name %s was improperly formatted. it did not follow the form of [<targetName]-shard-N", shardName)
	}

	return shardName[:index+1], nil
}
