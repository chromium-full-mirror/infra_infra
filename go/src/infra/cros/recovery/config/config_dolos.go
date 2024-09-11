// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"google.golang.org/protobuf/types/known/durationpb"
)

func dolosRepairPlan() *Plan {
	return &Plan{
		CriticalActions: []string{
			"Set dolos state:DOLOS_NOT_PRESENT",
			"Is Dolos present",
			"Device is sshable",
			"Uartname is known",
			"Check firmware version is correct.",
			"Update state",
			"Dolos does not needs reboot",
		},
		Actions: map[string]*Action{
			"Device is pingable": {
				ExecName:    "cros_ping",
				ExecTimeout: &durationpb.Duration{Seconds: 15},
			},
			"Device is sshable": {
				Dependencies: []string{
					"Set dolos state:NO_SSH",
					"Device is pingable",
				},
				ExecName:    "cros_ssh",
				ExecTimeout: &durationpb.Duration{Seconds: 30},
			},
			"Is Dolos present": {
				ExecName: "dolos_is_enabled",
			},
			"Uartname is known": {
				Dependencies: []string{
					"Is Dolos present",
				},
				ExecName: "dolos_is_uartname_cached",
				RecoveryActions: []string{
					"Discover uartname and save to cache",
				},
				AllowFailAfterRecovery: true,
			},
			"Discover uartname and save to cache": {
				Dependencies: []string{
					"Device is sshable",
					"Is Dolos present",
				},
				ExecName: "dolos_update_uartname_cache",
			},
			"Update state": {
				ExecName: "dolos_determine_and_set_dolos_state",
			},
			"Set dolos state:NO_SSH": {
				ExecName: "dolos_set_dolos_state",
				ExecExtraArgs: []string{
					"state:NO_SSH",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set dolos state:DOLOS_NOT_PRESENT": {
				ExecName: "dolos_set_dolos_state",
				ExecExtraArgs: []string{
					"state:DOLOS_NOT_PRESENT",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set dolos RPM OFF": {
				ExecName: "device_rpm_power_off",
				ExecExtraArgs: []string{
					"device_type:dolos",
				},
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Set dolos RPM ON": {
				ExecName: "device_rpm_power_on",
				ExecExtraArgs: []string{
					"device_type:dolos",
				},
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Set dut RPM OFF": {
				ExecName: "device_rpm_power_off",
				ExecExtraArgs: []string{
					"device_type:dut",
				},
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Set dut RPM ON": {
				ExecName: "device_rpm_power_on",
				ExecExtraArgs: []string{
					"device_type:dut",
				},
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Dolos does not needs reboot": {
				ExecName: "dolos_does_not_need_reboot",
				RecoveryActions: []string{
					"Reboot dolos",
				},
			},
			"Sleep 20s": {
				ExecName: "sample_sleep",
				ExecExtraArgs: []string{
					"sleep:20",
				},
				RunControl:             RunControl_ALWAYS_RUN,
				AllowFailAfterRecovery: true,
				MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Sleep 5s": {
				ExecName: "sample_sleep",
				ExecExtraArgs: []string{
					"sleep:5",
				},
				RunControl:             RunControl_ALWAYS_RUN,
				AllowFailAfterRecovery: true,
				MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Power cycle root servo": {
				ExecName:               "servo_power_cycle_root_servo",
				RunControl:             RunControl_ALWAYS_RUN,
				AllowFailAfterRecovery: true,
			},
			"Reboot dolos": {
				Docs: []string{
					"Dolos has two independent power sources, the wall power and the USB hub",
					"power.  Both have to be disconnected for a reboot.",
					"First switch off the RPM, then power cycle the USB hub port, then switch",
					"on the RPM to accomplish this.",
					"20s reboot time allows dolos to boot ( 4-5 seconds ) and the USB devices",
					"to be re-enumerated in the labstation kernel.",
				},
				Dependencies: []string{
					"Set dut RPM OFF",
					"Set dolos RPM OFF",
					"Sleep 5s",
					"Power cycle root servo",
					"Set dolos RPM ON",
					"Set dut RPM ON",
					"Sleep 20s",
				},
				ExecName: "sample_pass",
			},
			"Check firmware version is correct.": {
				ExecName: "dolos_check_firmware_up_to_date",
				RecoveryActions: []string{
					"dolos_update_firmware",
				},
			},
		},
	}
}
