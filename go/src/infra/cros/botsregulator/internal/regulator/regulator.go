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

	"go.chromium.org/infra/cros/botsregulator/internal/clients"
	"go.chromium.org/infra/cros/botsregulator/internal/provider"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

type regulator struct {
	bpiClient      provider.BPI
	opts           *RegulatorOptions
	swarmingClient clients.SwarmingClient
	ufsClient      clients.UFSClient
	botConfigs     []string
	configHive     map[string]string
}

func NewRegulator(ctx context.Context, opts *RegulatorOptions) (*regulator, error) {
	logging.Infof(ctx, "creating regulator with flags: %v\n", opts)
	bcfgs, err := botConfigs(opts)
	if err != nil {
		return nil, err
	}
	ch, err := configHive(opts)
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
		configHive:     ch,
	}, nil
}

// configHive returns config hive map from config-hive flags.
func configHive(opts *RegulatorOptions) (map[string]string, error) {
	ch := make(map[string]string)
	if opts.CfIDHives != "" {
		for _, f := range strings.Split(opts.CfIDHives, ",") {
			s := strings.SplitN(f, ":", 2)
			if len(s) != 2 || s[0] == "" || s[1] == "" {
				return nil, fmt.Errorf("invalid cfid:hive format.  cfIDHive=%q ", s)
			}
			c := s[0]
			h := s[1]
			if _, found := ch[c]; found {
				return nil, fmt.Errorf("config prefix %q already exists. please check config-hive flag", c)
			}
			ch[c] = h
		}
	}
	if len(ch) == 0 {
		return nil, fmt.Errorf("empty configHive")
	}
	return ch, nil
}

// botConfigs returns bots configs parsed from botconfigs flag.
func botConfigs(opts *RegulatorOptions) ([]string, error) {
	var bc []string
	bc = append(bc, strings.Split(opts.BotConfigs, ",")...)
	return bc, nil
}

// ConfigHive returns config hive map.
func (r *regulator) ConfigHive() map[string]string {
	return r.configHive
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

// ListAllRunningBots returns list of running swarming bots belong to regulator botconfigs.
func (r *regulator) ListAllRunningBots(ctx context.Context) ([]*apipb.BotInfo, error) {
	var allBots []*apipb.BotInfo
	for _, c := range r.botConfigs {
		bots, err := r.listRunningBots(ctx, c, r.opts.Zone)
		if err != nil {
			return nil, err
		}
		allBots = append(allBots, bots...)
	}
	return allBots, nil
}

// listRunningBots returns running Swarming bots with given regulator botconfigs and zone.
func (r *regulator) listRunningBots(ctx context.Context, botConfig, ufsZone string) ([]*apipb.BotInfo, error) {
	cursor := ""
	var bots []*apipb.BotInfo
	for {
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
	logging.Infof(ctx, "listRunningBots: botConfig: %s, ufsZone: %s, bots: %v\n", botConfig, ufsZone, len(bots))
	return bots, nil
}

// ConsolidateAvailableDUTs returns a list of available DUTs to create Swarming bots for.
// This list includes Scheduling Units and single DUTs, all sharing the same hive.
// The assumption is that all LSEs in a Scheduling Unit should share the same hive.
// This is enforced on UFS side.
func (r *regulator) ConsolidateAvailableDUTs(ctx context.Context, prefix string, dutIDMap map[string][]string, lses []*ufspb.MachineLSE, sus []*ufspb.SchedulingUnit) []string {
	// List of available DUTs requiring a Swarming bot.
	var ad []string
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
		if seen {
			// At least 1 DUT in the SU is in the lses which means the SU share the same cloudbot hive/prefix as lses
			s := ufsUtil.RemovePrefix(su.GetName())
			if dutInOtherHives(s, prefix, dutIDMap) {
				// The SU is still running on other prefix/hive.
				logging.Infof(ctx, "Scheduling Unit %s is still running on other hives; skipping", s)
				continue
			}
			ad = append(ad, s)
		}
	}
	for lse, seen := range lsesInSU {
		if seen {
			// The DUT is part of a scheduling unit.
			continue
		}
		if dutInOtherHives(lse, prefix, dutIDMap) {
			// The DUT is still running on other hives.
			logging.Infof(ctx, "DUT %s is still running on other hives; skipping", lse)
			continue
		}
		ad = append(ad, lse)
	}
	return ad
}

// dutInOtherHives returns true if dut exist in a running bot of other hives
func dutInOtherHives(dut, prefix string, dutIDMap map[string][]string) bool {
	if ids, ok := dutIDMap[dut]; ok {
		for _, id := range ids {
			if len(prefix) > len(id) || id[:len(prefix)] != prefix {
				// The dut is running on different bot prefix.
				return true
			}
		}
	}
	return false
}

// UpdateConfig is a wrapper around the current provider UpdateConfig method.
func (r *regulator) UpdateConfig(ctx context.Context, hns []string, cfID string) error {
	return r.bpiClient.UpdateConfig(ctx, hns, cfID)
}

// DutMapFromBots return a map of DUT name from a list of Swarming bots.
func (r *regulator) DutMapFromBots(ctx context.Context, dbs []*apipb.BotInfo) map[string][]string {
	duts := make(map[string][]string, len(dbs))
	for _, db := range dbs {
		for _, d := range db.GetDimensions() {
			if d.Key == "dut_name" {
				dutName := d.Value[0]
				if _, found := duts[dutName]; !found {
					duts[dutName] = []string{}
				} else {
					logging.Warningf(ctx, "more than one bot share the same DUT. dut=%q, bots=%q,%q", dutName, duts[dutName], db.GetBotId())
				}
				duts[dutName] = append(duts[dutName], db.GetBotId())
				break
			}
		}
	}
	return duts
}
