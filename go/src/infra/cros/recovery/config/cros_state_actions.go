// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

func addStateActions(actions map[string]*Action) {
	am := map[string]*Action{
		"Testbed has Servo": {
			ExecName:      "dut_servo_host_present",
			RunControl:    RunControl_RUN_ONCE,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Set state: needs_deploy": {
			Docs: []string{
				"The action set devices with request to be redeployed.",
			},
			ExecName:      "dut_set_state",
			ExecExtraArgs: []string{"state:needs_deploy"},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Set state: repair_failed": {
			Docs: []string{
				"The action set devices with state means that repair tsk did not success to recover the devices.",
			},
			ExecName:      "dut_set_state",
			ExecExtraArgs: []string{"state:repair_failed"},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Set state: needs_repair": {
			Docs: []string{
				"The action set devices with state means that DUT requires repair.",
			},
			ExecName:      "dut_set_state",
			ExecExtraArgs: []string{"state:needs_repair"},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Set state: ready": {
			Docs: []string{
				"The action set devices with state ready for the testing.",
			},
			Dependencies: []string{
				"All repair-requests resolved",
				"Reset DUT-state reason",
			},
			ExecName:      "dut_set_state",
			ExecExtraArgs: []string{"state:ready"},
			RunControl:    RunControl_RUN_ONCE,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"All repair-requests resolved": {
			Docs: []string{
				"Checks if all repair requests are resolved",
			},
			ExecName:      "dut_has_no_repair_requests",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_UPLOAD_ON_ERROR},
		},
		"Reset DUT-state reason": {
			Docs: []string{
				"Reset DUT-state-reason for good DUT as it becomes stale.",
			},
			ExecName:      "dut_reset_state_reason",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
	}
	for k, v := range am {
		if _, ok := actions[k]; ok {
			panic("duplicate key:" + k + " in actions map")
		}
		actions[k] = v
	}
}
