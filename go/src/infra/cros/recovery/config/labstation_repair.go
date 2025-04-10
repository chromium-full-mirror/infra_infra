// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"fmt"

	"google.golang.org/protobuf/types/known/durationpb"
)

// LabstationRepairConfig provides config for repair labstation task.
func LabstationRepairConfig() *Configuration {
	beforeLogName, beforeLogActions := labstationCollectionLogs("before")
	afterLogName, afterLogActions := labstationCollectionLogs("after")
	criticalActions := []string{
		"Set state: repair_failed",
		beforeLogName,
		"Device is SSHable",
		"System services is up",
		"Clean up logs if necessary",
		"Filesystem is writable",
		"Used Inodes percentage on stateful partition is lower than 50%",
		"Check servod dependencies",
		"cros_is_on_stable_version",
		"Update provisioned info",
		"booted_from_right_kernel",
		"reboot_by_request",
		"Reboot labstation if uptime longer than 7 days",
		// TODO(b/245824583): remove this action once the bug fixed.
		"Cleanup bluetooth",
		"Is crosid readable",
		"Update inventory info",
		"Set state: ready",
		afterLogName,
	}
	actions := map[string]*Action{
		"cros_is_on_stable_version": {
			Conditions: []string{
				"has_stable_version_cros_image",
				"cros_kernel_priority_has_not_changed",
				"Labstation not in auto-update exempted pool",
			},
			RecoveryActions: []string{
				"Install stable labstation image without reboot",
			},
			AllowFailAfterRecovery: true,
		},
		"Install stable labstation image without reboot": {
			Docs: []string{
				"Install stable labstation image but do not reboot.",
			},
			Conditions: []string{
				"has_stable_version_cros_image",
				"cros_kernel_priority_has_not_changed",
			},
			ExecName: "cros_provision",
			ExecExtraArgs: []string{
				"no_reboot",
			},
			ExecTimeout: &durationpb.Duration{Seconds: 3600},
		},
		"Labstation not in auto-update exempted pool": {
			Docs: []string{
				"There are some labstations we don't want they receive auto-update, e.g. labstations that used for image qualification purpose",
			},
			ExecName: "dut_not_in_pool",
			ExecExtraArgs: []string{
				"servo_verification",
				"labstation_tryjob",
				"labstation_canary",
				"labstation_block_autoupdate",
			},
		},
		"Update provisioned info": {
			Docs: []string{
				"Update OS version for provision info.",
			},
			ExecName:               "cros_update_provision_info",
			AllowFailAfterRecovery: true,
		},
		"labstation_langid_check": {
			Docs: []string{
				"This part is not ready.",
				"The action and will validate present of lang_id issue",
			},
			ExecName:               "sample_pass",
			AllowFailAfterRecovery: true,
		},
		"cros_stop_powerd": {
			ExecName: "cros_run_shell_command",
			ExecExtraArgs: []string{
				"stop",
				"powerd",
			},
			AllowFailAfterRecovery: true,
			RunControl:             RunControl_ALWAYS_RUN,
		},
		"cros_clean_tmp_owner_request": {
			Docs: []string{
				"In some cases, the update flow puts the TPM into a state such that it fails verification.",
				"We don't know why. However, this call papers over the problem by clearing the TPM during the reboot.",
				"We ignore failures from 'crossystem'.",
				"Although failure here is unexpected, and could signal a bug, the point of the exercise is to paper over problems.",
			},
			AllowFailAfterRecovery: true,
			RunControl:             RunControl_ALWAYS_RUN,
		},
		"labstation_uptime_6_hours": {
			ExecName: "cros_validate_uptime",
			ExecExtraArgs: []string{
				"min_duration:6",
			},
		},
		"Remove reboot requests": {
			Docs: []string{
				"Remove all requests for reboot on the host.",
				"The action has to be called after reboot of the device.",
			},
			ExecName:               "cros_remove_all_reboot_request",
			AllowFailAfterRecovery: true,
		},
		"reboot_by_request": {
			Docs: []string{
				"Some DUTs can request reboot labstation if they has issue with servo-nic or other issues with servo-host.",
				"We allowed to remove requests for reboot if we rebooted per request.",
			},
			Conditions: []string{
				"cros_has_reboot_request",
				"cros_has_no_servo_in_use",
				"labstation_uptime_6_hours",
			},
			// If condition passed then action will fail and request recovery actions.
			ExecName: "sample_fail",
			RecoveryActions: []string{
				"Labstation reboot",
				"Power cycle by RPM",
			},
		},
		"Reboot labstation if uptime longer than 7 days": {
			Docs: []string{
				"Check labstation uptime and trigger a reboot if it's longer than 7 days (168 hours).",
			},
			Conditions: []string{
				// No need to run this action if there is servo in use as we don't want reboot interrupt active servos.
				"cros_has_no_servo_in_use",
			},
			ExecName: "cros_validate_uptime",
			ExecExtraArgs: []string{
				"max_duration:168",
			},
			RecoveryActions: []string{
				"Labstation reboot",
				"Power cycle by RPM",
			},
		},
		"booted_from_right_kernel": {
			Docs: []string{
				"Verified if kernel has update and waiting for update.",
				"Kernel can wait for reboot as provisioning is not doing reboot by default for labstations.",
			},
			Conditions: []string{
				"cros_has_no_servo_in_use",
			},
			ExecName: "cros_kernel_priority_has_not_changed",
			RecoveryActions: []string{
				"Labstation reboot",
				"Power cycle by RPM",
			},
		},
		"Device is SSHable": {
			Docs: []string{
				"This verifier checks whether the host is accessible over ssh.",
			},
			RecoveryActions: []string{
				"Power cycle by RPM",
				"Power cycle by RPM with long delay",
			},
			ExecName:    "cros_ssh",
			ExecTimeout: &durationpb.Duration{Seconds: 30},
			RunControl:  RunControl_ALWAYS_RUN,
		},
		"Filesystem is writable": {
			Docs: []string{
				"This verifier checks whether the host filesystem is writable.",
			},
			ExecName:               "cros_is_file_system_writable",
			AllowFailAfterRecovery: true,
		},
		"Labstation reboot": {
			Docs: []string{
				"Perform reboot of the host and perform additional actions as necessary.",
				"If reboot succeed then we can remove all request for reboot as we just did it.",
			},
			Dependencies: []string{
				"cros_stop_powerd",
				"cros_clean_tmp_owner_request",
				"cros_allowed_reboot",
				"Simple reboot",
				"Sysrq reboot",
				"Sleep 10s",
				// Waiting to tell if success.
				"Wait to be SSHable",
				"Start system services",
				"Remove reboot requests",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Power cycle by RPM": {
			Docs: []string{
				"Action is always runnable.",
			},
			Conditions: []string{
				"rpm_action_enabled",
				"has_rpm_info",
			},
			Dependencies: []string{
				"rpm_power_cycle",
				// Waiting to tell if success.
				"Wait to be SSHable",
				"Start system services",
				"Remove reboot requests",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Power cycle by RPM with long delay": {
			Docs: []string{
				"Power cycle the labstation via RPM with longer delay between OFF/ON toggle.",
			},
			Conditions: []string{
				"has_rpm_info",
			},
			Dependencies: []string{
				"Power off by RPM",
				"Sleep 1 minute",
				"Power on by RPM",
				"Wait to be SSHable",
				"Start system services",
				"Remove reboot requests",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
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
			ExecTimeout: &durationpb.Duration{Seconds: 120},
			RunControl:  RunControl_ALWAYS_RUN,
		},
		"Simple reboot": {
			Docs: []string{
				"Simple un-blocker reboot.",
				"The action will not run if the labstation's filesystem I/O is blocked because /sbin/reboot may not work if the filesystem is hosed.",
			},
			Conditions: []string{
				"cros_filesystem_io_not_blocked",
			},
			ExecName: "cros_run_command",
			ExecExtraArgs: []string{
				"command:reboot",
				"background:true",
			},
			RunControl: RunControl_ALWAYS_RUN,
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
		"Update inventory info": {
			Docs: []string{
				"Updating device info in inventory.",
			},
			Dependencies: []string{
				"cros_update_hwid_to_inventory",
				"Read serial number from labstation",
			},
			ExecName:      "sample_pass",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Sysrq reboot": {
			Docs: []string{
				"Immediately reboot the system, without unmounting or syncing filesystems",
				"The action only runs when the filesystem is hosed where regular reboot executable will not work.",
			},
			Conditions: []string{
				"Filesystem IO blocked",
			},
			ExecName: "cros_run_shell_command",
			ExecExtraArgs: []string{
				"echo b > /proc/sysrq-trigger",
			},
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
		},
		"Filesystem IO blocked": {
			Docs: []string{
				"Filesystem I/O is blocked on the labstation.",
				"The action is expected to fail when filesystem I/O is not blocked on the labstation.",
			},
			Conditions: []string{
				"cros_filesystem_io_not_blocked",
			},
			ExecName:   "sample_fail",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Read serial number from labstation": {
			ExecName:               "cros_update_serial_number_inventory",
			AllowFailAfterRecovery: true,
		},
		"Clean up logs if necessary": {
			Docs: []string{
				"Check size of messages logs on labstation and cleanup if necessary.",
			},
			ExecName:               "cros_log_clean_up",
			AllowFailAfterRecovery: true,
		},
		"Attempt to remove bluetooth device": {
			Docs: []string{
				"Attempt to remove bluetooth device from the labstation.",
			},
			ExecName:               "cros_remove_bt_devices",
			AllowFailAfterRecovery: true,
		},
		"Attempt to power off bluetooth adapter": {
			Docs: []string{
				"Attempt to power off bluetooth adapter on the labstation.",
			},
			ExecName: "cros_run_shell_command",
			ExecExtraArgs: []string{
				"bluetoothctl power off",
			},
			AllowFailAfterRecovery: true,
		},
		"Cleanup bluetooth": {
			Docs: []string{
				"Attempt to remove bluetooth device and then power off BT adapter.",
				"This action should be removed once b/245824583 got fixed.",
			},
			Dependencies: []string{
				"Attempt to remove bluetooth device",
				"Attempt to power off bluetooth adapter",
			},
			ExecName:               "sample_pass",
			AllowFailAfterRecovery: true,
		},
		"Labstation image contains target GenesysLogic firmware": {
			Docs: []string{
				"Check if the current labstation OS image contains required GenesysLogic firmware",
			},
			ExecName: "cros_genesys_logic_firmware_image_exists",
		},
		"Update GenesysLogic Firmware for servos": {
			Docs: []string{
				"Attempt to update GenesysLogic firmware for all servos on the labstation if needed.",
				"The update run will be a no-op if a servo is already updated to the target firmware.",
			},
			Conditions: []string{
				"Labstation image contains target GenesysLogic firmware",
			},
			ExecName:               "cros_update_genesys_logic_firmware",
			AllowFailAfterRecovery: true,
		},
		"System services is up": {
			Docs: []string{
				"Check whether system-services is up and running",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_wait_for_system",
			RecoveryActions: []string{
				"Start system services",
				// In edge cases, labstation may needs a bit more time to wait for system-services to up.
				"Sleep 1 minute",
			},
		},
		"Start system services": {
			Docs: []string{
				"Start system-services on the labstation",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_run_command",
			ExecExtraArgs: []string{
				"host:dut",
				"command:start system-services",
			},
			AllowFailAfterRecovery: true,
			RunControl:             RunControl_ALWAYS_RUN,
		},
		"Write factory-install-reset to file system": {
			ExecName: "cros_run_shell_command",
			ExecExtraArgs: []string{
				"echo \"fast safe\" > /mnt/stateful_partition/factory_install_reset",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Install stable labstation image with reboot": {
			Docs: []string{
				"Install stable labstation image with reboot during provision process.",
			},
			Conditions: []string{
				"has_stable_version_cros_image",
			},
			ExecName:      "cros_provision",
			ExecTimeout:   &durationpb.Duration{Seconds: 3600},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Powerwash repair labstation": {
			Docs: []string{
				"Powerwash and then install stable_version image on the labstation.",
			},
			Conditions: []string{
				"Device is SSHable",
				// For Special pools, we don't want version get changed in automated way.
				"Labstation not in auto-update exempted pool",
			},
			Dependencies: []string{
				"Write factory-install-reset to file system",
				"Labstation reboot",
				"Install stable labstation image without reboot",
				"Labstation reboot",
			},
			ExecName: "sample_pass",
		},
		"Check servod dependencies": {
			Docs: []string{
				"Ensure critical dependencies for servod is there, this check may fail if labstation had a incomplete provision.",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_run_command",
			ExecExtraArgs: []string{
				"host:dut",
				"command:servod --sversion",
			},
			RecoveryActions: []string{
				"Powerwash repair labstation",
			},
		},
		"Sleep 10s": {
			ExecName: "sample_sleep",
			ExecExtraArgs: []string{
				"sleep:10",
			},
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Is crosid present": {
			Docs: []string{
				"Verify if crosid cli is present on the ChromeOS",
			},
			ExecName: "cros_run_command",
			ExecExtraArgs: []string{
				"host:",
				"command:which crosid",
			},
		},
		"Is crosid readable": {
			Docs: []string{
				"Verify crosid cli is responsive.",
			},
			Conditions: []string{
				"Device is SSHable",
				"Is crosid present",
			},
			ExecName: "cros_run_command",
			ExecExtraArgs: []string{
				"host:",
				"command:crosid",
			},
			RecoveryActions: []string{
				"Remove whitelabel_tag VPD field",
			},
			AllowFailAfterRecovery: true,
		},
		"Remove whitelabel_tag VPD field": {
			// See b/325495298 for context of why we need this.
			Docs: []string{
				"Remove whitelabel_tag field from VPD cache.",
			},
			Conditions: []string{
				"Device is SSHable",
			},
			ExecName: "cros_run_command",
			ExecExtraArgs: []string{
				"host:",
				"command:vpd -d whitelabel_tag",
			},
		},
		"Sleep 1 minute": {
			ExecName: "sample_sleep",
			ExecExtraArgs: []string{
				"sleep:60",
			},
			ExecTimeout:            &durationpb.Duration{Seconds: 70},
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Used Inodes percentage on stateful partition is lower than 50%": {
			Docs: []string{
				"Check Inodes useage on stateful partition and make sure it's lower than 50%",
			},
			Conditions: []string{
				"Device is SSHable",
			},
			ExecName: "cros_check_used_inode_percentage_lower_than_threshold",
			ExecExtraArgs: []string{
				"targetPath:/mnt/stateful_partition",
				"threshold:50",
			},
			RecoveryActions: []string{
				"Powerwash repair labstation",
			},
		},
	}
	for k, v := range beforeLogActions {
		if _, ok := actions[k]; ok {
			panic(fmt.Sprintf("Attempt to add duplicate action: %q", k))
		}
		actions[k] = v
	}
	for k, v := range afterLogActions {
		if _, ok := actions[k]; ok {
			panic(fmt.Sprintf("Attempt to add duplicate action: %q", k))
		}
		actions[k] = v
	}
	addStateActions(actions)
	return &Configuration{
		PlanNames: []string{
			PlanCrOS,
		},
		Plans: map[string]*Plan{
			PlanCrOS: {
				AllowFail:       false,
				CriticalActions: criticalActions,
				Actions:         actions,
			},
		},
	}
}

// Provide action name and actions to collect logs.
func labstationCollectionLogs(folderName string) (name string, actions map[string]*Action) {
	name = fmt.Sprintf("Collect logs (%s)", folderName)
	logFilename := fmt.Sprintf("log_collection_info_%s", folderName)
	actions = map[string]*Action{
		name: {
			Docs: []string{
				"Collect any pre-existing logs from before deletes such logs.",
				"Any logs collection are not critical, and we marks ",
				"that action attempt to perform to avoid repeating it.",
				fmt.Sprintf("Collection to folder: %s.", folderName),
			},
			Conditions: []string{
				"Device is SSHable",
				fmt.Sprintf("Confirm log collection info does not exist (%s)", folderName),
			},
			Dependencies: []string{
				fmt.Sprintf("Create log collection info (%s)", folderName),
				fmt.Sprintf("Copy messages (%s)", folderName),
				fmt.Sprintf("Copy eventlog.txt (%s)", folderName),
				fmt.Sprintf("Collect dmesg (%s)", folderName),
			},
			ExecName:               "sample_pass",
			RunControl:             RunControl_RUN_ONCE,
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		fmt.Sprintf("Create log collection info (%s)", folderName): {
			Docs: []string{
				"When the log collection completes, we create an info file that ",
				"indicates the successful completion of the collection process.",
			},
			Conditions: []string{
				fmt.Sprintf("Confirm log collection info does not exist (%s)", folderName),
			},
			ExecName: "cros_create_log_collection_info",
			ExecExtraArgs: []string{
				fmt.Sprintf("info_file:%s", logFilename),
			},
			RunControl:    RunControl_RUN_ONCE,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		fmt.Sprintf("Confirm log collection info does not exist (%s)", folderName): {
			Docs: []string{
				"Need to check whether the log collection info file already ",
				"exists in the file system. A pre-existing file indicates that ",
				"the collection of any pre-existing logs has already been ",
				"tried to be collected.",
			},
			ExecName: "cros_confirm_file_not_exists",
			ExecExtraArgs: []string{
				fmt.Sprintf("target_file:%s", logFilename),
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		fmt.Sprintf("Collect dmesg (%s)", folderName): {
			Docs: []string{
				"Collect the dmesg output.",
				fmt.Sprintf("Collection to folder: %s.", folderName),
			},
			ExecName: "cros_dmesg",
			ExecExtraArgs: []string{
				"human_readable:false",
				"device_type:dut",
				fmt.Sprintf("custom_dir:%s", folderName),
			},
			RunControl:             RunControl_RUN_ONCE,
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		fmt.Sprintf("Copy messages (%s)", folderName): {
			Docs: []string{
				"Try to collect /var/log/messages.",
				fmt.Sprintf("Collection to folder: %s.", folderName),
			},
			ExecName: "cros_copy_to_logs",
			ExecExtraArgs: []string{
				"src_host_type:dut",
				"src_path:/var/log/messages",
				"src_type:file",
				fmt.Sprintf("custom_dir:%s", folderName),
			},
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			AllowFailAfterRecovery: true,
		},
		fmt.Sprintf("Copy eventlog.txt (%s)", folderName): {
			Docs: []string{
				"Try to collect /var/log/eventlog.txt.",
				fmt.Sprintf("Collection to folder: %s.", folderName),
			},
			ExecName: "cros_copy_to_logs",
			ExecExtraArgs: []string{
				"src_host_type:dut",
				"src_path:/var/log/eventlog.txt",
				"src_type:file",
				fmt.Sprintf("custom_dir:%s", folderName),
			},
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			AllowFailAfterRecovery: true,
		},
	}
	return
}
