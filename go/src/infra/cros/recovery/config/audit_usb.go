// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

// CrosAuditUSBConfig audits the USB drive for a servo associated with a ChromeOS DUT.
func CrosAuditUSBConfig() *Configuration {

	return &Configuration{
		PlanNames: []string{
			PlanCrOSBase,
			// First we prepare servo to work on setup.
			// Make sure that the servo is in a good state and servod is up.
			PlanServo,
			// Core plan to validate access and check USB-drive.
			PlanCrOSAudit,
			PlanClosing,
		},
		Plans: map[string]*Plan{
			PlanServo:    setAllowFail(servoRepairPlan(), false),
			PlanCrOSBase: setAllowFail(crosBasePlan(basePlanTypeAudit), false),
			PlanCrOSAudit: {
				CriticalActions: []string{
					"Device is pingable (simple)",
					"Is Android based",
					"Audit USB-drive from DUT",
				},
				// We use CrOS repair actions as it has all action it requires.
				Actions:   crosRepairActions(),
				AllowFail: false,
			},
			PlanClosing: {
				CriticalActions: []string{
					"Close Servo-host",
					"Stop CFT containers",
				},
				Actions:   crosRepairClosingActions(),
				AllowFail: true,
			},
		},
	}
}
