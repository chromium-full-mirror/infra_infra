// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package migrator defines the CloudBots migration main flow.
package migrator

import (
	"context"
	"fmt"
	"math"
	"strings"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/config/cfgclient"

	"infra/cros/botsregulator/internal/clients"
	"infra/cros/botsregulator/internal/regulator"
	"infra/cros/botsregulator/protos"
	ufspb "infra/unifiedfleet/api/v1/models"
	ufsUtil "infra/unifiedfleet/app/util"
)

// migrationFile is the the name of the CloudBots migration file.
const migrationFile = "migration.cfg"

// migrationState represents a state of the migration where
// Cloudbots shows the machineLSEs with a cloudbots hive.
// Drone shows the machineLses with a non-cloudbots hive.
type migrationState struct {
	Cloudbots []string
	Drone     []string
}

type migrator struct {
	ufsClient clients.UFSClient
}

func NewMigrator(ctx context.Context, r *regulator.RegulatorOptions) (*migrator, error) {
	logging.Infof(ctx, "creating migrator \n")
	uc, err := clients.NewUFSClient(ctx, r.UFS)
	if err != nil {
		return nil, err
	}
	return &migrator{
		ufsClient: uc,
	}, nil
}

// GetMigrationConfig fetches CloudBots migration file from luci-config.
func (m *migrator) GetMigrationConfig(ctx context.Context) (*protos.Migration, error) {
	logging.Infof(ctx, "fetching migration file: %s \n", migrationFile)
	out := &protos.Migration{}
	err := cfgclient.Get(ctx, "services/${appid}", migrationFile, cfgclient.ProtoText(out), nil)
	if err != nil {
		return nil, errors.Annotate(err, "could not fetch migration file").Err()
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
			switch lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetHive() {
			case "cloudbots":
				bms[key].Cloudbots = append(bms[key].Cloudbots, stripped)
			case "e", "":
				// e and empty hive are the only values drone-queen captures.
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
	for bm, state := range bms {
		t := strings.Split(bm, "/")
		if len(t) != 2 {
			panic("boardModelToState keys should always contain one '/'")
		}
		board := t[0]
		model := t[1]
		if target, ok := cs.overrideBoardModel[bm]; ok {
			// Board/Model override.
			computeNextModelState(ctx, bm, target, state, migrationNext)
		} else if target, ok := cs.overrideBoardModel[fmt.Sprintf("*/%s", model)]; ok {
			// Model override.
			computeNextModelState(ctx, bm, target, state, migrationNext)
		} else if target, ok := cs.overrideBoardModel[fmt.Sprintf("%s/*", board)]; ok {
			// Board override.
			computeNextModelState(ctx, bm, target, state, migrationNext)
		} else if _, ok := cs.overrideLowRisks[model]; ok {
			// Low risk model override.
			computeNextModelState(ctx, bm, cs.minLowRiskModelsPercentage, state, migrationNext)
		} else {
			// No override.
			computeNextModelState(ctx, bm, cs.minCloudbotsPercentage, state, migrationNext)
		}
	}
	return migrationNext
}

// RunBatchUpdate calls UFS to update all the hive of the machineLSEs in migration state.
func (m *migrator) RunBatchUpdate(ctx context.Context, migrationNext *migrationState) error {
	logging.Infof(ctx, "starting batch update for cloudBots")
	errs := errors.NewLazyMultiError(len(migrationNext.Cloudbots) + len(migrationNext.Drone))
	cpt := 0
	for _, cb := range migrationNext.Cloudbots {
		req := clients.InitializeUpdateDUTRequest(cb, "cloudbots")
		ctx = clients.SetUFSNamespace(ctx, "os")
		_, err := m.ufsClient.UpdateMachineLSE(ctx, req)
		if err != nil {
			logging.Errorf(ctx, "failed to update machineLSE %s to hive cloudbots: %v", cb, err)
			errs.Assign(cpt, err)
		}
		cpt++
	}
	logging.Infof(ctx, "starting batch update for drone")
	for _, drone := range migrationNext.Drone {
		req := clients.InitializeUpdateDUTRequest(drone, "e")
		ctx = clients.SetUFSNamespace(ctx, "os")
		_, err := m.ufsClient.UpdateMachineLSE(ctx, req)
		if err != nil {
			logging.Errorf(ctx, "failed to update machineLSE %s to hive e: %v", drone, err)
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
func computeNextModelState(ctx context.Context, bm string, target int32, currentState, nextState *migrationState) {
	logging.Infof(ctx, "computeNextModelState: %s with target %d%%", bm, target)
	totalDUTs := float64(len(currentState.Cloudbots) + len(currentState.Drone))
	targetPercentage := float64(target)
	// Number of CloudBots DUTs for this model expected after this migration iteration.
	cloudbotsAmount := math.Ceil((targetPercentage * totalDUTs) / 100)
	diff := len(currentState.Cloudbots) - int(cloudbotsAmount)
	if diff == 0 {
		logging.Infof(ctx, "computeNextModelState: no change for board/model %s; skipping", bm)
	} else if diff < 0 {
		ncb := currentState.Drone[:int(math.Abs(float64(diff)))]
		nextState.Cloudbots = append(nextState.Cloudbots, ncb...)
		logging.Infof(ctx, "computeNextModelState: adding %v to CloudBots", ncb)
	} else {
		nsf := currentState.Cloudbots[:int(math.Abs(float64(diff)))]
		nextState.Drone = append(nextState.Drone, nsf...)
		logging.Infof(ctx, "computeNextModelState: adding %v to SFO36", nsf)
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
	for i, lse := range lses {
		req := clients.InitializeUpdateDUTRequest(lse, "e")
		ctx = clients.SetUFSNamespace(ctx, "os")
		_, err := m.ufsClient.UpdateMachineLSE(ctx, req)
		if err != nil {
			logging.Errorf(ctx, "failed to update machineLSE %s to hive drone: %v", lse, err)
			errs.Assign(i, err)
		}
	}
	return errs.Get()
}
