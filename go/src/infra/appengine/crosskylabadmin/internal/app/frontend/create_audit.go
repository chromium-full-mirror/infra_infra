// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"math"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"infra/appengine/crosskylabadmin/internal/app/config"
	"infra/appengine/crosskylabadmin/internal/app/frontend/routing"
	"infra/libs/fleet/scheduling/schedulers"
	"infra/libs/skylab/buildbucket"
	"infra/libs/skylab/common/heuristics"
)

// CreateAuditTask kicks off an audit job.
func CreateAuditTask(ctx context.Context, botID, pool, taskname, actions string, randFloat float64) (string, error) {
	// The actions field is a little bit tricky and consists of a comma-delimited list of actions.
	// We're also using Paris in a slightly different way than legacy.
	// Each audit action will correspond to one paris job, always.
	logging.Infof(ctx, "Creating audit task for %q with random input %f and actions %q taskname %q", botID, randFloat, actions, taskname)
	tn, err := buildbucket.NormalizeTaskName(taskname)
	if err != nil {
		logging.Errorf(ctx, "error when normalizing task name: %q", err)
	}

	sc, err := schedulers.NewSchedukeClientForAutomation(ctx, pool)
	if err != nil {
		return "", errors.Annotate(err, "CreateAuditTask: initializing Scheduke client").Err()
	}
	bbURL, cErr := createBuildbucketTask(ctx, sc, createBuildbucketTaskRequest{
		taskName: tn,
		taskType: buildbucket.CIPDProd,
		botID:    botID,
	})
	if cErr != nil {
		return "", errors.Annotate(cErr, "create audit task").Err()
	}

	logging.Infof(ctx, "Successfully launched audit task %q for bot %q", bbURL, botID)
	return bbURL, nil
}

// routeAuditTaskImpl routes an audit task (storage, rpm, USB) based on the rollout config that's tied to that specific task.
func routeAuditTaskImpl(ctx context.Context, r *config.RolloutConfig, hostname string, randFloat float64) (heuristics.TaskType, routing.Reason) {
	logging.Infof(ctx, "control transferred to routeAuditTaskImpl for hostname %q", hostname)
	if r == nil {
		return routing.Paris, routing.ParisNotEnabled
	}
	if !(0.0 <= randFloat && randFloat <= 1.0) {
		return routing.Paris, routing.InvalidRangeArgument
	}
	if err := r.ValidateNoRepairOnlyFields(); err != nil {
		logging.Errorf(ctx, "repair-only field detected for audit task: %s", err.Error())
		return routing.Paris, routing.RepairOnlyField
	}
	d := r.ComputePermilleData(ctx, hostname)
	if d == nil {
		return routing.Paris, routing.MalformedPolicy
	}
	// threshold is the chance of using Paris at all, which is equal to prod + latest.
	threshold := d.Prod + d.Latest
	// latestThreshold is a smaller threshold for using latest specifically.
	latestThreshold := d.Latest
	myValue := math.Round(1000.0 * randFloat)
	// If the threshold is zero, let's reject all possible values of myValue.
	// This way a threshold of zero actually means 0.0% instead of 0.1%.
	valueBelowThreshold := threshold != 0 && myValue <= threshold
	valueBelowLatestThreshold := latestThreshold != 0 && myValue <= latestThreshold
	logging.Infof(ctx, "Values in routeAuditTaskInfo %q %f %f %f", hostname, threshold, latestThreshold, myValue)
	switch {
	case valueBelowLatestThreshold:
		return routing.ParisLatest, routing.ScoreBelowThreshold
	case valueBelowThreshold:
		return routing.Paris, routing.ScoreBelowThreshold
	}
	if threshold == 0 {
		return routing.Paris, routing.ThresholdZero
	}
	return routing.Paris, routing.ScoreTooHigh
}
