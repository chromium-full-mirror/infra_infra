// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"google.golang.org/protobuf/types/known/durationpb"
)

func crosRepairClosingActions() map[string]*Action {
	return map[string]*Action{
		"Is servo_state:working": {
			Docs: []string{
				"check the servo's state is ServoStateWorking.",
			},
			ExecName:      "servo_match_state",
			ExecExtraArgs: []string{"state:WORKING"},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Servo-host known": {
			ExecName:      "dut_servo_host_present",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"No Servo-host": {
			Conditions: []string{
				"Servo-host known",
			},
			ExecName:      "sample_fail",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Remove request to reboot if servo is good": {
			Conditions: []string{
				"Servo-host known",
				"Is servo_state:working",
			},
			ExecName:               "cros_remove_reboot_request",
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_UPLOAD_ON_ERROR},
			AllowFailAfterRecovery: true,
		},
		"Close Servo-host": {
			Docs: []string{
				"Stop the servod daemon.",
				"Collect all logs and files from the host.",
			},
			Conditions: []string{
				"Servo-host known",
				"Servo-host is sshable",
			},
			Dependencies: []string{
				"Copy messages from servo-host",
				"Copy eventlog.txt from servo-host",
				"Collect dmesg logs from Servo-host",
				"Collect servod logs",
				"Remove in-use flag on servo-host",
				"Remove request to reboot if servo is good",
				"Turn off servo usbkey power",
				"Save UART capture",
			},
			ExecName:               "servo_host_servod_stop",
			RunControl:             RunControl_ALWAYS_RUN,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			AllowFailAfterRecovery: true,
		},
		"Collect dmesg logs from Servo-host": {
			Docs: []string{
				"Collect the entire output of dmesg",
			},
			ExecName: "cros_dmesg",
			ExecExtraArgs: []string{
				"device_type:servo_host",
				"use_host_dir:true",
			},
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			AllowFailAfterRecovery: true,
		},
		"Remove in-use flag on servo-host": {
			Conditions: []string{
				"Servo-host known",
			},
			ExecName:               "cros_remove_servo_in_use",
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			AllowFailAfterRecovery: true,
		},
		"Collect servod logs": {
			Docs: []string{
				"Try to collect all servod logs since latest start time.",
			},
			Conditions: []string{
				"Servo-host known",
			},
			ExecName:               "cros_collect_servod_logs",
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			AllowFailAfterRecovery: true,
		},
		"Copy messages from servo-host": {
			Docs: []string{
				"Try to collect /var/log/messages from servo-host.",
			},
			Conditions: []string{
				"Servo-host known",
			},
			ExecName: "cros_copy_to_logs",
			ExecExtraArgs: []string{
				"src_host_type:servo_host",
				"src_path:/var/log/messages",
				"src_type:file",
				"use_host_dir:true",
			},
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			AllowFailAfterRecovery: true,
		},
		"Copy eventlog.txt from servo-host": {
			Docs: []string{
				"Try to collect /var/log/eventlog.txt from servo-host.",
			},
			Conditions: []string{
				"Servo-host known",
			},
			ExecName: "cros_copy_to_logs",
			ExecExtraArgs: []string{
				"src_host_type:servo_host",
				"src_path:/var/log/eventlog.txt",
				"src_type:file",
				"use_host_dir:true",
			},
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			AllowFailAfterRecovery: true,
		},
		"Is not in cellular pool": {
			Docs: []string{
				"Verify that DUT is not in a pool that runs cellular tests",
			},
			ExecName: "dut_not_in_pool_regex",
			ExecExtraArgs: []string{
				"regex:(?i)^cellular|dut_pool_quota",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Update cellular modem state for non-cellular pools": {
			Docs: []string{
				"Set cellular modem state for DUTs in non-cellular pools.",
			},
			Conditions: []string{
				"Is not in cellular pool",
				"has_cellular_info",
			},
			ExecName: "set_cellular_modem_state",
			ExecExtraArgs: []string{
				"state:hardware_not_detected",
			},
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_UPLOAD_ON_ERROR},
		},
		"Update AMT state": {
			Docs: []string{
				"Update the AMT state for DUTs without AMT.",
			},
			Conditions: []string{
				"amt_manager_not_present",
			},
			ExecName: "amt_manager_set_state",
			ExecExtraArgs: []string{
				"state:NOT_APPLICABLE",
			},
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Update peripheral wifi state": {
			Docs: []string{
				"Update peripheral wifi state based on wifi router states",
			},
			ExecName:               "update_peripheral_wifi_state",
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_UPLOAD_ON_ERROR},
		},
		"Update wifi router features": {
			Docs: []string{
				"Update wifi router features based on the features of all wifi routers",
			},
			ExecName:               "update_wifi_router_features",
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_UPLOAD_ON_ERROR},
		},
		"Update chameleon state for chameleonless dut": {
			Docs: []string{
				"Update chameleon state to not applicable for chameleonless dut",
			},
			Conditions: []string{
				"chameleon_not_present",
			},
			ExecName:               "chameleon_state_not_applicable",
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_UPLOAD_ON_ERROR},
		},
		"Failure count above threshold": {
			Docs: []string{
				"Check if the number of recovery task failures ",
				"on a servo-attached DUT is greater than a threshold",
			},
			Conditions: []string{
				"Servo-host known",
			},
			ExecName: "metrics_check_task_failures",
			ExecExtraArgs: []string{
				"task_name:recovery",
				"repair_failed_count:6",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_UPLOAD_ON_ERROR},
		},
		"Failure count above threshold (Servo-less)": {
			Docs: []string{
				"Check if the number of recovery task failures ",
				"on a non-servo-attached DUT is greater than a threshold",
			},
			Conditions: []string{
				"No Servo-host",
			},
			ExecName: "metrics_check_task_failures",
			ExecExtraArgs: []string{
				"task_name:recovery",
				"repair_failed_count:3",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_UPLOAD_ON_ERROR},
		},
		"Update DUT state for failures more than threshold": {
			Docs: []string{
				"Set the DUT state to the value passed in the ",
				"extra args.",
			},
			Conditions: []string{
				"DUT state is repair_failed",
				"Failure count above threshold",
				"Failure count above threshold (Servo-less)",
				"Set state: needs_manual_repair",
			},
			ExecName: "dut_set_state_reason",
			ExecExtraArgs: []string{
				"allow_override:false",
				"reason:REPAIR_RETRY_REACHED_THRESHOLD",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Failure count are not above threshold": {
			Docs: []string{
				"Check if the number of times the recovery task ",
				"has failed is not greater than a threshold value",
			},
			Dependencies: []string{
				"Failure count above threshold",
			},
			ExecName:      "sample_fail",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_UPLOAD_ON_ERROR},
		},
		"DUT state is not ready": {
			Docs: []string{
				`Check if the DUT state is anything other than "ready".`,
			},
			ExecName: "dut_state_match",
			ExecExtraArgs: []string{
				"invert:true",
				"state:ready",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"DUT state is not needs_replacement": {
			Docs: []string{
				`Check if the DUT state is anything other than "needs_replacement".`,
			},
			ExecName: "dut_state_match",
			ExecExtraArgs: []string{
				"invert:true",
				"state:needs_replacement",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"DUT state is repair_failed": {
			Docs: []string{
				"Check if the DUT's state is in repair_failed state, if not then fail.",
			},
			ExecName: "dut_state_match",
			ExecExtraArgs: []string{
				"state:repair_failed",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Servo state demands manual repair": {
			Docs: []string{
				"Check whether the state of servo mandates a ",
				"manual repair on the DUT.",
			},
			ExecName: "dut_servo_state_required_manual_attention",
			ExecExtraArgs: []string{
				"servo_states:NEED_REPLACEMENT,DUT_NOT_CONNECTED",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Update DUT state based on servo state": {
			Docs: []string{
				"Set the DUT state based on the state of servo. ",
				"This applies only if the count of failures is ",
				"not above a threshold amount",
			},
			Conditions: []string{
				"DUT state is repair_failed",
				"Failure count are not above threshold",
				"Servo-host known",
				"Servo state demands manual repair",
				"Set state: needs_manual_repair",
			},
			ExecName: "dut_set_state_reason",
			ExecExtraArgs: []string{
				"allow_override:false",
				"reason:CRITICAL_SERVO_ISSUE",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Turn off servo usbkey power": {
			Docs: []string{
				"Ensure that servo usbkey power is in off state.",
			},
			Conditions: []string{
				"Servo-host known",
			},
			ExecName: "servo_set",
			ExecExtraArgs: []string{
				"command:image_usbkey_pwr",
				"string_value:off",
			},
			RunControl:             RunControl_ALWAYS_RUN,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			AllowFailAfterRecovery: true,
		},
		"Stop servod": {
			Docs: []string{
				"Stop the servod daemon.",
				"Allowed to fail as can be run when servod is not running.",
			},
			Dependencies: []string{
				"Save UART capture",
			},
			ExecName:               "servo_host_servod_stop",
			RunControl:             RunControl_ALWAYS_RUN,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			AllowFailAfterRecovery: true,
		},
		"Servo-host is sshable": {
			Docs: []string{
				"Stop the servod daemon.",
				"Allowed to fail as can be run when servod is not running.",
			},
			ExecName: "cros_access",
			ExecExtraArgs: []string{
				"device_type:servo",
			},
			ExecTimeout: &durationpb.Duration{
				Seconds: 15,
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			RunControl:    RunControl_RUN_ONCE,
		},
		"Save UART capture": {
			Dependencies: []string{
				"Stop UART capture",
			},
			ExecName:               "servod_save_uart_capture",
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Stop UART capture": {
			ExecName:               "servod_stop_uart_capture",
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Set state: needs_manual_repair": {
			Docs: []string{
				"Set DUT state as needs_manual_repair.",
			},
			ExecName: "dut_set_state",
			ExecExtraArgs: []string{
				"state:needs_manual_repair",
			},
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
	}
}
