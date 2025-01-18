// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

// CrosAuditStorageConfig audits the internal storage for a ChromeOS DUT.
func CrosAuditStorageConfig() *Configuration {
	return &Configuration{
		PlanNames: []string{
			PlanCrOSBase,
			PlanCrOSAudit,
		},
		Plans: map[string]*Plan{
			PlanCrOSBase: setAllowFail(crosBasePlan(basePlanTypeAudit), false),
			PlanCrOSAudit: {
				CriticalActions: []string{
					"Device is pingable (simple)",
					"Mark DUT as Android if applicable",
					"Audit storage",
				},
				Actions: crosRepairActions(),
			},
		},
	}
}
