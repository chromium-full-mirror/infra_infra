// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cron defines the service's cron job.
package cron

import (
	"context"
	"sort"

	"go.chromium.org/luci/common/logging"

	"infra/cros/botsregulator/internal/regulator"
)

var dronePrefix = "crossk"

// Regulate is BotsRegulator main flow.
// It fetches available DUTs from UFS based on specific filters
// and sends out the result to a predefined Bots Provider Interface.
func Regulate(ctx context.Context, opts *regulator.RegulatorOptions) error {
	logging.Infof(ctx, "starting regulate-bots")
	r, err := regulator.NewRegulator(ctx, opts)
	if err != nil {
		return err
	}
	sus, err := r.ListAllSchedulingUnits(ctx)
	if err != nil {
		return err
	}
	dbs, err := r.ListAllRunningBots(ctx)
	if err != nil {
		return err
	}
	dutIDMap := r.DutMapFromBots(ctx, dbs)
	ch := r.ConfigHive()
	sortedKey := make([]string, 0, len(ch))
	for c := range ch {
		sortedKey = append(sortedKey, c)
	}
	sort.Strings(sortedKey)
	for _, c := range sortedKey {
		h := ch[c]
		if c == dronePrefix {
			// Drone DUTs are not handled by botsregulator
			logging.Infof(ctx, "Skipping drone config")
			continue
		}
		lses, err := r.ListAllMachineLSEsByHive(ctx, h)
		if err != nil {
			return err
		}
		if len(lses) == 0 {
			logging.Infof(ctx, "no lse found, exiting early")
			continue
		}
		logging.Infof(ctx, "lses: %v\n", lses)
		ad := r.ConsolidateAvailableDUTs(ctx, c, dutIDMap, lses, sus)
		logging.Infof(ctx, "available DUTs: %v\n", ad)
		err = r.UpdateConfig(ctx, ad, c)
		if err != nil {
			return err
		}
	}
	logging.Infof(ctx, "ending regulate-bots")
	return nil
}
