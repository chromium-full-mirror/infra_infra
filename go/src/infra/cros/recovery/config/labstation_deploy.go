// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"fmt"

	"google.golang.org/protobuf/types/known/durationpb"
)

// LabstationDeployConfig provides config for deploy labstation task.
func LabstationDeployConfig() *Configuration {
	beforeLogName, beforeLogActions := labstationCollectionLogs("before")
	afterLogName, afterLogActions := labstationCollectionLogs("after")

	criticalActions := []string{
		"Set state: needs_deploy",
		"check_host_info",
		beforeLogName,
		"Device is SSHable",
		"Update inventory info",
		"Remove whitelabel_tag VPD field if necessary",
		"Installed OS is stable",
		"Remove reboot requests from host",
		"Update provisioned info",
		"Validate RPM info",
		"Set state: ready",
		afterLogName,
	}
	actions := map[string]*Action{
		"check_host_info": {
			Docs: []string{
				"Check basic info for deployment.",
			},
			Dependencies: []string{
				"dut_has_board_name",
				"dut_has_model_name",
			},
			ExecName:      "sample_pass",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			RunControl:    RunControl_RUN_ONCE,
		},
		"Update inventory info": {
			Docs: []string{
				"Updating device info in inventory.",
			},
			Dependencies: []string{
				"cros_update_hwid_to_inventory",
				"cros_update_serial_number_inventory",
			},
			ExecName:      "sample_pass",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Installed OS is stable": {
			Docs: []string{
				"Verify that OS on the device is stable.",
				"Labstation will be rebooted to make it ready for use.",
			},
			Conditions: []string{
				"has_stable_version_cros_image",
			},
			ExecName: "cros_is_on_stable_version",
			RecoveryActions: []string{
				"Install stable OS",
				"Power cycle by RPM",
			},
		},
		"Install stable OS": {
			Docs: []string{
				"Install stable OS on the device.",
				"Labstation will be rebooted to make it ready for use.",
			},
			Conditions: []string{
				"has_stable_version_cros_image",
				"cros_not_on_stable_version",
			},
			ExecName:    "cros_provision",
			ExecTimeout: &durationpb.Duration{Seconds: 3600},
		},
		"Remove reboot requests from host": {
			Docs: []string{
				"Remove reboot request flag files.",
			},
			ExecName:               "cros_remove_all_reboot_request",
			AllowFailAfterRecovery: true,
		},
		"Update provisioned info": {
			Docs: []string{
				"Update OS version for provision info.",
			},
			ExecName: "cros_update_provision_info",
		},
		"Validate RPM info": {
			Docs: []string{
				"Validate and update rpm_state.",
				"The execs is not ready yet.",
			},
			Conditions: []string{
				"rpm_action_enabled",
				"has_rpm_info",
			},
			ExecName:    "rpm_audit_without_battery",
			ExecTimeout: &durationpb.Duration{Seconds: 900},
			RecoveryActions: []string{
				"Power cycle by RPM",
			},
		},
		"Device is SSHable": {
			Docs: []string{
				"This verifier checks whether the host is accessible over ssh.",
			},
			ExecName:    "cros_ssh",
			ExecTimeout: &durationpb.Duration{Seconds: 30},
			RunControl:  RunControl_ALWAYS_RUN,
			RecoveryActions: []string{
				"Power cycle by RPM",
			},
		},
		"Power cycle by RPM": {
			Docs: []string{
				"Power cycle the labstation via RPM.",
			},
			Conditions: []string{
				"rpm_action_enabled",
				"has_rpm_info",
			},
			ExecName: "rpm_power_cycle",
			// 60 seconds timeout via HTTP based call and 60 seconds fallback to RPM service.
			ExecTimeout: &durationpb.Duration{Seconds: 120},
			RunControl:  RunControl_ALWAYS_RUN,
		},
		"VPD cache has whitelabel_tag field": {
			Docs: []string{
				"Check if unwanted whitelabel_tag field in VPD cache.",
			},
			ExecName: "cros_run_command",
			ExecExtraArgs: []string{
				"host:",
				"command:vpd -l | grep whitelabel_tag",
			},
		},
		"Remove whitelabel_tag VPD field if necessary": {
			// See b/325495298 for context of why we need this.
			Docs: []string{
				"Remove whitelabel_tag field from VPD cache when the field exists.",
			},
			Conditions: []string{
				"VPD cache has whitelabel_tag field",
			},
			ExecName: "cros_run_command",
			ExecExtraArgs: []string{
				"host:",
				"command:vpd -d whitelabel_tag",
			},
		},
	}
	for k, v := range beforeLogActions {
		if _, ok := actions[k]; ok {
			panic(fmt.Sprintf("Attempt to add duplicate action: %q", k))
		}
		actions[k] = v
	}
	for k, v := range afterLogActions {
		if _, ok := actions[k]; ok {
			panic(fmt.Sprintf("Attempt to add duplicate action: %q", k))
		}
		actions[k] = v
	}
	addStateActions(actions)
	return &Configuration{
		PlanNames: []string{PlanCrOS},
		Plans: map[string]*Plan{
			PlanCrOS: {
				AllowFail:       false,
				CriticalActions: criticalActions,
				Actions:         actions,
			},
		},
	}
}
