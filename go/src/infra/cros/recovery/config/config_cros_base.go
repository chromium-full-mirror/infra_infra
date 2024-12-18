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
		"Start ADB-base",
		"Start Servo-Nexus",
	)
	return &Plan{
		CriticalActions: ca,
		Actions:         crosBaseActions(),
	}
}

func crosBaseActions() map[string]*Action {
	actions := map[string]*Action{}
	addCrosCftContainers(actions)
	addEnvActions(actions)
	addStateActions(actions)
	return actions
}
