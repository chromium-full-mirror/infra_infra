// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

// List of known plans.
//
// Note that plans are named by the type of device that they refer to.
// There is exactly one plan per type of device.
const (
	PlanAndroid         = "android"
	PlanCrOS            = "cros"
	PlanCrOSAudit       = "cros_audit"
	PlanCrOSDeepRepair  = "cros_deep_repair"
	PlanServo           = "servo"
	PlanServoDeepRepair = "servo_deep_repair"
	PlanServoFwUpdate   = "servo_fw_update"
	PlanChameleon       = "chameleon"
	PlanBluetoothPeer   = "bluetooth_peer"
	PlanWifiRouter      = "wifi_router"
	PlanHMR             = "human_motion_robot"
	PlanDolos           = "dolos"
	PlanAMT             = "amt_manager"
	// That is final plan which will run always if present in configuration.
	// The goal is execution final step to clean up stages if something left
	// over in the devices.
	PlanClosing = "close"
)
