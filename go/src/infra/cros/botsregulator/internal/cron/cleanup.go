// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cron

import (
	"context"

	"go.chromium.org/luci/common/logging"

	"infra/cros/botsregulator/internal/migrator"
	"infra/cros/botsregulator/internal/regulator"
)

// Cleanup flow rolls back excluded DUTs from Cloudbots to Drone.
// Excluded DUTs are determined by the config migration file.
func Cleanup(ctx context.Context, r *regulator.RegulatorOptions) error {
	logging.Infof(ctx, "starting cleanup-bots")
	m, err := migrator.NewMigrator(ctx, r)
	if err != nil {
		return err
	}
	cfg, err := m.GetMigrationConfig(ctx)
	if err != nil {
		return err
	}
	logging.Infof(ctx, "migration config: %v \n", cfg)
	cs := migrator.NewConfigSearchable(ctx, cfg.Config)
	logging.Infof(ctx, "config searchable: %v \n", cs)
	lses, err := m.ListSFOCloudbotsMachineLSEs(ctx)
	if err != nil {
		return err
	}
	logging.Infof(ctx, "lses: %v\n", len(lses))
	duts := m.GetExcludedDUTs(ctx, lses, cs)
	logging.Infof(ctx, "length: %v, excluded DUTs: %v \n", len(duts), duts)
	err = m.RunBatchRollback(ctx, duts)
	if err != nil {
		return err
	}
	logging.Infof(ctx, "ending cleanup-bots")
	return nil
}
