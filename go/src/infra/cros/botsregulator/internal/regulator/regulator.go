// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package regulator defines the service main flow.
package regulator

import (
	"context"
	"fmt"
	"strings"

	"go.chromium.org/luci/common/logging"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"

	"infra/cros/botsregulator/internal/clients"
	"infra/cros/botsregulator/internal/provider"
	ufspb "infra/unifiedfleet/api/v1/models"
	ufsUtil "infra/unifiedfleet/app/util"
)

type regulator struct {
	bpiClient      provider.BPI
	opts           *RegulatorOptions
	swarmingClient clients.SwarmingClient
	ufsClient      clients.UFSClient
	botConfigs     []string
}

func NewRegulator(ctx context.Context, opts *RegulatorOptions) (*regulator, error) {
	logging.Infof(ctx, "creating regulator with flags: %v\n", opts)
	bcfgs, err := botConfigs(opts)
	if err != nil {
		return nil, err
	}
	bc, err := provider.NewProviderFromEnv(ctx, opts.BPI)
	if err != nil {
		return nil, err
	}
	sc, err := clients.NewSwarmingClient(ctx, opts.Swarming)
	if err != nil {
		return nil, err
	}
	uc, err := clients.NewUFSClient(ctx, opts.UFS)
	if err != nil {
		return nil, err
	}
	return &regulator{
		bpiClient:      bc,
		opts:           opts,
		swarmingClient: sc,
		ufsClient:      uc,
		botConfigs:     bcfgs,
	}, nil
}

// botConfigs returns bots configs parsed from botconfigs flag.
func botConfigs(opts *RegulatorOptions) ([]string, error) {
	var bc []string
	bc = append(bc, strings.Split(opts.BotConfigs, ",")...)
	return bc, nil
}

// BotConfigs returns regulator botconfigs.
func (r *regulator) BotConfigs() []string {
	return r.botConfigs
}

// ListAllMachineLSEsByHive fetches machineLSEs from UFS by hive.
func (r *regulator) ListAllMachineLSEsByHive(ctx context.Context, hive string) ([]*ufspb.MachineLSE, error) {
	ctx = clients.SetUFSNamespace(ctx, r.opts.Namespace)
	filters := []string{fmt.Sprintf("hive=%s", hive)}
	res, err := r.ufsClient.BatchListMachineLSEs(ctx, filters, 0, true, false)
	if err != nil {
		return nil, err
	}
	lses := make([]*ufspb.MachineLSE, len(res))
	for i, p := range res {
		lses[i] = p.(*ufspb.MachineLSE)
	}
	return lses, nil
}

// ListAllSchedulingUnits fetches ALL Scheduling Units from UFS.
func (r *regulator) ListAllSchedulingUnits(ctx context.Context) ([]*ufspb.SchedulingUnit, error) {
	ctx = clients.SetUFSNamespace(ctx, r.opts.Namespace)
	res, err := r.ufsClient.BatchListSchedulingUnits(ctx, nil, 0, false, false)
	if err != nil {
		return nil, err
	}
	sus := make([]*ufspb.SchedulingUnit, len(res))
	for i, p := range res {
		sus[i] = p.(*ufspb.SchedulingUnit)
	}
	return sus, nil
}

// ListAllDroneBots returns list of running Drone Swarming bots.
func (r *regulator) ListAllDroneBots(ctx context.Context, botConfig, ufsZone string) ([]*apipb.BotInfo, error) {
	cursor := ""
	var bots []*apipb.BotInfo
	for {
		// TODO(b/328810743): The botsRequest dimensions should be derived from flag options to support Satlab flow.
		resp, err := r.swarmingClient.ListBots(ctx, &apipb.BotsRequest{
			Limit:  1000,
			Cursor: cursor,
			Dimensions: []*apipb.StringPair{
				{
					Key:   "bot_config",
					Value: botConfig,
				},
				{
					Key:   "ufs_zone",
					Value: ufsZone,
				},
			},
			IsDead: apipb.NullableBool_FALSE,
		})
		if err != nil {
			return nil, err
		}
		bots = append(bots, resp.Items...)
		cursor = resp.Cursor
		if cursor == "" {
			break
		}
	}
	return bots, nil
}

// ConsolidateAvailableDUTs returns a list of available DUTs to create Swarming bots for.
// This list includes Scheduling Units and single DUTs, all sharing the same hive.
// The assumption is that all LSEs in a Scheduling Unit should share the same hive.
// This is enforced on UFS side.
func (r *regulator) ConsolidateAvailableDUTs(ctx context.Context, dbs []*apipb.BotInfo, lses []*ufspb.MachineLSE, sus []*ufspb.SchedulingUnit) []string {
	// List of available DUTs requiring a Swarming bot.
	var ad []string
	// Map of Drone DUTs for easy search.
	droneDUTs := dutMapFromBots(dbs)
	// Map of all lses sharing the same hive (e.g. cloudbots).
	lsesInSU := make(map[string]bool, len(lses))
	for _, lse := range lses {
		l := ufsUtil.RemovePrefix(lse.GetName())
		lsesInSU[l] = false
	}
	// Filtering SUs by hive.
	for _, su := range sus {
		seen := false
		for _, lse := range su.GetMachineLSEs() {
			if _, ok := lsesInSU[lse]; ok {
				lsesInSU[lse] = true
				seen = true
			}
		}
		// At least 1 DUT in the SU has the corresponding hive.
		if seen {
			s := ufsUtil.RemovePrefix(su.GetName())
			// The SU is still running on Drone.
			if _, ok := droneDUTs[s]; ok {
				logging.Infof(ctx, "Scheduling Unit %s is still running on Drone; skipping", s)
				continue
			}
			ad = append(ad, s)
		}
	}
	for lse, seen := range lsesInSU {
		// The DUT is part of a scheduling unit.
		if seen {
			continue
		}
		// The DUT is still running on Drone.
		if _, ok := droneDUTs[lse]; ok {
			logging.Infof(ctx, "DUT %s is still running on Drone; skipping", lse)
			continue
		}
		ad = append(ad, lse)
	}
	return ad
}

// UpdateConfig is a wrapper around the current provider UpdateConfig method.
func (r *regulator) UpdateConfig(ctx context.Context, hns []string) error {
	return r.bpiClient.UpdateConfig(ctx, hns, r.opts.CfID)
}

// dutMapFromBots return a map of DUT name from a list of Swarming bots.
func dutMapFromBots(dbs []*apipb.BotInfo) map[string]struct{} {
	duts := make(map[string]struct{}, len(dbs))
	for _, db := range dbs {
		for _, d := range db.GetDimensions() {
			if d.Key == "dut_name" {
				duts[d.Value[0]] = struct{}{}
				break
			}
		}
	}
	return duts
}
