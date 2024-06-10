// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package run

import (
	suschpb "go.chromium.org/chromiumos/infra/proto/go/testplans"

	"infra/cros/cmd/kron/common"
	"infra/cros/cmd/kron/configparser"
)

var (
	allowedConfigs = map[string]bool{}
)

// isAllowed checks the migration rules to determine if a config has been
// migrated to Kron or not.
func isAllowed(config *suschpb.SchedulerConfig) bool {
	// Disallow multi-dut and firmware configs.
	if configparser.IsMultiDut(config) || configparser.IsFirmware(config) {
		return false
	}

	// Allow NEW_BUILD and DAILY configs.
	if config.GetLaunchCriteria().GetLaunchProfile() == suschpb.SchedulerConfig_LaunchCriteria_NEW_BUILD || config.GetLaunchCriteria().GetLaunchProfile() == suschpb.SchedulerConfig_LaunchCriteria_DAILY {
		return true
	}

	// Allow Explicitly included configs.
	if _, ok := allowedConfigs[config.GetName()]; ok {
		return true
	}

	// Exclude everything else.
	return false
}

// filterConfigs iterates through the triggered SuSch Configs and scrubs out all
// configs which are not on the allowlist.
//
// TODO(b/319273876): Remove slow migration logic upon completion of
// transition from SuiteScheduler to Kron.
func filterConfigs(configs []*suschpb.SchedulerConfig) []*suschpb.SchedulerConfig {
	filteredMap := []*suschpb.SchedulerConfig{}

	// Check each triggered config to ensure that they are not disallowed by the
	// current migration rules.
	for _, config := range configs {
		if !isAllowed(config) {
			common.Stdout.Printf("Config %s was filtered out by the current migration rules.", config.Name)
			continue
		}

		filteredMap = append(filteredMap, config)
	}

	return filteredMap
}
