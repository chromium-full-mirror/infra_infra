// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package migrator

import (
	"context"
	"fmt"
	"regexp"

	"go.chromium.org/luci/common/logging"

	"infra/cros/botsregulator/protos"
)

type configSearchable struct {
	minCloudbotsPercentage        int32
	minLowRiskModelsPercentage    int32
	minLargeMemoryPercentage      int32
	canaryPercentage              int32
	excludeDUTs                   []*regexp.Regexp
	excludePools                  map[string]struct{}
	overrideBoardModel            map[string]int32
	overrideLowRisks              map[string]struct{}
	largeMemoryOverrideBoardModel map[string]int32
}

// NewConfigSearchable returns an easily searchable struct composed of maps instead of slices.
func NewConfigSearchable(ctx context.Context, config *protos.Config) *configSearchable {
	obm := make(map[string]int32)
	lmobm := make(map[string]int32)
	// Override board/model.
	for _, override := range config.Overrides {
		key := fmt.Sprintf("%s/%s", override.Board, override.Model)
		if _, ok := obm[key]; !ok {
			obm[key] = override.Percentage
		} else {
			logging.Errorf(ctx, "board/model combination: %s/%s has already been processed. Check for duplicate in %s", override.Board, override.Model, migrationFile)
		}
	}
	// Large memory override board/model.
	for _, override := range config.LargeMemoryOverrides {
		key := fmt.Sprintf("%s/%s", override.Board, override.Model)
		if _, ok := lmobm[key]; !ok {
			lmobm[key] = override.Percentage
		} else {
			logging.Errorf(ctx, "large memory board/model combination: %s/%s has already been processed. Check for duplicate in %s", override.Board, override.Model, migrationFile)
		}
	}
	// Low risk models.
	lr := make(map[string]struct{})
	for _, m := range config.LowRiskModels {
		if _, ok := lr[m]; !ok {
			lr[m] = struct{}{}
		} else {
			logging.Errorf(ctx, "low rik model %s has already been processed. Check for duplicate in %s", m, migrationFile)
		}
	}
	// Exclude DUTs.
	var regs []*regexp.Regexp
	for _, s := range config.ExcludeDuts {
		reg, err := regexp.Compile(s)
		if err != nil {
			logging.Errorf(ctx, "%s is not a valid regular expression. Check for invalid in %s", s, migrationFile)
		} else {
			regs = append(regs, reg)
		}
	}
	// Exclude pools.
	pools := make(map[string]struct{})
	for _, pool := range config.ExcludePools {
		if _, ok := pools[pool]; !ok {
			pools[pool] = struct{}{}
		} else {
			logging.Errorf(ctx, "exclude pool %s has already been processed. Check for duplicate in %s", pool, migrationFile)
		}
	}
	searchable := &configSearchable{
		minCloudbotsPercentage:        config.MinCloudbotsPercentage,
		minLowRiskModelsPercentage:    config.MinLowRiskModelsPercentage,
		minLargeMemoryPercentage:      config.MinLargeMemoryPercentage,
		canaryPercentage:              config.CanaryPercentage,
		excludeDUTs:                   regs,
		excludePools:                  pools,
		overrideBoardModel:            obm,
		overrideLowRisks:              lr,
		largeMemoryOverrideBoardModel: lmobm,
	}
	return searchable
}
