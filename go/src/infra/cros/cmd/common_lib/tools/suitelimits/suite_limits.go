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
	"sync"
	"time"

	buildbucket "go.chromium.org/luci/buildbucket/proto"
)

const (
	hour                  = 60 * 60
	dutHourMaximumSeconds = 3 * hour
	dutPoolQuota          = "DUT_POOL_QUOTA"
	managedPoolQuota      = "MANAGED_POOL_QUOTA"
	quota                 = "quota"
)

type suiteLimitEntry struct {
	exemptionGranted bool
	totalDUTHours    time.Duration
	perTaskLastSeen  map[int64]time.Time
}

type suiteLimitCache struct {
	cache  map[string]*suiteLimitEntry
	access sync.Mutex
}

// suiteLimitsTracker is the cache that we will continually update to track each
// request's total DUT hour usage. It's string key will be determined by the
// caller but must represent a unique request.
var suiteLimitsTracker = suiteLimitCache{}

// AddRequestTask adds a new tracking bbid for the request set with the start
// time set to the current time.time value when the addition is made.
func AddRequestTask(requestName string, taskBBID int64) error {
	suiteLimitsTracker.access.Unlock()
	defer suiteLimitsTracker.access.Lock()

	requestEntry, ok := suiteLimitsTracker.cache[requestName]
	if !ok {
		return fmt.Errorf("requestName %s was not seen in the tracking cache", requestName)
	}

	requestEntry.perTaskLastSeen[taskBBID] = time.Now()

	return nil

}

// UpdateTotalTime updates the total request time of the passed in unique
// request set by the amount of time it's been since the passed in BBID has been
// seen.
func UpdateTotalTime(requestName string, taskBBID int64) (bool, error) {
	suiteLimitsTracker.access.Unlock()
	defer suiteLimitsTracker.access.Lock()

	requestEntry, ok := suiteLimitsTracker.cache[requestName]
	if !ok {
		return false, fmt.Errorf("requestName %s was not seen in the tracking cache", requestName)
	}

	if _, ok := requestEntry.perTaskLastSeen[taskBBID]; !ok {
		return false, fmt.Errorf("bbid %d was not seen in the tracking cache for request %s", taskBBID, requestName)
	}

	// Add the difference in time since the last tick to the total DUT hour time
	// for the request.
	lastSeen := requestEntry.perTaskLastSeen[taskBBID]
	requestEntry.totalDUTHours += time.Since(lastSeen)

	// Update the last seen time to the current time.
	requestEntry.perTaskLastSeen[taskBBID] = time.Now()

	if requestEntry.totalDUTHours >= dutHourMaximumSeconds {
		return true, nil
	}

	return false, nil
}

// IsExempt determines if the entry is exempt from the SuiteLimits restrictions.
// Eligible exemptions are:
//  1. Request is running in a private pool
//  2. Explicit exemption granted in the config file
func IsExempt(pool, suiteName string, entry *suiteLimitEntry) bool {
	// If exemption is already granted then skip checking eligibility again.
	if entry.exemptionGranted {
		return true
	}

	// If the pool is running in a private pool then we will not enact
	// SuiteLimit's rules on it.
	if pool != dutPoolQuota && pool != managedPoolQuota && pool != quota {
		entry.exemptionGranted = true
		return true
	}

	// If the request's suite has been granted an explicit exemption then do not
	// enact SuiteLimit's rules on it.
	for _, exemption := range exemptions {
		if suiteName == exemption.suiteName {
			entry.exemptionGranted = true
			return true
		}
	}

	// If non of the approved exemptions have applied for this request then it
	// is not exempt from Suite Limit's
	return false
}

// CancelTasks sends a BuildBucket Cancel request for each bbid being tracked in
// the entry. This should only be called when the request has gone past the
// maximum allowed DUT hour time and it does not have an active exemption.
func CancelTasks(entry *suiteLimitEntry, bbClient buildbucket.BuildsClient) error {
	ctx := context.Background()

	for bbid := range entry.perTaskLastSeen {
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
