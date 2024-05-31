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

// Migrate flow migrates and rolls back DUTs based on a config migration file.
func Migrate(ctx context.Context, r *regulator.RegulatorOptions, l *LastSeenConfig) error {
	logging.Infof(ctx, "starting migrate-bots")
	m, err := migrator.NewMigrator(ctx, r)
	if err != nil {
		return err
	}
	cfg, err := m.GetMigrationConfig(ctx)
	if err != nil {
		return err
	}
	logging.Infof(ctx, "migration config: %v \n", cfg)
	digest := []byte(cfg.String())
	if l.WasSeen(digest) && !l.IsExpired() {
		logging.Infof(ctx, "cached config is up to date; exiting migration \n", l)
		return nil
	}
	cs := migrator.NewConfigSearchable(ctx, cfg.Config)
	logging.Infof(ctx, "config searchable: %v \n", cs)
	mcs, err := m.ListSFOMachines(ctx)
	if err != nil {
		return err
	}
	lses, err := m.ListSFOMachineLSEs(ctx)
	if err != nil {
		return err
	}
	bms, err := m.ComputeBoardModelToState(ctx, mcs, lses, cs)
	if err != nil {
		return err
	}
	ms := m.ComputeNextMigrationState(ctx, bms, cs)
	logging.Infof(ctx, "ms: %v", ms)
	err = m.RunBatchUpdate(ctx, ms)
	if err != nil {
		return err
	}
	l.MarkAsSeen(digest)
	logging.Infof(ctx, "ending migrate-bots")
	return nil
}
