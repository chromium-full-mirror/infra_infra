// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import "google.golang.org/protobuf/types/known/durationpb"

// CrosBrowserDUTRepairConfig provides config for repair tasks for ChromeOS DUTs for browser testing.
func CrosBrowserDUTRepairConfig() *Configuration {
	return &Configuration{
		PlanNames: []string{
			PlanCrOS,
		},
		Plans: map[string]*Plan{
			PlanCrOS: {
				AllowFail: false,
				CriticalActions: []string{
					"Set state: repair_failed",
					"Device is SSHable",
					"Ensure firmware is in good state",
					"RO Firmware validations without servo",
					"Has repair-request for re-provision",
					"Update provisioned info",
					"Set state: ready",
				},
				Actions: CrOSBrowserDUTRepairActions(),
			},
			PlanClosing: setAllowFail(&Plan{
				CriticalActions: []string{
					"Update DUT state for failures more than threshold",
				},
				Actions: crosRepairClosingActions()}, true),
		},
	}
}

func CrOSBrowserDUTRepairActions() map[string]*Action {
	browserActions := map[string]*Action{
		"Device is SSHable": {
			Docs: []string{
				"Verify that device is reachable by SSH.",
				"Limited to 15 seconds.",
			},
			ExecName:    "cros_ssh",
			ExecTimeout: &durationpb.Duration{Seconds: 15},
			RecoveryActions: []string{
				"Power cycle DUT by RPM and wait for ping",
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Power cycle DUT by RPM and wait for ping": {
			Docs: []string{
				"Perform RPM cycle and wait to device to boot back.",
			},
			Conditions: []string{
				"has_rpm_info",
			},
			Dependencies: []string{
				"rpm_power_cycle",
				"Wait to be pingable (normal boot)",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Ensure firmware is in good state": {
			Docs: []string{
				"Ensure that firmware is in good state.",
			},
			Conditions: []string{
				"dut_is_not_browser_legacy_duts",
			},
			Dependencies: []string{
				"Internal storage is responsive",
			},
			ExecName: "cros_is_firmware_in_good_state",
			RecoveryActions: []string{
				"Fix FW on the DUT to match stable-version and wait to boot",
			},
		},
		"RO Firmware validations without servo": {
			Docs: []string{
				"Check if the version of RO firmware on DUT matches the stable firmware version.",
			},
			Conditions: []string{
				"dut_is_not_browser_legacy_duts",
				"has_stable_version_fw_version",
				"has_stable_version_fw_image",
			},
			ExecName: "cros_is_on_ro_firmware_stable_version",
			ExecExtraArgs: []string{
				"only_check_numbers:true",
			},
			RecoveryActions: []string{
				"Fix FW on the DUT to match stable-version and wait to boot",
			},
		},
		"Internal storage is responsive": {
			Docs: []string{
				"Verify that internal storage is responsive",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_is_file_system_writable",
			ExecExtraArgs: []string{
				"paths:/mnt/stateful_partition,/var/tmp,/mnt/stateful_partition/encrypted",
			},
			RecoveryActions: []string{
				"Quick provision OS",
				"Repair by powerwash",
			},
		},
	}
	for name, action := range crosRepairActions() {
		if _, ok := browserActions[name]; ok {
			continue
		}
		browserActions[name] = action
	}
	return browserActions
}
