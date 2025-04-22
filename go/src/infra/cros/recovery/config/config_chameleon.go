// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"google.golang.org/protobuf/types/known/durationpb"
)

func chameleonPlan() *Plan {
	return &Plan{
		CriticalActions: []string{
			"Mark as bad",
			"Device is pingable",
			"cros_access",
			"Update AudioBox JackPlugger State",
			"Mark as good",
		},
		Actions: map[string]*Action{
			"Mark as bad": {
				ExecName:      "chameleon_state_broken",
				RunControl:    RunControl_RUN_ONCE,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Mark as good": {
				ExecName:      "chameleon_state_working",
				RunControl:    RunControl_RUN_ONCE,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Device is pingable": {
				ExecTimeout: &durationpb.Duration{Seconds: 15},
				ExecName:    "cros_ping",
				RecoveryActions: []string{
					"Chameleon RPM power cycle",
				},
			},
			"Chameleon RPM power cycle": {
				Docs: []string{
					"Power cycle chameleon if rpm exists",
					"Ensure chameleon is SSHable on after power cycle",
					"Try to wait device to be sshable after the device being rebooted.",
				},
				Conditions: []string{
					"Has chameleon rpm info",
				},
				Dependencies: []string{
					"Power cycle chameleon by RPM",
				},
				ExecName:    "cros_access",
				ExecTimeout: &durationpb.Duration{Seconds: 150},
				RunControl:  RunControl_ALWAYS_RUN,
			},
			"Has chameleon rpm info": {
				Docs: []string{
					"Check if chameleon rpm exists",
				},
				ExecName: "device_has_rpm_info",
				ExecExtraArgs: []string{
					"device_type:chameleon",
				},
				RunControl:    RunControl_RUN_ONCE,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Power cycle chameleon by RPM": {
				Docs: []string{
					"Run rpm power cycle on chameleon",
				},
				ExecName: "device_rpm_power_cycle",
				ExecExtraArgs: []string{
					"device_type:chameleon",
				},
			},
			"Update AudioBox JackPlugger State": {
				Docs: []string{
					"Update the status of AudioBox JackPlugger.",
				},
				ExecName:               "chameleon_check_audiobox_jackplugger",
				AllowFailAfterRecovery: true,
			},
		},
	}
}
