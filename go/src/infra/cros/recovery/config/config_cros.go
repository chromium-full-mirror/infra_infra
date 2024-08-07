// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

// setAllowFail updates allowFail property and return plan.
func setAllowFail(p *Plan, allowFail bool) *Plan {
	p.AllowFail = allowFail
	return p
}

// CrosRepairConfig provides config for repair cros setup in the lab task.
func CrosRepairConfig() *Configuration {
	return &Configuration{
		PlanNames: []string{
			PlanDolos,
			PlanServo,
			PlanBluetoothPeer,
			PlanWifiRouter,
			PlanCrOS,
			PlanChameleon,
			PlanHMR,
			PlanAMT,
			PlanClosing,
		},
		Plans: map[string]*Plan{
			PlanServo:         setAllowFail(servoRepairPlan(), true),
			PlanCrOS:          setAllowFail(crosRepairPlan(), false),
			PlanChameleon:     setAllowFail(chameleonPlan(), true),
			PlanBluetoothPeer: setAllowFail(btpeerRepairPlan(), true),
			PlanWifiRouter:    setAllowFail(wifiRouterRepairPlan(), true),
			PlanHMR:           setAllowFail(hmrRepairPlan(), true),
			PlanDolos:         setAllowFail(dolosRepairPlan(), true),
			PlanAMT:           setAllowFail(amtRepairPlan(), true),
			PlanClosing:       setAllowFail(crosClosePlan(), true),
		}}
}

// CrosRepairWithDeepRepairConfig provides config for combination of deep repair + normal repair.
func CrosRepairWithDeepRepairConfig() *Configuration {
	return &Configuration{
		PlanNames: []string{
			PlanDolos,
			PlanServoDeepRepair,
			PlanCrOSDeepRepair,
			PlanServo,
			PlanBluetoothPeer,
			PlanWifiRouter,
			PlanCrOS,
			PlanChameleon,
			PlanHMR,
			PlanAMT,
			PlanClosing,
		},
		Plans: map[string]*Plan{
			PlanServoDeepRepair: setAllowFail(deepRepairServoPlan(), true),
			// We allow CrOSDeepRepair to fail(so the task continue) as some of actions in it may result to a later normal repair success.
			PlanCrOSDeepRepair: setAllowFail(deepRepairCrosPlan(), true),
			PlanServo:          setAllowFail(servoRepairPlan(), true),
			PlanCrOS:           setAllowFail(crosRepairPlan(), false),
			PlanChameleon:      setAllowFail(chameleonPlan(), true),
			PlanBluetoothPeer:  setAllowFail(btpeerRepairPlan(), true),
			PlanWifiRouter:     setAllowFail(wifiRouterRepairPlan(), true),
			PlanHMR:            setAllowFail(hmrRepairPlan(), true),
			PlanDolos:          setAllowFail(dolosRepairPlan(), true),
			PlanAMT:            setAllowFail(amtRepairPlan(), true),
			PlanClosing:        setAllowFail(crosClosePlan(), true),
		}}
}

// CrosDeployConfig provides config for deploy cros setup in the lab task.
func CrosDeployConfig() *Configuration {
	return &Configuration{
		PlanNames: []string{
			PlanDolos,
			PlanServo,
			PlanCrOS,
			PlanChameleon,
			PlanBluetoothPeer,
			PlanWifiRouter,
			PlanHMR,
			PlanAMT,
			PlanClosing,
		},
		Plans: map[string]*Plan{
			PlanServo:         setAllowFail(servoRepairPlan(), false),
			PlanCrOS:          setAllowFail(crosDeployPlan(), false),
			PlanChameleon:     setAllowFail(chameleonPlan(), true),
			PlanBluetoothPeer: setAllowFail(btpeerRepairPlan(), true),
			PlanWifiRouter:    setAllowFail(wifiRouterRepairPlan(), true),
			PlanHMR:           setAllowFail(hmrRepairPlan(), true),
			PlanDolos:         setAllowFail(dolosRepairPlan(), true),
			PlanAMT:           setAllowFail(amtRepairPlan(), true),
			PlanClosing:       setAllowFail(crosClosePlan(), true),
		},
	}
}

// crosClosePlan provides plan to close cros repair/deploy tasks.
func crosClosePlan() *Plan {
	return &Plan{
		CriticalActions: []string{
			"Update peripheral wifi state",
			"Update wifi router features",
			"Update chameleon state for chameleonless dut",
			"Update DUT state based on servo state",
			"Update DUT state for failures more than threshold",
			"Update cellular modem state for non-cellular pools",
			"Close Servo-host",
		},
		Actions: crosRepairClosingActions(),
	}
}
