// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package metrics

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
)

const (
	// MetricsKind is the name/key stored in the karte metric.
	MetricsKind = "metrics_kind"
)

// metricsFoundAtLastTimeExec checks whether the specific actionKind has been found within the specified time window.
// By default, it is using DUT's ResourceName as the search parameter in Karte.
//
// @params: actionArgs should be in the format of:
// Ex: ["metrics_kind:x", "time_frame_hours:x"]
func metricsFoundAtLastTimeExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	// metricsKind is the name/key stored in the karte metric.
	metricsKind := argsMap.AsString(ctx, MetricsKind, "")
	if metricsKind == "" {
		return errors.Reason("metrics found at last time: the provided metrics_kind is empty").Err()
	}
	// timeFrameHours is the time window for searching the last metric of this metricsKind being recorded.
	// Default to be 24h.
	timeFrameHours := argsMap.AsDuration(ctx, "time_frame_hours", 24, time.Hour)
	metric := info.GetMetrics()
	if metric == nil {
		return errors.Reason("metrics found at last time: karte metric has not been initilized").Err()
	}
	karteQuery := &metrics.Query{
		// TODO(@gregorynisbet): When karte' Search API is capable of taking in asset tag,
		// change the query to use asset tag instead of using hostname.
		Hostname:   info.GetDut().Name,
		ActionKind: metricsKind,
	}
	queryRes, err := metric.Search(ctx, karteQuery)
	if err != nil {
		return errors.Annotate(err, "metrics found at last time").Err()
	}
	matchedQueryResCount := len(queryRes.Actions)
	if matchedQueryResCount == 0 {
		return errors.Reason("No match of the metrics kind: %q found in karte.", metricsKind).Err()
	}
	// Grabbing the most recent Karte response for this particular metrics kind.
	karteAction := queryRes.Actions[0]
	lastTime := karteAction.StopTime
	log.Infof(ctx, "Found last time: %v of metric kind: %q on the DUT: %v", lastTime, metricsKind)
	if time.Since(lastTime) < timeFrameHours {
		return nil
	}
	return errors.Reason("metrics found at last time: no metric kind of: %q found within the last %v", metricsKind, timeFrameHours).Err()
}

// checkTaskFailuresExec counts the number of failed attempts (including the current attempt) and
// exits successfully if the threshold is reached or exceeded.
//
// This exec assumes that the current task is failing and *counts it towards the number of
// currently-failing tasks*. Whenever it is called, care should be taken to exit early without
// invoking it if the current task appears successful.
//
// See b:264309811 comment #10 and b:264309811 comment #14 for details.
func checkTaskFailuresExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	taskName := argsMap.AsString(ctx, "task_name", "")
	repairFailedCountTarget := argsMap.AsInt(ctx, "repair_failed_count", 49)
	previousRepairFailedCount, err := metrics.CountFailedRepairFromMetrics(ctx, info.GetDut().Name, taskName, info.GetMetrics())
	if err != nil {
		return errors.Annotate(err, "check task failures").Err()
	}
	// When determining whether we have reached or exceeded the target or not,
	// we DO count the current repair attempt.
	//
	// The current repair attempt is not a previous repair attempt, therefore we
	// must add one to the previous repair failure count.
	repairFailedCount := previousRepairFailedCount + 1
	if repairFailedCount >= repairFailedCountTarget {
		log.Infof(ctx, "The number of repair attempts including current attempt %d reached or exceeded the threshold of %d", repairFailedCount, repairFailedCountTarget)
		return nil
	}
	return errors.Reason("check task failures: Fail count: %d", repairFailedCount).Err()
}

func init() {
	execs.Register("metrics_found_at_last_time", metricsFoundAtLastTimeExec)
	execs.Register("metrics_check_task_failures", checkTaskFailuresExec)
}
