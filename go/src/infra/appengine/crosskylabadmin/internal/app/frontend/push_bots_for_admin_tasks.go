// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"fmt"
	"sort"
	"time"

	"go.chromium.org/luci/common/data/strpair"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	fleet "infra/appengine/crosskylabadmin/api/fleet/v1"
	"infra/appengine/crosskylabadmin/internal/app/clients"
	"infra/appengine/crosskylabadmin/internal/app/config"
	"infra/appengine/crosskylabadmin/internal/ufs"
	"infra/cros/recovery/logger/metrics"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
)

const labstationRebootKind = `action:Labstation reboot`

type adminTaskBotPusher struct {
	metricsClient  metrics.Metrics
	ufsClient      ufs.Client
	swarmingClient clients.SwarmingClient
}

// getLabstations takes a start time and a stop time and returns the labstations with reboot events in that time range.
func (p *adminTaskBotPusher) getLabstations(ctx context.Context, startTime time.Time, stopTime time.Time) ([]string, error) {
	if p.metricsClient == nil {
		return nil, errors.New("getLabstations: metricsClient cannot be nil")
	}
	// TODO(gregorynisbet): look at "action:Power cycle by RPM" as well.
	results, err := p.metricsClient.Search(ctx, &metrics.Query{
		ActionKind: labstationRebootKind,
	})
	if err != nil {
		return nil, err
	}
	labstationMap := map[string]struct{}{}
	skippedLabstationTally := 0
	for _, action := range results.Actions {
		// TODO(gregorynisbet): Filter tasks based on time in the query, not here.
		if action.StartTime.Before(startTime) {
			skippedLabstationTally++
			continue
		}
		if action.Status == metrics.ActionStatusSuccess {
			labstationMap[action.Hostname] = struct{}{}
		}
	}
	logging.Debugf(ctx, "getLabstations: number of labstation events skipped %d", skippedLabstationTally)
	var labstations []string
	for k := range labstationMap {
		labstations = append(labstations, k)
	}
	sort.Strings(labstations)
	return labstations, err
}

// getDUTsForLabstations gets all the DUTs associated with a labstation.
func (p *adminTaskBotPusher) getDUTsForLabstations(ctx context.Context, labstations []string) ([]string, error) {
	if p.ufsClient == nil {
		return nil, nil
	}
	var duts []string
	resp, err := p.ufsClient.GetDUTsForLabstation(ctx, &ufsAPI.GetDUTsForLabstationRequest{
		Hostname: labstations,
	})
	if err != nil {
		return nil, err
	}
	for _, item := range resp.GetItems() {
		for _, hostname := range item.GetDutName() {
			duts = append(duts, fmt.Sprintf("crossk-%s", hostname))
		}
	}
	return duts, nil
}

// getDUTsWithRecentLabstationReboots gets DUTs that are associated with recent labstation reboots.
func (p *adminTaskBotPusher) getDUTsWithRecentLabstationReboots(ctx context.Context, startTime time.Time, stopTime time.Time) ([]string, error) {
	labstations, err := p.getLabstations(ctx, startTime, stopTime)
	if err != nil {
		return nil, err
	}
	duts, err := p.getDUTsForLabstations(ctx, labstations)
	if err != nil {
		return nil, err
	}
	return duts, nil
}

// repairRecentDuts repairs DUTs whose labstations have rebooted in the given time range.
func (p *adminTaskBotPusher) repairDUTsWithRecentLabstationReboots(ctx context.Context, startTime time.Time, stopTime time.Time, skipHostMap map[string]bool) error {
	cfg := config.Get(ctx)
	duts, err := p.getDUTsWithRecentLabstationReboots(ctx, startTime, stopTime)
	if err != nil {
		return err
	}
	logging.Infof(ctx, "repairDUTsWithRecentLabstationReboots - number of duts before filtering skipped hosts: %s", len(duts))

	var filteredDuts []string
	for _, dut := range duts {
		if skipHostMap[dut] {
			logging.Infof(ctx, "skipped dut: %s", dut)
			continue
		}
		filteredDuts = append(filteredDuts, dut)
	}
	logging.Infof(ctx, "repairDUTsWithRecentLabstationReboots - number of duts after filtering skipped hosts: %s", len(filteredDuts))
	// TODO(gregorynisbet): Do we want to consider other states here besides repair failed?
	err = clients.PushRepairDUTs(ctx, filteredDuts, "repair_failed", cfg.Swarming.BotPool)
	if err != nil {
		return err
	}
	return nil
}

// pushRepairDUTsForGivenPool pushes repair jobs for duts in a given pool.
// sc           -- the swarming client
// swarmingPool -- the swarming-level pool (NOT "label-pool") to push duts in
// dutState     -- the DUT state (e.g. "ready", "needs-repair")
// dims         -- a list of additional dimensions to map
// holdouts     -- a list of bot names to exclude (NOT dut names). Holdouts is read-only, so this parameter may be nil.
func (p *adminTaskBotPusher) pushRepairDUTsForGivenPool(ctx context.Context, swarmingPool string, dutState string, dims strpair.Map, skipHostMap map[string]bool) error {
	if p.swarmingClient == nil {
		return errors.New("swarmingClient cannot be nil in pushRepairDUTsForGivenPool")
	}
	rawBots, err := p.swarmingClient.ListAliveIdleBotsInPool(ctx, swarmingPool, dims)

	if err != nil {
		return errors.Annotate(err, "failed to list alive idle bots with dut_state %q", dutState).Err()
	}
	logging.Infof(ctx, "successfully get %d alive idle cros bots with dut_state %q in pool %q.", len(rawBots), dutState, swarmingPool)
	//Parse BOT id to schedule tasks for readability.
	repairBOTs := identifyBotsForRepair(ctx, rawBots)
	var filteredRepairBOTs []string
	for _, dut := range repairBOTs {
		if skipHostMap[dut] {
			logging.Infof(ctx, "skipped dut: %s", dut)
			continue
		}
		filteredRepairBOTs = append(filteredRepairBOTs, dut)
	}
	logging.Infof(ctx, "pushRepairDUTsForGivenPool - number of alive bots after filtering skipped hosts: %s", len(filteredRepairBOTs))
	err = clients.PushRepairDUTs(ctx, filteredRepairBOTs, dutState, swarmingPool)
	if err != nil {
		logging.Infof(ctx, "Push repair bots in pool %q: %v", swarmingPool, err)
		return errors.Annotate(err, "Failed to push repair duts in pool %q", swarmingPool).Err()
	}
	return nil
}

// pushBotsForAdminTasksImpl pushes the bots for admin tasks.
func (p *adminTaskBotPusher) pushBotsForAdminTasksImpl(ctx context.Context, req *fleet.PushBotsForAdminTasksRequest) (*fleet.PushBotsForAdminTasksResponse, error) {
	now := time.Now()

	if p.swarmingClient == nil {
		return nil, errors.Reason("swarming client cannot be nil").Err()
	}
	cfg := config.Get(ctx)
	dutState, ok := clients.DutStateRevMap[req.GetTargetDutState()]
	if !ok {
		return nil, fmt.Errorf("DutState=%#v does not map to swarming value", req.GetTargetDutState())
	}

	var merr errors.MultiError
	// Schedule admin tasks to idle DUTs.
	dims := make(strpair.Map)
	dims[clients.DutStateDimensionKey] = []string{dutState}

	// Get skipped bots from the config
	var skipHostList []string
	switch req.TargetDutState {
	case fleet.DutState_NeedsRepair:
		skipHostList = cfg.GetParis().GetDutRepair().GetSkipHosts()
	case fleet.DutState_NeedsManualRepair:
		skipHostList = cfg.GetParis().GetDutRepairOnNeedsManualRepair().GetSkipHosts()
	case fleet.DutState_RepairFailed:
		skipHostList = cfg.GetParis().GetDutRepairOnRepairFailed().GetSkipHosts()
	}
	logging.Infof(ctx, "pushBotsForAdminTasksImpl - number of skip hosts: %s", len(skipHostList))

	skipHostMap := make(map[string]bool, len(skipHostList))
	for _, value := range skipHostList {
		skipHostMap[value] = true
	}

	// When we sweep all the devices for "needs_repair" devices, then we additionally need to check for
	// "repair_failed" devices associated with labstations that have recently rebooted.
	// When a labstation reboots, this is basically a fresh opportunity for the DUT to be recovered.
	if dutState == "needs_repair" {
		var err error
		// The cron job that runs smart scheduling runs every 2 minutes.
		err = p.repairDUTsWithRecentLabstationReboots(ctx, now.Add(-2*time.Minute), now.Add(1*time.Minute), skipHostMap)
		if err != nil {
			return nil, err
		}
	}

	// Loop through all the Swarming Pool configs and push duts for repair.
	for _, pool := range cfg.GetSwarming().GetPoolCfgs() {
		if err := p.pushRepairDUTsForGivenPool(ctx, pool.GetPoolName(), dutState, dims, skipHostMap); err != nil {
			e := errors.Annotate(err, "Failed to push repair duts in pool %q", pool.GetPoolName()).Err()
			logging.Infof(ctx, "Fail to pushed repair duts with dut_state %q in pool %q: %s", dutState, pool.GetPoolName(), e)
			merr = append(merr, e)
		} else {
			logging.Infof(ctx, "Successfully pushed repair duts with dut_state %q in pool %q.", dutState, pool.GetPoolName())
		}
	}
	if len(merr) > 0 {
		return nil, merr
	}
	logging.Infof(ctx, "Successfully pushed repair duts in all pools")
	return &fleet.PushBotsForAdminTasksResponse{}, nil
}
