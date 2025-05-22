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
			PlanCrOSBase,
			PlanDolos,
			PlanServo,
			PlanBluetoothPeer,
			PlanWifiRouter,
			PlanCrOS,
			PlanChameleon,
			PlanHMR,
			PlanAMT,
			PlanPASIT,
			PlanClosing,
		},
		Plans: map[string]*Plan{
			PlanServo:         setAllowFail(servoRepairPlan(), true),
			PlanCrOSBase:      setAllowFail(crosBasePlan(basePlanTypeRepair), false),
			PlanCrOS:          setAllowFail(crosRepairPlan(), false),
			PlanChameleon:     setAllowFail(chameleonPlan(), true),
			PlanBluetoothPeer: setAllowFail(btpeerRepairPlan(), true),
			PlanWifiRouter:    setAllowFail(wifiRouterRepairPlan(), true),
			PlanHMR:           setAllowFail(hmrRepairPlan(), true),
			PlanDolos:         setAllowFail(dolosRepairPlan(), true),
			PlanAMT:           setAllowFail(amtRepairPlan(), true),
			PlanPASIT:         setAllowFail(pasitRepairPlan(), true),
			PlanClosing:       setAllowFail(crosClosePlan(), true),
		}}
}

// CrosVerifyConfig provides config for verify testbed in the lab.
func CrosVerifyConfig() *Configuration {
	plans := CrosRepairConfig().GetPlans()
	// remove closing plan as it will be added by default.
	delete(plans, PlanClosing)
	return &Configuration{
		PlanNames: []string{
			PlanCrOSBase,
			PlanCrOS,
		},
		Plans: plans,
	}
}

// CrosRepairWithDeepRepairConfig provides config for combination of deep repair + normal repair.
func CrosRepairWithDeepRepairConfig() *Configuration {
	return &Configuration{
		PlanNames: []string{
			PlanCrOSBase,
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
			PlanPASIT,
			PlanClosing,
		},
		Plans: map[string]*Plan{
			PlanServoDeepRepair: setAllowFail(deepRepairServoPlan(), true),
			// We allow CrOSDeepRepair to fail(so the task continue) as some of actions in it may result to a later normal repair success.
			PlanCrOSDeepRepair: setAllowFail(deepRepairCrosPlan(), true),
			PlanServo:          setAllowFail(servoRepairPlan(), true),
			PlanCrOSBase:       setAllowFail(crosBasePlan(basePlanTypeRepair), false),
			PlanCrOS:           setAllowFail(crosRepairPlan(), false),
			PlanChameleon:      setAllowFail(chameleonPlan(), true),
			PlanBluetoothPeer:  setAllowFail(btpeerRepairPlan(), true),
			PlanWifiRouter:     setAllowFail(wifiRouterRepairPlan(), true),
			PlanHMR:            setAllowFail(hmrRepairPlan(), true),
			PlanDolos:          setAllowFail(dolosRepairPlan(), true),
			PlanAMT:            setAllowFail(amtRepairPlan(), true),
			PlanPASIT:          setAllowFail(pasitRepairPlan(), true),
			PlanClosing:        setAllowFail(crosClosePlan(), true),
		}}
}

// MHRepairConfig provides config for repair AndroidOS setup.
func MHRepairConfig() *Configuration {
	return &Configuration{
		PlanNames: []string{
			PlanCrOSBase,
			PlanDolos,
			PlanServo,
			PlanBluetoothPeer,
			PlanWifiRouter,
			PlanCrOS,
			PlanChameleon,
			PlanHMR,
			PlanAMT,
			PlanPASIT,
			PlanClosing,
		},
		Plans: map[string]*Plan{
			PlanServo:         setAllowFail(servoRepairPlan(), true),
			PlanCrOSBase:      setAllowFail(crosBasePlan(basePlanTypeRepair), false),
			PlanCrOS:          setAllowFail(mhRepairPlan(), false),
			PlanChameleon:     setAllowFail(chameleonPlan(), true),
			PlanBluetoothPeer: setAllowFail(btpeerRepairPlan(), true),
			PlanWifiRouter:    setAllowFail(wifiRouterRepairPlan(), true),
			PlanHMR:           setAllowFail(hmrRepairPlan(), true),
			PlanDolos:         setAllowFail(dolosRepairPlan(), true),
			PlanAMT:           setAllowFail(amtRepairPlan(), true),
			PlanPASIT:         setAllowFail(pasitRepairPlan(), true),
			PlanClosing:       setAllowFail(crosClosePlan(), true),
		},
	}
}

// MHVerifyConfig provides config for verify AndroidOS setup.
func MHVerifyConfig() *Configuration {
	plans := MHRepairConfig().GetPlans()
	// remove closing plan as it will be added by default.
	delete(plans, PlanClosing)
	return &Configuration{
		PlanNames: []string{
			PlanCrOSBase,
			PlanCrOS,
		},
		Plans: plans,
	}
}

// MHRepairWithDeepRepairConfig provides config for combination of deep repair + normal repair.
func MHRepairWithDeepRepairConfig() *Configuration {
	return &Configuration{
		PlanNames: []string{
			PlanCrOSBase,
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
			PlanPASIT,
			PlanClosing,
		},
		Plans: map[string]*Plan{
			PlanServoDeepRepair: setAllowFail(deepRepairServoPlan(), true),
			// We allow DeepRepair to fail(so the task continue) as some of actions in it may result to a later normal repair success.
			PlanCrOSDeepRepair: setAllowFail(deepMHRepairCrosPlan(), true),
			PlanServo:          setAllowFail(servoRepairPlan(), true),
			PlanCrOSBase:       setAllowFail(crosBasePlan(basePlanTypeRepair), false),
			PlanCrOS:           setAllowFail(mhRepairPlan(), false),
			PlanChameleon:      setAllowFail(chameleonPlan(), true),
			PlanBluetoothPeer:  setAllowFail(btpeerRepairPlan(), true),
			PlanWifiRouter:     setAllowFail(wifiRouterRepairPlan(), true),
			PlanHMR:            setAllowFail(hmrRepairPlan(), true),
			PlanDolos:          setAllowFail(dolosRepairPlan(), true),
			PlanAMT:            setAllowFail(amtRepairPlan(), true),
			PlanPASIT:          setAllowFail(pasitRepairPlan(), true),
			PlanClosing:        setAllowFail(crosClosePlan(), true),
		}}
}

// CrosDeployConfig provides config for deploy cros setup in the lab.
func CrosDeployConfig() *Configuration {
	return &Configuration{
		PlanNames: []string{
			PlanCrOSBase,
			PlanServoFwUpdate,
			PlanDolos,
			PlanServo,
			PlanCrOSDeploy,
			PlanCrOS,
			PlanChameleon,
			PlanBluetoothPeer,
			PlanWifiRouter,
			PlanHMR,
			PlanAMT,
			PlanPASIT,
			PlanClosing,
		},
		Plans: map[string]*Plan{
			PlanServo:         setAllowFail(servoRepairPlan(), false),
			PlanServoFwUpdate: setAllowFail(servoPreDeployPlan(), false),
			PlanCrOSDeploy:    setAllowFail(crosDeployPlan(), false),
			PlanCrOSBase:      setAllowFail(crosBasePlan(basePlanTypeDeploy), false),
			PlanCrOS:          setAllowFail(crosRepairPlan(), false),
			PlanChameleon:     setAllowFail(chameleonPlan(), true),
			PlanBluetoothPeer: setAllowFail(btpeerRepairPlan(), true),
			PlanWifiRouter:    setAllowFail(wifiRouterRepairPlan(), true),
			PlanHMR:           setAllowFail(hmrRepairPlan(), true),
			PlanDolos:         setAllowFail(dolosRepairPlan(), true),
			PlanAMT:           setAllowFail(amtRepairPlan(), true),
			PlanPASIT:         setAllowFail(pasitRepairPlan(), true),
			PlanClosing:       setAllowFail(crosClosePlan(), true),
		},
	}
}

// MHDeployConfig provides config for deploy cros setup in the MH.
func MHDeployConfig() *Configuration {
	return &Configuration{
		PlanNames: []string{
			PlanCrOSBase,
			PlanServoFwUpdate,
			PlanDolos,
			PlanServo,
			PlanCrOSDeploy,
			PlanCrOS,
			PlanChameleon,
			PlanBluetoothPeer,
			PlanWifiRouter,
			PlanHMR,
			PlanAMT,
			PlanPASIT,
			PlanClosing,
		},
		Plans: map[string]*Plan{
			PlanServo:         setAllowFail(servoRepairPlan(), false),
			PlanServoFwUpdate: setAllowFail(servoPreDeployPlan(), false),
			PlanCrOSDeploy:    setAllowFail(mhDeployPlan(), false),
			PlanCrOSBase:      setAllowFail(crosBasePlan(basePlanTypeDeploy), false),
			PlanCrOS:          setAllowFail(mhRepairPlan(), false),
			PlanChameleon:     setAllowFail(chameleonPlan(), true),
			PlanBluetoothPeer: setAllowFail(btpeerRepairPlan(), true),
			PlanWifiRouter:    setAllowFail(wifiRouterRepairPlan(), true),
			PlanHMR:           setAllowFail(hmrRepairPlan(), true),
			PlanDolos:         setAllowFail(dolosRepairPlan(), true),
			PlanAMT:           setAllowFail(amtRepairPlan(), true),
			PlanPASIT:         setAllowFail(pasitRepairPlan(), true),
			PlanClosing:       setAllowFail(crosClosePlan(), true),
		},
	}
}

// crosClosePlan provides plan to close cros repair/deploy tasks.
func crosClosePlan() *Plan {
	return &Plan{
		CriticalActions: []string{
			"Update AMT state",
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
