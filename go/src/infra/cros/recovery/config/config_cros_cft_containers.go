// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

// Actions to start and stop cft containers.
func addCrosCftContainers(actions map[string]*Action) {
	am := map[string]*Action{
		"CrosToolRunner is up": {
			Docs: []string{
				"Verify that cros-tool-runner service is up and running, ",
				"the tool expected to start as part of system preparation.",
			},
			ExecName: "ctr_is_up",
		},
		"Start ADB-base": {
			Docs: []string{
				"Pull and run adb-base container",
			},
			Conditions: []string{
				"Is not cloudbot",
				"CrosToolRunner is up",
			},
			Dependencies: []string{
				// Always first stop in case somethine left out from last run.
				"Stop ADB-base",
			},
			ExecName:               "ctr_start_adb_container",
			AllowFailAfterRecovery: true,
		},
		"Stop ADB-base": {
			Docs: []string{
				"Stop adb-base container",
			},
			ExecName:               "ctr_stop_adb_container",
			AllowFailAfterRecovery: true,
		},
		"Start Servo-Nexus": {
			Docs: []string{
				"Pull and run servo-nexus container",
			},
			Conditions: []string{
				"Testbed has Servo",
				"CrosToolRunner is up",
			},
			Dependencies: []string{
				// Always first stop in case somethine left out from last run.
				"Stop Servo-Nexus",
			},
			ExecName:               "ctr_servo_nexus_start_container",
			AllowFailAfterRecovery: true,
		},
		"Stop Servo-Nexus": {
			Docs: []string{
				"Stop Servo-Nexus container",
			},
			ExecName:               "ctr_servo_nexus_stop_container",
			AllowFailAfterRecovery: true,
		},
		"Start Foil-provision": {
			Docs: []string{
				"Pull and run foil-provision container",
			},
			Conditions: []string{
				"CrosToolRunner is up",
			},
			Dependencies: []string{
				// Always first stop in case somethine left out from last run.
				"Stop Foil-provision",
			},
			ExecName:               "ctr_start_foil_provision_container",
			AllowFailAfterRecovery: true,
		},
		"Stop Foil-provision": {
			Docs: []string{
				"Stop Foil-provision container",
			},
			ExecName:               "ctr_stop_foil_provision_container",
			AllowFailAfterRecovery: true,
		},
		"Is not cloudbot": {
			Docs: []string{
				"Check if the process doesn't run on cloudbot.",
			},
			ExecName: "env_is_not_cloudbot",
		},
	}
	for k, v := range am {
		if _, ok := actions[k]; ok {
			panic("duplicate key:" + k + " in actions")
		}
		actions[k] = v
	}
}
