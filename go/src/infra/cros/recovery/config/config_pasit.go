// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"google.golang.org/protobuf/types/known/durationpb"
)

// pasitRepairPlan includes actions to repair PASIT testbeds.
func pasitRepairPlan() *Plan {
	return &Plan{
		CriticalActions: []string{
			"Do not run on Mobile Harness box",
			"Start container",
			"Reset switches",
			"Audit switches",
			"Stop container",
		},
		Actions: map[string]*Action{
			"Do not run on Mobile Harness box": {
				Docs: []string{
					"Check that the process is not running on Mobile Harness box.",
				},
				ExecName:      "env_is_not_mh_box",
				RunControl:    RunControl_RUN_ONCE,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Start container": {
				Docs: []string{
					"Starts a PassPort container and opens a client to it.",
				},
				Conditions: []string{
					"ctr_passport_address_not_in_scope",
					"CrosToolRunner is up",
				},
				ExecName:    "ctr_passport_start",
				ExecTimeout: &durationpb.Duration{Seconds: 120},
			},
			"Reset switches": {
				Docs: []string{
					"Resets all switches connected to the host.",
				},
				ExecName:    "ctr_passport_reset_switches",
				ExecTimeout: &durationpb.Duration{Seconds: 120},
			},
			"Audit switches": {
				Docs: []string{
					"Verifies that the switches reported by passport match those reported by PassPort.",
				},
				ExecName:               "pasit_audit_switches",
				AllowFailAfterRecovery: true,
			},
			"CrosToolRunner is up": {
				Docs: []string{
					"Verify that cros-tool-runner service is up and running, ",
					"the tool expected to start as part of system preparation.",
				},
				ExecName: "ctr_is_up",
			},
			"Stop container": {
				Docs: []string{
					"Stops the PassPort container (crt only).",
				},
				ExecName:               "ctr_passport_stop",
				AllowFailAfterRecovery: true,
			},
		},
	}
}
