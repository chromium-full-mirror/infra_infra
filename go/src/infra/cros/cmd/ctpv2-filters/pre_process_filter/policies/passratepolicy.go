// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policies

import (
	"fmt"
	"log"
	"math"
	"strconv"

	"cloud.google.com/go/bigquery"
	"golang.org/x/exp/slices"
	"google.golang.org/api/iterator"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/pre_process_filter/interfaces"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/pre_process_filter/structs"
)

// PassRatePolicy is a helper struct for GetFile.
type PassRatePolicy struct {
	req                    *api.FilterFlakyRequest_PassRatePolicy
	variant                string
	milestone              string
	forceMapEnable         map[string]struct{}
	forceMapDisable        map[string]struct{}
	requiredPassRate       float64
	requiredRuns           int
	numOfMilestones        int
	missingTcList          map[string]struct{}
	data                   map[string]structs.SignalFormat
	otherData              map[string]structs.SignalFormat
	requiredRecentPassRate float64
	requiredRecentRuns     int
	log                    *log.Logger
}

type resSchema struct {
	Build                string
	Normalized_test      string
	Board                string
	Success_permille     float64
	Total_runs           int
	Fail_runs            int
	Rec_Success_Permille float64
	Rec_Total_Runs       int
}

// Helper method to turn a list into a set. Because Go doesn't have built in sets. nice.
func listToMap(forced []string) map[string]struct{} {
	listMap := make(map[string]struct{})
	for _, test := range forced {
		listMap[test] = struct{}{}
	}
	return listMap
}

// Helper method to turn the policy FilterTestConfigs into a set.
func policyFilterTestConfigsToMap(cfgs []*api.FilterTestConfig, board string) (map[string]struct{}, map[string]struct{}) {
	enabled := make(map[string]struct{})
	disabled := make(map[string]struct{})
	for _, filterconfig := range cfgs {
		fmt.Println(filterconfig.GetSetting())
		if filterconfig.GetSetting() == api.FilterTestConfig_ENABLED {
			if slices.Contains(filterconfig.GetBoard(), board) || slices.Contains(filterconfig.GetBoard(), "*") {
				enabled[filterconfig.GetTest()] = struct{}{}
			}
		} else if filterconfig.GetSetting() == api.FilterTestConfig_DISABLED {
			if slices.Contains(filterconfig.GetBoard(), board) || slices.Contains(filterconfig.GetBoard(), "*") {
				disabled[filterconfig.GetTest()] = struct{}{}
			}
		}
	}
	return enabled, disabled
}

// StabilityFromPolicy returns the stability information computed from the policy given. Uses BQ results directly for test history.
func StabilityFromPolicy(req *api.FilterFlakyRequest_PassRatePolicy, variant string, milestone string, tcList map[string]struct{}, log *log.Logger) (map[string]structs.SignalFormat, error) {
	policy := PassRatePolicy{
		req:                    req,
		variant:                variant,
		milestone:              milestone,
		forceMapEnable:         make(map[string]struct{}),
		forceMapDisable:        make(map[string]struct{}),
		requiredPassRate:       float64(req.PassRatePolicy.PassRate),
		numOfMilestones:        int(req.PassRatePolicy.NumOfMilestones),
		requiredRuns:           int(req.PassRatePolicy.MinRuns),
		requiredRecentPassRate: float64(req.PassRatePolicy.PassRateRecent),
		requiredRecentRuns:     int(req.PassRatePolicy.MinRunsRecent),
		missingTcList:          tcList,
		data:                   make(map[string]structs.SignalFormat),
		otherData:              make(map[string]structs.SignalFormat),
		log:                    log,
	}

	err := policy.stabilityFromPolicy()
	if err != nil {
		return nil, err
	}
	return policy.data, nil
}

func (q *PassRatePolicy) determineSignalFromQuery(testname string, passRate float64, recPassRate float64, recRuns int, minRuns int, filterRemaining int) (bool, int) {
	// If the test is force_enabled, do this.
	if _, found := q.forceMapEnable[testname]; found {
		q.log.Printf("testname %s marked as forced disabled.", testname)
		return true, filterRemaining
	} else if _, found := q.forceMapDisable[testname]; found {
		return false, filterRemaining
	} else if passRate >= float64(q.requiredPassRate) {
		return true, filterRemaining
	} else if q.requiredPassRate != 0 && q.requiredRecentRuns != 0 {
		if recPassRate >= q.requiredRecentPassRate && recRuns >= q.requiredRecentRuns {
			return true, filterRemaining // Was bad, but recent signal is very strongly good.
		}
	}
	// reduce the filter remaining by one, and if there is <=0 remaining, allow it.
	if filterRemaining <= 0 {
		q.log.Printf("testname %s < required PassRate %v with %v: IS UNSTABLE BUT ALLOWED DUE TO MAX FILTER # REACHED.\n", testname, q.requiredPassRate, passRate)
		return true, filterRemaining
	}

	// Only decrement the filter remaining if the TC is actually in the list of TC.
	_, ok := q.missingTcList[testname]
	if ok {
		filterRemaining--
	}
	return false, filterRemaining
}

func (q *PassRatePolicy) stabilityFromPolicy() error {
	mileStone, _ := strconv.Atoi(q.milestone)
	mileStroneRegex := mileStoneRegex(q.numOfMilestones, mileStone)
	q.forceMapEnable, q.forceMapDisable = policyFilterTestConfigsToMap(q.req.PassRatePolicy.Testconfigs, q.variant)

	// Query using all possible milestones. We will only search for results in the current on the first iterations.
	bqIter, err := interfaces.QueryForResults(q.variant, mileStroneRegex, q.log)
	if err != nil {
		return fmt.Errorf("unable to determine stabily: %w", err)
	}

	// Iterate through the results.
	// TODO, consider combining results from different milestones.
	// Might be useful when # runs required is like "20"; but we have 10 from 2 different milestones.
	q.data = q.iterThroughData(bqIter, q.milestone)

	if len(q.missingTcList) > 0 && q.numOfMilestones > 0 {
		q.populateMissingTests()
	}

	return nil

}

func (q *PassRatePolicy) populateMissingTests() {
	for k := range q.missingTcList {
		_, ok := q.otherData[k]
		if ok {
			q.log.Printf("Populating test %s from previous milestone.\n", k)
			q.data[k] = q.otherData[k]
		}
	}
}

func maxAllowedToBeFiltered(req *api.PassRatePolicy, totalTests int) int {
	maxI := req.MaxFilteredInt
	if maxI == 0 {
		maxI = math.MaxInt32
	}
	maxP := req.MaxFilteredPercent
	if maxP == 0 {
		maxP = 100
	}
	percentAsInt := math.Round(float64(totalTests * int(maxP) / 100))
	return int(math.Min(float64(maxI), percentAsInt))
}

func (q *PassRatePolicy) iterThroughData(bqIter *bigquery.RowIterator, milestone string) map[string]structs.SignalFormat {
	data := make(map[string]structs.SignalFormat)
	totalTC := len(q.missingTcList)
	filtersRemaining := maxAllowedToBeFiltered(q.req.PassRatePolicy, totalTC)

	for {
		var resp resSchema
		err := bqIter.Next(&resp)
		if err == iterator.Done { // from "google.golang.org/api/iterator"
			break
		}
		if err != nil {
			// Intentionally do not rause this error, just log it for now.
			// Sometimes something silly can happen and we get `NULL` from the query for some items,
			// this is where its going to be raised. For now, lets log && continue to not break every test.
			q.log.Printf("INFORMATIONAL ERR - could not decode error on loop: %s\n", err)
		}
		var signal bool
		signal, filtersRemaining = q.determineSignalFromQuery(
			resp.Normalized_test, resp.Success_permille, resp.Rec_Success_Permille,
			resp.Rec_Total_Runs, int(q.req.PassRatePolicy.MinRuns),
			filtersRemaining)

		// Not enough runs? skip!
		if resp.Total_runs < int(q.req.PassRatePolicy.MinRuns) {
			continue
		}

		if !signal {
			q.log.Printf("testname %s < required PassRate %v with %v: MARKED UNSTABLE (might still run due to overrides).\n", resp.Normalized_test, q.requiredPassRate, resp.Success_permille)

		}

		fmted := structs.SignalFormat{
			Runs:           resp.Total_runs,
			Failruns:       resp.Fail_runs,
			Passrate:       resp.Success_permille,
			Signal:         signal,
			PassrateRecent: resp.Rec_Success_Permille,
			RunsRecent:     resp.Rec_Total_Runs,
		}
		// Not in milestone save in memory for later use!
		if resp.Build != milestone {
			q.otherData[resp.Normalized_test] = fmted
			continue
		}
		data[resp.Normalized_test] = fmted
		// Found and is current milestone, remove from the missing list.
		delete(q.missingTcList, resp.Normalized_test)
	}
	return data
}
