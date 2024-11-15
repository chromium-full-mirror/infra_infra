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
				"Stop ADB-base",
			},
			ExecName: "ctr_start_adb_container",
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
				"Is not cloudbot",
				"Testbed has Servo",
				"CrosToolRunner is up",
			},
			Dependencies: []string{
				"Stop Servo-Nexus",
			},
			ExecName: "ctr_servo_nexus_start_container",
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
				"Is not cloudbot",
				"CrosToolRunner is up",
			},
			Dependencies: []string{
				"Stop Foil-provision",
			},
			ExecName: "ctr_start_foil_provision_container",
		},
		"Stop Foil-provision": {
			Docs: []string{
				"Stop Foil-provision container",
			},
			ExecName:               "ctr_stop_foil_provision_container",
			AllowFailAfterRecovery: true,
		},
		"Detect CacheService address": {
			Docs: []string{
				"Collect address of CacheService rom labService.",
			},
			ExecName: "cache_service_address_detection",
		},
	}
	for k, v := range am {
		if _, ok := actions[k]; ok {
			panic("duplicate key:" + k + " in actions map")
		}
		actions[k] = v
	}
}
