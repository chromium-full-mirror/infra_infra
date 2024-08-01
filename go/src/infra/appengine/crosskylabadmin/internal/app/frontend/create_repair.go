// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/server/auth"

	fleet "infra/appengine/crosskylabadmin/api/fleet/v1"
	"infra/appengine/crosskylabadmin/internal/app/clients"
	"infra/appengine/crosskylabadmin/internal/app/config"
	"infra/appengine/crosskylabadmin/internal/app/frontend/routing"
	"infra/appengine/crosskylabadmin/site"
	"infra/cros/recovery/karte"
	"infra/cros/recovery/logger/metrics"
	schedulingapi "infra/libs/fleet/scheduling/api"
	"infra/libs/fleet/scheduling/schedulers"
	"infra/libs/skylab/buildbucket"
	"infra/libs/skylab/common/heuristics"
)

// UFSErrorPolicy controls how UFS errors are handled.
type ufsErrorPolicy string

// UFS error policy constants.
// Error policy constants are defined in go/src/infra/appengine/crosskylabadmin/app/config/config.proto.
//
// Strict   -- fail on UFS error even if we don't need the result
// Fallback -- if we encounter a UFS error, fall back to the legacy path.
// Lax      -- if we do not need the UFS response to make a decision, do not fail the request.
const (
	// The strict policy causes all UFS error requests to be treated as fatal and causes the request to fail.
	ufsErrorPolicyStrict         ufsErrorPolicy = "strict"
	ufsErrorPolicyFallback                      = "fallback"
	ufsErrorPolicyLax                           = "lax"
	maxConsequentRecFailureCount                = 4
)

// NormalizeError policy normalizes a string into the canonical name for a policy.
func normalizeErrorPolicy(policy string) (ufsErrorPolicy, error) {
	policy = strings.ToLower(policy)
	switch policy {
	case "", "default", "fallback":
		return ufsErrorPolicyFallback, nil
	case "strict":
		return ufsErrorPolicyStrict, nil
	case "lax":
		return ufsErrorPolicyLax, nil
	}
	return "", fmt.Errorf("unrecognized policy: %q", policy)
}

// getRolloutConfig gets the applicable rolloutConfig.
func getRolloutConfig(ctx context.Context, taskType string, isLabstation bool, expectedState string) (*config.RolloutConfig, error) {
	if taskType == "" {
		return nil, errors.Reason("get rollout config: taskType cannot be empty").Err()
	}
	if taskType != "repair" {
		return nil, errors.Reason("getRolloutConfig: tasks other than repair are not supported, %q given", taskType).Err()
	}
	if isLabstation {
		return config.Get(ctx).GetParis().GetLabstationRepair(), nil
	}
	if expectedState == "" {
		return nil, errors.Reason("get rollout config: expectedState cannot be empty").Err()
	}
	switch expectedState {
	case "ready":
		return nil, errors.Reason("get rollout config: refusing to schedule repair task on ready dut").Err()
	case "needs_repair":
		return config.Get(ctx).GetParis().GetDutRepair(), nil
	case "repair_failed":
		return config.Get(ctx).GetParis().GetDutRepairOnRepairFailed(), nil
	case "needs_manual_repair":
		return config.Get(ctx).GetParis().GetDutRepairOnNeedsManualRepair(), nil
	}
	return nil, errors.Reason("get rollout config: expected state %q is not recognized", expectedState).Err()
}

func createKarteClient(ctx context.Context) (metrics.Metrics, error) {
	cfg := config.Get(ctx)
	transport, err := auth.GetRPCTransport(ctx, auth.AsSelf)
	if err != nil {
		return nil, errors.Annotate(err, "failed to get RPC transport").Err()
	}
	return karte.NewMetricsWithHttp(ctx, &http.Client{
		Transport: transport,
	}, cfg.GetKarte().GetHost(), site.DefaultPRPCOptions)
}

func findProperRecoveryTask(ctx context.Context, expectedState, dutName string, karteC metrics.Metrics) buildbucket.TaskName {
	// Only consider to schedule deep repair for repair_failed DUTs
	if expectedState != clients.DutStateRevMap[fleet.DutState_RepairFailed] {
		return buildbucket.Recovery
	}
	recFailCount, err := metrics.CountFailedRepairFromMetrics(ctx, dutName, buildbucket.Recovery.String(), karteC)
	if err != nil {
		logging.Infof(ctx, "Fail to get consequent failure count for recovery task: %s", err)
		return buildbucket.Recovery
	}
	if recFailCount >= maxConsequentRecFailureCount {
		return buildbucket.DeepRecovery
	}
	return buildbucket.Recovery
}

// GetPoolCfg finds a PoolCfg for a given swarming pool if available, otherwise returns nil.
func GetPoolCfg(ctx context.Context, poolName string) *config.Swarming_PoolCfg {
	cfg := config.Get(ctx)
	for _, c := range cfg.Swarming.PoolCfgs {
		if c.PoolName == poolName {
			logging.Infof(ctx, "Found Pool config for %q pool", poolName)
			return c
		}
	}
	return nil
}

// CreateRepairTask kicks off a repair job.
//
// This function will either schedule a legacy repair task or a PARIS repair task.
// Note that the ufs client can be nil.
func CreateRepairTask(ctx context.Context, dutName string, expectedState string, pools []string, randFloat float64, poolCfg *config.Swarming_PoolCfg) (string, error) {
	logging.Infof(ctx, "Creating repair task for %q expected state %q with random input %f", dutName, expectedState, randFloat)
	// If we encounter an error picking paris or legacy, do the safe thing and use legacy.
	taskType, err := RouteTask(
		ctx,
		RouteTaskParams{
			taskType:      "repair",
			dutName:       dutName,
			expectedState: expectedState,
			pools:         pools,
		},
		randFloat,
	)
	if err != nil {
		logging.Errorf(ctx, "error when getting task type: %s", err)
	}

	cipdVersion := buildbucket.CIPDProd
	if taskType == heuristics.LatestTaskType {
		cipdVersion = buildbucket.CIPDLatest
	}

	r := createBuildbucketTaskRequest{
		taskName:      buildbucket.Recovery,
		taskType:      cipdVersion,
		dutName:       dutName,
		expectedState: expectedState,
		builderBucket: poolCfg.GetBuilderBucket(),
		botPrefix:     poolCfg.GetBotPrefix(),
		ufsNamespace:  poolCfg.UFSCtxNamespace(),
	}

	karteC, err := createKarteClient(ctx)
	if err != nil {
		logging.Infof(ctx, "Fail to create karte client, skip stats checking")
	} else {
		r.taskName = findProperRecoveryTask(ctx, expectedState, dutName, karteC)
	}
	sc, err := schedulers.NewSchedukeClientForAutomation(ctx, pools[0])
	if err != nil {
		logging.Errorf(ctx, "Create Repair task. Fail to create Scheduke client! %w", err)
		// That is ok to do nothing if we fail as then we will not use Scheduke and fall to BB.
		sc = nil
		// return "", errors.Annotate(err, "create repair task").Err()
	}
	url, err := createBuildbucketTask(ctx, sc, r)
	return url, errors.Annotate(err, "create repair task").Err()
}

// DUTRoutingInfo is all the deterministic information about a DUT that is necessary to decide
// whether to use a legacy task or a paris task.
//
// We need to know whether a DUT is a labstation or not.
// We also need to know its hostname so we can choose the pattern stanza that applies to it.
type dutRoutingInfo struct {
	hostname   string
	labstation bool
	pools      []string
}

// RouteLabstationRepairTask takes a repair task for a labstation and routes it.
func routeRepairTaskImpl(ctx context.Context, r *config.RolloutConfig, info *dutRoutingInfo, randFloat float64) (heuristics.TaskType, routing.Reason) {
	if info == nil {
		logging.Errorf(ctx, "info cannot be nil, falling back to legacy")
		return routing.Paris, routing.NilArgument
	}
	// Check for malformed input data that would cause us to be unable to make a decision.
	if len(info.pools) == 0 {
		return routing.Paris, routing.NoPools
	}

	d := r.ComputePermilleData(ctx, info.hostname)

	// threshold is the chance of using Paris at all, which is equal to prod + latest.
	threshold := d.Prod + d.Latest
	// latestThreshold is a smaller threshold for using latest specifically.
	latestThreshold := d.Latest
	myValue := math.Round(1000.0 * randFloat)
	// If the threshold is zero, let's reject all possible values of myValue.
	// This way a threshold of zero actually means 0.0% instead of 0.1%.
	valueBelowThreshold := threshold != 0 && myValue <= threshold
	valueBelowLatestThreshold := latestThreshold != 0 && myValue <= latestThreshold
	if r.GetOptinAllDuts() {
		switch {
		case valueBelowLatestThreshold:
			return routing.ParisLatest, routing.ScoreBelowThreshold
		case valueBelowThreshold:
			return routing.Paris, routing.ScoreBelowThreshold
		default:
			return routing.Paris, routing.ScoreTooHigh
		}
	}
	if threshold == 0 {
		return routing.Paris, routing.ThresholdZero
	}
	if !r.GetOptinAllDuts() && len(r.GetOptinDutPool()) > 0 && isDisjoint(info.pools, r.GetOptinDutPool()) {
		return routing.Paris, routing.WrongPool
	}
	switch {
	case valueBelowLatestThreshold:
		return routing.ParisLatest, routing.ScoreBelowThreshold
	case valueBelowThreshold:
		return routing.Paris, routing.ScoreBelowThreshold
	default:
		return routing.Paris, routing.ScoreTooHigh
	}
}

// createBuildbucketTaskRequest consists of the parameters needed to schedule a buildbucket repair task.
type createBuildbucketTaskRequest struct {
	// taskName is the name of the task, e.g. taskname.Recovery
	taskName      buildbucket.TaskName
	taskType      buildbucket.CIPDVersion
	dutName       string
	expectedState string
	// Build bucket to be used to schedule swarming task
	builderBucket string
	// Bot prefix of a swarming bot.
	botPrefix string
	// UFS namespace to be used for the given bot
	ufsNamespace string
}

// CreateBuildbucketTask creates a new task (repair by default) for the provided DUT.
// Err should be non-nil if and only if a task was created.
// We rely on this signal to decide whether to fall back to the legacy flow.
func createBuildbucketTask(ctx context.Context, sc schedulingapi.TaskSchedulingAPI, params createBuildbucketTaskRequest) (string, error) {
	if params.taskName == "" {
		params.taskName = buildbucket.Recovery
	}
	if err := buildbucket.ValidateTaskName(params.taskName); err != nil {
		return "", errors.Annotate(err, "create buildbucket task: unsupported task name: %q", params.taskName).Err()
	}
	if err := params.taskType.Validate(); err != nil {
		return "", errors.Annotate(err, "create buildbucket repair task: invalid task type %v", params.taskType).Err()
	}
	logging.Infof(ctx, "Using new repair flow for dut %q with expected state %q", params.dutName, params.expectedState)
	transport, err := auth.GetRPCTransport(ctx, auth.AsSelf)
	if err != nil {
		return "", errors.Annotate(err, "failed to get RPC transport").Err()
	}
	hc := &http.Client{
		Transport: transport,
	}
	bc, err := buildbucket.NewClient(ctx, hc, site.DefaultPRPCOptions)
	if err != nil {
		logging.Errorf(ctx, "error creating buildbucket client: %q", err)
		return "", errors.Annotate(err, "create buildbucket repair task").Err()
	}
	p := &buildbucket.Params{
		UnitName:    params.dutName,
		TaskName:    params.taskName.String(),
		BuilderName: buildbucket.TaskNameToBuilderNamePerVersion(params.taskName, params.taskType),
		// Set the build bucket information to the swarming task
		BuilderBucket:  params.builderBucket,
		EnableRecovery: true,
		// TODO(gregorynisbet): This is our own name, move it to the config.
		AdminService: "chromeos-skylab-bot-fleet.appspot.com",
		// NOTE: We use the UFS service, not the Inventory service here.
		InventoryService: config.Get(ctx).GetUFS().GetHost(),
		// Default is 'OS', set this value when configured in PoolCfg.
		InventoryNamespace: params.ufsNamespace,
		NoStepper:          false,
		NoMetrics:          false,
		UpdateInventory:    true,
		ExpectedState:      params.expectedState,
		// TODO(gregorynisbet): Pass config file to labpack task.
		Configuration: "",
	}
	url, _, err := buildbucket.CreateTask(ctx, bc, sc, params.taskType, p, "crosskylabadmin")
	if err != nil {
		// CrOSSkylabAdmin is getting an error periodically where we fail to create a buildbucket task as of 2023-11-16.
		logging.Errorf(ctx, "error scheduling task %q on builder %q for device %q with expected state %q: %s", p.TaskName, p.BuilderName, p.UnitName, p.ExpectedState, err)
		return "", errors.Annotate(err, "create buildbucket repair task").Err()
	}
	return url, nil
}

// IsDisjoint returns true if and only if two sequences have no elements in common.
func isDisjoint(a []string, b []string) bool {
	bMap := make(map[string]bool, len(b))
	for _, item := range b {
		bMap[item] = true
	}
	for _, item := range a {
		if bMap[item] {
			return false
		}
	}
	return true
}
