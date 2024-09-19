// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

// crosBasePlan creates a plan that must always be pass.
// All actions indicate that something is wrong and there is nothing we can do about it.
func crosBasePlan(isRepair bool) *Plan {
	var ca []string
	if isRepair {
		ca = append(ca, "Set state: repair_failed")
	} else {
		ca = append(ca, "Set state: needs_deploy")
	}
	ca = append(ca,
		"DUT has board info",
		"DUT has model info",
		"CrosToolRunner is up",
		"Start ADB container",
	)
	return &Plan{
		CriticalActions: ca,
		Actions:         crosBaseActions(),
	}
}

func crosBaseActions() map[string]*Action {
	return map[string]*Action{
		"DUT has board info": {
			ExecName:      "dut_has_board_name",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"DUT has model info": {
			ExecName:      "dut_has_model_name",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"CrosToolRunner is up": {
			Docs: []string{
				"Verify that cros-tool-runner service is up and running, ",
				"the tool expected to start as part of system preparation.",
			},
			ExecName: "ctr_is_up",
		},
		"Start ADB container": {
			Docs: []string{
				"Pull and run adb-base container",
			},
			Dependencies: []string{
				// Always first stop in case somethine left out from last run.
				"Stop ADB container",
			},
			ExecName: "ctr_start_adb_container",
		},
		"Stop ADB container": {
			Docs: []string{
				"Stop adb-base container",
			},
			ExecName:               "ctr_stop_adb_container",
			AllowFailAfterRecovery: true,
		},
		"Set state: needs_deploy": {
			Docs: []string{
				"The action set devices with request to be redeployed.",
			},
			ExecName: "dut_set_state",
			ExecExtraArgs: []string{
				"state:needs_deploy",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Set state: repair_failed": {
			Docs: []string{
				"The action set devices with state means that repair tsk did not success to recover the devices.",
			},
			ExecName: "dut_set_state",
			ExecExtraArgs: []string{
				"state:repair_failed",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
	}
}
