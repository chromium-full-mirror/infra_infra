// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"fmt"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"infra/appengine/crosskylabadmin/internal/app/config"
	"infra/appengine/crosskylabadmin/internal/app/frontend/routing"
	"infra/libs/skylab/common/heuristics"
)

// RouteTaskParams are the parameters needed to route a task between legacy and paris
// and between the paris prod version and paris prod canary.
//
// RouteTaskParams deliberately excludes the context and randFloat, the entropy required.
type RouteTaskParams struct {
	taskType      string
	dutName       string
	expectedState string
	pools         []string
}

// RouteTask routes a task for a given bot.
//
// The possible return values are:
// - "legacy"  (for legacy, which is the default)
// -       ""  (indicates an error, should be treated as equivalent to "legacy" by callers)
// -  "paris"  (for PARIS, which is new)
// -  "latest" (indicates the latest version of paris)
func RouteTask(ctx context.Context, p RouteTaskParams, randFloat float64) (heuristics.TaskType, error) {
	if p.taskType == "" {
		return heuristics.ProdTaskType, errors.New("route task: task type cannot be empty")
	}
	switch p.taskType {
	case "repair":
		return routeRepairTask(ctx, p.dutName, p.expectedState, p.pools, randFloat)
	case "audit_rpm":
		return routeAuditRPMTask(ctx, p.dutName, randFloat)
	}
	return heuristics.ProdTaskType, fmt.Errorf("route task: unrecognized task name %q", p.taskType)
}

// routeAuditRPMTask routes an audit RPM task to a specific implementation: legacy, paris, or latest.
func routeAuditRPMTask(ctx context.Context, dutName string, randFloat float64) (heuristics.TaskType, error) {
	provider, reason := routeAuditTaskImpl(ctx, config.Get(ctx).GetParis().GetAuditRpm(), dutName, randFloat)
	logging.Infof(ctx, "Routing audit RPM task for dut %q with random input %f using provider %q for reason %d", dutName, randFloat, provider, reason)
	return provider, nil
}

// routeRepairTask routes a repair task for a given bot.
//
// The possible return values are:
// - "legacy"  (for legacy, which is the default)
// -       ""  (indicates an error, should be treated as equivalent to "legacy" by callers)
// -  "paris"  (for PARIS, which is new)
// -  "latest" (latest version of paris)
//
// routeRepairTask takes as an argument randFloat (which is a float64 in the closed interval [0, 1]).
// This argument is, by design, all the entropy that randFloat will need. Taking this as an argument allows
// routeRepairTask itself to be deterministic because the caller is responsible for generating the random
// value.
func routeRepairTask(ctx context.Context, dutName string, expectedState string, pools []string, randFloat float64) (heuristics.TaskType, error) {
	if !(0.0 <= randFloat && randFloat <= 1.0) {
		return heuristics.ProdTaskType, fmt.Errorf("Route repair task: randfloat %f is not in [0, 1]", randFloat)
	}
	isLabstation := heuristics.LooksLikeLabstation(dutName)
	rolloutConfig, err := getRolloutConfig(ctx, "repair", isLabstation, expectedState)
	if err != nil {
		return heuristics.ProdTaskType, errors.Annotate(err, "route repair task").Err()
	}
	out, r := routeRepairTaskImpl(
		ctx,
		rolloutConfig,
		&dutRoutingInfo{
			hostname:   dutName,
			labstation: isLabstation,
			pools:      pools,
		},
		randFloat,
	)
	reason, ok := routing.ReasonMessageMap[r]
	if !ok {
		logging.Infof(ctx, "Unrecognized reason %d", int64(r))
	}
	logging.Infof(ctx, "Sending device repair to %q because %q", out, reason)
	return out, nil
}
