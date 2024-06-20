// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"google.golang.org/protobuf/types/known/durationpb"
)

func servoRepairPlan() *Plan {
	return &Plan{
		CriticalActions: []string{
			"Set state:MISSING_CONFIG",
			"Servo is know in the setup",
			"Servod port specified",
			"Servo serial is specified",
			"Initialize docker container",
			"Device is SSHable",
			"Mark labstation as servod is in-use",
			"Read release info",
			"Power-cycle by smart-hub",
			"Has enough free disk space",
			"Cache latest servod start time",
			"Servo_v4(p1) main present",
			"All servo's fw updated",
			"Save UART capture",
			"Start servod daemon",
			"Start UART capture",
			"Servod is responsive to dut-control",
			"Read servo serial by servod harness",
			"Set cold_reset for c2d2",
			"Verify servo connected to the DUT",
			"Debug header servo present",
			"Cold reset pin is detected",
			"Warm reset pin is detected (servo_micro)",
			"Charger connected",
			"Check if PD is src state",
			"Verify CCD GSC connection detected",
			"Servod detect all children components",
			"Servo topology",
			"Update USB drive info",
			"Initialize DUT part for servo",
			"Verify cr50 console",
			"Cr50 testlab is enabled",
			"Verify EC",
			"Record good servo type",
			"Set state:WORKING",
		},
		Actions: map[string]*Action{
			"Start UART capture": {
				ExecName:               "servod_start_uart_capture",
				AllowFailAfterRecovery: true,
				MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
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
			"Servo is know in the setup": {
				Docs: []string{
					"Verify if setup data has any data related to servo-host which mean servo is present in setup.",
				},
				Dependencies: []string{
					"Set state:WRONG_CONFIG",
				},
				ExecName:   "dut_servo_host_present",
				RunControl: RunControl_RUN_ONCE,
			},
			"Servo serial is specified": {
				Docs: []string{
					"Check if root servo serial is present.",
				},
				Dependencies: []string{
					"Set state:WRONG_CONFIG",
				},
				ExecName:   "dut_servo_has_serial",
				RunControl: RunControl_RUN_ONCE,
			},
			"Device is pingable": {
				Docs: []string{
					"Verify that device is reachable by ping.",
					"Limited to 15 seconds.",
				},
				Conditions: []string{
					"Servod container is not used",
				},
				ExecName: "cros_ping",
				ExecTimeout: &durationpb.Duration{
					Seconds: 15,
				},
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Device is SSHable": {
				Docs: []string{
					"Verify that device is reachable by SSH.",
					"Limited to 15 seconds.",
				},
				Dependencies: []string{
					"Set state:NO_SSH",
				},
				ExecTimeout: &durationpb.Duration{Seconds: 15},
				ExecName:    "cros_ssh",
				RunControl:  RunControl_ALWAYS_RUN,
				RecoveryActions: []string{
					"Wait for labstation to load",
				},
			},
			"Wait for labstation to load": {
				Docs: []string{
					"Sometimes we can try to connect when labstation is the middle of reboot, so we wait.",
					"Labstation is expected to complete the reboot within 2 minutes.",
				},
				ExecTimeout: &durationpb.Duration{Seconds: 120},
				ExecName:    "cros_ssh",
			},
			"Cache latest servod start time": {
				Docs: []string{
					"Cache servod start time based on previous runs.",
					"If we fail all logs will be collected",
				},
				Conditions: []string{
					"Servod container is not used",
				},
				ExecName:               "cros_register_servod_logs_start",
				AllowFailAfterRecovery: true,
			},
			"Start servod daemon": {
				Docs: []string{
					"Start servod daemon on servo-host",
				},
				Dependencies: []string{
					"Device is SSHable",
					"Set state:SERVO_HOST_ISSUE",
				},
				ExecName:    "servo_host_servod_init",
				ExecTimeout: &durationpb.Duration{Seconds: 120},
				RecoveryActions: []string{
					"Reboot servo device",
					"Stop servod and request to use recovery-mode for servod",
					"Stop servod",
					"Reset EC from DUT and stop",
					"Reflash Cr50 fw and stop",
					"Reset GSC from DUT and stop servod",
					"Create request to reboot labstation",
				},
			},
			"Set cold_reset for c2d2": {
				Docs: []string{
					"https://issuetracker.google.com/302370064 Use gsc_ec_reset instead of gsc_reset for c2d2 devices.",
					"This is faft ccd should be open and in factory mode, so gsc_ec_reset should be accessible.",
				},
				Conditions: []string{
					"Servo used c2d2",
				},
				ExecName: "servo_set",
				ExecExtraArgs: []string{
					"command:cold_reset_select",
					"string_value:gsc_ec_reset",
				},
				AllowFailAfterRecovery: true,
			},
			"Stop servod and request to use recovery-mode for servod": {
				Docs: []string{
					"This recovery action to made adjust how we start servod.",
					"Specify to start servod with REC_MODE=1.",
				},
				Dependencies: []string{
					"Specify to use REC_MODE=1 for servo",
					"Stop servod",
				},
				ExecName:   "sample_pass",
				RunControl: RunControl_RUN_ONCE,
			},
			"Specify to use REC_MODE=1 for servo": {
				Docs: []string{
					"Create a file to specify use REC_MODE=1 when start servod.",
				},
				ExecName:   "servo_create_flag_to_use_recovery_mode",
				RunControl: RunControl_RUN_ONCE,
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
				AllowFailAfterRecovery: true,
			},
			"Initialize docker container": {
				Docs: []string{
					"Initiate docker to have access to the host.",
					"Servod is not needed as on this stage we just verify that servo host is good.",
					"If start container with servod and root servo device is not connected it will fail.",
				},
				Dependencies: []string{
					"Set state:NO_SSH",
				},
				Conditions: []string{
					"Uses servod container",
				},
				ExecName: "servo_host_servod_init",
				ExecExtraArgs: []string{
					"no_servod:true",
				},
				ExecTimeout: &durationpb.Duration{Seconds: 360},
				RecoveryActions: []string{
					"Stop servod",
				},
			},
			"Stop servod daemon on servo-host": {
				Docs: []string{
					"Make sure servod daemon is not running on servo-host.",
					"If container then run without daemon.",
					"If daemon is running it will be stopped.",
				},
				ExecName: "servo_host_servod_init",
				ExecExtraArgs: []string{
					"no_servod:true",
				},
				ExecTimeout: &durationpb.Duration{Seconds: 360},
			},
			"Servod port specified": {
				Docs: []string{
					"Verify that servod port is present in servo data.",
				},
				Dependencies: []string{
					"Set state:WRONG_CONFIG",
				},
				ExecName:   "servo_servod_port_present",
				RunControl: RunControl_RUN_ONCE,
			},
			"Is labstation": {
				Docs: []string{
					"Condition to check if the servohost is a labstation.",
				},
				ExecName: "servo_host_is_labstation",
			},
			"Uses servod container": {
				Docs: []string{
					"Condition to check if servo uses servod container.",
				},
				ExecName: "servo_uses_servod_container",
			},
			"Mark labstation as servod is in-use": {
				Docs: []string{
					"Create lock file is_in_use.",
				},
				Conditions: []string{
					"Is labstation",
				},
				Dependencies: []string{
					"Device is SSHable",
					"Set state:SERVO_HOST_ISSUE",
				},
				ExecName: "cros_create_servo_in_use",
				RecoveryActions: []string{
					"Sleep 1s",
					"Create request to reboot labstation",
				},
			},
			"Has enough free disk space": {
				Docs: []string{
					"Check if stateful partition have enough disk space that is at least 0.5GB.",
				},
				Conditions: []string{
					"Servod container is not used",
				},
				Dependencies: []string{
					"Device is SSHable",
					"Set state:BROKEN",
				},
				ExecName: "cros_has_enough_storage_space",
				ExecExtraArgs: []string{
					"/mnt/stateful_partition:0.5",
				},
				RecoveryActions: []string{
					"Remove logs and other files",
					"Create request to reboot labstation",
				},
			},
			"Remove logs and other files": {
				Docs: []string{
					"Clean up the old servod files as well as labstation.",
				},
				Conditions: []string{
					"Is labstation",
				},
				Dependencies: []string{
					"servo_labstation_disk_cleanup",
					"Remove logs older 5 days",
				},
				ExecName: "sample_pass",
			},
			"Remove logs older 5 days": {
				Docs: []string{
					"Clean up the old servod logs which older than 5 days.",
				},
				ExecName: "servo_servod_old_logs_cleanup",
				ExecExtraArgs: []string{
					"max_days:5",
				},
			},
			"Servod container is not used": {
				ExecName: "servo_uses_servod_container",
				ExecExtraArgs: []string{
					"reverse:true",
				},
			},
			"Servo topology": {
				Docs: []string{
					"Make sure the servo has the required number of servo components.",
				},
				Conditions: []string{
					"Is a Chromebook",
				},
				Dependencies: []string{
					"Servo topology min one child",
					"Servo topology min two children",
				},
				ExecName: "sample_pass",
			},
			"Servo topology min one child": {
				Docs: []string{
					"Verify that setup has at least one servo child.",
					"Usually that is ccd_gsc|cr50 or servo_micro or c2d2.",
				},
				Dependencies: []string{
					"Device is SSHable",
					"Set state:TOPOLOGY_ISSUE",
				},
				ExecName: "servo_topology_update",
				ExecExtraArgs: []string{
					"min_child:1",
					"persist_topology:true",
				},
				RecoveryActions: []string{
					"Stop servod",
					"Reboot servo device",
					"Toggle DTS Mode and Servo Role",
					"Toggle PD once and stop",
					"Toggle PD (5 times) and stop",
					"Try fake disconnect and stop",
					"Toggle CC line and stop",
					"Reboot by EC console and stop",
					"Cold reset the DUT by servod and stop",
					"Reset GSC from DUT and stop servod",
					"Reset EC from DUT and stop",
					"Reset GSC from DUT and stop servod",
					"Create request to reboot labstation",
				},
			},
			"Servo topology min two children": {
				Docs: []string{
					"Verify that setup has two servo children.",
					"Usually that is ccd_gsc|cr50 with servo_micro or c2d2.",
				},
				Conditions: []string{
					"is_dual_setup",
				},
				Dependencies: []string{
					"Device is SSHable",
					"Set state:TOPOLOGY_ISSUE",
				},
				ExecName: "servo_topology_update",
				ExecExtraArgs: []string{
					"min_child:2",
					"persist_topology:true",
				},
				RecoveryActions: []string{
					"Stop servod",
					"Reboot servo device",
					"Toggle DTS Mode and Servo Role",
					"Toggle PD once and stop",
					"Toggle PD (5 times) and stop",
					"Try fake disconnect and stop",
					"Toggle CC line and stop",
					"Reboot by EC console and stop",
					"Cold reset the DUT by servod and stop",
					"Reflash Cr50 fw and stop",
					"Reset EC from DUT and stop",
					"Reset GSC from DUT and stop servod",
					"Create request to reboot labstation",
				},
			},
			"Servo_v4(p1) main present": {
				Docs: []string{
					"Verify that servo_v4(p1) board is present",
				},
				Dependencies: []string{
					"Device is SSHable",
					"Set state:NOT_CONNECTED",
				},
				ExecName: "servo_v4_root_present",
				ExecExtraArgs: []string{
					"update_topology:true",
				},
				RecoveryActions: []string{
					"Recover servo with Power-cycle by smart-hub",
					"Create request to reboot labstation",
				},
			},
			"All servo's fw updated": {
				Docs: []string{
					"Check whether servo devices required firmware update.",
					"Check running agains version specified by servo_updater channel.",
				},
				Dependencies: []string{
					"Device is SSHable",
					"Set state:SERVO_UPDATER_ISSUE",
				},
				ExecName:    "servo_fw_need_update",
				ExecTimeout: &durationpb.Duration{Seconds: 300},
				RecoveryActions: []string{
					"Sleep 1s", //first try to re-read
					"Update all servo's firmware",
				},
			},
			"Read servo serial by servod harness": {
				Docs: []string{
					"Try to read servo serial by XMLRPC request to servod.",
				},
				Dependencies: []string{
					"Set state:SERVOD_PROXY_ISSUE",
				},
				ExecName: "servod_echo",
				RecoveryActions: []string{
					"Stop servod",
					"Reboot servo device",
					"Toggle DTS Mode and Servo Role",
					"Toggle PD once and stop",
					// Other actions just in case as we do not expect to run them.
					"Try fake disconnect and stop",
					"Toggle CC line and stop",
					"Reboot by EC console and stop",
					"Cold reset the DUT by servod and stop",
					"Reflash Cr50 fw and stop",
					"Reset EC from DUT and stop",
				},
			},
			"Read ppdut5_mv value": {
				Docs: []string{
					"Read and print ppdut5_mv control value to logs.",
				},
				ExecName: "servo_check_servod_control",
				ExecExtraArgs: []string{
					"command:ppdut5_mv",
				},
			},
			"Read ppchg5_mv value": {
				Docs: []string{
					"Read and print ppchg5_mv control value to logs.",
				},
				ExecName: "servo_check_servod_control",
				ExecExtraArgs: []string{
					"command:ppchg5_mv",
				},
				RecoveryActions: []string{
					"Stop servod",
				},
			},
			"Has ppchg5_mv control": {
				Docs: []string{
					"Read and print ppchg5_mv control value to logs.",
				},
				ExecName: "servod_has",
				ExecExtraArgs: []string{
					"command:ppchg5_mv",
				},
			},
			"Charger connected": {
				Docs: []string{
					"Verify that power for servo is provided.",
					"Applicable when we use type-c servo and RPM.",
				},
				Conditions: []string{
					"Is servo_v4(p1) used with type-c connector",
					"Has ppchg5_mv control",
				},
				Dependencies: []string{
					"Read ppdut5_mv value",
					"Read ppchg5_mv value",
					"Set state:SERVOD_ISSUE",
				},
				ExecName: "servo_control_min_double_value",
				ExecExtraArgs: []string{
					"control:ppchg5_mv",
					"min_value:4000",
				},
				RecoveryActions: []string{
					"Power on DUT by RPM",
				},
				AllowFailAfterRecovery: true,
			},
			"Power on DUT by RPM": {
				Docs: []string{
					"Power ON the RPM outlet.",
				},
				Conditions: []string{
					"has_rpm_info",
				},
				ExecName: "rpm_power_on",
			},
			"Check if PD is src state": {
				Docs: []string{
					"Verify that PD is src power to the DUT.",
					"Action can fail as not always the power is delivered by servo.",
				},
				Conditions: []string{
					"Is servo_v4(p1) used with type-c connector",
					"Has ppchg5_mv control",
				},
				Dependencies: []string{
					"Set state:SERVOD_ISSUE",
					"Read ppdut5_mv value",
					"Read ppchg5_mv value",
					"Charger connected",
				},
				ExecName: "servo_check_servod_control",
				ExecExtraArgs: []string{
					"command:servo_pd_role",
					"expected_string_value:src",
				},
				RecoveryActions: []string{
					"Toggle PD once and stop",
					"Toggle PD (5 times) and stop",
					"Try fake disconnect and stop",
					"Toggle CC line and stop",
					"Toggle DTS Mode and Servo Role",
				},
				AllowFailAfterRecovery: true,
			},
			"Verify CCD GSC connection detected": {
				Docs: []string{
					"Run basic cr50/ti50 detections checks.",
				},
				Conditions: []string{
					"Is a Chromebook",
					"Servo main device is CCD",
				},
				Dependencies: []string{
					"Set state:SBU_LOW_VOLTAGE",
					"Servo SBU voltage is good",
					"Set state:CR50_NOT_ENUMERATED",
					"Servo Cr50 enumerated",
				},
				ExecName: "sample_pass",
			},
			"Servo SBU voltage is good": {
				Docs: []string{
					"Verify that SBU voltage is in expected range (2500mv).",
				},
				Conditions: []string{
					"Is servo_v4(p1) used with type-c connector",
					"Servod detect voltage issue",
				},
				ExecName: "servo_cr50_low_sbu",
				RecoveryActions: []string{
					"Stop servod",
					"Reboot servo device",
					"Toggle DTS Mode and Servo Role",
					"Toggle PD once and stop",
					"Toggle PD (5 times) and stop",
					"Try fake disconnect and stop",
					"Toggle CC line and stop",
					"Reboot by EC console and stop",
					"Cold reset the DUT by servod and stop",
					"Reflash Cr50 fw and stop",
					"Reset EC from DUT and stop",
					"Reset GSC from DUT and stop servod",
					"Create request to reboot labstation",
				},
			},
			"Servod detect voltage issue": {
				Docs: []string{
					"Verify that servod is detected required children.",
				},
				ExecName: "servo_check_servod_control",
				ExecExtraArgs: []string{
					"command:dut_sbu_voltage_float_fault",
					"expected_string_value:on",
				},
				RecoveryActions: []string{
					"Stop servod",
					"Reboot servo device",
					"Toggle DTS Mode and Servo Role",
					"Toggle PD once and stop",
					"Toggle PD (5 times) and stop",
					"Try fake disconnect and stop",
					"Toggle CC line and stop",
					"Reboot by EC console and stop",
					"Reset EC from DUT and stop",
					"Cold reset the DUT by servod and stop",
					"Reflash Cr50 fw and stop",
					"Reset GSC from DUT and stop servod",
				},
			},
			"Servo Cr50 enumerated": {
				Docs: []string{
					"Verify that Cr50/GSC is enumerated or not.",
				},
				Conditions: []string{
					"Is servo_v4(p1) used with type-c connector",
					// If this pass then we have issue.
					"Servod detect voltage issue",
				},
				ExecName: "sample_fail",
				// The action failed when issue detected by conditions.
				// That mean device is not enumerated and we cannot detect it.
				// We need wake it up from DUT side.
				RecoveryActions: []string{
					"Stop servod",
					"Reboot servo device",
					"Toggle DTS Mode and Servo Role",
					"Toggle PD once and stop",
					"Toggle PD (5 times) and stop",
					"Try fake disconnect and stop",
					"Toggle CC line and stop",
					"Reboot by EC console and stop",
					"Reset EC from DUT and stop",
					"Cold reset the DUT by servod and stop",
					"Reflash Cr50 fw and stop",
					"Reset GSC from DUT and stop servod",
				},
			},
			"Servo main device is GSC chip": {
				Docs: []string{
					"Verify that main device is c2d2/cr50/GSC",
				},
				ExecName: "servo_main_device_is_gsc",
			},
			"Servo used c2d2": {
				Docs: []string{
					"Verify that servo uses c2d2",
				},
				ExecName: "servo_type_regex_match",
				ExecExtraArgs: []string{
					"regex:c2d2",
				},
			},
			"Servo main device is CCD": {
				Docs: []string{
					"Verify that main device is CCD",
				},
				Dependencies: []string{
					"Is servo_v4(p1) used with type-c connector",
				},
				ExecName: "servo_main_device_is_ccd",
			},
			"Expected CCD factory settings": {
				Docs: []string{
					"This devices should use testlab to open CCD and reset capabilities to factory settings.",
				},
				Dependencies: []string{
					"Is a Chromebook",
				},
				ExecName: "servo_ccd_expect_have_factory_reset",
			},
			"Verify cr50 console": {
				Docs: []string{
					"Verify that Cr50 console is responsive.",
				},
				Conditions: []string{
					"Is a Chromebook",
					"Expected CCD factory settings",
				},
				Dependencies: []string{
					"Initialize DUT part for servo",
					"Set state:CR50_CONSOLE_MISSING",
				},
				ExecName: "servod_can_read_all",
				ExecExtraArgs: []string{
					"commands:cr50_ccd_level,cr50_testlab,cr50_ccd_state_flags",
					"any_one:true",
				},
				RecoveryActions: []string{
					"Stop servod",
					"Reboot servo device",
					"Toggle DTS Mode and Servo Role",
					"Toggle PD once and stop",
					"Toggle PD (5 times) and stop",
					"Try fake disconnect and stop",
					"Toggle CC line and stop",
					"Reboot by EC console and stop",
					"Cold reset the DUT by servod and stop",
					"Reflash Cr50 fw and stop",
					"Reset EC from DUT and stop",
					"Reset GSC from DUT and stop servod",
				},
			},
			"Cr50 testlab is enabled": {
				Docs: []string{
					"Verify that testlab flag is enabled in GSC chip.",
					"Expect that cr50/GSC will required to set cr50 testlab is enabled.",
				},
				Conditions: []string{
					"Is a Chromebook",
					"Is not in cr50 pools",
					"Expected CCD factory settings",
				},
				Dependencies: []string{
					"Set state:CCD_TESTLAB_ISSUE",
				},
				ExecName: "servo_check_servod_control",
				ExecExtraArgs: []string{
					"command:cr50_testlab",
					"expected_string_value:on",
				},
				RecoveryActions: []string{
					"Stop servod",
					"Reboot servo device",
					"Try fake disconnect and stop",
					"Toggle CC line and stop",
				},
			},
			"Is not in cr50 pools": {
				Docs: []string{
					"Verify that DUT is not in a cr-50 pools.",
				},
				ExecName: "dut_not_in_pool_regex",
				ExecExtraArgs: []string{
					"regex:(?i)^faft-cr50",
				},
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Open gsc testlab": {
				Docs: []string{
					"If servo uses c2d2/cr50/gsc to control the DUT, open testlab will allowed to work (cr50_reboot, cold_reset, warm_reset)",
				},
				Conditions: []string{
					"Is a Chromebook",
					"Expected CCD factory settings",
				},
				ExecExtraArgs: []string{
					"command:cr50_testlab",
					"string_value:open",
				},
				ExecName:               "servo_set",
				RunControl:             RunControl_ALWAYS_RUN,
				AllowFailAfterRecovery: true,
			},
			"Reset CCD to factory settings": {
				Docs: []string{
					"Reset CCD to the factory settings.",
				},
				Conditions: []string{
					"Is a Chromebook",
					"Expected CCD factory settings",
				},
				ExecExtraArgs: []string{
					"command:cr50_uart_cmd",
					"string_value:ccd reset factory",
				},
				ExecName:               "servo_set",
				RunControl:             RunControl_ALWAYS_RUN,
				AllowFailAfterRecovery: true,
			},
			"Initialize DUT part for servo": {
				Docs: []string{
					"Call servod to init dependencies for DUT",
				},
				Dependencies: []string{
					"Set state:BROKEN",
					"Set main servo device",
					"Open gsc testlab",
					"Reset CCD to factory settings",
				},
				ExecName:    "init_dut_for_servo",
				ExecTimeout: &durationpb.Duration{Seconds: 120},
				RecoveryActions: []string{
					"Stop servod",
					"Reboot servo device",
					"Toggle DTS Mode and Servo Role",
					"Toggle PD once and stop",
					"Toggle PD (5 times) and stop",
					"Try fake disconnect and stop",
					"Toggle CC line and stop",
					"Reboot by EC console and stop",
					"Cold reset the DUT by servod and stop",
					"Reflash Cr50 fw and stop",
					"Reset EC from DUT and stop",
					"Reset GSC from DUT and stop servod",
				},
			},
			"pwr_button_supported_models": {
				Docs:     []string{"power button check is not applicable for these models"},
				ExecName: "dut_check_model",
				ExecExtraArgs: []string{
					"string_values:arkham,gale,mistral,storm,whirlwind",
					"invert_result:true",
				},
				RunControl:    RunControl_RUN_ONCE,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Verify power button signal": {
				Docs: []string{
					"verify that pwr_button signal is present.",
					"If signal is not present then probably we have issue with servo connection.",
				},
				Conditions: []string{
					"pwr_button_supported_models",
				},
				ExecExtraArgs: []string{
					"command:pwr_button",
					"expected_string_value:release",
				},
				ExecName: "servo_check_servod_control",
				RecoveryActions: []string{
					"Stop servod",
					"Reboot servo device",
					"Toggle DTS Mode and Servo Role",
					"Toggle PD once and stop",
					"Toggle PD (5 times) and stop",
					"Try fake disconnect and stop",
					"Toggle CC line and stop",
					"Reboot by EC console and stop",
					"Cold reset the DUT by servod and stop",
					"Reset EC from DUT and stop",
					"Force reflash servo_micro fw and stop",
					"Reflash Cr50 fw and stop",
				},
				AllowFailAfterRecovery: true,
			},
			"Verify servo connected to the DUT": {
				Docs: []string{
					"Verify if servo connected to the DUTand received required voltage from it.",
				},
				Conditions: []string{
					"Is a Chromebook",
					"Is servo_v4(p1) with type-a connector",
					"DUT has CrOS EC",
				},
				Dependencies: []string{
					"Set state:DUT_NOT_CONNECTED",
				},
				ExecName: "servo_low_ppdut5",
				RecoveryActions: []string{
					"Stop servod",
					"Reboot servo device",
					"Power cycle by RPM with delay and stop",
					"Toggle PD (5 times) and stop",
					"Try fake disconnect and stop",
					"Toggle CC line and stop",
					"Reboot by EC console and stop",
					"Cold reset the DUT by servod and stop",
					"Reset EC from DUT and stop",
					"Force reflash servo_micro fw and stop",
				},
			},
			"Power cycle by RPM with delay and stop": {
				Docs: []string{
					"Perform RPM cycle and wait to device to boot back.",
					"RPM applicable only for servo-micro testbeds.",
				},
				Conditions: []string{
					"is_servo_micro",
					"RPM config present",
				},
				Dependencies: []string{
					"Set RPM OFF",
					"Sleep 35s",
					"Set RPM ON",
					"Try Cold reset by servod",
					"Stop servod",
				},
				ExecName:   "sample_pass",
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Set RPM OFF": {
				ExecName: "device_rpm_power_off",
				ExecExtraArgs: []string{
					"device_type:dut",
				},
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Set RPM ON": {
				ExecName: "device_rpm_power_on",
				ExecExtraArgs: []string{
					"device_type:dut",
				},
				RunControl: RunControl_ALWAYS_RUN,
			},
			"RPM config present": {
				Docs: []string{
					"Verifies that the RPM configuration provided.",
				},
				ExecName: "device_has_rpm_info",
				ExecExtraArgs: []string{
					"device_type:dut",
				},
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Servo type-a hub connected": {
				Docs: []string{
					"Verifier to check connection Servo type-a to DUT.",
					"Working only for labstation with servo_micro.",
				},
				Conditions: []string{
					"Is labstation",
					"is_servo_micro",
					"DUT has CrOS EC",
					// Followed is condition to check if voltage is low means servo_micro is not connected.
					"DUT is UP by EC response",
				},
				ExecName: "servo_low_ppdut5",
				RecoveryActions: []string{
					"Stop servod",
					"Try fake disconnect and stop",
					"Reboot by EC console and stop",
					"Cold reset the DUT by servod and stop",
					"Reset EC from DUT and stop",
				},
			},
			"DUT is UP by EC response": {
				Docs: []string{
					"Check if DUT is up.",
					"Verification based on EC response.",
				},
				Conditions: []string{
					"DUT has CrOS EC",
				},
				ExecName: "servo_check_servod_control",
				ExecExtraArgs: []string{
					"command:ec_system_powerstate",
					"expected_string_value:S0",
				},
			},
			"Verify EC": {
				Conditions: []string{
					"Is a Chromebook",
					"DUT has CrOS EC",
				},
				Dependencies: []string{
					"Set state:EC_BROKEN",
					"Verify EC console",
					"Set state:BAD_RIBBON_CABLE",
					"Verify power button signal",
					"Set state:LID_OPEN_FAILED",
					"Is lid open",
					"Verify battery by servo",
				},
				ExecName: "sample_pass",
			},
			"DUT has CrOS EC": {
				Docs: []string{
					"Verify if DUT has ChromeOS firmware for EC",
				},
				ExecExtraArgs: []string{
					"command:supports_cros_ec_communication",
					"expected_string_value:yes",
				},
				ExecName: "servo_check_servod_control",
			},
			"Verify EC console": {
				Conditions: []string{
					"DUT has CrOS EC",
				},
				ExecName: "servod_can_read_all",
				ExecExtraArgs: []string{
					"commands:ec_system_powerstate,ec_board",
					"any_one:true",
				},
				RecoveryActions: []string{
					"Stop servod",
					"Try fake disconnect and stop",
					"Toggle CC line and stop",
					"Toggle PD once and stop",
					"Reboot by EC console and stop",
					"Cold reset the DUT by servod and stop",
					"Reset EC from DUT and stop",
				},
			},
			"battery_last_charge_readable": {
				ExecExtraArgs: []string{
					"command:battery_full_charge_mah",
				},
				ExecName: "servo_check_servod_control",
			},
			"Verify battery by servo": {
				// Do not update the servo-state as this check is for the DUT.
				Docs: []string{
					"Audit battery via servod",
				},
				Conditions: []string{
					"DUT has CrOS EC",
					"battery_last_charge_readable",
				},
				ExecName:               "servo_battery_charging",
				AllowFailAfterRecovery: true,
			},
			"Update USB drive info": {
				Docs: []string{
					"Try to update the information of the servo usbkey in inventory and karte.",
				},
				Dependencies: []string{
					"Verify that USB drive is detectable",
				},
				ExecName:               "servo_update_usbkey_history",
				AllowFailAfterRecovery: true,
			},
			"Verify that USB drive is detectable": {
				Docs: []string{
					"Will detect the path to USB Drive on servo-host.",
					"Verify that usb-key is responsive",
				},
				Dependencies: []string{
					"Device is SSHable",
				},
				ExecName: "servo_usbkey_is_detected",
				ExecExtraArgs: []string{
					"file_check:true",
				},
				AllowFailAfterRecovery: true,
				RecoveryActions: []string{
					"Recover servo with Power-cycle by smart-hub",
				},
			},
			"Is servo_v4(p1) used with type-c connector": {
				Docs: []string{
					"Verify whether servo_V4(p1) device is connect to DUT using Type-C connection.",
				},
				ExecExtraArgs: []string{
					"command:root.dut_connection_type",
					"expected_string_value:type-c",
				},
				ExecName: "servo_check_servod_control",
			},
			"Is lid open": {
				Docs: []string{
					"Verify lid of the is open",
					"Allowed to fail as check if ont effect the servo functionality.",
				},
				ExecName: "servod_lidopen",
				RecoveryActions: []string{
					"Stop servod",
					"Reboot by EC console and stop",
					"Cold reset the DUT by servod and stop",
					"Reflash Cr50 fw and stop",
					"Reset EC from DUT and stop",
				},
				AllowFailAfterRecovery: true,
			},
			"Is servo_v4(p1) with type-a connector": {
				Docs: []string{
					"Verify whether servo V4(p1) device is connect to DUT using Type-A connection.",
				},
				ExecExtraArgs: []string{
					"command:root.dut_connection_type",
					"expected_string_value:type-a",
				},
				ExecName: "servo_check_servod_control",
			},
			"is_dual_setup": {
				Docs: []string{
					"Check whether the servo device has dual setup. This check only applies to the devices that have the dual setup configured on them.",
				},
				ExecName: "is_dual_setup_configured",
			},
			"is_not_dual_setup": {
				Conditions: []string{
					"is_dual_setup",
				},
				ExecName: "sample_fail",
			},
			"Set main servo device": {
				Docs: []string{
					"Set main device is it not set before.",
					"Applicable if we have more than one child servo device.",
				},
				Conditions: []string{
					"Servod knows about active_dut_controller control",
				},
				ExecName: "servod_set_main_device",
				RecoveryActions: []string{
					"Stop servod",
					"Toggle PD (5 times) and stop",
					"Toggle CC line and stop",
					"Reboot by EC console and stop",
					"Cold reset the DUT by servod and stop",
					"Reset EC from DUT and stop",
				},
			},
			"Update all servo's firmware": {
				Docs: []string{
					"Try to update in  normal ways 3 times, if fail allow run force update.",
				},
				Dependencies: []string{
					"Stop servod daemon on servo-host",
				},
				ExecName: "servo_update_servo_firmware",
				ExecExtraArgs: []string{
					"try_attempt_count:3",
					"try_force_update_after_fail:true",
				},
				ExecTimeout: &durationpb.Duration{Seconds: 600},
				RunControl:  RunControl_RUN_ONCE,
			},
			"Force reflash servo_micro fw and stop": {
				Docs: []string{
					"Try to update servo micro firmware",
				},
				Conditions: []string{
					"Is labstation",
					"is_servo_micro",
					"Is ok to force update servo_micro firmware",
				},
				Dependencies: []string{
					"Force update servo_micro firmware",
					"Stop servod",
				},
				ExecName: "sample_pass",
			},
			"Force update servo_micro firmware": {
				Docs: []string{
					"Try to update servo micro firmware",
				},
				Conditions: []string{
					"Is labstation",
					"is_servo_micro",
					"Is ok to force update servo_micro firmware",
				},
				ExecExtraArgs: []string{
					"force_update:true",
					"ignore_version:true",
					"servo_board:servo_micro",
				},
				ExecTimeout: &durationpb.Duration{Seconds: 180},
				ExecName:    "servo_update_servo_firmware",
			},
			"Is ok to force update servo_micro firmware": {
				Docs: []string{
					"Verify that it is time when we can try to re-flash fw on servo micro.",
					"Re-flashing limited to once per once per 2 weeks to avoid over-flashing the servo device.",
				},
				Conditions: []string{
					"Last servo_micro fw updated within 2 weeks",
				},
				ExecName: "sample_fail",
			},
			"Last servo_micro fw updated within 2 weeks": {
				Docs: []string{
					"Confirm that servo micro fw update action has occurred in the past 2 weeks. (336 hours)",
				},
				ExecExtraArgs: []string{
					"metrics_kind:servo_firmware_update_servo_micro",
					"time_frame_hours:336",
				},
				ExecName: "metrics_found_at_last_time",
			},
			"Warm reset control known by servo": {
				Docs: []string{
					"Verify is servod expected to have warm_reset control",
				},
				ExecName: "servo_check_servod_control",
				ExecExtraArgs: []string{
					"command:warm_reset",
				},
			},
			"Warm reset pin is detected (servo_micro)": {
				// TODO(b/248631441): need monitor before make it critical.
				Docs: []string{
					"Verify that warm_reset pin is detected by servod.",
					"If pin is not present then issue can be related to incorrect connected servo or issue with connector.",
				},
				Conditions: []string{
					"Is a Chromebook",
					"is_servo_micro",
					"Warm reset control known by servo",
				},
				Dependencies: []string{
					"Device is SSHable",
					"Set state:WARM_RESET_PIN_ISSUE",
				},
				ExecName: "servo_check_servod_control",
				ExecExtraArgs: []string{
					"command:warm_reset",
					"expected_string_value:off",
				},
				AllowFailAfterRecovery: true,
			},
			"Cold reset pin is detected": {
				Conditions: []string{
					"Is a Chromebook",
					"Is servo_v4(p1) with type-a connector",
				},
				Dependencies: []string{
					"Device is SSHable",
					"Set state:COLD_RESET_PIN_ISSUE",
				},
				ExecName: "servo_check_servod_control",
				ExecExtraArgs: []string{
					"command:cold_reset",
					"expected_string_value:off",
				},
				RecoveryActions: []string{
					"Stop servod",
					"Reboot by EC console and stop",
					"Reset EC from DUT and stop",
				},
			},
			"Servod is responsive to dut-control": {
				Docs: []string{
					"Uses a servod control to check whether the servod daemon is responsive.",
				},
				Dependencies: []string{
					"Device is SSHable",
					"Set state:SERVOD_ISSUE",
				},
				ExecName:    "servo_servod_echo_host",
				ExecTimeout: &durationpb.Duration{Seconds: 30},
				RecoveryActions: []string{
					"Stop servod and request to use recovery-mode for servod",
					"Stop servod",
					"Reboot servo device",
					"Reboot by EC console and stop",
					"Cold reset the DUT by servod and stop",
					"Reset EC from DUT and stop",
					"Create request to reboot labstation",
				},
			},
			"Record good servo type": {
				Docs: []string{
					"Record servo type information.",
					"The action need always work if not then we have issue.",
				},
				Dependencies: []string{
					"Set state:BROKEN",
				},
				ExecName: "servo_update_servo_type_label",
			},
			"Servo uses debug header components": {
				Docs: []string{
					"Verify that servo has components which are not started from ccd_.",
				},
				ExecName: "servo_has_debug_header",
			},
			"Debug header servo present": {
				Docs: []string{
					"Check if servod detected debug header components as expected.",
				},
				Conditions: []string{
					"Is a Chromebook",
					"Servo uses debug header components",
				},
				Dependencies: []string{
					"Set state:DEBUG_HEADER_SERVO_MISSING",
				},
				ExecName: "servo_check_servod_control",
				ExecExtraArgs: []string{
					"command:dut_controller_missing_fault",
					"expected_string_value:off",
				},
				RecoveryActions: []string{
					"Stop servod",
					"Reboot servo device",
					"Reset EC from DUT and stop",
					"Create request to reboot labstation",
				},
			},
			"Servod detect all children components": {
				Docs: []string{
					"Check if servod detected all required children components.",
				},
				Conditions: []string{
					"Is a Chromebook",
				},
				Dependencies: []string{
					"Set state:SERVOD_DUT_CONTROLLER_MISSING",
				},
				ExecName: "servo_check_servod_control",
				ExecExtraArgs: []string{
					"command:dut_controller_missing_fault",
					"expected_string_value:off",
				},
				RecoveryActions: []string{
					"Stop servod",
					"Reboot servo device",
					"Toggle DTS Mode and Servo Role",
					"Toggle PD once and stop",
					"Toggle PD (5 times) and stop",
					"Try fake disconnect and stop",
					"Toggle CC line and stop",
					"Reboot by EC console and stop",
					"Cold reset the DUT by servod and stop",
					"Reset EC from DUT and stop",
					"Reflash Cr50 fw and stop",
					"Reset GSC from DUT and stop servod",
					"Create request to reboot labstation",
				},
			},
			"Servod knows about active_dut_controller control": {
				ExecName: "servo_check_servod_control",
				ExecExtraArgs: []string{
					"command:active_dut_controller",
				},
			},
			"servod_restart_dut": {
				ExecName: "sample_pass",
			},
			"Toggle PD once and stop": {
				Docs: []string{
					"Toggle the servod command servo_pd_role only once. And then stop the servod afterwards.",
					"TODO(otabek): Add dependency for servo initialize.",
				},
				Dependencies: []string{
					"Toggle PD once",
					"Stop servod",
				},
				ExecName:   "sample_pass",
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Toggle PD once": {
				Docs: []string{
					"Toggle the servod command servo_pd_role only once.",
				},
				ExecExtraArgs: []string{
					"toggle_times:1",
					"wait_in_retry:5",
					"wait_before_retry:1",
				},
				RunControl: RunControl_ALWAYS_RUN,
				ExecName:   "servo_servod_toggle_pd_role",
			},
			"Toggle PD (5 times) and stop": {
				Docs: []string{
					"Toggle the servod command servo_pd_role 5 times. And then stop the servod afterwards.",
				},
				Conditions: []string{
					"Is servo_v4(p1) used with type-c connector",
				},
				Dependencies: []string{
					"Toggle PD 5 times",
				},
				ExecName:   "servo_host_servod_stop",
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Toggle PD 5 times": {
				Docs: []string{
					"Toggle the servod command servo_pd_role 5 times.",
				},
				ExecName: "servo_servod_toggle_pd_role",
				ExecExtraArgs: []string{
					"toggle_times:5",
					"wait_in_retry:5",
					"wait_before_retry:1",
				},
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Set state:MISSING_CONFIG": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:MISSING_CONFIG",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:WRONG_CONFIG": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:WRONG_CONFIG",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:NO_SSH": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:NO_SSH",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:BROKEN": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:BROKEN",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:NOT_CONNECTED": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:NOT_CONNECTED",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:NEED_REPLACEMENT": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:NEED_REPLACEMENT",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:TOPOLOGY_ISSUE": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:TOPOLOGY_ISSUE",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:CR50_NOT_ENUMERATED": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:CR50_NOT_ENUMERATED",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:SERVOD_DUT_CONTROLLER_MISSING": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:SERVOD_DUT_CONTROLLER_MISSING",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:SERVO_UPDATER_ISSUE": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:SERVO_UPDATER_ISSUE",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:SERVOD_ISSUE": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:SERVOD_ISSUE",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:SERVO_HOST_ISSUE": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:SERVO_HOST_ISSUE",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:SERVOD_PROXY_ISSUE": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:SERVOD_PROXY_ISSUE",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:COLD_RESET_PIN_ISSUE": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:COLD_RESET_PIN_ISSUE",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:WARM_RESET_PIN_ISSUE": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:WARM_RESET_PIN_ISSUE",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:POWER_BUTTON_PIN_ISSUE": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:POWER_BUTTON_PIN_ISSUE",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:DEBUG_HEADER_SERVO_MISSING": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:DEBUG_HEADER_SERVO_MISSING",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:SBU_LOW_VOLTAGE": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:SBU_LOW_VOLTAGE",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:DUT_NOT_CONNECTED": {
				ExecExtraArgs: []string{"state:DUT_NOT_CONNECTED"},
				ExecName:      "servo_set_servo_state",
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:CR50_CONSOLE_MISSING": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:CR50_CONSOLE_MISSING",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:EC_BROKEN": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:EC_BROKEN",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:BAD_RIBBON_CABLE": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:BAD_RIBBON_CABLE",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:LID_OPEN_FAILED": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:LID_OPEN_FAILED",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:CCD_TESTLAB_ISSUE": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:CCD_TESTLAB_ISSUE",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:WORKING": {
				ExecName: "servo_set_servo_state",
				ExecExtraArgs: []string{
					"state:WORKING",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Try fake disconnect and stop": {
				Docs: []string{
					"Try to repair servod by mimic reconnection of servo.",
				},
				Conditions: []string{
					"is_servo_type_ccd",
				},
				Dependencies: []string{
					"Try fake disconnect",
					"Stop servod",
				},
				ExecName:   "sample_pass",
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Try fake disconnect": {
				Conditions: []string{
					"is_servo_type_ccd",
				},
				ExecName: "servo_fake_disconnect_dut",
				ExecExtraArgs: []string{
					"delay_in_ms:100",
					"timeout_in_ms:2000",
				},
			},
			"Toggle CC line and stop": {
				Docs: []string{
					"Try to repair servod by toggling cc and stop servod after.",
				},
				Dependencies: []string{
					"Toggle CC lines",
					"Stop servod",
				},
				ExecName:   "sample_pass",
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Toggle CC lines": {
				Docs: []string{
					"Toggle cc line connected between servo and DUT to wake up the connection.",
				},
				Conditions: []string{
					"is_servo_type_ccd",
				},
				ExecName: "servo_servod_cc_toggle",
				ExecExtraArgs: []string{
					"cc_off_timeout:10",
					"cc_on_timeout:30",
				},
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Toggle DTS Mode and Servo Role": {
				Docs: []string{
					"Toggle dts mode and servo role to try and recover CCD.",
				},
				Conditions: []string{
					"is_servo_type_ccd",
				},
				ExecName:   "servo_servod_dts_and_servo_role_toggle_exec",
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Reboot by EC console and stop": {
				Docs: []string{
					"Try to reboot DUT's EC by servod UART console and stop servod after that.",
				},
				Dependencies: []string{
					"Reboot by DUT's EC UART",
					"Stop servod",
				},
				ExecName:   "sample_pass",
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Reboot by DUT's EC UART": {
				Docs: []string{
					"Try to reboot EC on DUT using servod command.",
					"It reboots just the embedded controllers on the DUT.",
				},
				ExecExtraArgs: []string{
					"wait_timeout:1",
					"value:reboot",
				},
				ExecName:   "servo_set_ec_uart_cmd",
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Cold reset the DUT by servod and stop": {
				Docs: []string{
					"Try to reboot DUT by resetting power state command on servod.",
				},
				Dependencies: []string{
					"Cold reset by servod",
					"Stop servod",
				},
				ExecName:   "sample_pass",
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Cold reset by servod": {
				Docs: []string{
					"Try to reboot DUT by resetting power state command on servod.",
				},
				ExecName: "servo_power_state_reset",
				ExecExtraArgs: []string{
					"wait_timeout:1",
					"more_logs:true",
				},
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Try Cold reset by servod": {
				Docs: []string{
					"Try to reboot DUT by resetting power state command on servod.",
				},
				ExecName: "servo_power_state_reset",
				ExecExtraArgs: []string{
					"wait_timeout:1",
					"more_logs:true",
				},
				AllowFailAfterRecovery: true,
				RunControl:             RunControl_ALWAYS_RUN,
			},
			"Create request to reboot labstation": {
				Docs: []string{
					"Try to create reboot flag file request.",
					"The action always fails as Servo will be fixed after reboot.",
				},
				Conditions: []string{
					"Is labstation",
				},
				Dependencies: []string{
					"cros_create_reboot_request",
				},
				ExecName:   "sample_fail",
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Reflash Cr50 fw and stop": {
				Docs: []string{
					"Try to reflash cr50 firmware and reboot AP from DUT side to wake it up.",
				},
				Conditions: []string{
					"DUT is SSHable",
					"is_servo_type_ccd",
					"Is reflash Cr50 was done more 24 hours ago",
				},
				Dependencies: []string{
					"Reflash Cr50 fw on DUT",
					"Stop servod",
				},
				ExecName:   "sample_pass",
				RunControl: RunControl_RUN_ONCE,
			},
			"Reflash Cr50 fw on DUT": {
				Docs: []string{
					"Try to reflash cr50 firmware and reboot AP from DUT side to wake it up.",
					"Reboot after the fw flash is successful.",
				},
				Dependencies: []string{
					"DUT is SSHable",
				},
				ExecName: "cros_reflash_cr50_fw",
				ExecExtraArgs: []string{
					"flash_timeout:120",
					"wait_timeout:30",
				},
				ExecTimeout: &durationpb.Duration{Seconds: 150},
				RunControl:  RunControl_RUN_ONCE,
			},
			"Reset GSC from DUT and stop servod": {
				Docs: []string{
					"Try to reset GSC from DUT side to wake it up.",
				},
				Conditions: []string{
					"DUT is SSHable",
					"Is servo_v4(p1) used with type-c connector",
				},
				Dependencies: []string{
					"Reset GSC on DUT",
					"Stop servod",
				},
				ExecName:   "sample_pass",
				RunControl: RunControl_RUN_ONCE,
			},
			"Reset GSC on DUT": {
				Docs: []string{
					"Try to reflash cr50 firmware and reboot AP from DUT side to wake it up.",
					"The command recommended by cr50 team http://b/241161724#comment24.",
					"Reboot after the fw flash is successful.",
				},
				Conditions: []string{
					"DUT is SSHable",
				},
				ExecName: "cros_run_command",
				ExecExtraArgs: []string{
					"host:dut",
					"command:trunks_send --raw 80010000000c200000000013",
				},
				RunControl: RunControl_RUN_ONCE,
				// Command triggers reboot and action hangs on rebooted connection.
				AllowFailAfterRecovery: true,
			},
			"DUT is SSHable": {
				Docs: []string{
					"verify if DUT is SSH-able",
				},
				ExecName:    "cros_ssh_dut",
				ExecTimeout: &durationpb.Duration{Seconds: 15},
				RunControl:  RunControl_ALWAYS_RUN,
			},
			"DUT is not SSHable": {
				Docs: []string{
					"Verify if DUT is not SSH-able",
				},
				Conditions: []string{
					"DUT is SSHable",
				},
				ExecName:   "sample_fail",
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Is reflash Cr50 was done more 24 hours ago": {
				Docs: []string{
					"Verify that it is time when we can try to re-flash fw on cr50 (H1).",
					"Re-flashing limited to once per once per day to avoid over-flashing the device.",
				},
				Conditions: []string{
					"Is reflash Cr50 was done within 24 hours",
				},
				ExecName: "sample_fail",
			},
			"Is reflash Cr50 was done within 24 hours": {
				Docs: []string{
					"Confirm that no cr50 reflash action has occurred in the past 24 hours.",
				},
				ExecExtraArgs: []string{
					"metrics_kind:cr50_flash",
					"time_frame_hours:24",
				},
				ExecName: "metrics_found_at_last_time",
			},
			"Reset EC from DUT and stop": {
				Docs: []string{
					"Try to reset EC from DUT side to wake CR50 up and then stop the servod.",
				},
				Conditions: []string{
					"DUT is SSHable",
					"is_servo_type_ccd",
				},
				Dependencies: []string{
					"cros_reset_ec",
					"Stop servod",
				},
				ExecName:   "sample_pass",
				RunControl: RunControl_ALWAYS_RUN,
			},
			"cros_reset_ec": {
				Docs: []string{
					"Try to reset EC from DUT side by running connads wake up the device as it will trigger recovering ec, cr50, and other fw.",
				},
				Dependencies: []string{
					"DUT is SSHable",
				},
				ExecName: "cros_reset_ec",
				ExecExtraArgs: []string{
					"wait_timeout:30",
				},
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Power-cycle by smart-hub": {
				Docs: []string{
					"Try to reset(power-cycle) the servo via smart usbhub.",
				},
				Conditions: []string{
					// We try restart only if we lost network to the dut.
					"DUT is not SSHable",
				},
				Dependencies: []string{
					"Device is SSHable",
				},
				ExecName: "servo_power_cycle_root_servo",
				ExecExtraArgs: []string{
					"reset_timeout:60",
					"wait_timeout:20",
					"reset_authorized:false",
				},
				ExecTimeout:            &durationpb.Duration{Seconds: 120},
				RunControl:             RunControl_RUN_ONCE,
				AllowFailAfterRecovery: true,
			},
			"Recover servo with Power-cycle by smart-hub": {
				Docs: []string{
					"Try to power-cycle the servo via smart usbhub to recover it.",
				},
				ExecName: "servo_power_cycle_root_servo",
				ExecExtraArgs: []string{
					"reset_timeout:60",
					"wait_timeout:20",
					"reset_authorized:false",
				},
				ExecTimeout: &durationpb.Duration{Seconds: 120},
				RunControl:  RunControl_ALWAYS_RUN,
			},
			"Sleep 1s": {
				ExecName: "sample_sleep",
				ExecExtraArgs: []string{
					"sleep:1",
				},
				RunControl:             RunControl_ALWAYS_RUN,
				AllowFailAfterRecovery: true,
				MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Sleep 35s": {
				ExecName: "sample_sleep",
				ExecExtraArgs: []string{
					"sleep:35",
				},
				RunControl:             RunControl_ALWAYS_RUN,
				AllowFailAfterRecovery: true,
				MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Read release info": {
				// TODO(otabek): Think to save the result to logs.
				Docs: []string{
					"Read host release data for future analysis.",
				},
				Conditions: []string{
					"Servod container is not used",
				},
				Dependencies: []string{
					"Device is SSHable",
					"Set state:SERVO_HOST_ISSUE",
				},
				ExecName: "cros_run_command",
				ExecExtraArgs: []string{
					// Do not specify host to receive currect host.
					"host:",
					"command:cat /etc/lsb-release",
				},
				RunControl:             RunControl_RUN_ONCE,
				AllowFailAfterRecovery: true,
			},
			"Reboot servo device": {
				Docs: []string{
					"Reboot servo device via servodtool",
				},
				Dependencies: []string{
					"Device is SSHable",
					"Stop servod",
				},
				ExecName: "servo_reboot",
				ExecExtraArgs: []string{
					"reboot_timeout:30",
					"wait_timeout:30",
				},
				ExecTimeout: &durationpb.Duration{Seconds: 70},
				RunControl:  RunControl_RUN_ONCE,
			},
			"Is a Chromebook": {
				Docs: []string{
					"Verify that the device is a Chromebook by checking for non-Chromebook boards",
				},
				ExecName: "dut_check_board",
				ExecExtraArgs: []string{
					"string_values:aurora,reven",
					"invert_result:true",
				},
				RunControl:    RunControl_RUN_ONCE,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
		},
	}
}
