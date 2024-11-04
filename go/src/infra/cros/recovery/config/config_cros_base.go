// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

type basePlanType string

const (
	basePlanTypeRepair basePlanType = "repair"
	basePlanTypeDeploy basePlanType = "deploy"
	basePlanTypeAudit  basePlanType = "audit"
)

// crosBasePlan creates a plan that must always be pass.
// All actions indicate that something is wrong and there is nothing we can do about it.
func crosBasePlan(pt basePlanType) *Plan {
	var ca []string
	switch pt {
	case basePlanTypeRepair:
		ca = append(ca, "Set state: repair_failed")
	case basePlanTypeDeploy:
		ca = append(ca, "Set state: needs_deploy")
	case basePlanTypeAudit:
		ca = append(ca, "Set state: needs_repair")
	}
	ca = append(ca,
		"DUT has board info",
		"DUT has model info",
		"Start ADB-base",
		"Start Servo-Nexus",
	)
	return &Plan{
		CriticalActions: ca,
		Actions:         crosBaseActions(),
	}
}

func crosBaseActions() map[string]*Action {
	actions := map[string]*Action{
		"DUT has board info": {
			ExecName:      "dut_has_board_name",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"DUT has model info": {
			ExecName:      "dut_has_model_name",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
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
		"Set state: needs_repair": {
			Docs: []string{
				"The action set devices with state means that DUT requires repair.",
			},
			ExecName: "dut_set_state",
			ExecExtraArgs: []string{
				"state:needs_repair",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Testbed has Servo": {
			ExecName: "dut_servo_host_present",
			MetricsConfig: &MetricsConfig{
				UploadPolicy: MetricsConfig_SKIP_ALL,
			},
		},
	}
	addCrosCftContainers(actions)
	return actions
}
