// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"fmt"

	"google.golang.org/protobuf/types/known/durationpb"
)

// DownloadImageToServoUSBDrive creates configuration to download image to USB-drive connected to the servo.
func DownloadImageToServoUSBDrive(gsImagePath, imageName string) *Configuration {
	rc := CrosRepairConfig()
	rc.PlanNames = []string{
		PlanServo,
		PlanCrOS,
	}
	if rc.Plans[PlanClosing] == nil {
		panic("Closing plan is expected but not found!")
	}
	rc.Plans[PlanClosing].CriticalActions = []string{
		"Close Servo-host",
	}
	var newArgs []string
	if gsImagePath != "" {
		newArgs = append(newArgs, fmt.Sprintf("os_image_path:%s", gsImagePath))
	} else if imageName != "" {
		newArgs = append(newArgs, fmt.Sprintf("os_name:%s", imageName))
	}
	const targetAction = "Call servod to download image to USB-key"
	rc.Plans[PlanCrOS].CriticalActions = []string{targetAction}
	rc.Plans[PlanCrOS].GetActions()[targetAction].ExecExtraArgs = newArgs
	return rc
}

// ReserveDutConfig creates configuration to reserve a dut
func ReserveDutConfig() *Configuration {
	return &Configuration{
		PlanNames: []string{
			PlanCrOS,
		},
		Plans: map[string]*Plan{
			PlanCrOS: {
				CriticalActions: []string{
					"Reserve DUT",
				},
				Actions: map[string]*Action{
					"Reserve DUT": {
						Dependencies: []string{
							"Set DUT reason",
						},
						ExecName: "dut_state_reserved",
					},
					"Set DUT reason": {
						ExecName: "dut_state_reason_set_from_tags",
						ExecExtraArgs: []string{
							"tag_name:comment",
						},
						AllowFailAfterRecovery: true,
					},
				},
			},
		},
	}
}

// RestoreHWIDFromInventoryConfig reads the configuration from the inventory.
func RestoreHWIDFromInventoryConfig() *Configuration {
	return &Configuration{
		PlanNames: []string{
			PlanCrOS,
		},
		Plans: map[string]*Plan{
			PlanCrOS: {
				CriticalActions: []string{
					"dut_has_hwid",
					"cros_ssh",
					"Set HWID of the DUT from inventory",
					"Simple reboot",
					"Sleep 1s",
					"Wait to be accessable",
					"cros_match_hwid_to_inventory",
				},
				Actions: crosRepairActions(),
			},
		},
	}
}

// RecoverCBIFromInventoryConfig restores backup CBI contents from UFS
func RecoverCBIFromInventoryConfig() *Configuration {
	return &Configuration{
		PlanNames: []string{
			PlanCrOS,
		},
		Plans: map[string]*Plan{
			PlanCrOS: {
				CriticalActions: []string{
					"Recover and Validate CBI",
				},
				Actions: crosRepairActions(),
			},
		},
	}
}

// FixBatteryCutOffConfig creates a custom configuration to recover by battery cut-off
func FixBatteryCutOffConfig() *Configuration {
	customFixPlan := "cros_battery_cut"
	return &Configuration{
		PlanNames: []string{
			PlanServo,
			customFixPlan,
			PlanCrOS,
			PlanChameleon,
			PlanBluetoothPeer,
			PlanWifiRouter,
			PlanHMR,
			PlanAMT,
			PlanClosing,
		},
		Plans: map[string]*Plan{
			// Not allowed to fail as servo is critical for the fix plan.
			PlanServo: servoRepairPlan(),
			// If fix didn't work then no need to run repair plans.
			customFixPlan: {
				CriticalActions: []string{
					"Is servod running",
					"Battery cut-off by servo EC console",
					"Sleep 10 seconds",
					"servo_fake_disconnect_dut",
					"Sleep 60 seconds",
				},
				Actions: crosRepairActions(),
			},
			PlanCrOS:          setAllowFail(crosRepairPlan(), false),
			PlanChameleon:     setAllowFail(chameleonPlan(), true),
			PlanBluetoothPeer: setAllowFail(btpeerRepairPlan(), true),
			PlanWifiRouter:    setAllowFail(wifiRouterRepairPlan(), true),
			PlanHMR:           setAllowFail(hmrRepairPlan(), true),
			PlanAMT:           setAllowFail(amtRepairPlan(), true),
			PlanClosing:       setAllowFail(crosClosePlan(), true),
		},
	}
}

// EnableSerialConsoleConfig creates a custom configuration to flash serial firmware to DUT.
func EnableSerialConsoleConfig() *Configuration {
	return &Configuration{
		PlanNames: []string{
			PlanServo,
			PlanCrOS,
			PlanClosing,
		},
		Plans: map[string]*Plan{
			// Not allowed to fail as servo is critical for the fix plan.
			PlanServo: setAllowFail(servoRepairPlan(), false),
			PlanCrOS: {
				CriticalActions: []string{
					"Is servod running",
					"Set GBB flags to enable dev mode and boot from usb by servo",
					"Flash AP (FW) with enabled serial console",
					"Cold reset DUT by servo",
					"Sleep 10 seconds",
				},
				Actions: crosRepairActions(),
			},
			PlanClosing: setAllowFail(crosClosePlan(), true),
		},
	}
}

// SetFwTargets creates a custom configuration to update fw-targets.
func SetFwTargets(ecTarget, apTarget string) *Configuration {
	return &Configuration{
		PlanNames: []string{
			PlanCrOS,
		},
		Plans: map[string]*Plan{
			PlanCrOS: {
				CriticalActions: []string{
					"Set Fw-targets",
				},
				Actions: map[string]*Action{
					"Set Fw-targets": {
						ExecName: "cros_set_fw_targets",
						ExecExtraArgs: []string{
							fmt.Sprintf("ec_target:%s", ecTarget),
							fmt.Sprintf("ap_target:%s", apTarget),
						},
					},
				},
			},
		},
	}
}

func LabstationRpmPowerCycleConfig(timeToWait int) *Configuration {
	if timeToWait < 10 {
		// Minimum time needed between switch RPM action.
		timeToWait = 10
	}
	return &Configuration{
		PlanNames: []string{
			PlanCrOS,
		},
		Plans: map[string]*Plan{
			PlanCrOS: {
				CriticalActions: []string{
					"Device is accessable",
					"Power off by RPM",
					"Wait",
					"Power on by RPM",
					"Wait to be SSHable",
					"Remove reboot requests",
				},
				Actions: map[string]*Action{
					"Device is accessable": {
						Docs: []string{
							"This verifier checks whether the host is accessible over ssh.",
						},
						ExecName:               "cros_ssh",
						ExecTimeout:            &durationpb.Duration{Seconds: 30},
						RunControl:             RunControl_ALWAYS_RUN,
						AllowFailAfterRecovery: true,
						MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
					},
					"Power off by RPM": {
						Docs: []string{
							"Power off the labstation via RPM.",
						},
						Conditions: []string{
							"rpm_action_enabled",
							"has_rpm_info",
						},
						ExecName: "rpm_power_off",
						// 60 seconds timeout via HTTP based call and 60 seconds fallback to RPM service.
						ExecTimeout:            &durationpb.Duration{Seconds: 120},
						RunControl:             RunControl_ALWAYS_RUN,
						AllowFailAfterRecovery: true,
						MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
					},
					"Power on by RPM": {
						Docs: []string{
							"Power on the labstation via RPM.",
						},
						Conditions: []string{
							"rpm_action_enabled",
							"has_rpm_info",
						},
						ExecName: "rpm_power_on",
						// 60 seconds timeout via HTTP based call and 60 seconds fallback to RPM service.
						ExecTimeout:   &durationpb.Duration{Seconds: 120},
						RunControl:    RunControl_ALWAYS_RUN,
						MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
					},
					"Wait": {
						ExecName: "sample_sleep",
						ExecExtraArgs: []string{
							fmt.Sprintf("sleep:%d", timeToWait),
						},
						ExecTimeout:            &durationpb.Duration{Seconds: int64(timeToWait + 10)},
						RunControl:             RunControl_ALWAYS_RUN,
						AllowFailAfterRecovery: true,
						MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
					},
					"Wait to be SSHable": {
						Docs: []string{
							"Try to wait device to be sshable during after the device being rebooted.",
						},
						// Labstation may take some time to fully up(e.g. network service ready) after an update.
						// So giving it 10 minutes in here to allow more buffer.
						ExecTimeout:   &durationpb.Duration{Seconds: 600},
						ExecName:      "cros_ssh",
						RunControl:    RunControl_ALWAYS_RUN,
						MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
					},
					"Remove reboot requests": {
						Docs: []string{
							"Remove all requests for reboot on the host.",
							"The action has to be called after reboot of the device.",
						},
						ExecName:               "cros_remove_all_reboot_request",
						AllowFailAfterRecovery: true,
						MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
					},
				},
			},
		},
	}
}
