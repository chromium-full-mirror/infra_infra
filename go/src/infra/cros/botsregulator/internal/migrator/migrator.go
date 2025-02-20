// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package migrator defines the CloudBots migration main flow.
package migrator

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/config"
	"go.chromium.org/luci/config/cfgclient"

	"go.chromium.org/infra/cros/botsregulator/internal/clients"
	"go.chromium.org/infra/cros/botsregulator/internal/regulator"
	"go.chromium.org/infra/cros/botsregulator/protos"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

const (
	// migrationFile is the the name of the CloudBots migration file.
	migrationFile = "migration.cfg"

	cloudbotsHive            = "cloudbots"
	cloudbotsHiveCanary      = "cloudbots-canary"
	cloudbotsLargeHive       = "cloudbots-large"
	cloudbotsLargeHiveCanary = "cloudbots-large-canary"
	droneHive                = "e"
)

// migrationState represents a state of the migration where
// CloudbotsSmall shows the machineLSEs with a cloudbots hive.
// CloudbotsLarge shows the machineLSEs with a cloudbots-large hive.
// CloudbotsSmallCanary shows the machineLSEs with a cloudbots-canary hive.
// CloudbotsLargeCanary shows the machineLSEs with a cloudbots-large-canary hive.
// Drone shows the machineLses with a non-cloudbots hive.
type migrationState struct {
	CloudbotsSmall       []string
	CloudbotsSmallCanary []string
	CloudbotsLarge       []string
	CloudbotsLargeCanary []string
	Drone                []string
}

type migrator struct {
	cfgClient config.Interface
	ufsClient clients.UFSClient
}

func NewMigrator(ctx context.Context, r *regulator.RegulatorOptions) (*migrator, error) {
	logging.Infof(ctx, "creating migrator \n")
	uc, err := clients.NewUFSClient(ctx, r.UFS)
	if err != nil {
		return nil, err
	}
	cc := clients.NewConfigClient(ctx)
	return &migrator{
		cfgClient: cc,
		ufsClient: uc,
	}, nil
}

// GetMigrationConfig fetches CloudBots migration file from luci-config.
func (m *migrator) GetMigrationConfig(ctx context.Context) (*protos.Migration, error) {
	logging.Infof(ctx, "fetching migration file: %s \n", migrationFile)
	out := &protos.Migration{}
	cfg, err := m.cfgClient.GetConfig(ctx, "services/${appid}", migrationFile, false)
	if err != nil {
		return nil, errors.Annotate(err, "could not fetch migration file").Err()
	}
	dest := cfgclient.ProtoText(out)
	if err := dest(cfg.Content); err != nil {
		return nil, errors.Annotate(err, "could not parse migration file").Err()
	}
	return out, nil
}

// ListSFOMachines only returns the machines located in sfo36/em25.
func (m *migrator) ListSFOMachines(ctx context.Context) ([]*ufspb.Machine, error) {
	logging.Infof(ctx, "fetching machines in SFO36")
	ctx = clients.SetUFSNamespace(ctx, "os")
	filters := []string{"zone=ZONE_SFO36_OS"}
	res, err := m.ufsClient.BatchListMachines(ctx, filters, 0, false, false)
	if err != nil {
		return nil, err
	}
	mcs := make([]*ufspb.Machine, len(res))
	for i, r := range res {
		mcs[i] = r.(*ufspb.Machine)
	}
	return mcs, nil
}

// ListSFOMachineLSEs only returns the machineLSEs located in sfo36/em25.
func (m *migrator) ListSFOMachineLSEs(ctx context.Context) ([]*ufspb.MachineLSE, error) {
	logging.Infof(ctx, "fetching machineLSEs in SFO36")
	ctx = clients.SetUFSNamespace(ctx, "os")
	filters := []string{"zone=ZONE_SFO36_OS"}
	res, err := m.ufsClient.BatchListMachineLSEs(ctx, filters, 0, false, false)
	if err != nil {
		return nil, err
	}
	lses := make([]*ufspb.MachineLSE, len(res))
	for i, r := range res {
		lses[i] = r.(*ufspb.MachineLSE)
	}
	return lses, nil
}

// ListSFOCloudbotsMachineLSEs only returns the cloudbots machineLSEs located in sfo36/em25.
func (m *migrator) ListSFOCloudbotsMachineLSEs(ctx context.Context) ([]*ufspb.MachineLSE, error) {
	logging.Infof(ctx, "fetching Cloudbots machineLSEs in SFO36")
	ctx = clients.SetUFSNamespace(ctx, "os")
	filters := []string{"zone=ZONE_SFO36_OS & hive=cloudbots"}
	res, err := m.ufsClient.BatchListMachineLSEs(ctx, filters, 0, false, false)
	if err != nil {
		return nil, err
	}
	lses := make([]*ufspb.MachineLSE, len(res))
	for i, r := range res {
		lses[i] = r.(*ufspb.MachineLSE)
	}
	return lses, nil
}

// ComputeBoardModelToState returns a map of board/model to migration state.
// This map represents the current state of the migration for each board/model combination in UFS.
func (m *migrator) ComputeBoardModelToState(ctx context.Context, mcs []*ufspb.Machine, lses []*ufspb.MachineLSE, searchable *configSearchable) (map[string]*migrationState, error) {
	logging.Infof(ctx, "reconciliating machines and machineLSEs")
	machines := make(map[string]*ufspb.Machine, len(mcs))
	for _, mc := range mcs {
		machines[ufsUtil.RemovePrefix(mc.GetName())] = mc
	}
	bms := make(map[string]*migrationState)
	for _, lse := range lses {
		stripped := ufsUtil.RemovePrefix(lse.GetName())
		// Filtering out DUTs based on DUT name.
		if reg, ok := shouldExcludeDUT(searchable, stripped); ok {
			logging.Infof(ctx, "dut: %s matches regexp: %s in exclude_duts in %s; skipping", stripped, reg, migrationFile)
			continue
		}
		// Filtering out DUTs based on pool name.
		if pool, ok := shouldExcludePool(searchable, lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPools()); ok {
			logging.Infof(ctx, "pool: %s found in exclude_pools in %s for DUT %s; skipping", pool, migrationFile, stripped)
			continue
		}
		for _, machine := range lse.GetMachines() {
			m, ok := machines[machine]
			if !ok {
				// Should not happen. These lses are filtered by sfo36 zone.
				logging.Errorf(ctx, "machine: %s from lse: %v is not present in sfo machine set", machine, lse.GetName())
				continue
			}
			key := fmt.Sprintf("%s/%s", m.GetChromeosMachine().GetBuildTarget(), m.GetChromeosMachine().GetModel())
			if _, ok := bms[key]; !ok {
				bms[key] = &migrationState{}
			}
			var h string
			if lse.GetChromeosMachineLse().GetDeviceLse().GetLabstation() != nil {
				h = lse.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetHive()
			} else {
				h = lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetHive()
			}
			switch h {
			case cloudbotsHive:
				bms[key].CloudbotsSmall = append(bms[key].CloudbotsSmall, stripped)
			case cloudbotsLargeHive:
				bms[key].CloudbotsLarge = append(bms[key].CloudbotsLarge, stripped)
			case cloudbotsHiveCanary:
				bms[key].CloudbotsSmallCanary = append(bms[key].CloudbotsSmallCanary, stripped)
			case cloudbotsLargeHiveCanary:
				bms[key].CloudbotsLargeCanary = append(bms[key].CloudbotsLargeCanary, stripped)
			case droneHive, "":
				// droneHive and empty hive are the only values drone-queen captures.
				// TODO(b/338233053): change to e after backfill.
				bms[key].Drone = append(bms[key].Drone, stripped)
			}
		}
	}
	return bms, nil
}

// ComputeNextMigrationState returns 2 slices of machineLSEs to be migrated/rolled backed based on the config file provided.
// This function does not filter out DUTs. The DUT exclusion happens earlier in the flow.
func (m *migrator) ComputeNextMigrationState(ctx context.Context, bms map[string]*migrationState, cs *configSearchable) *migrationState {
	logging.Infof(ctx, "computing the next migration state")
	// MachinesLSEs to be converted to CloudBots or Drone.
	migrationNext := &migrationState{}
	sortedKeys := make([]string, 0, len(bms))
	for bm := range bms {
		sortedKeys = append(sortedKeys, bm)
	}
	sort.Strings(sortedKeys)
	for _, bm := range sortedKeys {
		state := bms[bm]
		var targetSmall, targetLarge int32
		t := strings.Split(bm, "/")
		if len(t) != 2 {
			panic("boardModelToState keys should always contain one '/'")
		}
		board := t[0]
		model := t[1]
		// Small bot percentage
		if target, ok := cs.overrideBoardModel[bm]; ok {
			// Board/Model override.
			targetSmall = target
		} else if target, ok := cs.overrideBoardModel[fmt.Sprintf("*/%s", model)]; ok {
			// Model override.
			targetSmall = target
		} else if target, ok := cs.overrideBoardModel[fmt.Sprintf("%s/*", board)]; ok {
			// Board override.
			targetSmall = target
		} else if _, ok := cs.overrideLowRisks[model]; ok {
			// Low risk model override.
			targetSmall = cs.minLowRiskModelsPercentage
		} else {
			// No override.
			targetSmall = cs.minCloudbotsPercentage
		}
		// Large memory bot percentage
		if target, ok := cs.largeMemoryOverrideBoardModel[bm]; ok {
			// Board/Model override.
			targetLarge = target
		} else if target, ok := cs.largeMemoryOverrideBoardModel[fmt.Sprintf("*/%s", model)]; ok {
			// Model override.
			targetLarge = target
		} else if target, ok := cs.largeMemoryOverrideBoardModel[fmt.Sprintf("%s/*", board)]; ok {
			// Board override.
			targetLarge = target
		} else {
			// No override.
			targetLarge = cs.minLargeMemoryPercentage
		}
		computeNextModelState(ctx, bm, targetSmall, targetLarge, cs.canaryPercentage, state, migrationNext)
	}
	return migrationNext
}

// RunBatchUpdate calls UFS to update all the hive of the machineLSEs in migration state.
func (m *migrator) RunBatchUpdate(ctx context.Context, migrationNext *migrationState) error {
	errs := errors.NewLazyMultiError(len(migrationNext.CloudbotsSmall) + len(migrationNext.CloudbotsSmallCanary) + len(migrationNext.CloudbotsLarge) + len(migrationNext.CloudbotsLargeCanary) + len(migrationNext.Drone))
	cpt := 0
	ctx = clients.SetUFSNamespace(ctx, "os")
	logging.Infof(ctx, "starting batch update for small cloudBots")
	for _, cbsmall := range migrationNext.CloudbotsSmall {
		req := clients.InitializeUpdateLSERequest(cbsmall, cloudbotsHive)
		_, err := m.ufsClient.UpdateMachineLSE(ctx, req)
		if err != nil {
			logging.Errorf(ctx, "failed to update machineLSE %s to hive %s: %v", cbsmall, cloudbotsHive, err)
			errs.Assign(cpt, err)
		}
		cpt++
	}
	logging.Infof(ctx, "starting batch update for small canary cloudBots")
	for _, cbcsmall := range migrationNext.CloudbotsSmallCanary {
		req := clients.InitializeUpdateLSERequest(cbcsmall, cloudbotsHiveCanary)
		_, err := m.ufsClient.UpdateMachineLSE(ctx, req)
		if err != nil {
			logging.Errorf(ctx, "failed to update machineLSE %s to hive %s: %v", cbcsmall, cloudbotsHiveCanary, err)
			errs.Assign(cpt, err)
		}
		cpt++
	}
	logging.Infof(ctx, "starting batch update for large cloudBots")
	for _, cblarge := range migrationNext.CloudbotsLarge {
		req := clients.InitializeUpdateLSERequest(cblarge, cloudbotsLargeHive)
		_, err := m.ufsClient.UpdateMachineLSE(ctx, req)
		if err != nil {
			logging.Errorf(ctx, "failed to update machineLSE %s to hive %s: %v", cblarge, cloudbotsLargeHive, err)
			errs.Assign(cpt, err)
		}
		cpt++
	}
	logging.Infof(ctx, "starting batch update for large canary cloudBots")
	for _, cbclarge := range migrationNext.CloudbotsLargeCanary {
		req := clients.InitializeUpdateLSERequest(cbclarge, cloudbotsLargeHiveCanary)
		_, err := m.ufsClient.UpdateMachineLSE(ctx, req)
		if err != nil {
			logging.Errorf(ctx, "failed to update machineLSE %s to hive %s: %v", cbclarge, cloudbotsLargeHiveCanary, err)
			errs.Assign(cpt, err)
		}
		cpt++
	}
	logging.Infof(ctx, "starting batch update for drone")
	for _, drone := range migrationNext.Drone {
		req := clients.InitializeUpdateLSERequest(drone, droneHive)
		_, err := m.ufsClient.UpdateMachineLSE(ctx, req)
		if err != nil {
			logging.Errorf(ctx, "failed to update machineLSE %s to hive %s: %v", drone, droneHive, err)
			errs.Assign(cpt, err)
		}
		cpt++
	}
	return errs.Get()
}

// computeNextModelState computes the DUTs to migrate/roll back
// based on a target percentage of CloudBots DUTs and a current state.
// This results in appending DUTs to nextState.
// These DUTs will get their hive switched further down.
func computeNextModelState(ctx context.Context, bm string, targetSmall, targetLarge, canaryPercentage int32, currentState, nextState *migrationState) {
	logging.Infof(ctx, "computeNextModelState: %s with small bot target %d%%, large bot target %d%%", bm, targetSmall, targetLarge)
	totalDUTs := float64(len(currentState.CloudbotsSmall) + len(currentState.CloudbotsLarge) + len(currentState.CloudbotsSmallCanary) + len(currentState.CloudbotsLargeCanary) + len(currentState.Drone))
	targetSmallPercentage := float64(targetSmall)
	targetLargePercentage := float64(targetLarge)
	if targetSmallPercentage+targetLargePercentage > 100 {
		targetSmallPercentage = 100 - targetLargePercentage
		logging.Warningf(ctx, "computeNextModelState: %s, the sum of both small and large bot exceed 100%%, use new small bot target %d%%", targetSmallPercentage)
	}
	var moveBots []string
	// Number of CloudBots, Drone, Small and Large CloudBots DUTs for this model expected after this migration iteration.
	cloudbotsLargeAmount := math.Ceil((targetLargePercentage * totalDUTs) / 100)
	cloudbotsSmallAmount := math.Ceil((targetSmallPercentage * totalDUTs) / 100)
	if totalDUTs < cloudbotsLargeAmount+cloudbotsSmallAmount {
		// There is a change total cloudbots is greater than total duts since we round up the dut percentage
		cloudbotsSmallAmount = totalDUTs - cloudbotsLargeAmount
	}
	droneAmount := totalDUTs - cloudbotsLargeAmount - cloudbotsSmallAmount
	// Recalculate cloudbots amount for canary percentage
	cloudbotsLargeAmountCanary := math.Ceil((float64(canaryPercentage) * cloudbotsLargeAmount) / 100)
	cloudbotsLargeAmount = cloudbotsLargeAmount - cloudbotsLargeAmountCanary
	cloudbotsSmallAmountCanary := math.Ceil((float64(canaryPercentage) * cloudbotsSmallAmount) / 100)
	cloudbotsSmallAmount = cloudbotsSmallAmount - cloudbotsSmallAmountCanary

	// Number of surplus DUTs in each category.
	surplusDrone := float64(len(currentState.Drone)) - droneAmount
	surplusCloudBotsSmall := float64(len(currentState.CloudbotsSmall)) - cloudbotsSmallAmount
	surplusCloudBotsSmallCanary := float64(len(currentState.CloudbotsSmallCanary)) - cloudbotsSmallAmountCanary
	surplusCloudBotsLarge := float64(len(currentState.CloudbotsLarge)) - cloudbotsLargeAmount
	surplusCloudBotsLargeCanary := float64(len(currentState.CloudbotsLargeCanary)) - cloudbotsLargeAmountCanary

	if surplusDrone == 0 && surplusCloudBotsSmall == 0 && surplusCloudBotsLarge == 0 && surplusCloudBotsSmallCanary == 0 && surplusCloudBotsLargeCanary == 0 {
		logging.Infof(ctx, "computeNextModelState: no change for board/model %s; skipping", bm)
		return
	}
	if surplusDrone > 0 {
		surplusBots := currentState.Drone[:int(surplusDrone)]
		moveBots = append(moveBots, surplusBots...)
		logging.Infof(ctx, "computeNextModelState: removing %v from SFO36", surplusBots)
	}
	if surplusCloudBotsSmall > 0 {
		surplusBots := currentState.CloudbotsSmall[:int(surplusCloudBotsSmall)]
		moveBots = append(moveBots, surplusBots...)
		logging.Infof(ctx, "computeNextModelState: removing %v from small cloudbots", surplusBots)
	}
	if surplusCloudBotsSmallCanary > 0 {
		surplusBots := currentState.CloudbotsSmallCanary[:int(surplusCloudBotsSmallCanary)]
		moveBots = append(moveBots, surplusBots...)
		logging.Infof(ctx, "computeNextModelState: removing %v from small canary cloudbots", surplusBots)
	}
	if surplusCloudBotsLarge > 0 {
		surplusBots := currentState.CloudbotsLarge[:int(surplusCloudBotsLarge)]
		moveBots = append(moveBots, surplusBots...)
		logging.Infof(ctx, "computeNextModelState: removing %v from large cloudbots", surplusBots)
	}
	if surplusCloudBotsLargeCanary > 0 {
		surplusBots := currentState.CloudbotsLargeCanary[:int(surplusCloudBotsLargeCanary)]
		moveBots = append(moveBots, surplusBots...)
		logging.Infof(ctx, "computeNextModelState: removing %v from large canary cloudbots", surplusBots)
	}
	start := 0
	if surplusDrone < 0 {
		nb := moveBots[start : start+int(math.Abs(surplusDrone))]
		nextState.Drone = append(nextState.Drone, nb...)
		start += len(nb)
		logging.Infof(ctx, "computeNextModelState: adding %v to SFO36", nb)
	}
	if surplusCloudBotsSmall < 0 {
		nb := moveBots[start : start+int(math.Abs(surplusCloudBotsSmall))]
		nextState.CloudbotsSmall = append(nextState.CloudbotsSmall, nb...)
		start += len(nb)
		logging.Infof(ctx, "computeNextModelState: adding %v to Small CloudBots", nb)
	}
	if surplusCloudBotsSmallCanary < 0 {
		nb := moveBots[start : start+int(math.Abs(surplusCloudBotsSmallCanary))]
		nextState.CloudbotsSmallCanary = append(nextState.CloudbotsSmallCanary, nb...)
		start += len(nb)
		logging.Infof(ctx, "computeNextModelState: adding %v to Small Canary CloudBots", nb)
	}
	if surplusCloudBotsLarge < 0 {
		nb := moveBots[start : start+int(math.Abs(surplusCloudBotsLarge))]
		nextState.CloudbotsLarge = append(nextState.CloudbotsLarge, nb...)
		start += len(nb)
		logging.Infof(ctx, "computeNextModelState: adding %v to Large CloudBots", nb)
	}
	if surplusCloudBotsLargeCanary < 0 {
		nb := moveBots[start : start+int(math.Abs(surplusCloudBotsLargeCanary))]
		nextState.CloudbotsLargeCanary = append(nextState.CloudbotsLargeCanary, nb...)
		logging.Infof(ctx, "computeNextModelState: adding %v to Large Canary CloudBots", nb)
	}
}

// shouldExcludeDUT returns true if the DUT name matches any of the exclude_duts regex in the config file.
func shouldExcludeDUT(searchable *configSearchable, stripped string) (string, bool) {
	for _, reg := range searchable.excludeDUTs {
		if reg.MatchString(stripped) {
			return reg.String(), true
		}
	}
	return "", false
}

// shouldExcludePool returns true if the DUT pools can be found in the exclude_pools set.
func shouldExcludePool(searchable *configSearchable, pools []string) (string, bool) {
	for _, pool := range pools {
		if _, ok := searchable.excludePools[pool]; ok {
			return pool, true
		}
	}
	return "", false
}

// GetExcludedDUTs returns a list of excluded DUTs to be rolled back to drone.
func (m *migrator) GetExcludedDUTs(ctx context.Context, lses []*ufspb.MachineLSE, cs *configSearchable) []string {
	var rollbackDUTs []string
	for _, lse := range lses {
		stripped := ufsUtil.RemovePrefix(lse.GetName())
		if _, ok := shouldExcludeDUT(cs, stripped); ok {
			rollbackDUTs = append(rollbackDUTs, stripped)
		} else if _, ok := shouldExcludePool(cs, lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPools()); ok {
			rollbackDUTs = append(rollbackDUTs, stripped)
		}
	}
	return rollbackDUTs
}

// RunBatchRollback updates the hive to all the specified lses,
// effectively rolling them back to Drone.
func (m *migrator) RunBatchRollback(ctx context.Context, lses []string) error {
	logging.Infof(ctx, "starting batch update for cloudBots")
	errs := errors.NewLazyMultiError(len(lses))
	ctx = clients.SetUFSNamespace(ctx, "os")
	for i, lse := range lses {
		req := clients.InitializeUpdateLSERequest(lse, "e")
		_, err := m.ufsClient.UpdateMachineLSE(ctx, req)
		if err != nil {
			logging.Errorf(ctx, "failed to update machineLSE %s to hive drone: %v", lse, err)
			errs.Assign(i, err)
		}
	}
	return errs.Get()
}
