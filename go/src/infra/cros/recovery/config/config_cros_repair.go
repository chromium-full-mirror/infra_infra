// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"fmt"

	"google.golang.org/protobuf/types/known/durationpb"

	"go.chromium.org/infra/cros/recovery/internal/gbb"
)

func crosRepairPlan() *Plan {
	return &Plan{
		CriticalActions: crosRepairCriticalActions(),
		Actions:         crosRepairActions(),
	}
}

func mhRepairPlan() *Plan {
	return &Plan{
		CriticalActions: []string{
			"Mark as Android based",
			"Set CacheService address",
			"Has repair-request for re-image USB-key",
			"Has repair-request for reflash-firmware",
			"Android: Has repair-request for re-image by USB-key",
			"Android:Device is pingable",
			"ADB Connect DUT",
			"Android is accessable",
			"ADB set Android as always awake",
			"Read bootId",
			"Device Uptime",
			"Android: Has repair-request for re-provision",
			"Reset provisioned info",
			"Verify that DUT is not in DEV mode",
			"Verify that DUT has default GBB flags",
			"Missing HWID",
			"Match HWID",
			"Reset provisioned info",
			"All repair-requests resolved",
			"Reset DUT-state reason",
			"Servo is in WORKING state",
			"Set state: ready",
		},
		Actions: crosRepairActions(),
	}
}

func crosRepairCriticalActions() []string {
	actions := []string{
		"Mark labstation as servod is in-use",
		"Has repair-request for re-image USB-key",
		"Has repair-request for reflash-firmware",
		"Has repair-request for re-image by USB-key",
		"Device is pingable",
		"Mark DUT as Android if applicable",
		"Chrome OS checks",
		"Android OS checks",
		"All repair-requests resolved",
		"Reset DUT-state reason",
		"Servo is in WORKING state",
		"Set state: ready",
	}
	return actions
}

func crosRepairActions() map[string]*Action {
	actions := map[string]*Action{
		"Chrome OS checks": {
			Docs: []string{
				"Run DUT readiness checks for Chrome based DUTs.",
			},
			Conditions: []string{
				"Is ChromeOS based?",
			},
			Dependencies: []string{
				"Mark as ChromeOS based",
				// Clear provisioning to prevent Android recovery.
				"Reset provisioned info",
				"Enable verbose network logging for cellular DUTs",
				"Collect logs and crashinfo",
				"Device is SSHable",
				"Read bootId",
				"Device Uptime",
				"Verify access to cache",
				"Set dev_boot_usb is enabled",
				"Internal storage is responsive",
				"Kernel does not know issues",
				"Stateful partition has enough free index nodes",
				"Stateful partition has enough free space",
				"Stateful partition (encrypted) has enough free space",
				"Verify if booted from priority kernel",
				"Verify rootfs is on fs-verity",
				"Check KVM is enabled",
				"Has repair-request for re-provision",
				"Check if last provision was good",
				"Read OS version",
				"Python is present",
				"Verify that device is not enrolled",
				"Check power sources",
				"Check TPM statuses",
				"Read TPM ownership",
				"Verify tpm_fwver is updated correctly",
				"Verify tpm_kernver is updated correctly",
				"Verify present of gsctool",
				"Audit battery",
				"Audit storage (SMART only)",
				"Audit wifi",
				"Audit bluetooth",
				"Audit camera",
				"Audit cellular",
				"Audit cellular modem on non-cellular pools",
				"Stop if DUT needs replacement",
				"FWMP is overriding WP",
				"Ensure firmware is in good state",
				"RO Firmware version matches the recovery-version",
				"Verify servo keyboard firmware",
				"FPMCU is working",
				"Validate btclient adapter output",
				"FW WP is disabled",
				"Check if OS on required version for camerabox tablet",
				"Check audio latency toolkit state",
				"Collect audio beamforming type",
				"Login UI is up",
				"Can list RW VPD Keys",
				"Verify keys of RW_VPD",
				"Set VPD region:us",
				"Check VPD has value for stable_device_secret_DO_NOT_SHARE",
				"Verify RO_VPD sku_number",
				"Verify RO_VPD dsm_calib",
				"Verify RO_VPD data on DUT",
				"Verify system info",
				"Collect firmware target",
				"Collect gpu_id",
				"Collect fingerprint",
				"Update Servo NIC mac address",
				"Backup CBI",
				"Check CBI",
				"Update provisioned info",
				"Is crosid readable",
				"Update special device labels",
				"Collect dmesg logs from DUT",
				"Disable verbose network logging for cellular DUTs",
				"Verify bootId and compare",
			},
			ExecName:      "sample_pass",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_UPLOAD_ON_ERROR},
		},
		"Mark DUT as Android if applicable": {
			Docs: []string{
				"Mark DUT as Android, based on ADB responses.",
			},
			Conditions: []string{
				"Do not run on CloudBot",
				// Always reset first.
				"Mark as ChromeOS based",
				"DUT is not SSHable (simple)",
				"ADB Connect DUT",
			},
			ExecName: "cros_set_as_android_based",
		},
		"Mark as Android based": {
			Docs: []string{
				"Mark DUT as Android based.",
			},
			ExecName:               "cros_set_as_android_based",
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
		},
		"Mark as ChromeOS based": {
			Docs: []string{
				"Mark DUT as Chrome based.",
			},
			ExecName:      "cros_set_as_chrome_based",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			RunControl:    RunControl_ALWAYS_RUN,
		},
		"Is ChromeOS based?": {
			Docs: []string{
				"Check if device is marked as ChromeOS based",
			},
			ExecName:      "cros_is_not_android_based",
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_UPLOAD_ON_ERROR},
		},
		"Can become ChromeOS-based": {
			Docs: []string{
				"Check if DUT can be provisioned to ChromeOS.",
				"Devices runngin under MH is not allowed.",
			},
			Dependencies: []string{
				"Do not run on Mobile Harness box",
			},
			ExecName:      "dut_is_chromeos_hw",
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Mark labstation as servod is in-use": {
			Docs: []string{
				"Create lock file is_in_use.",
			},
			Conditions: []string{
				"Only applies to labstation",
			},
			ExecName:               "cros_create_servo_in_use",
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_UPLOAD_ON_ERROR},
		},
		"Labstation is SSHable": {
			Docs: []string{
				"Verify that labstation is SSHable.",
				"Limited to 15 seconds.",
			},
			Dependencies: []string{
				"Only applies to labstation",
			},
			ExecName: "cros_ssh",
			ExecExtraArgs: []string{
				"device_type:servo",
			},
			ExecTimeout:   &durationpb.Duration{Seconds: 15},
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_UPLOAD_ON_ERROR},
		},
		"Only applies to labstation": {
			Docs: []string{
				"Condition to check if the servohost is a labstation.",
			},
			ExecName:      "servo_host_is_labstation",
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_UPLOAD_ON_ERROR},
		},
		"Device is pingable": {
			Docs: []string{
				"Verify that device is reachable by ping.",
				"Limited to 15 seconds.",
			},
			ExecName: "cros_ping",
			ExecTimeout: &durationpb.Duration{
				Seconds: 15,
			},
			RecoveryActions: []string{
				"Cold reset by servo and wait for ping",
				"Pulse GSC_RST_L with servo and wait for ping",
				"Reset servo_v4.1 ethernet and wait for ping",
				"Power cycle DUT by RPM and wait for ping",
				"Trigger kernel panic to reset the whole board and try ssh to DUT",
				"Restore AC detection by EC console and wait for ping",
				"Install Android OS by booting from servo USB-drive",
				"Foil: Install Android OS from servo USB-drive",
				"Update FW and install Android OS from servo USB-drive",
				"Force reimage to ChromeOS in DEV mode",
				"Install ChromeOS by booting from servo USB-drive",
				"Install OS in recovery mode by booting from servo USB-drive (no storage check)",
				"Update FW from fw-image by servo and wait for boot",
				"Update fingerpprint FW from USB drive",
				"Install OS in recovery mode by booting from servo USB-drive",
				"Install OS in recovery mode by booting from servo USB-drive (Flex with AMT)",
				"Install OS in DEV mode by USB-drive",
				"Reset power using servo if booted from USB",
				"Battery cut-off by servo and wait for SSH",
				"Place reboot request for labstation",
			},
			RunControl: RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{
				// Always upload so we can track recovery.
				UploadPolicy: MetricsConfig_DEFAULT_UPLOAD_POLICY,
			},
		},
		"Android:Device is pingable": {
			Docs: []string{
				"Verify that device is pingable.",
				"Limited to 15 seconds.",
				"Only for Mobile Harness box execution.",
			},
			ExecName: "cros_ping",
			ExecTimeout: &durationpb.Duration{
				Seconds: 15,
			},
			RecoveryActions: []string{
				"Cold reset by servo and wait for ping",
				"Reset servo_v4.1 ethernet and wait for ping",
				"Install Android OS by booting from servo USB-drive",
				"Update FW and install Android OS from servo USB-drive",
				"Place reboot request for labstation",
			},
			RunControl: RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{
				// Always upload so we can track recovery.
				UploadPolicy: MetricsConfig_DEFAULT_UPLOAD_POLICY,
			},
		},
		"Device is pingable (simple)": {
			Docs: []string{
				"Verify that device is reachable by ping.",
				"Limited to 15 seconds.",
			},
			ExecName: "cros_ping",
			ExecTimeout: &durationpb.Duration{
				Seconds: 15,
			},
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Device is SSHable": {
			Docs: []string{
				"Verify that device is reachable by SSH.",
				"Limited to 15 seconds.",
			},
			ExecName:    "cros_ssh",
			ExecTimeout: &durationpb.Duration{Seconds: 15},
			RecoveryActions: []string{
				"Cold reset by servo and wait for ping",
				"Install OS in recovery mode by booting from servo USB-drive (no storage check)",
				"Update FW from fw-image by servo and wait for boot",
				"Try fake disconnect and wait to be access",
				"Update fingerpprint FW from USB drive",
				"Install ChromeOS by booting from servo USB-drive",
				"Install OS in recovery mode by booting from servo USB-drive",
				"Install OS in recovery mode by booting from servo USB-drive (Flex with AMT)",
				"Install OS in DEV mode by USB-drive",
				"Reset power using servo if booted from USB",
			},
			RunControl: RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{
				UploadPolicy: MetricsConfig_DEFAULT_UPLOAD_POLICY,
			},
		},
		"Device is SSHable (simple)": {
			Docs: []string{
				"Verify that device is reachable by SSH.",
				"Limited to 15 seconds.",
			},
			ExecName:    "cros_ssh",
			ExecTimeout: &durationpb.Duration{Seconds: 15},
			RunControl:  RunControl_ALWAYS_RUN,
		},
		"DUT is not SSHable (simple)": {
			Docs: []string{
				"Verify that device is not reachable by SSH.",
			},
			Conditions: []string{
				"Device is SSHable (simple)",
			},
			ExecName:   "sample_fail",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Device Uptime": {
			Docs: []string{
				"Print uptime of the DUT.",
			},
			ExecName:               "cros_uptime_print",
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Stop if DUT needs replacement": {
			Docs: []string{
				"Plan stopper if the DUT has state 'needs_replacement'.",
			},
			ExecName: "dut_state_match",
			ExecExtraArgs: []string{
				"state:needs_replacement",
				"invert:true",
			},
			RecoveryActions: []string{
				"Place PROVISION repair-requests",
			},
		},
		"Place PROVISION repair-requests": {
			Docs: []string{
				"Place repair-request: provision to reimage the DUT.",
				"The request will be applied when plan restarted and reached action to address it.",
			},
			ExecName: "dut_add_repair_requests",
			ExecExtraArgs: []string{
				"requests:PROVISION",
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Place REIMAGE_BY_USBKEY repair-requests": {
			Docs: []string{
				"Place repair-request: reimage the DUT from USB.",
				"The request will be applied when plan restarted and reached action to address it.",
			},
			ExecName: "dut_add_repair_requests",
			ExecExtraArgs: []string{
				"requests:REIMAGE_BY_USBKEY",
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Place REFLASH_FW repair-requests": {
			Docs: []string{
				"Place repair-request: reflash firmware of the DUT.",
				"The request will be applied when plan restarted and reached action to address it.",
			},
			ExecName: "dut_add_repair_requests",
			ExecExtraArgs: []string{
				"requests:REFLASH_FW",
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Force reimage to ChromeOS in DEV mode": {
			Docs: []string{
				"Mark DUT as Chrome based.",
				"Place repair-request: reflash firmware of the DUT and re-image by USB-drive.",
				"The request will be applied when plan restarted and reached action to address it.",
			},
			Conditions: []string{
				"Can become ChromeOS-based",
				"Is Android based by ADB or provision-info",
				"Is a Chromebook",
				"Recovery version has OS image path",
				"Is recovery-version has firmware image path",
				"Is servod running",
				"Is servo USB key detected",
			},
			Dependencies: []string{
				"Mark as ChromeOS based",
				"Mark labstation as servod is in-use",
				"Flash EC (FW) by servo (allowed failed)",
				"Sleep 120 seconds",
				"Disable software write protection via servo",
				"Flash AP (FW) by servo with GBB (dev mode + USB boot) (allowed failed)",
				"Download stable version OS image to servo usbkey if necessary (allow fail)",
				"Remove REFLASH_FW repair-request",
				"Boot DUT from USB in DEV mode",
				"Run install after boot from USB-drive",
				"Cold reset DUT by servo and wait to boot",
				"Wait to be SSHable (normal boot)",
				"Remove REIMAGE_BY_USBKEY repair-request",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Audit storage": {
			Docs: []string{
				"Audit DUT internal storage.",
			},
			Conditions: []string{
				"Is ChromeOS based?",
			},
			Dependencies: []string{
				"Device is SSHable (simple)",
				// Just to be sure that audit executed on good OS version.
				"DUT has correct cros image version",
				"Audit storage (SMART only)",
				"Audit device storage using badblocks",
			},
			ExecName: "sample_pass",
		},
		"Audit storage (SMART only)": {
			Docs: []string{
				"Quick audit internal storage by reading SMART data.",
				"The check updates storage state.",
				"The check is not critical as update storage state.",
			},
			ExecName:               "cros_audit_storage_smart",
			AllowFailAfterRecovery: true,
		},
		"Audit device storage using badblocks": {
			Docs: []string{
				"Use the badblocks command to audit the storage on the DUT",
			},
			ExecName: "cros_audit_storage_bad_blocks",
			ExecTimeout: &durationpb.Duration{
				Seconds: 6000,
			},
			ExecExtraArgs: []string{
				"badblocks_mode:auto",
				"rw_badblocks_timeout:5400",
				"ro_badblocks_timeout:3600",
			},
			AllowFailAfterRecovery: true,
		},
		"Verify system info": {
			Conditions: []string{
				"Is a Chromebook",
			},
			Dependencies: []string{
				"Default boot set as internal storage",
				"Verify that DUT is not in DEV mode",
				"Verify that DUT has default GBB flags",
				"Missing HWID",
				"Missing serial-number",
				"Match HWID",
				"Match serial-number",
			},
			ExecName:      "sample_pass",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Restore HWID from inventory": {
			Docs: []string{
				"Restoring HWID on the host from the inventory data.",
				"Using recovery from the host as flashing firmware by servo is very slow.",
				"HWID change will be checked on re-run.",
			},
			Dependencies: []string{
				"Is a Chromebook",
				"Is HWID known",
				"Device is SSHable",
				"Set HWID of the DUT from inventory",
				"Simple reboot",
				"Sleep 1s",
			},
			ExecName:               "cros_ssh",
			ExecTimeout:            &durationpb.Duration{Seconds: 150},
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
		},
		"Set HWID of the DUT from inventory": {
			Docs: []string{
				"Update HWID on the DUT by inventory data.",
				"The logic used update FW by futility can take time.",
			},
			Dependencies: []string{
				"Disable software-controlled write-protect for 'internal'",
				"Disable software-controlled write-protect for 'ec'",
			},
			ExecName:    "cros_update_hwid_from_inventory_to_host",
			ExecTimeout: &durationpb.Duration{Seconds: 240},
		},
		"Read OS version": {
			Docs: []string{
				"Read and log current OS version.",
			},
			ExecName: "cros_read_os_version",
			RecoveryActions: []string{
				"Quick provision OS",
				"Repair by powerwash",
				"Install ChromeOS by booting from servo USB-drive",
				"Install OS in recovery mode by booting from servo USB-drive",
				"Install OS in DEV mode, with force to DEV-mode",
			},
		},
		"Python is present": {
			Docs: []string{
				"Verify that device has python on it.",
			},
			Dependencies: []string{
				"Internal storage is responsive",
			},
			ExecName: "cros_has_python_interpreter_working",
			RecoveryActions: []string{
				"Quick provision OS",
				"Install OS in recovery mode by booting from servo USB-drive (no storage check)",
				"Install OS in DEV mode by USB-drive",
			},
		},
		"Check if last provision was good": {
			Docs: []string{
				"Check if last provision fail on the DUT",
			},
			Dependencies: []string{
				"Internal storage is responsive",
			},
			ExecName: "cros_is_last_provision_successful",
			RecoveryActions: []string{
				"Quick provision OS",
				"Install OS in recovery mode by booting from servo USB-drive (no storage check)",
				"Install OS in recovery mode by booting from servo USB-drive (with FW update)",
				"Install OS in recovery mode by booting from servo USB-drive (Flex with AMT)",
				"Install OS in DEV mode, with force to DEV-mode",
			},
		},
		"Check KVM is enabled": {
			Docs: []string{
				"Check KVM is enabled from KVM device path on the DUT",
			},
			Dependencies: []string{
				"Internal storage is responsive",
			},
			ExecName: "cros_run_command",
			ExecExtraArgs: []string{
				"host:dut",
				"command:ls /dev/kvm",
				"background:false",
			},
			ExecTimeout: &durationpb.Duration{
				Seconds: 15,
			},
			RecoveryActions: []string{
				"Cold reset by servo and wait for ping",
			},
			AllowFailAfterRecovery: true,
		},
		"Has repair-request for re-provision": {
			Docs: []string{
				"Check if PROVISION repair-request is present.",
			},
			ExecName: "dut_has_any_repair_requests",
			ExecExtraArgs: []string{
				"requests:PROVISION",
			},
			RecoveryActions: []string{
				// Android actions.
				"Install Android OS by booting from servo USB-drive",
				"Foil: Install Android OS from servo USB-drive",
				"Update FW and install Android OS from servo USB-drive",
				"Force reimage to ChromeOS in DEV mode",
				// Legacy actions.
				"Quick provision OS",
				"Install OS in recovery mode by booting from servo USB-drive (no storage check)",
				"Install OS in recovery mode by booting from servo USB-drive (with FW update)",
				"Install OS in recovery mode by booting from servo USB-drive (Flex with AMT)",
				"Install OS in DEV mode, with force to DEV-mode",
			},
		},
		"Android: Has repair-request for re-provision": {
			Docs: []string{
				"Check if PROVISION repair-request is present.",
			},
			ExecName: "dut_has_any_repair_requests",
			ExecExtraArgs: []string{
				"requests:PROVISION",
			},
			RecoveryActions: []string{
				"Install Android OS by booting from servo USB-drive",
				"Foil: Install Android OS from servo USB-drive",
				"Update FW and install Android OS from servo USB-drive",
			},
		},
		"Has repair-request for re-image USB-key": {
			Docs: []string{
				"Check if UPDATE_USBKEY_IMAGE repair-request is present.",
			},
			ExecName: "dut_has_any_repair_requests",
			ExecExtraArgs: []string{
				"requests:UPDATE_USBKEY_IMAGE",
			},
			RecoveryActions: []string{
				"Download stable image to USB-key",
				"Call servod to download image to USB-key",
			},
		},
		"Has repair-request for re-image by USB-key": {
			Docs: []string{
				"Check if REIMAGE_BY_USBKEY repair-request is present.",
			},
			ExecName: "dut_has_any_repair_requests",
			ExecExtraArgs: []string{
				"requests:REIMAGE_BY_USBKEY",
			},
			RecoveryActions: []string{
				"Install OS in recovery mode by booting from servo USB-drive (no storage check)",
				"Install OS in recovery mode by booting from servo USB-drive (with FW update)",
				"Install OS in recovery mode by booting from servo USB-drive (Flex with AMT)",
				"Install OS in DEV mode, with force to DEV-mode",
			},
		},
		"Android: Has repair-request for re-image by USB-key": {
			Docs: []string{
				"Check if REIMAGE_BY_USBKEY repair-request is present.",
			},
			ExecName: "dut_has_any_repair_requests",
			ExecExtraArgs: []string{
				"requests:REIMAGE_BY_USBKEY",
			},
			RecoveryActions: []string{
				"Install Android OS by booting from servo USB-drive",
				"Update FW and install Android OS from servo USB-drive",
			},
		},
		"Has repair-request for reflash-firmware": {
			Docs: []string{
				"Check if REFLASH_FW repair-request is present.",
			},
			ExecName: "dut_has_any_repair_requests",
			ExecExtraArgs: []string{
				"requests:REFLASH_FW",
			},
			RecoveryActions: []string{
				"Update FW from fw-image by servo and wait for boot",
			},
		},
		"Remove PROVISION repair-request": {
			Docs: []string{
				"Remove a PROVISION repair-request.",
			},
			ExecName: "dut_remove_repair_requests",
			ExecExtraArgs: []string{
				"requests:PROVISION",
			},
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Remove REIMAGE_BY_USBKEY repair-request": {
			Docs: []string{
				"Remove REIMAGE_BY_USBKEY and PROVISION repair-requests.",
			},
			ExecName: "dut_remove_repair_requests",
			ExecExtraArgs: []string{
				"requests:PROVISION,REIMAGE_BY_USBKEY",
			},
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Remove UPDATE_USBKEY_IMAGE repair-request": {
			Docs: []string{
				"Remove UPDATE_USBKEY_IMAGE from repair-requests.",
			},
			ExecName: "dut_remove_repair_requests",
			ExecExtraArgs: []string{
				"requests:UPDATE_USBKEY_IMAGE",
			},
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Remove REFLASH_FW repair-request": {
			Docs: []string{
				"Remove REFLASH_FW repair-requests.",
			},
			ExecName: "dut_remove_repair_requests",
			ExecExtraArgs: []string{
				"requests:REFLASH_FW",
			},
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Verify that device is not enrolled": {
			Docs: []string{
				"Verify that the device's enrollment state is clean.",
			},
			Dependencies: []string{
				"Internal storage is responsive",
			},
			ExecName: "cros_is_enrollment_in_clean_state",
			RecoveryActions: []string{
				"Cleanup the enrollment state and wait for boot",
			},
			AllowFailAfterRecovery: true,
		},
		"Cleanup the enrollment state and wait for boot": {
			Docs: []string{
				"Cleanup the enrollment state.",
				"The recovery process may fail, but it may still solve the problem.",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_enrollment_cleanup",
			ExecExtraArgs: []string{
				"repair_timeout:120",
				"clear_tpm_owner_timeout:60",
				"file_deletion_timeout:120",
				"reboot_timeout_down:120",
				"reboot_timeout_up:240",
				"tpm_timeout:150",
				"fwmp_cleanup_enabled:true",
				"fwmp_skip_error:false",
				"owner_detect_timeout:10",
				"fwmp_skip_reboot:true",
			},
			ExecTimeout:            &durationpb.Duration{Seconds: 1200},
			AllowFailAfterRecovery: true,
		},
		"Check power sources": {
			Docs: []string{"Check for the AC power, and battery charging capability."},
			Conditions: []string{
				"Is a Chromebook",
				"cros_is_not_virtual_machine",
			},
			Dependencies: []string{
				"Power is recognized by DUT",
				"Battery is changing or have accepted level",
			},
			ExecName:      "sample_pass",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Battery is changing or have accepted level": {
			Docs: []string{
				"Check the battery charging state.",
			},
			Conditions: []string{
				"cros_is_battery_expected",
				"cros_is_not_virtual_machine",
				"Battery is expected on device",
				"Battery is present on device",
			},
			ExecName: "cros_is_battery_chargable_or_good_level",
			RecoveryActions: []string{
				"Recover by disabling factory settings",
				"Power cycle DUT by RPM and wait for ping",
				"Cold reset by servo and wait for ping",
				"Repair by powerwash",
				"Install ChromeOS by booting from servo USB-drive",
				"Install OS in recovery mode by booting from servo USB-drive",
				"Install OS in DEV mode by USB-drive",
			},
		},
		"Recover by disabling factory settings": {
			Docs: []string{
				"Automated process to disable factory settings.",
			},
			Conditions: []string{
				"rpm_action_enabled",
				"Device without cros EC",
				"Battery is expected on device",
				"Battery is present on device",
			},
			Dependencies: []string{
				"Disable factory settings on the DUT",
				"Shutdown DUT by SSH",
				"Sleep 60 seconds",
				"Set RPM OFF",
				"Sleep 10 seconds",
				"Power ON DUT by servo",
				"Sleep 60 seconds",
				"Set RPM ON",
				// If servo pd used then always recover it last.
				"Set servo PD to src",
			},
			ExecName: "sample_pass",
		},
		"Set RPM OFF": {
			Conditions: []string{
				"rpm_action_enabled",
			},
			ExecName: "device_rpm_power_off",
			ExecExtraArgs: []string{
				"device_type:dut",
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Set RPM ON": {
			Conditions: []string{
				"rpm_action_enabled",
			},
			ExecName: "device_rpm_power_on",
			ExecExtraArgs: []string{
				"device_type:dut",
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Disable factory settings on the DUT": {
			Conditions: []string{
				"Device is SSHable",
			},
			ExecName: "cros_run_command",
			ExecExtraArgs: []string{
				"host:dut",
				"command:gsctool -a -F disable",
			},
		},
		"Shutdown DUT by SSH": {
			Conditions: []string{
				"Device is SSHable",
			},
			ExecName: "cros_run_command",
			ExecExtraArgs: []string{
				"host:dut",
				"command:shutdown -h now",
				"background:true",
			},
		},
		"Power is recognized by DUT": {
			Docs: []string{
				"Verify that power is recognized by the DUT as marker as online.",
			},
			ExecName: "cros_is_ac_power_connected",
			RecoveryActions: []string{
				"Power cycle DUT by RPM and wait for ping",
				"Cold reset by servo and wait for ping",
				"Repair by powerwash",
				"Install ChromeOS by booting from servo USB-drive",
				"Install OS in recovery mode by booting from servo USB-drive",
				"Install OS in DEV mode by USB-drive",
			},
		},
		"Check TPM statuses": {
			Docs: []string{
				"Verify that TPM statuses is ok.",
			},
			Conditions: []string{
				"Is a Chromebook",
				"cros_is_not_virtual_machine",
				"cros_is_tpm_present",
			},
			ExecName: "cros_is_tpm_in_good_status",
			RecoveryActions: []string{
				"ChromeOS TMP recovery (not critical)",
				"Quick provision OS",
				"Repair by powerwash",
				"Install ChromeOS by booting from servo USB-drive",
				"Install OS in recovery mode by booting from servo USB-drive",
				"Install OS in DEV mode by USB-drive",
			},
		},
		"Read TPM ownership": {
			Docs: []string{
				"That is initiate action to detect issues for b/246476353",
				"No recovery action just verify and report of issue.",
				"Verify that we can read ownership on device.",
			},
			Conditions: []string{
				"Device is SSHable",
				"Is hwsec-ownership-id expected",
			},
			ExecName: "cros_run_command",
			ExecExtraArgs: []string{
				"host:dut",
				"command:hwsec-ownership-id id",
			},
			// Action is only for analysis at this stage.
			AllowFailAfterRecovery: true,
		},
		"Print block devices of the DUT": {
			Docs: []string{
				"Lsblk is used to display details about block devices and these blocks.",
			},
			ExecName: "cros_run_command",
			ExecExtraArgs: []string{
				"host:dut",
				"command:lsblk",
			},
			// Action is only for analysis at this stage.
			AllowFailAfterRecovery: true,
			MetricsConfig: &MetricsConfig{
				UploadPolicy: MetricsConfig_SKIP_ALL,
			},
		},
		"Is hwsec-ownership-id expected": {
			Docs: []string{
				"The hwsec-ownership-id is expected from R101 version of ChromeOS on the DUT.",
			},
			Conditions: []string{
				"Device is SSHable",
			},
			ExecName: "cros_is_on_expected_version",
			ExecExtraArgs: []string{
				"min_version:101",
			},
		},
		"FWMP is overriding WP": {
			Docs: []string{
				"if FWMP is override WP then GSC can not manage it.",
				"It will be shown in gsctool -aw output, the fix is remove it.",
			},
			Conditions: []string{
				//TODO(b:231609148: Flex device don't have security chip and gsctool.
				"Is a Chromebook",
				"Device not in MP Signed AP FW pool",
				"DUT has Cr50 phase label",
				"Is gsctool present on the host",
			},
			ExecName: "cros_is_fwmp_override_wp",
			RecoveryActions: []string{
				"Cleanup the enrollment state and wait for boot",
			},
			AllowFailAfterRecovery: true,
		},
		"Ensure firmware is in good state": {
			Docs: []string{
				"Ensure that firmware is in good state.",
			},
			Conditions: []string{
				"Is a Chromebook",
				"Device not in MP Signed AP FW pool",
			},
			Dependencies: []string{
				"Internal storage is responsive",
			},
			ExecName: "cros_is_firmware_in_good_state",
			RecoveryActions: []string{
				"Fix FW on the DUT to match stable-version and wait to boot",
				"Update FW from fw-image by servo and wait for boot",
			},
		},
		"Check CBI": {
			Docs: []string{
				"Checks the CBI contents for corruption. go/cbi-auto-recovery-dd",
			},
			Conditions: []string{
				"Is a Chromebook",
				"CBI is present",
				"UFS contains CBI contents",
			},
			ExecName: "cros_cbi_contents_are_valid",
			// Realistically, this should always complete in a few seconds.
			// However, because we run mutliple ssh commands with a generous timeout
			// if all of those commands (because of a slow or bad connection)
			// take the maximum allotted time, we could timeout with
			// the default 1 minute timeout.
			ExecTimeout: &durationpb.Duration{Seconds: 180},
			// Allow fail as reads to EEPROM may sporadically fail.
			AllowFailAfterRecovery: true,
			RecoveryActions: []string{
				"Recover and Validate CBI",
			},
		},
		"Recover and Validate CBI": {
			Docs: []string{
				"Manages the CBI repair workflow.",
				"Checks that the CBI contents are valid after restoring contents",
				"from UFS as some DUTs have been observed to return 0 on attempted writes",
				"but not actually change the CBI contents stored in EEPROM",
			},
			Conditions: []string{
				"Hardware write protection is disabled",
				"CBI is present",
				"UFS contains CBI contents",
			},
			Dependencies: []string{
				"Restore CBI contents from UFS",
				"Simple reboot",
				"Wait to be SSHable (normal boot)",
				"Invalidate CBI cache",
			},
			ExecName:    "cros_cbi_contents_are_valid",
			ExecTimeout: &durationpb.Duration{Seconds: 180},
		},
		"Restore CBI contents from UFS": {
			Docs: []string{
				"Restore backup CBI contents from UFS.",
			},
			Dependencies: []string{
				"Hardware write protection is disabled",
				"CBI is present",
				"UFS contains CBI contents",
			},
			ExecName: "cros_restore_cbi_contents_from_ufs",
		},
		"CBI contents are valid": {
			Docs: []string{
				"Check if CBI contents on the DUT contain valid CBI magic and all required fields are present",
			},
			Dependencies: []string{
				"CBI is present",
			},
			ExecName:   "cros_cbi_contents_are_valid",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Invalidate CBI cache": {
			Docs: []string{
				"Invalidate the current CBI cache to ensure that any existing contents are up to date.",
			},
			Dependencies: []string{
				"CBI is present",
			},
			ExecName: "cros_invalidate_cbi_cache",
		},
		"Backup CBI": {
			Docs: []string{
				"Store CBI contents in UFS",
			},
			Conditions: []string{
				"Is a Chromebook",
				"CBI is present",
				"UFS does not contain CBI contents",
			},
			Dependencies: []string{
				"CBI contents are valid",
			},
			ExecName:               "cros_backup_cbi",
			AllowFailAfterRecovery: true,
		},
		"UFS contains CBI contents": {
			Docs: []string{
				"Check if UFS contents are stored in UFS.",
			},
			ExecName: "cros_ufs_contains_cbi_contents",
		},
		"UFS does not contain CBI contents": {
			Docs: []string{
				"Check if UFS contents are not stored in UFS.",
			},
			ExecName: "cros_ufs_does_not_contain_cbi_contents",
		},
		"CBI is present": {
			Docs: []string{
				"Check if CBI is present on the DUT (most devices manufactured after 2020 should have CBI) go/cros-board-info",
			},
			ExecName: "cros_cbi_is_present",
		},
		"Hardware write protection is disabled": {
			Docs: []string{
				"Checks if crossystem wpsw_cur is set to 0 (hardware write protection is disabled). Required before writing to CBI EEPROM or other on board EC chips.",
			},
			ExecName: "cros_is_hardware_write_protection_disabled",
		},
		"Login UI is up": {
			Docs: []string{
				"Check the command 'stop ui' won't crash the DUT.",
			},
			ExecName:    "cros_stop_start_ui",
			ExecTimeout: &durationpb.Duration{Seconds: 45},
			RecoveryActions: []string{
				"Cold reset by servo and wait for ping",
				"Quick provision OS",
				"Repair by powerwash",
				"Install OS in recovery mode by booting from servo USB-drive",
				"Install OS in DEV mode by USB-drive",
			},
		},
		"Start UI": {
			Docs: []string{
				"If the stop and start of UI timesout, it is an indication that ",
				"the UI has crashed. This action attempts to start the UI as an ",
				"attempt to recover the crashed UI.",
			},
			ExecName: "cros_start_ui",
		},
		"Verify keys of RW_VPD": {
			Docs: []string{
				"Verify that keys: 'should_send_rlz_ping', 'gbind_attribute', 'ubind_attribute' are present in vpd RW_VPD partition.",
			},
			Conditions: []string{
				"Is a Chromebook",
			},
			ExecName: "cros_are_required_rw_vpd_keys_present",
			RecoveryActions: []string{
				"Restore RW VPD Keys",
				"Quick provision OS",
				"Repair by powerwash",
				"Install ChromeOS by booting from servo USB-drive",
				"Install OS in recovery mode by booting from servo USB-drive",
			},
			AllowFailAfterRecovery: true,
		},
		"Set VPD region:us": {
			Docs: []string{
				"Set VPD region as us.",
			},
			Conditions: []string{
				"Is a Chromebook",
			},
			ExecName: "cros_set_vpd_value",
			ExecExtraArgs: []string{
				"key:region",
				"value:us",
			},
			AllowFailAfterRecovery: true,
		},
		"Check VPD has value for stable_device_secret_DO_NOT_SHARE": {
			Docs: []string{
				"Verify that value for key 'stable_device_secret_DO_NOT_SHARE' present.",
			},
			Conditions: []string{
				"Is a Chromebook",
			},
			ExecName: "cros_check_vpd_value",
			ExecExtraArgs: []string{
				"key:stable_device_secret_DO_NOT_SHARE",
			},
			RecoveryActions: []string{
				"Set random stable device secret",
			},
			AllowFailAfterRecovery: true,
		},
		"Set random stable device secret": {
			Docs: []string{
				"Set a random stable device secret in RO_VPD",
			},
			ExecName:               "cros_set_random_ro_vpd_stable_device_secret",
			AllowFailAfterRecovery: true,
		},
		"Can list RW VPD Keys": {
			Docs: []string{
				"Check whether the RW VPD keys can be listed without any errors.",
			},
			Conditions: []string{
				"Is a Chromebook",
			},
			ExecName: "cros_can_list_rw_vpd_keys",
			RecoveryActions: []string{
				"Recover from RW VPD keys listing errors",
			},
		},
		"Recover from RW VPD keys listing errors": {
			Docs: []string{
				"Check whether the RW VPD keys can be listed without any errors.",
			},
			Dependencies: []string{
				"Provision OS if needed",
				"Erase RW VPD Keys",
				"Restore RW VPD Keys",
				"Check RW VPD Keys",
			},
			ExecName: "sample_pass",
		},
		"Check RW VPD Keys": {
			Docs: []string{
				"Verify that keys: 'should_send_rlz_ping', 'gbind_attribute', 'ubind_attribute' are present in vpd RW_VPD partition.",
				"This action is not critical, and only logs any missing RW VPD keys.",
			},
			Conditions: []string{
				"Is a Chromebook",
			},
			ExecName:               "cros_are_required_rw_vpd_keys_present",
			AllowFailAfterRecovery: true,
		},
		"Device has incorrect cros image version": {
			Docs: []string{
				"Check whether the cros image version on the device is not as expected.",
			},
			Dependencies: []string{
				"Recovery version has OS image path",
				"cros_is_on_stable_version",
			},
			ExecName: "sample_fail",
		},
		"Erase RW VPD Keys": {
			Docs: []string{
				"Reset the RW VPD keys by resetting the flash memory of RW_VPD on the device.",
			},
			ExecName: "cros_run_shell_command",
			ExecExtraArgs: []string{
				"flashrom -p internal -i RW_VPD -E",
			},
		},
		"Restore RW VPD Keys": {
			Docs: []string{
				"Restore any possible RW VPD keys from the known default values.",
			},
			ExecName: "cros_restore_rw_vpd_keys",
		},
		"Verify servo keyboard firmware": {
			Conditions: []string{
				"Is a Chromebook",
				"Device not in MP Signed AP FW pool",
				"Is servod running",
				"is_servo_keyboard_image_tool_present",
			},
			Dependencies: []string{
				"servo_init_usb_keyboard",
				"lufa_keyboard_found",
			},
			ExecName: "cros_run_shell_command",
			ExecExtraArgs: []string{
				"lsusb -vv -d 03eb:2042 |grep \"Remote Wakeup\"",
			},
			RecoveryActions: []string{
				"Flash keyboard map",
			},
			AllowFailAfterRecovery: true,
		},
		"Flash keyboard map": {
			Conditions: []string{
				"Is servod running",
			},
			Dependencies: []string{
				"set_at_hwb_on",
				"set_atmega_rst_on",
				"Sleep for atmega reset",
				"set_atmega_rst_off",
				"Sleep for atmega reset",
				"set_at_hwb_off",
				"Sleep for usb present delay",
				"Check if expected Atmel chip",
				"Transfer keyboard hex",
				"Erase keyboard map",
				"Write keyboard map",
			},
			ExecName: "cros_run_shell_command",
			ExecExtraArgs: []string{
				"lsusb -vv -d 03eb:2042 |grep \"Remote Wakeup\"",
			},
			AllowFailAfterRecovery: true,
		},
		"set_at_hwb_on": {
			Docs: []string{
				"set servo's 'at_hwb' command to 'on' value.",
			},
			Dependencies: []string{
				"Setup has servo info",
			},
			ExecName: "servo_set",
			ExecExtraArgs: []string{
				"command:at_hwb",
				"string_value:on",
			},
		},
		"set_atmega_rst_on": {
			Docs: []string{
				"set servo's 'atmega_rst' command to 'on' value.",
			},
			Dependencies: []string{
				"Setup has servo info",
			},
			ExecName: "servo_set",
			ExecExtraArgs: []string{
				"command:atmega_rst",
				"string_value:on",
			},
		},
		"Sleep for atmega reset": {
			Docs: []string{
				"In legacy repair, the atmega reset delay in 0.2 seconds. ",
				"However, here we are being more conservative, and sleep ",
				"for 1 second.",
			},
			ExecName: "sample_sleep",
			ExecExtraArgs: []string{
				"sleep:1",
			},
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"set_atmega_rst_off": {
			Docs: []string{
				"set servo's 'atmega_rst' command to 'off' value.",
			},
			Dependencies: []string{
				"Setup has servo info",
			},
			ExecName: "servo_set",
			ExecExtraArgs: []string{
				"command:atmega_rst",
				"string_value:off",
			},
		},
		"set_at_hwb_off": {
			Docs: []string{
				"set servo's 'at_hwb' command to 'off' value.",
			},
			Dependencies: []string{
				"Setup has servo info",
			},
			ExecName: "servo_set",
			ExecExtraArgs: []string{
				"command:at_hwb",
				"string_value:off",
			},
		},
		"Sleep for usb present delay": {
			Docs: []string{
				"We need to wait for USB Present Delay, which is 1 second.",
			},
			ExecName: "sample_sleep",
			ExecExtraArgs: []string{
				"sleep:1",
			},
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Check if expected Atmel chip": {
			Docs: []string{
				"We check whether the chip is of the expected type.",
			},
			ExecName: "cros_run_shell_command",
			ExecExtraArgs: []string{
				"lsusb -d 03eb: | grep \"Atmel Corp. atmega32u4 DFU bootloader\"",
			},
			ExecTimeout: &durationpb.Duration{
				Seconds: 30,
			},
		},
		"Transfer keyboard hex": {
			Docs: []string{
				"We will transfer the keyboard hex embedded in repair package to the DUT.",
			},
			ExecName: "transfer_keyboard_hex",
		},
		"Erase keyboard map": {
			Docs: []string{
				"Erase pre-existing keyboard map.",
			},
			ExecName: "cros_run_shell_command",
			ExecExtraArgs: []string{
				"dfu-programmer atmega32u4 erase --force",
			},
			ExecTimeout: &durationpb.Duration{
				Seconds: 120,
			},
		},
		"Write keyboard map": {
			Docs: []string{
				"Write new keyboard map using the copied keyboard hex.",
			},
			ExecName: "cros_run_shell_command",
			ExecExtraArgs: []string{
				"dfu-programmer atmega32u4 flash /tmp/keyboard.hex",
			},
			ExecTimeout: &durationpb.Duration{
				Seconds: 120,
			},
		},
		"Update Servo NIC mac address": {
			Docs: []string{
				"Update mac address of servo-NIC in servod.",
			},
			Conditions: []string{
				"Setup has servo info",
				"Is a Chromebook",
				"servod_control_exist_for_mac_address",
			},
			ExecName:               "servo_audit_nic_mac_address",
			AllowFailAfterRecovery: true,
		},
		"servod_control_exist_for_mac_address": {
			Conditions: []string{
				"Setup has servo info",
			},
			ExecName: "servo_check_servod_control",
			ExecExtraArgs: []string{
				"command:macaddr",
			},
		},
		"servo_init_usb_keyboard": {
			Docs: []string{
				"set servo's 'init_usb_keyboard' command to 'on' value.",
			},
			Dependencies: []string{
				"Setup has servo info",
			},
			ExecName: "servo_set",
			ExecExtraArgs: []string{
				"command:init_usb_keyboard",
				"string_value:on",
			},
		},
		"is_servo_keyboard_image_tool_present": {
			Docs: []string{
				"check if the servo keyboard image specified by the name of dfu-programmer can be found in DUT cli.",
			},
			Dependencies: []string{
				"Setup has servo info",
			},
			ExecName: "cros_is_tool_present",
			ExecExtraArgs: []string{
				"tools:dfu-programmer",
			},
		},
		"lufa_keyboard_found": {
			Docs: []string{
				"check if the lufa keyboard can be found by finding the match of the model information of it.",
			},
			ExecName: "cros_run_shell_command",
			ExecExtraArgs: []string{
				"lsusb -d 03eb:2042 |grep \"LUFA Keyboard Demo\"",
			},
		},
		"Servo state is working": {
			Docs: []string{
				"check the servo's state is WORKING.",
			},
			ExecName: "servo_match_state",
			ExecExtraArgs: []string{
				"state:WORKING",
			},
		},
		"Servo is in WORKING state": {
			Docs: []string{
				"Some pool requires servo on WORKING state.",
				"This action does not have any recovery actions.",
			},
			Conditions: []string{
				"Pools require Servo in WORKING state",
			},
			ExecName: "servo_match_state",
			ExecExtraArgs: []string{
				"state:WORKING",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Servo-state:WORKING": {
			Docs: []string{
				"Validate that servo-state:WORKING.",
				"That is condition only",
			},
			ExecName: "servo_match_state",
			ExecExtraArgs: []string{
				"state:WORKING",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"RO Firmware version matches the recovery-version": {
			Docs: []string{
				"Check if the version of RO firmware on DUT matches the stable firmware version.",
			},
			Conditions: []string{
				"Is a Chromebook",
				"Device not in MP Signed AP FW pool",
				"Check stable firmware version exists",
				"Is recovery-version has firmware image path",
				"Pools required to manage FW on the device",
			},
			Dependencies: []string{
				"Internal storage is responsive",
			},
			ExecName: "cros_is_on_ro_firmware_stable_version",
			ExecExtraArgs: []string{
				"only_check_numbers:true",
			},
			RecoveryActions: []string{
				"Fix FW on the DUT to match stable-version and wait to boot",
				"Update FW from fw-image by servo and wait for boot",
			},
			AllowFailAfterRecovery: true,
		},
		"Fix FW on the DUT to match stable-version and wait to boot": {
			Docs: []string{
				"Update firmware from the host and reboot, then wait for host be available for SSH.",
			},
			Dependencies: []string{
				"Fix FW on the DUT to match stable-version",
				"Simple reboot",
			},
			ExecName:    "cros_ssh",
			ExecTimeout: &durationpb.Duration{Seconds: 150},
			RunControl:  RunControl_ALWAYS_RUN,
		},
		"Fix FW on the DUT to match stable-version": {
			Docs: []string{
				"Download firmware image based on stable_version and install via firmware updater from DUT",
				"Update FW required the DUT to be run on stable-version OS.",
				"The reboot is not triggered as part of the action.",
				"Set timeout to 120 minutes = 10 minutes for download+100 minutes for find and extract AP/EC images+10 minutes for run updater.",
			},
			Conditions: []string{
				"Recovery version has OS image path",
				"Is recovery-version has firmware image path",
			},
			Dependencies: []string{
				"Provision OS if needed",
				"Disable software-controlled write-protect for 'internal'",
				"Disable software-controlled write-protect for 'ec'",
			},
			ExecName:    "cros_update_firmware_from_firmware_image_on_dut",
			ExecTimeout: &durationpb.Duration{Seconds: 7200},
			ExecExtraArgs: []string{
				"mode:recovery",
				"force:true",
				"update_ec_attempt_count:1",
				"update_ap_attempt_count:1",
				"updater_timeout:600",
				"use_cache_extractor:true",
				"use_fw_targets_from_inventory:true",
			},
			// Allowed to fail as part of b/236417969 to check affect of it.
			AllowFailAfterRecovery: true,
		},
		"Call provision for DUT": {
			Docs: []string{
				"Call provision OS of the DUT.",
			},
			ExecName:    "cros_provision",
			ExecTimeout: &durationpb.Duration{Seconds: 3600},
			RunControl:  RunControl_ALWAYS_RUN,
		},
		"Provision OS if needed": {
			Docs: []string{
				"Perform provision OS if device is not running on it.",
			},
			Conditions: []string{
				"Can become ChromeOS-based",
				"Recovery version has OS image path",
				"DUT not on stable version",
			},
			Dependencies: []string{
				"Device is SSHable",
				"Call provision for DUT",
				"Remove PROVISION repair-request",
			},
			ExecName: "sample_pass",
		},
		"Verify present of gsctool": {
			Docs: []string{
				"Confirm that the GSC tool is function.",
				"Applicable only if device has Google security chip.",
			},
			Conditions: []string{
				//TODO(b:231609148: Flex device don't have security chip and gsctool.
				"Is a Chromebook",
				"DUT has Cr50 phase label",
			},
			Dependencies: []string{
				"Device is SSHable (simple)",
			},
			ExecName: "cros_run_shell_command",
			ExecExtraArgs: []string{
				"gsctool -a -f",
			},
			RecoveryActions: []string{
				"Quick provision OS",
				"Repair by powerwash",
				"Install ChromeOS by booting from servo USB-drive",
				"Install OS in recovery mode by booting from servo USB-drive",
				"Install OS in DEV mode by USB-drive",
			},
			AllowFailAfterRecovery: true,
		},
		"Audit battery": {
			Docs: []string{
				"Check battery on the DUT is normal and update battery hardware state accordingly.",
			},
			Conditions: []string{
				//TODO(b:234761994, Flex device does not have charge_full file)
				"Is a Chromebook",
				"cros_is_battery_expected",
				"cros_is_not_virtual_machine",
				"Battery is expected on device",
				"Battery is present on device",
				"Internal storage is responsive",
			},
			Dependencies: []string{
				"Battery temperature below threshold",
			},
			ExecName:               "cros_audit_battery",
			AllowFailAfterRecovery: true,
		},
		"Battery temperature below threshold": {
			Docs: []string{
				"Read and print battery_tempc control value to logs.",
				"Part of analysis of b/267384675",
			},
			Conditions: []string{
				"Not Satlab device",
				"Servo state is working",
				"Is servod running",
				"Servod has battery_tempc control",
			},
			ExecName: "servo_check_servod_control",
			ExecExtraArgs: []string{
				"command:battery_tempc",
				"expected_float_value_greater:45.0",
			},
			AllowFailAfterRecovery: true,
		},
		"Device without cros EC": {
			Docs: []string{
				"Check that is board without chromeOS EC.",
			},
			ExecName: "dut_check_board",
			ExecExtraArgs: []string{
				"string_values:drallion,sarien",
				"invert_result:false",
			},
			RunControl:    RunControl_RUN_ONCE,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Battery is expected on device": {
			Docs: []string{
				"Verifies that device is expected to have battery based on DUT info.",
			},
			ExecName: "dut_has_battery",
		},
		"Battery is present on device": {
			Docs: []string{
				"Verifies if battery present is reported as present in power supply info.",
			},
			ExecName:      "cros_is_battery_present",
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"No Battery is present on device": {
			Conditions: []string{
				"Battery is present on device",
			},
			ExecName:      "sample_fail",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Validate btclient adapter output": {
			Docs: []string{
				// See: b/405154161 for context
				"Bluetooth tests can cause DUTS to get into a bad state, requiring a reboot to reset.",
			},
			Conditions: []string{
				"Is ChromeOS based?",
				"Has Bluetooth peers",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_run_command",
			ExecExtraArgs: []string{
				"host:dut",
				"command:btclient -c \"adapter show\"",
				"background:false",
			},
			RunControl: RunControl_ALWAYS_RUN,
			RecoveryActions: []string{
				"Repair by Reboot",
				// In case normal reboot doesn't work, use a bigger hammer.
				"Cold reset by servo and wait for ping",
			},
			AllowFailAfterRecovery: true,
		},
		"Has Bluetooth peers": {
			Docs: []string{
				"Check if a device has bluetooth peers",
			},
			ExecName:      "cros_has_btpeers",
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_UPLOAD_ON_ERROR},
		},
		"Audit USB-drive from DUT": {
			Docs: []string{
				"Audit the USB drive.",
				"Run badblocks to test USB-drive from DUT side.",
				"Timeout is 1 hour.",
			},
			Conditions: []string{
				"Is ChromeOS based?",
			},
			Dependencies: []string{
				"Device is SSHable (simple)",
				"Servo state is working",
				"Is servod running",
				"Device NOT booted from USB-drive",
				"Print block devices of the DUT",
				"Check if USB-key drop connection after sleep",
			},
			ExecName: "audit_usb_from_dut_side",
			ExecExtraArgs: []string{
				// A few minutes to give time for clean up before timeout.
				"audit_timeout_min:58",
			},
			ExecTimeout: &durationpb.Duration{
				Seconds: 3600,
			},
			RecoveryActions: []string{
				// Update OS of DUT in case it provide flakiness and retry again.
				"Quick provision OS",
			},
		},
		"Audit wifi": {
			Docs: []string{
				"Check wifi on the DUT is normal and update wifi hardware state accordingly.",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_audit_wifi",
			RecoveryActions: []string{
				"Cold reset by servo and wait for ping",
			},
			AllowFailAfterRecovery: true,
		},
		"Audit bluetooth": {
			Docs: []string{
				"Check bluetooth on the DUT is normal and update bluetooth hardware state accordingly.",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_audit_bluetooth",
			ExecExtraArgs: []string{
				"cmd_timeout:30",
			},
			RecoveryActions: []string{
				"Cold reset by servo and wait for ping",
			},
			AllowFailAfterRecovery: true,
		},
		"Is in cellular pool": {
			Docs: []string{
				"Verify that DUT is not in a cellular pool.",
			},
			ExecName: "dut_is_in_pool_regex",
			ExecExtraArgs: []string{
				"regex:(?i)^cellular",
			},
			RunControl: RunControl_RUN_ONCE,
			MetricsConfig: &MetricsConfig{
				UploadPolicy: MetricsConfig_SKIP_ALL,
			},
		},
		"Is in non-cellular pool that runs cellular tests": {
			Docs: []string{
				"Verify that DUT is not in a cellular pool but is still in a pool",
				"that runs cellular tests.",
			},
			ExecName: "dut_is_in_pool_regex",
			ExecExtraArgs: []string{
				"regex:(?i)^dut_pool_quota$",
			},
			RunControl: RunControl_RUN_ONCE,
			MetricsConfig: &MetricsConfig{
				UploadPolicy: MetricsConfig_SKIP_ALL,
			},
		},
		"Is not starfish device": {
			Docs: []string{
				"Verify that DUT is not a starfish device",
			},
			Conditions: []string{
				"Is in cellular pool",
				"has_cellular_info",
			},
			ExecName: "carrier_not_in",
			ExecExtraArgs: []string{
				"carriers:STARFISH,STARFISH_PLUS",
			},
			RunControl: RunControl_RUN_ONCE,
			MetricsConfig: &MetricsConfig{
				UploadPolicy: MetricsConfig_SKIP_ALL,
			},
		},
		"Is starfish device": {
			Docs: []string{
				"Verify that DUT is a starfish device",
			},
			Conditions: []string{
				"Is in cellular pool",
				"has_cellular_info",
			},
			ExecName: "carrier_is_in",
			ExecExtraArgs: []string{
				"carriers:STARFISH,STARFISH_PLUS",
			},
			RunControl: RunControl_RUN_ONCE,
			MetricsConfig: &MetricsConfig{
				UploadPolicy: MetricsConfig_SKIP_ALL,
			},
		},
		"Has live carrier": {
			Docs: []string{
				"Verify that DUT has a connectable carrier and not a test device.",
			},
			Conditions: []string{
				"Is in cellular pool",
				"has_cellular_info",
			},
			ExecName: "carrier_not_in",
			ExecExtraArgs: []string{
				"carriers:CMW500,CMX500,PINLOCK,TESTESIM,STARFISH,STARFISHPLUS,ROAMSIM,MULTISIM,ESIM",
			},
			RunControl: RunControl_RUN_ONCE,
			MetricsConfig: &MetricsConfig{
				UploadPolicy: MetricsConfig_SKIP_ALL,
			},
		},
		"Cellular modem is not in failed state": {
			Docs: []string{
				"Verifies that the modem is in a valid state. Even if the modem",
				" hardware is fine, the modem may still be in a failed state for",
				" a variety of reasons, but commonly this is due to a missing",
				" or invalid SIM card.",
			},
			Conditions: []string{
				"Is in cellular pool",
				"cros_has_mmcli",
			},
			Dependencies: []string{
				"Device is SSHable",
				"Cellular modem is up",
			},
			ExecName: "cros_modem_state_not_in",
			ExecExtraArgs: []string{
				"modem_timeout:15",
				"states:FAILED",
			},
			MetricsConfig: &MetricsConfig{
				UploadPolicy: MetricsConfig_SKIP_ALL,
			},
		},
		"Cellular modem is up": {
			Docs: []string{
				"Check cellular modem on the DUT is normal and update cellular modem state accordingly.",
			},
			Conditions: []string{
				"Is in cellular pool",
				"cros_has_mmcli",
				"has_cellular_info",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_audit_cellular_modem",
			ExecExtraArgs: []string{
				"wait_manager_when_not_expected:120",
				"wait_manager_when_expected:15",
			},
			RecoveryActions: []string{
				"Cold reset by servo and wait for ping",
			},
			ExecTimeout: &durationpb.Duration{
				Seconds: 180,
			},
		},
		"Audit cellular modem": {
			Docs: []string{
				"Check cellular modem on the DUT is normal and update cellular modem state accordingly.",
				"Identical to 'Cellular modem is up' action but is allowed to fail",
			},
			Dependencies: []string{
				"Cellular modem is up",
			},
			ExecName:               "sample_pass",
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			AllowFailAfterRecovery: true,
		},
		"Audit cellular modem on non-cellular pools": {
			Docs: []string{
				"Check cellular modem on the DUT is normal and update cellular modem state accordingly.",
				"Differs from the normal Audit cellular modem by using a reduced timeout and no recovery actions",
				"to limit impact on quota pools.",
			},
			Conditions: []string{
				"Is in non-cellular pool that runs cellular tests",
				"cros_has_mmcli",
				"has_cellular_info",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_audit_cellular_modem",
			ExecExtraArgs: []string{
				"wait_manager_when_not_expected:15",
				"wait_manager_when_expected:15",
			},
			ExecTimeout: &durationpb.Duration{
				Seconds: 180,
			},
			AllowFailAfterRecovery: true,
		},
		"Audit cellular network connection": {
			Docs: []string{
				"Verify DUT is able to connect to the default cellular network.",
			},
			Conditions: []string{
				"Is in cellular pool",
				"cros_has_mmcli",
				"has_cellular_info",
				"Has live carrier",
			},
			Dependencies: []string{
				"Device is SSHable",
				"Cellular modem is up",
				"Cellular modem is not in failed state",
			},
			ExecName: "cros_audit_cellular_connection",
			ExecExtraArgs: []string{
				"wait_connected_timeout:40",
				"connect_attempts:3",
			},
			ExecTimeout: &durationpb.Duration{
				Seconds: 180,
			},
			AllowFailAfterRecovery: true,
		},
		"Audit cellular starfish": {
			Docs: []string{
				"Verify DUT is able to connect to the default cellular network.",
			},
			Conditions: []string{
				"Is in cellular pool",
				"Is starfish device",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName:               "cros_audit_cellular_starfish",
			AllowFailAfterRecovery: true,
		},
		"Collect additional cellular labels": {
			Docs: []string{
				"Automatically populates/constructs additional cellular labels",
				"based on manually defined labels.",
			},
			Conditions: []string{
				"Is in cellular pool",
				"has_cellular_info",
			},
			ExecName: "cros_collect_supported_carriers",
			ExecExtraArgs: []string{
				"offline_carriers:TESTESIM,ESIM,MULTISIM,ROAMSIM,PINLOCK,CMW500,CMX500",
			},
			AllowFailAfterRecovery: true,
		},
		"Audit cellular configuration": {
			Docs: []string{
				"Verifies that cellular configuration in UFS is complete and",
				"matches what is detected on the DUT.",
			},
			Conditions: []string{
				"Is in cellular pool",
				"has_cellular_info",
			},
			ExecName:               "cros_audit_cellular_configuration",
			AllowFailAfterRecovery: true,
		},
		"Audit cellular": {
			Docs: []string{
				"Audit cellular peripherals states and report metrics.",
			},
			Conditions: []string{
				"Is in cellular pool",
			},
			Dependencies: []string{
				"Audit cellular modem",
				"Update cellular sim labels if missing",
				"Update cellular modem labels if missing",
				"Audit cellular network connection",
				"Audit cellular starfish",
				"Audit cellular configuration",
				"Collect additional cellular labels",
				"Collect var/log/messages from DUT",
				"Collect var/log/net.log from DUT",
			},
			ExecName:               "sample_pass",
			AllowFailAfterRecovery: true,
		},
		"Update cellular modem labels": {
			Docs: []string{
				"Detects the modem labels in swarming.",
			},
			Conditions: []string{
				"Is in cellular pool",
				"has_cellular_info",
			},
			Dependencies: []string{
				"Cellular modem is up",
			},
			ExecName:   "cros_update_cellular_modem_labels",
			RunControl: RunControl_RUN_ONCE,
		},
		"Update cellular sim labels": {
			Docs: []string{
				"Detects the sim labels in swarming.",
			},
			Conditions: []string{
				"Is in cellular pool",
				"has_cellular_info",
				"Is not starfish device",
			},
			Dependencies: []string{
				"Cellular modem is up",
				"cros_has_only_one_sim_profile",
			},
			ExecTimeout: &durationpb.Duration{
				Seconds: 360,
			},
			ExecName:   "cros_update_cellular_sim_labels",
			RunControl: RunControl_RUN_ONCE,
		},
		"Update cellular modem labels if missing": {
			Docs: []string{
				"Audits and updates cellular modem labels only if they're not currently populated.",
			},
			Conditions: []string{
				"Is in cellular pool",
				"has_cellular_info",
				"cros_imei_empty",
			},
			Dependencies: []string{
				"Cellular modem is up",
				"Update cellular modem labels",
			},
			ExecName: "sample_pass",
		},
		"Update cellular sim labels if missing": {
			Docs: []string{
				"Audits and updates cellular SIM labels only if none are populated.",
			},
			Conditions: []string{
				"Is in cellular pool",
				"has_cellular_info",
				"cros_sim_info_empty",
			},
			Dependencies: []string{
				"Cellular modem is up",
				"Update cellular sim labels",
			},
			ExecName: "sample_pass",
		},
		"Verify tpm_fwver is updated correctly": {
			Docs: []string{
				"For dev-signed firmware, tpm_fwver reported from crossystem should always be 0x10001.",
				"Firmware update on DUTs with incorrect tpm_fwver may fail due to firmware rollback protection.",
			},
			Conditions: []string{
				"Is a Chromebook",
			},
			Dependencies: []string{
				"Internal storage is responsive",
			},
			ExecName: "cros_match_dev_tpm_firmware_version",
			RecoveryActions: []string{
				"Quick provision OS",
				"ChromeOS TMP recovery (not critical)",
				"Install ChromeOS by booting from servo USB-drive",
				"Install OS in recovery mode by booting from servo USB-drive",
				"Install OS in DEV mode by USB-drive",
				"Repair by powerwash",
			},
		},
		"Verify tpm_kernver is updated correctly": {
			Docs: []string{
				"For dev-signed firmware, tpm_kernver reported from crossystem should always be 0x10001.",
				"Firmware update on DUTs with incorrect tpm_kernver may fail due to firmware rollback protection.",
			},
			Conditions: []string{
				"Is a Chromebook",
			},
			Dependencies: []string{
				"Internal storage is responsive",
			},
			ExecName: "cros_match_dev_tpm_kernel_version",
			RecoveryActions: []string{
				"ChromeOS TMP recovery (not critical)",
				"Install ChromeOS by booting from servo USB-drive",
				"Install OS in recovery mode by booting from servo USB-drive",
				"Install OS in DEV mode by USB-drive",
			},
		},
		"ChromeOS TMP recovery (not critical)": {
			Docs: []string{
				"Run chromeos-tpm-recovery on DUT to reset TPM.",
				"That is experimental recovery action.",
			},
			ExecName: "cros_run_shell_command",
			ExecExtraArgs: []string{
				"chromeos-tpm-recovery",
			},
			AllowFailAfterRecovery: true,
		},
		"Default boot set as internal storage": {
			Docs: []string{
				"Check if the default boot drive is disk.",
			},
			Conditions: []string{
				"Is a Chromebook",
				"Is ChromeOS based?",
			},
			Dependencies: []string{
				"Internal storage is responsive",
			},
			ExecName: "cros_is_default_boot_from_disk",
			RecoveryActions: []string{
				"Set default boot as disk",
				"Quick provision OS",
				"Restore HWID from inventory",
			},
		},
		"Verify that DUT is not in DEV mode": {
			Docs: []string{
				"Verify that devices is not in DEV mode.",
				"Mostly devices in the lab required to be in Secure mode, not DEV mode.",
			},
			Conditions: []string{
				"Is a Chromebook",
				"Pools required to be in Secure mode",
			},
			ExecName: "cros_is_not_in_dev_mode",
			RecoveryActions: []string{
				"Switch to secure-mode and reboot",
				"Install Android OS by booting from servo USB-drive",
				"Foil: Install Android OS from servo USB-drive",
				"Quick provision OS",
			},
		},
		"Verify that DUT has default GBB flags": {
			Docs: []string{
				"Check if the device booted with default GBB flags.",
			},
			Conditions: []string{
				"Is a Chromebook",
				"Pools required to manage FW on the device",
			},
			ExecName: "cros_is_booted_in_secure_mode",
			RecoveryActions: []string{
				"Switch to secure-mode and reboot",
				"Install Android OS by booting from servo USB-drive",
				"Foil: Install Android OS from servo USB-drive",
				"Quick provision OS",
			},
		},
		"Is not booted in secure mode (condition)": {
			Docs: []string{
				"Check if the device is not booted in secure mode.",
			},
			Conditions: []string{
				"Booted in secure mode (condition)",
			},
			ExecName:   "sample_fail",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Booted in secure mode (condition)": {
			Docs: []string{
				"Check if the device booted in secure mode.",
			},
			ExecName:   "cros_is_booted_in_secure_mode",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Missing HWID": {
			Docs: []string{
				"Verify if device missing HWID because deployment was skipped.",
			},
			Conditions: []string{
				"Is a Chromebook",
				"Not Satlab device",
				"Is HWID empty",
			},
			Dependencies: []string{
				"Set state: needs_deploy",
			},
			ExecName:      "sample_fail",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_UPLOAD_ON_ERROR},
		},
		"Match HWID": {
			Docs: []string{
				"Match HWID to value in inventory",
				"Allowed to fail if HWID is not matched",
			},
			Conditions: []string{
				"Is a Chromebook",
				"Is HWID known",
			},
			ExecName: "cros_match_hwid_to_inventory",
			RecoveryActions: []string{
				"Restore HWID from inventory",
			},
			AllowFailAfterRecovery: true,
		},
		"Missing serial-number": {
			Docs: []string{
				"Verify if device missing serial number because deployment was skipped.",
			},
			Conditions: []string{
				"Is a Chromebook",
				"Is ChromeOS based?",
				"Not Satlab device",
				"Is serial-number empty",
			},
			Dependencies: []string{
				"Set state: needs_deploy",
			},
			ExecName: "sample_fail",
		},
		"Match serial-number": {
			Docs: []string{
				"Match serial number to value in inventory",
			},
			Conditions: []string{
				"Is a Chromebook",
				"Is ChromeOS based?",
				"Is serial-number known",
			},
			ExecName: "cros_match_serial_number_inventory",
			RecoveryActions: []string{
				"Restore serial-number",
			},
			AllowFailAfterRecovery: true,
		},
		"Restore serial-number": {
			Docs: []string{
				"Restore serial number from inventory to the host",
			},
			ExecName: "cros_restore_serial_number",
		},
		"Is HWID known": {
			Docs: []string{
				"Check whether the DUT information includes its HWID.",
			},
			ExecName:      "dut_has_hwid",
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Is HWID empty": {
			Docs: []string{
				"Check whether the DUT information includes its HWID.",
			},
			Conditions: []string{
				"Is HWID known",
			},
			ExecName:      "sample_fail",
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Is serial-number known": {
			Docs: []string{
				"Check whether the DUT information includes its ",
				"serial number.",
			},
			ExecName:      "dut_has_serial_number",
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Is serial-number empty": {
			Docs: []string{
				"Check whether the DUT information includes its ",
				"serial number.",
			},
			Conditions: []string{
				"Is serial-number known",
			},
			ExecName:      "sample_fail",
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Not Satlab device": {
			Docs: []string{
				"Verify that DUT name is not belong Satlab.",
			},
			Conditions: []string{
				"Is Satlab device",
			},
			ExecName:      "sample_fail",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Is Satlab device": {
			Docs: []string{
				"Verify that DUT name is belong Satlab.",
			},
			ExecName: "dut_regex_name_match",
			ExecExtraArgs: []string{
				"regex:^satlab",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Read storage type from DUT": {
			Docs: []string{
				"Read storage type from DUT.",
			},
			Conditions: []string{
				"Is a Chromebook",
			},
			ExecName:               "cros_update_storage_type_to_inventory",
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
		},
		"Read DUT serial-number from DUT": {
			Conditions: []string{
				"Not Satlab device",
			},
			ExecName:   "cros_update_serial_number_inventory",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Read DUT serial-number from DUT (Satlab)": {
			Conditions: []string{
				"Is Satlab device",
			},
			ExecName:               "cros_update_serial_number_inventory",
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
		},
		"Read HWID from DUT": {
			Conditions: []string{
				"Not Satlab device",
			},
			ExecName:   "cros_update_hwid_to_inventory",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Read HWID from DUT (Satlab)": {
			Conditions: []string{
				"Is Satlab device",
			},
			ExecName:               "cros_update_hwid_to_inventory",
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
		},
		"Internal storage is responsive": {
			Docs: []string{
				"Verify that internal storage is responsive",
			},
			Dependencies: []string{
				"Is ChromeOS based?",
				"Device is SSHable",
			},
			ExecName: "cros_is_file_system_writable",
			ExecExtraArgs: []string{
				"paths:/mnt/stateful_partition,/var/tmp",
			},
			RecoveryActions: []string{
				"Cold reset by servo and wait for ping",
				"Quick provision OS",
				"Repair by powerwash",
				"Install ChromeOS by booting from servo USB-drive",
				"Install OS in recovery mode by booting from servo USB-drive",
				"Install OS in DEV mode by USB-drive",
			},
		},
		"Kernel does not know issues": {
			Docs: []string{
				"Verify instrenal storage is writable.",
				"If linux has some hardware error then file system can became read-only.",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_has_critical_kernel_error",
			RecoveryActions: []string{
				"Cold reset by servo and wait for ping",
				"Quick provision OS",
				"Repair by powerwash",
				"Install ChromeOS by booting from servo USB-drive",
				"Install OS in recovery mode by booting from servo USB-drive",
				"Install OS in DEV mode by USB-drive",
			},
		},
		"Stateful partition has enough free index nodes": {
			Docs: []string{
				"Check the stateful partition path has enough index nodes.",
			},
			ExecName: "cros_has_enough_index_nodes",
			ExecExtraArgs: []string{
				"/mnt/stateful_partition:100",
			},
			RecoveryActions: []string{
				"Quick provision OS",
				"Repair by powerwash",
				"Install ChromeOS by booting from servo USB-drive",
				"Install OS in recovery mode by booting from servo USB-drive",
				"Install OS in DEV mode by USB-drive",
			},
		},
		"Stateful partition has enough free space": {
			Docs: []string{
				"Check the stateful partition have enough disk space.",
				"Expected to have free 0.1GB storage unit.",
			},
			ExecName: "cros_has_enough_storage_space",
			ExecExtraArgs: []string{
				"/mnt/stateful_partition:0.7",
			},
			RecoveryActions: []string{
				"Quick provision OS",
				"Repair by powerwash",
				"Install ChromeOS by booting from servo USB-drive",
				"Install OS in recovery mode by booting from servo USB-drive",
				"Install OS in DEV mode by USB-drive",
			},
		},
		"Stateful partition (encrypted) has enough free space": {
			Docs: []string{
				"Check the encrypted stateful partition have enough disk space.",
				"Expected to have free 0.1GB storage unit.",
			},
			ExecName: "cros_has_enough_storage_space",
			ExecExtraArgs: []string{
				"/var/tmp:0.1",
			},
			RecoveryActions: []string{
				"Quick provision OS",
				"Repair by powerwash",
				"Install ChromeOS by booting from servo USB-drive",
				"Install OS in recovery mode by booting from servo USB-drive",
				"Install OS in DEV mode by USB-drive",
			},
		},
		"Update special device labels": {
			Docs: []string{
				"Read special labels everytime as part of repair process.",
			},
			Dependencies: []string{
				"Read device SKU",
				"Read Cr50 PHASE",
				"Read Cr50 key ID",
				"Read if audio loopback present",
				"Read dlm_sku_id",
				"Update RO_VPD from DUT to Inventory",
				"Read storage type from DUT",
			},
			ExecName:      "sample_pass",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Read Cr50 PHASE": {
			Docs: []string{
				"Update the cr50 phase label from device.",
			},
			Conditions: []string{
				"Is gsctool present on the host",
			},
			ExecName:               "cros_update_cr50_label",
			AllowFailAfterRecovery: true,
		},
		"Read Cr50 key ID": {
			Docs: []string{
				"Update the cr50 key ID from device.",
			},
			Conditions: []string{
				"Is gsctool present on the host",
			},
			ExecName:               "cros_update_cr50_key_id_label",
			AllowFailAfterRecovery: true,
		},
		"Read if audio loopback present": {
			Docs: []string{
				"Update the audio_loop_back label on the cros Device.",
			},
			Conditions: []string{
				"Audio loopback state is not WORKING",
			},
			ExecName:               "cros_update_audio_loopback_state_label",
			AllowFailAfterRecovery: true,
		},
		"Audio loopback state is not WORKING": {
			Docs: []string{
				"Confirm that the DUT's audio loopback state is in not working state",
			},
			Conditions: []string{
				"cros_is_audio_loopback_state_working",
			},
			ExecName: "sample_fail",
		},
		"Is gsctool present on the host": {
			Docs: []string{
				"Checks if the cr 50 firmware exists on the DUT by running the gsctool version command.",
				"The action working as condition. Please do not exclude based on labels.",
			},
			Dependencies: []string{
				"Device is SSHable (simple)",
			},
			ExecName: "cros_run_shell_command",
			ExecExtraArgs: []string{
				"gsctool -a -f",
			},
		},
		"DUT has Cr50 phase label": {
			Docs: []string{
				"Check if the DUT has Cr50.",
			},
			ExecName: "dut_has_cr50",
		},
		"Read device SKU": {
			Docs: []string{
				"Update the device_sku label from the device if not present in inventory data.",
			},
			Conditions: []string{
				"Is a Chromebook",
				"dut_does_not_have_device_sku",
			},
			ExecName:               "cros_update_device_sku",
			AllowFailAfterRecovery: true,
		},
		"dut_does_not_have_device_sku": {
			Docs: []string{
				"Confirm that the DUT itself does not have device_sku label.",
			},
			Conditions: []string{
				"dut_has_device_sku",
			},
			ExecName: "sample_fail",
		},
		"Read dlm_sku_id": {
			Docs: []string{
				"Update the dlm_sku_id label from the device if not present in inventory data.",
			},
			Conditions: []string{
				"Is a Chromebook",
			},
			ExecName:               "cros_update_dlm_sku_id",
			AllowFailAfterRecovery: true,
		},
		"Servo USB-Key needs to be reflashed": {
			Docs: []string{
				"Check if it is time to download image to servo usbkey.",
				"If so, then download the stable image to usbkey.",
			},
			Conditions: []string{
				"It is time to update USB-drive image",
			},
			Dependencies: []string{
				"Download stable image to USB-key",
			},
			ExecName:      "sample_pass",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Stable version image is missing from servo usbkey": {
			Docs: []string{
				"This is a reverse action which fails when required image is already cached in servo usbkey.",
				"The purpose is to serve as a condition of Download stable image to USB-key action, so that we don't do duplicate download.",
				"If this action fails, it means the servo usbkey already have required stable_version OS image cached.",
			},
			Conditions: []string{
				"Servo usbkey has stable image",
			},
			ExecName: "sample_fail",
		},
		"Servo usbkey has stable image": {
			Docs: []string{
				"Check if the usbkey has the stable_version OS image.",
				"TODO: Collect data on the usual number of retries and tweak the default",
			},
			ExecName: "servo_usbkey_has_stable_image",
			ExecExtraArgs: []string{
				"retry_count:3",
				"retry_interval:1",
				"usb_file_check:true",
			},
		},
		"Download stable version OS image to servo usbkey if necessary": {
			Docs: []string{
				"This action will download model specific stable version OS image to servo usbkey.",
				"The action will be skipped if the required image is already loaded.",
			},
			Conditions: []string{
				"Setup has servo info",
				"Has a stable-version service",
				"Stable version image is missing from servo usbkey",
			},
			Dependencies: []string{
				"servo_servod_echo_host",
				"Is servo USB key detected",
				"Call servod to download image to USB-key",
				"Remove UPDATE_USBKEY_IMAGE repair-request",
			},
			ExecName:      "sample_pass",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Call servod to download image to USB-key": {
			Docs: []string{
				"This action calls servod to download stable version OS image to servo USB-key.",
			},
			ExecName:    "servo_download_image_to_usb",
			ExecTimeout: &durationpb.Duration{Seconds: 3000},
			RunControl:  RunControl_ALWAYS_RUN,
		},
		"Call servod to download provision image to USB-key": {
			Docs: []string{
				"This action calls servod to download provision image custom kernel to servo USB-key.",
			},
			ExecName: "servo_download_provision_image_to_usb",
			ExecExtraArgs: []string{
				"image_version:v6.1.1",
			},
			ExecTimeout: &durationpb.Duration{Seconds: 600},
			RunControl:  RunControl_ALWAYS_RUN,
		},
		"Is servo USB key detected": {
			Docs: []string{
				"The action used as codiion.",
				"The action verify that USB-key is detected and readable.",
			},
			Dependencies: []string{
				"Is servod running",
			},
			ExecName: "servo_usbkey_is_detected",
			ExecExtraArgs: []string{
				"file_check:true",
			},
		},
		"Check if USB-key drop connection after sleep": {
			Docs: []string{
				"The action verify that USB-key is detected and readable.",
				"The check is performed with sleep for 2 minutes to verify that USB-key would stay and be able detected",
			},
			ExecName: "servo_usbkey_is_detected",
			ExecExtraArgs: []string{
				"file_check:true",
				"check_drop_connection:true",
				"check_drop_connection_timeout:120",
			},
			ExecTimeout: &durationpb.Duration{Seconds: 200},
		},
		"Download stable version OS image to servo usbkey if necessary (allow fail)": {
			Docs: []string{
				"This action will download model specific stable version OS image to servo usbkey.",
				"The action will be skipped if the required image is already loaded.",
			},
			Conditions: []string{
				"Setup has servo info",
				"Has a stable-version service",
				"Stable version image is missing from servo usbkey",
			},
			Dependencies: []string{
				"servo_servod_echo_host",
				"Is servo USB key detected",
				"Call servod to download image to USB-key",
				"Remove UPDATE_USBKEY_IMAGE repair-request",
			},
			ExecName:               "sample_pass",
			AllowFailAfterRecovery: true,
			RunControl:             RunControl_RUN_ONCE,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Download stable image to USB-key": {
			Docs: []string{
				"Download lab stable image on servo USB-key",
				"Download the image can take longer if labstation download parallel a few images.",
				"This step is allowed to complete successfully even if some",
				" errors happen during download because the image can already",
				" be present on the USB-drive.",
			},
			Dependencies: []string{
				"Setup has servo info",
				"Is servo USB key detected",
				"Call servod to download image to USB-key",
				"Remove UPDATE_USBKEY_IMAGE repair-request",
			},
			ExecName:               "sample_pass",
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"It is time to update USB-drive image": {
			Docs: []string{
				"Check if it is time to force download image to usbkey",
				"from the number of failed recoveries since last successful PARIS repair task.",
			},
			ExecName: "cros_is_time_to_force_download_image_to_usbkey",
			ExecExtraArgs: []string{
				"task_name:recovery",
				"repair_failed_count:1",
				"repair_failed_interval:10",
			},
		},
		"Update provisioned info": {
			Docs: []string{
				"Update cros_version and job_repo_url fields of provision info.",
			},
			ExecName: "cros_update_provision_info",
			ExecExtraArgs: []string{
				"update_job_repo_url:true",
			},
			RecoveryActions: []string{
				// Sleep works as retry.
				"Sleep 1s",
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Reset provisioned info": {
			Docs: []string{
				"Reset all provision info.",
			},
			ExecName:      "cros_reset_provision_info",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			RunControl:    RunControl_ALWAYS_RUN,
		},
		"Switch to secure-mode and reboot": {
			Docs: []string{
				"This repair action utilizes the dependent actions to set the",
				" GBB flags and disable booting into dev-mode. Then it reboots",
				" the DUT.",
			},
			Conditions: []string{
				//TODO(b:231640496): flex board unpingable after switching to secure-mode.
				"Is a Chromebook",
				"Pools required to be in Secure mode",
				"Is not booted in secure mode (condition)",
			},
			Dependencies: []string{
				"Reset GBB flags by host",
				"Disables booting into DEV-mode",
				"Simple reboot",
				"Wait to be pingable (normal boot)",
				"ADB reconnect",
			},
			ExecName:    "cros_ssh",
			ExecTimeout: &durationpb.Duration{Seconds: 150},
			RunControl:  RunControl_ALWAYS_RUN,
		},
		"Reset GBB flags by host": {
			Docs: []string{
				"This action sets the GBB flags to the default value.",
			},
			Dependencies: []string{
				"Disable software-controlled write-protect for 'internal'",
				"Disable software-controlled write-protect for 'ec'",
			},
			ExecName: "cros_set_gbb_flags",
			ExecExtraArgs: []string{
				fmt.Sprintf("gbb_flags:0x%x", gbb.DefaultFlags),
			},
			ExecTimeout:            &durationpb.Duration{Seconds: 180},
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
		},
		"Disables booting into DEV-mode": {
			Docs: []string{
				"This action disables booting into dev-mode.",
			},
			ExecName:               "cros_switch_to_secure_mode",
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
		},
		"Is Flex device": {
			Docs: []string{
				"Check that DUT is a Flex board",
			},
			ExecName: "dut_check_board",
			ExecExtraArgs: []string{
				"string_values:aurora,reven",
				"invert_result:false",
			},
			RunControl:    RunControl_RUN_ONCE,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Is a Chromebook": {
			Docs: []string{
				"Check that DUT is a Chromebook by checking for non-Chromebook boards",
			},
			ExecName: "dut_check_board",
			ExecExtraArgs: []string{
				"string_values:aurora,reven",
				"invert_result:true",
			},
			RunControl:    RunControl_RUN_ONCE,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Quick provision OS": {
			Docs: []string{
				"Install stable OS on the device.",
			},
			Conditions: []string{
				"Can become ChromeOS-based",
				"Recovery version has OS image path",
				"Device is SSHable",
				"Internal storage is responsive",
			},
			Dependencies: []string{
				"Call provision for DUT",
				"Remove PROVISION repair-request",
			},
			ExecName: "sample_pass",
		},
		"Wait to be SSHable (normal boot)": {
			// No recovery actions as that is help action.
			Docs: []string{
				"Try to wait device to be sshable after the device being rebooted.",
				"Waiting time 150 seconds.",
			},
			ExecName:      "cros_ssh",
			ExecTimeout:   &durationpb.Duration{Seconds: 150},
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Wait to be SSHable (short waiting)": {
			// No recovery actions as that is help action.
			Docs: []string{
				"Try to wait device to be accesable shortly.",
				"Waiting time 30 seconds.",
			},
			ExecName:      "cros_ssh",
			ExecTimeout:   &durationpb.Duration{Seconds: 30},
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Wait to be pingable (normal boot)": {
			// No recovery actions as that is help action.
			Docs: []string{
				"Wait DUT to be pingable after some action on it.",
				"Waiting time 150 seconds.",
			},
			ExecName:      "cros_ping",
			ExecTimeout:   &durationpb.Duration{Seconds: 150},
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Trigger kernel panic to reset the whole board and try ssh to DUT": {
			Docs: []string{
				"This repair action repairs a Chrome device by sending a system request to the kernel.",
				"TODO: (blocked by: b/221083688) Collect logs from a successfully repaired DUT.",
			},
			Conditions: []string{
				"Is a Chromebook",
				"Is servod running",
			},
			Dependencies: []string{
				"Trigger kernel panic by servod",
				"Wait to be SSHable (normal boot)",
			},
			ExecName: "sample_pass",
		},
		"Trigger kernel panic by servod": {
			Docs: []string{
				"This repair action repairs a Chrome device by sending a system request to the kernel.",
			},
			Conditions: []string{
				"Is servod running",
			},
			Dependencies: []string{
				"Setup has servo info",
			},
			ExecExtraArgs: []string{
				"count:3",
				"retry_interval:2",
			},
			ExecName: "servo_trigger_kernel_panic",
		},
		"Cr50 reset by servo wait for SSH": {
			Docs: []string{
				"Repair a Chrome Device by resetting cr50 by servo.",
				"Then, using servo to initialize dut again.",
			},
			Conditions: []string{
				"Not Satlab device",
				"Is servod running",
			},
			Dependencies: []string{
				"servod_has_control_cr50_reboot",
				"Trigger power_state:cr50_reset",
				"Re-initialize DUT part of servo",
				"Wait to be SSHable (normal boot)",
			},
			ExecName: "sample_pass",
		},
		"Skip condition": {
			Docs: []string{
				"Condition which always fail to make possible to skip any action when needed.",
			},
			ExecName: "sample_fail",
		},
		"Re-initialize DUT part of servo": {
			Docs: []string{
				"cr50 reset will clear some some init like `ccd testlab open` so we want to re-initialize servo after cr50 reset if the main device uses cr50/gsc console commands.",
			},
			Conditions: []string{
				"Servo main device is GSC chip",
			},
			Dependencies: []string{
				"Setup has servo info",
				"Sleep 1s",
			},
			ExecName:    "init_dut_for_servo",
			ExecTimeout: &durationpb.Duration{Seconds: 120},
		},
		"Servo main device is GSC chip": {
			Docs: []string{
				"Verify that main device is c2d2/cr50/GSC",
			},
			Dependencies: []string{
				"Setup has servo info",
				"Only applies to labstation",
			},
			ExecName: "servo_main_device_is_gsc",
		},
		"servod_has_control_cr50_reboot": {
			Docs: []string{
				"Checks whether the servod has the command control: cr50_reboot.",
			},
			Conditions: []string{
				"Setup has servo info",
			},
			ExecExtraArgs: []string{
				"command:cr50_reboot",
			},
			ExecName: "servo_check_servod_control",
		},
		"Trigger power_state:cr50_reset": {
			Docs: []string{
				"Repair a ChromeOS Device by resetting cr50 by servo.",
			},
			Dependencies: []string{
				"Setup has servo info",
			},
			ExecExtraArgs: []string{
				"command:power_state",
				"string_value:cr50_reset",
			},
			ExecName:               "servo_set",
			AllowFailAfterRecovery: true,
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
		"Read BIOS from DUT by servo": {
			Docs: []string{
				"Read GBB flags from the DUT by servo.",
			},
			Dependencies: []string{
				"Setup has servo info",
				"Is servod running",
			},
			ExecName: "cros_read_gbb_by_servo",
			ExecExtraArgs: []string{
				"remove_file:false",
			},
			ExecTimeout: &durationpb.Duration{Seconds: 600},
		},
		"Set GBB flags to enable dev mode and boot from usb by servo": {
			Docs: []string{
				"Force to set GBB flags to boot in DEV mode and enable to boot from USB-drive.",
				"Allowed to fail as flags can applied but fail by some reason",
			},
			Dependencies: []string{
				"Is servod running",
			},
			ExecName: "cros_set_gbb_by_servo",
			ExecExtraArgs: []string{
				fmt.Sprintf("gbb_flags:0x%x", gbb.DevUsbDefault),
			},
			ExecTimeout:            &durationpb.Duration{Seconds: 300},
			AllowFailAfterRecovery: true,
			RunControl:             RunControl_ALWAYS_RUN,
		},
		"Power cycle DUT by RPM and wait for ping": {
			Docs: []string{
				"Perform RPM cycle and wait to device to boot back.",
			},
			Conditions: []string{
				"RPM config present",
				"rpm_action_enabled",
			},
			Dependencies: []string{
				"rpm_power_cycle",
				"Set servo PD to src",
				"Try cold reset DUT by servo",
				"Wait to be pingable (normal boot)",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Set servo PD to src": {
			Docs: []string{
				"Set servo PD to src to power the DUT.",
				"If servo is type-c it can switch PD to snk.",
			},
			ExecName: "servo_set",
			ExecExtraArgs: []string{
				"command:servo_pd_role",
				"string_value:src",
			},
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
		},
		"Is not in audio box": {
			Docs: []string{
				"Verify that setup is not audio box",
			},
			Conditions: []string{
				"dut_is_in_audio_box",
			},
			ExecName: "sample_fail",
		},
		"Collect dmesg logs from DUT": {
			Docs: []string{
				"Collect the entire output of dmesg",
			},
			ExecName: "cros_dmesg",
			ExecExtraArgs: []string{
				"device_type:dut",
			},
			AllowFailAfterRecovery: true,
		},
		"Restore AC detection by EC console and wait for ping": {
			Docs: []string{
				"Try to recover AC detection through servod's ec control.",
				"This action wraps the recovery action and waits for the device to come back online.",
			},
			Conditions: []string{
				"Is a Chromebook",
				"Is servod running",
				"DUT has CrOS EC",
				"cros_is_battery_expected",
			},
			Dependencies: []string{
				"Servo recover AC power",
				"Wait to be pingable (normal boot)",
			},
			ExecName: "sample_pass",
		},
		"Disable software-controlled write-protect for 'internal'": {
			Docs: []string{
				"Disable write-protect fprom 'internal'.",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_disable_fprom_write_protect",
			ExecExtraArgs: []string{
				"fprom:internal",
			},
			ExecTimeout:            &durationpb.Duration{Seconds: 300},
			AllowFailAfterRecovery: true,
			RunControl:             RunControl_ALWAYS_RUN,
		},
		"Disable software-controlled write-protect for 'ec'": {
			Docs: []string{
				"Disable write-protect fprom 'ec'.",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_disable_fprom_write_protect",
			ExecExtraArgs: []string{
				"fprom:ec",
			},
			ExecTimeout:            &durationpb.Duration{Seconds: 300},
			AllowFailAfterRecovery: true,
			RunControl:             RunControl_ALWAYS_RUN,
		},
		"Servo recover AC power": {
			Docs: []string{
				"Try to recover AC detection through servod's ec control.",
				"The DUT may take time to become pingable again,",
				"so we use a wrapper action to wait.",
			},
			Conditions: []string{
				"Is servod running",
				"Is a Chromebook",
			},
			Dependencies: []string{
				"DUT has CrOS EC",
				"cros_is_battery_expected",
			},
			ExecName: "servo_recover_ac_power",
			ExecExtraArgs: []string{
				"wait_timeout:120",
			},
			ExecTimeout: &durationpb.Duration{Seconds: 600},
		},
		"DUT has CrOS EC": {
			Docs: []string{
				"Verify if DUT has ChromeOS firmware for EC",
			},
			Dependencies: []string{
				"Is servod running",
			},
			ExecExtraArgs: []string{
				"command:supports_cros_ec_communication",
				"expected_string_value:yes",
			},
			ExecName: "servo_check_servod_control",
		},
		"Install OS in recovery mode by booting from servo USB-drive": {
			Docs: []string{
				"This action installs the test image on DUT utilizing the features of servo.",
				"DUT will be booted in recovery mode.",
			},
			Conditions: []string{
				"Can become ChromeOS-based",
				"Is a Chromebook",
				"Recovery version has OS image path",
				"Is servod running",
				"Is servo USB key detected",
			},
			Dependencies: []string{
				"Servo USB-Key needs to be reflashed",
				"Download stable version OS image to servo usbkey if necessary (allow fail)",
				"Mark as ChromeOS based",
				"Boot DUT in recovery and install from USB-drive",
				"Wait to be SSHable (normal boot)",
				"Remove REIMAGE_BY_USBKEY repair-request",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Install OS in recovery mode by booting from servo USB-drive (with FW update)": {
			Docs: []string{
				"This action installs the test image on DUT utilizing the features of servo.",
				"DUT will be booted in recovery mode.",
			},
			Conditions: []string{
				"Can become ChromeOS-based",
				"Recovery version has OS image path",
				"Is recovery-version has firmware image path",
				"Is servod running",
				"Is a Chromebook",
				"Is servo USB key detected",
			},
			Dependencies: []string{
				"Mark as ChromeOS based",
				"Flash EC (FW) by servo (allowed failed)",
				"Sleep 120 seconds",
				"Disable software write protection via servo",
				"Flash AP (FW) by servo with GBB (dev mode + USB boot)",
				"Servo USB-Key needs to be reflashed",
				"Download stable version OS image to servo usbkey if necessary (allow fail)",
				"Boot DUT in recovery and install from USB-drive",
				"Wait to be SSHable (normal boot)",
				"Remove REIMAGE_BY_USBKEY repair-request",
				"Remove REFLASH_FW repair-request",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Install OS in recovery mode by booting from servo USB-drive (no storage check)": {
			Docs: []string{
				"This action installs the test image on DUT utilizing the features of servo.",
				"DUT will be booted in recovery mode.",
			},
			Conditions: []string{
				"Can become ChromeOS-based",
				"Recovery version has OS image path",
				"Is servod running",
				"Is a Chromebook",
				"Is servo USB key detected",
			},
			Dependencies: []string{
				"Servo USB-Key needs to be reflashed",
				"Download stable version OS image to servo usbkey if necessary (allow fail)",
				"Mark as ChromeOS based",
				"Boot DUT in recovery and install from USB-drive (no storage check)",
				"Wait to be SSHable (normal boot)",
				"Remove REIMAGE_BY_USBKEY repair-request",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Install OS in recovery mode by booting from servo USB-drive (Flex with AMT)": {
			Docs: []string{
				"The action design only for Flex devices.",
				"This action installs the test image on DUT utilizing the features of servo.",
				"Uses Intel AMT (vPro) to power off and power on the DUT.",
				"When DUT sees USB-key it will always try to boot from it.",
			},
			Conditions: []string{
				"Can become ChromeOS-based",
				"Is Flex device",
				"Is servod running",
				"Is servo USB key detected",
				"Device has Intel AMT",
			},
			Dependencies: []string{
				"Mark as ChromeOS based",
				"Servo USB-Key needs to be reflashed",
				"Download stable version OS image to servo usbkey if necessary (allow fail)",
				"Direct USB-drive to DUT",
				"Power-off the DUT using AMT",
				"Sleep 1 seconds",
				"Power-on the DUT using AMT",
				"Wait to be SSHable (normal boot)",
				"Print active devices",
				"Is Flex booted from USB-drive",
				"Run chromeos-install for Flex",
				"Sleep 1 seconds",
				"Power-off the DUT using AMT",
				"Sleep 1 seconds",
				"Direct USB-drive to servo host",
				"Power-on the DUT using AMT",
				"Wait to be SSHable (normal boot)",
				"Remove REIMAGE_BY_USBKEY repair-request",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Run chromeos-install for Flex": {
			Docs: []string{
				"Run chromeos-install for Flex DUTs with detecting destination.",
				"Flex device does not detect destination as part of chromeos-install script.",
			},
			Dependencies: []string{
				"Is Flex device",
			},
			ExecName: "cros_run_command",
			ExecExtraArgs: []string{
				"host:dut",
				"command:chromeos-install --dst $(lsblk --bytes --output NAME  --paths -I 259 -n -d) --yes",
				"background:false",
			},
			ExecTimeout: &durationpb.Duration{Seconds: 600},
			RunControl:  RunControl_ALWAYS_RUN,
		},
		"Is Flex booted from USB-drive": {
			Docs: []string{
				"Check if device booted from USB in installer mode.",
			},
			Conditions: []string{
				"Is Flex device",
			},
			ExecName: "cros_run_command",
			ExecExtraArgs: []string{
				"host:dut",
				"command:is_running_from_installer |grep yes",
				"background:false",
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Print active devices": {
			Docs: []string{
				"Print active devices visible for DUT.",
			},
			ExecName: "cros_run_command",
			ExecExtraArgs: []string{
				"host:dut",
				"command:lsblk",
				"background:false",
			},
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
		},
		"Direct USB-drive to DUT": {
			Docs: []string{
				"Switch servo's USB-drive to point to DUT.",
			},
			Dependencies: []string{
				"Is servod running",
			},
			ExecName: "servo_set",
			ExecExtraArgs: []string{
				"command:image_usbkey_direction",
				"string_value:dut_sees_usbkey",
			},
			RunControl:  RunControl_ALWAYS_RUN,
			ExecTimeout: &durationpb.Duration{Seconds: 20},
		},
		"Direct USB-drive to servo host": {
			Docs: []string{
				"Switch servo's USB-drive to point to servo-host.",
			},
			Dependencies: []string{
				"Is servod running",
			},
			ExecName: "servo_set",
			ExecExtraArgs: []string{
				"command:image_usbkey_direction",
				"string_value:servo_sees_usbkey",
			},
			RunControl:  RunControl_ALWAYS_RUN,
			ExecTimeout: &durationpb.Duration{Seconds: 20},
		},
		"Download and install OS in DEV mode using USB-drive": {
			Docs: []string{
				"This action installs the test image on DUT after booking the DUT in dev mode.",
				"The action is only for deployment as not limited by pools.",
			},
			Conditions: []string{
				"Is servod running",
			},
			Dependencies: []string{
				"Download stable version OS image to servo usbkey if necessary (allow fail)",
				"Install OS in DEV mode by USB-drive",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Install OS in DEV mode by USB-drive": {
			Docs: []string{
				"This action installs the test image on DUT after booking the DUT in dev mode.",
			},
			Conditions: []string{
				"Can become ChromeOS-based",
				"Is a Chromebook",
				"Recovery version has OS image path",
				"Is servod running",
				"Is servo USB key detected",
			},
			Dependencies: []string{
				"Boot DUT from USB in DEV mode",
				"Mark as ChromeOS based",
				"Run install after boot from USB-drive",
				"Cold reset DUT by servo and wait to boot",
				"Wait to be SSHable (normal boot)",
				"Remove REIMAGE_BY_USBKEY repair-request",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Install OS in DEV mode, with force to DEV-mode": {
			Docs: []string{
				"Install OS on the device from USB-key when device is in DEV-mode.",
			},
			Conditions: []string{
				"Is servod running",
				"Is a Chromebook",
				"Is servo USB key detected",
				"Recovery version has OS image path",
			},
			Dependencies: []string{
				"Set GBB flags to enable dev mode and boot from usb by servo",
				"Install OS in DEV mode by USB-drive",
			},
			ExecName: "sample_pass",
		},
		"Cold reset DUT by servo and wait to boot": {
			Docs: []string{
				"Cold reset device by servo and wait for DUT to become ping-able.",
			},
			Conditions: []string{
				"Is servod running",
			},
			Dependencies: []string{
				"Cold reset DUT by servo",
				"Wait to be pingable (normal boot)",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Reset power using servo if booted from USB": {
			Docs: []string{
				"This action will reboot the DUT using servo if device ",
				"is not booted after off/on performed as part of ",
				"re-imaging the device from USB device.",
			},
			Conditions: []string{
				"Is servod running",
			},
			Dependencies: []string{
				"Cold reset DUT by servo",
				"Wait to be pingable (normal boot)",
			},
			ExecName: "sample_pass",
		},
		"Cold reset by servo and wait for ping": {
			Docs: []string{
				"This repair action will use servod command to reset power_state on the DUT.",
			},
			Conditions: []string{
				"Is servod running",
			},
			Dependencies: []string{
				"Cold reset DUT by servo",
				"Wait to be pingable (normal boot)",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Servo is not C2D2": {
			Docs: []string{
				"Checks if the servo has C2D2. This configuration doesn't support gsc_reset. b/319465194",
			},
			ExecName: "is_servo_c2d2",
			ExecExtraArgs: []string{
				"reverse:true",
			},
		},
		"Pulse GSC_RST_L with servo and wait for ping": {
			Docs: []string{
				"This repair action call `gsc_reset:on sleep:1 gsc_reset:off` by servo.",
				"The action works with servo_micro/c2d2 if gsc_reset present.",
			},
			Conditions: []string{
				"Is servod running",
				"servo_has_debug_header",
				"Servo is not C2D2",
				"Has gsc_reset control",
			},
			Dependencies: []string{
				"Assert GSC_RST_L by servo",
				"Sleep 1 seconds",
				"Deassert GSC_RST_L by servo",
				"Wait to be pingable (normal boot)",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Cold reset DUT by servo": {
			Docs: []string{
				"Cold reset device by servo and do not wait.",
			},
			Dependencies: []string{
				"Is servod running",
			},
			ExecName: "servo_power_state_reset",
			ExecExtraArgs: []string{
				"timeout:30",
				"more_logs:true",
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Try cold reset DUT by servo": {
			Docs: []string{
				"Try to cold-reset DUT by servo and do not wait.",
				"The action may fail if the servo is missing or unresponsive.",
			},
			Dependencies: []string{
				"Is servod running",
			},
			ExecName: "servo_power_state_reset",
			ExecExtraArgs: []string{
				"timeout:30",
				"wait_timeout:1",
				"more_logs:true",
			},
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
		},
		"Has gsc_reset control": {
			Docs: []string{
				"Read and print gsc_reset control value to logs.",
			},
			ExecName: "servod_has",
			ExecExtraArgs: []string{
				"command:gsc_reset",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Servod has battery_tempc control": {
			Docs: []string{
				"Read and print battery_tempc control value to logs.",
			},
			ExecName: "servod_has",
			ExecExtraArgs: []string{
				"command:battery_tempc",
			},
		},
		"Deassert GSC_RST_L by servo": {
			Docs: []string{
				"Release GSC from reset.",
			},
			Dependencies: []string{
				"Is servod running",
				"Has gsc_reset control",
			},
			ExecName: "servo_set",
			ExecExtraArgs: []string{
				"command:gsc_reset",
				"string_value:off",
				"timeout:10",
			},
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Assert GSC_RST_L by servo": {
			Docs: []string{
				"Assert GSC_RST_L by servo and do not wait.",
			},
			Dependencies: []string{
				"Is servod running",
				"Has gsc_reset control",
			},
			ExecName: "servo_set",
			ExecExtraArgs: []string{
				"command:gsc_reset",
				"string_value:on",
				"timeout:10",
			},
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Power OFF DUT by servo": {
			Docs: []string{
				"Turn DUT OFF by servo and do not wait.",
			},
			Dependencies: []string{
				"Is servod running",
			},
			ExecName: "servo_set",
			ExecExtraArgs: []string{
				"command:power_state",
				"string_value:off",
				"timeout:30",
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Power ON DUT by servo": {
			Docs: []string{
				"Turn DUT ON by servo and do not wait.",
			},
			Dependencies: []string{
				"Is servod running",
			},
			ExecName: "servo_set",
			ExecExtraArgs: []string{
				"command:power_state",
				"string_value:on",
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Pools required to manage FW on the device": {
			Docs: []string{
				"Verify that device we check in the pool which not required fw management.",
			},
			ExecName: "dut_not_in_pool",
			ExecExtraArgs: []string{
				"faft-test",
				"faft-test-tot",
				"faft-test-experiment",
				"faft_test_debug",
				"faft-cr50",
				"faft-cr50-debug",
				"faft-cr50-experimental",
				"faft-cr50-tot",
				"faft-experimental",
				"satlab_faft",
			},
		},
		"Pools allowed to stay in DEV mode": {
			Docs: []string{
				"Verify that pools are allowed to stay in DEV mode.",
			},
			ExecName: "dut_is_in_pool",
			ExecExtraArgs: []string{
				"crouton",
				"faft-test",
				"faft-test-au",
				"faft-test-tot",
				"nyc-meet-lab",
				"satlab_faft",
				// Device with MP AP firmware must be in dev mode to boot test OS image
				"mp_firmware_testing",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Pools require Servo in WORKING state": {
			Docs: []string{
				"List of pools that require a good servo.",
			},
			ExecName: "dut_is_in_pool",
			ExecExtraArgs: []string{
				"faft-cr50",
				"faft-cr50-debug",
				"faft-cr50-tot",
				"faft-experimental",
				"faft-test",
				"satlab_faft",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Pools required to be in Secure mode": {
			Docs: []string{
				"Verify that DUT need to be in Secure mode.",
			},
			Conditions: []string{
				"Pools allowed to stay in DEV mode",
			},
			ExecName: "sample_fail",
		},
		"Device not in MP Signed AP FW pool": {
			Docs: []string{
				"Verify that DUT is not in the pool that requires MP signed AP firmware.",
				"The firmware validation only applies to dev signed AP firmware",
				"currently. Depending on how widespread MP signed AP firmware",
				"testing is, we could add a parallel validation for MP AP firmware.",
			},
			ExecName: "dut_not_in_pool",
			ExecExtraArgs: []string{
				"mp_firmware_testing",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Set default boot as disk and reboot": {
			Docs: []string{
				"Set default boot from disk and reboot.",
			},
			Dependencies: []string{
				"Set default boot as disk",
				"Simple reboot",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Check stable firmware version exists": {
			Docs: []string{
				"Check the DUT has model specific firmware stable_version configured.",
				"Flex device are exampted from this check as they don't run cros firmware",
			},
			Conditions: []string{
				"Is a Chromebook",
				"Has a stable-version service",
			},
			ExecName: "has_stable_version_fw_version",
		},
		"Check stable faft version exists": {
			Docs: []string{
				"Check the DUT has model specific faft stable_version configured.",
				"Flex device are exampted from this check as they don't run cros firmware",
				"Satlab DUTs are exampted from this check given some early stage device don't have firmware branch GS bucket setup yet.",
			},
			Conditions: []string{
				"Is a Chromebook",
				"Not Satlab device",
				"Has a stable-version service",
			},
			ExecName: "has_stable_version_fw_image",
		},
		// TODO: Resolve duplication with another action when satlab condition resolved.
		"Is recovery-version has firmware image path": {
			Docs: []string{
				"Verify that recovery version has firmware image path.",
			},
			Dependencies: []string{
				"Has a stable-version service",
			},
			ExecName:      "has_stable_version_fw_image",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Recovery version has OS image path": {
			Docs: []string{
				"Verify that recovery version has OS image path.",
			},
			Dependencies: []string{
				"Has a stable-version service",
			},
			ExecName:      "has_stable_version_cros_image",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Recovery version misses fw-image path": {
			Docs: []string{
				"Verify that the DUT doesn't have model specific stable_version record in faft section",
			},
			Conditions: []string{
				"Has a stable-version service",
				"Is recovery-version has firmware image path",
			},
			ExecName:      "sample_fail",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Simple reboot": {
			Docs: []string{
				"Simple un-blocker reboot.",
			},
			ExecName: "cros_run_command",
			ExecExtraArgs: []string{
				"host:dut",
				"command:reboot",
				"background:true",
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Set default boot as disk": {
			Docs: []string{
				"Set default boot from disk by crossystem.",
			},
			ExecName: "cros_update_crossystem",
			ExecExtraArgs: []string{
				"command:dev_default_boot",
				"value:disk",
				"check_after_update:true",
			},
		},
		"Device NOT booted from USB-drive": {
			Docs: []string{
				"Verify that device was not booted from USB-drive.",
			},
			Conditions: []string{
				//TODO(b:231627956): Flex board cannot run crossystem set_default_boot
				"Is a Chromebook",
				"Device booted from USB-drive",
			},
			RecoveryActions: []string{
				"Set default boot as disk and reboot",
			},
			ExecName:   "sample_fail",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Device booted from USB-drive": {
			Docs: []string{
				"Verify that device was booted from USB-drive.",
			},
			ExecName:   "cros_booted_from_external_storage",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Write factory-install-reset to file system": {
			ExecName: "cros_run_shell_command",
			ExecExtraArgs: []string{
				"echo \"fast safe\" > /mnt/stateful_partition/factory_install_reset",
			},
			AllowFailAfterRecovery: true,
		},
		"Repair by powerwash": {
			Docs: []string{
				"Install the stable test image designated for the DUT.",
			},
			Conditions: []string{
				"Can become ChromeOS-based",
				"Device is SSHable",
				"Internal storage is responsive",
			},
			Dependencies: []string{
				"Write factory-install-reset to file system",
				"Simple reboot",
				"Wait to be SSHable (normal boot)",
				"Call provision for DUT",
				"Remove PROVISION repair-request",
			},
			ExecName: "sample_pass",
		},
		"Repair by Reboot": {
			Docs: []string{
				"Do simple reboot of the DUT force to update DUT cache settings.",
			},
			Conditions: []string{
				"Device is SSHable",
			},
			Dependencies: []string{
				"Simple reboot",
			},
			ExecName:    "cros_ssh",
			ExecTimeout: &durationpb.Duration{Seconds: 150},
			RunControl:  RunControl_ALWAYS_RUN,
		},
		"Flash AP (FW) with enabled serial console": {
			Docs: []string{
				"Download fw-image specified in stable version and flash AP to the DUT by servo",
				"Set timeout for 90 minutes for now as = 10m(download)+2*20m(find/extract file)+40m(ap-update with retry).",
				"We will retry up to 3 times since there may be flakiness on flash AP via servo.",
			},
			Conditions: []string{
				"Is servod running",
			},
			Dependencies: []string{
				"Is recovery-version has firmware image path",
			},
			ExecName: "cros_update_fw_with_fw_image_by_servo",
			ExecExtraArgs: []string{
				"update_ap_attempt_count:3",
				"download_timeout:600",
				fmt.Sprintf("gbb_flags:0x%x", gbb.DevUsbDefault),
				"use_cache_extractor:true",
				"use_serial_fw_target:true",
				"use_fw_targets_from_inventory:true",
			},
			ExecTimeout: &durationpb.Duration{
				Seconds: 5400,
			},
			AllowFailAfterRecovery: true,
		},
		"Flash EC (FW) by servo (allowed failed)": {
			Docs: []string{
				"Download fw-image specified in stable version and flash EC to the DUT by servo",
				"Set timeout for 110 minutes for now as = 10m(download)+4*20m(find/extract file)+20m(ec-update with retry).",
				"We will retry up to 5 times since there is flakiness on flash EC.",
			},
			Conditions: []string{
				"Is servod running",
			},
			Dependencies: []string{
				"Is recovery-version has firmware image path",
			},
			ExecName: "cros_update_fw_with_fw_image_by_servo",
			ExecExtraArgs: []string{
				"update_ec_attempt_count:5",
				"download_timeout:600",
				"use_cache_extractor:true",
				"use_fw_targets_from_inventory:true",
			},
			ExecTimeout:            &durationpb.Duration{Seconds: 6600},
			AllowFailAfterRecovery: true,
		},
		"Flash AP (FW) by servo with GBB (dev mode + USB boot)": {
			Docs: []string{
				"Download fw-image specified in stable version and flash AP to the DUT by servo",
				"Set timeout for 90 minutes for now as = 10m(download)+2*20m(find/extract file)+40m(ap-update with retry).",
				"We will retry up to 3 times since there may be flakiness on flash AP via servo.",
			},
			Conditions: []string{
				"Is servod running",
			},
			Dependencies: []string{
				"Is recovery-version has firmware image path",
			},
			ExecName: "cros_update_fw_with_fw_image_by_servo",
			ExecExtraArgs: []string{
				"update_ap_attempt_count:3",
				"download_timeout:600",
				fmt.Sprintf("gbb_flags:0x%x", gbb.DevUsbDefault),
				"use_cache_extractor:true",
				"use_fw_targets_from_inventory:true",
			},
			ExecTimeout: &durationpb.Duration{
				Seconds: 5400,
			},
			AllowFailAfterRecovery: true,
		},
		"Flash AP (FW) by servo with GBB (dev mode + USB boot) (allowed failed)": {
			Docs: []string{
				"Download fw-image specified in stable version and flash AP only to the DUT by servo",
				"Set timeout for 90 minutes for now as = 10m(download)+2*20m(find/extract file)+40m(ap-update with retry).",
				"The time will be updated later based on collected metrics",
				"Each operation with extraction files can take up to a few minutes.",
				"The AP update on the DUT can take up to 30 minutes",
			},
			Conditions: []string{
				"Is servod running",
			},
			Dependencies: []string{
				"Is recovery-version has firmware image path",
			},
			ExecName: "cros_update_fw_with_fw_image_by_servo",
			ExecExtraArgs: []string{
				"update_ec_attempt_count:0",
				"update_ap_attempt_count:3",
				"download_timeout:600",
				fmt.Sprintf("gbb_flags:0x%x", gbb.DevUsbDefault),
				"use_cache_extractor:true",
				"use_fw_targets_from_inventory:true",
			},
			ExecTimeout: &durationpb.Duration{
				Seconds: 5400,
			},
		},
		"Update FW from fw-image with factory mode from DUT": {
			Docs: []string{
				"Download firmware image to DUT, and install via firmware updater.",
				"Update firmware from faft stable image with chromeos-firmwareupdate tool",
				"--mode=facotry will be specified when run chromeos-firmwareupdate",
				"Set timeout to 120 minutes = 10 minutes for download + 100 minutes ",
				"for find and extract AP/EC images + 10 minutes for run updater.",
			},
			Conditions: []string{
				"Is recovery-version has firmware image path",
			},
			ExecName: "cros_update_firmware_from_firmware_image_on_dut",
			ExecExtraArgs: []string{
				"mode:factory",
				"force:true",
				"updater_timeout:600",
				"update_ec_attempt_count:1",
				"update_ap_attempt_count:1",
				"use_cache_extractor:true",
				"use_fw_targets_from_inventory:true",
			},
			ExecTimeout: &durationpb.Duration{Seconds: 7200},
			RunControl:  RunControl_ALWAYS_RUN,
		},
		"Update FW from host OS image with factory mode": {
			Docs: []string{
				"Run chromeos-firmwareupdate with factory mode.",
				"The reboot is not triggered as part of the action.",
				"The action is not strict to not block repair actions.",
				"Only runs when recovery-version does not have firmware ",
				"image path, e.g. it's an early stage devices.",
			},
			Conditions: []string{
				"Recovery version misses fw-image path",
			},
			ExecName: "cros_run_firmware_update",
			ExecExtraArgs: []string{
				"mode:factory",
				"force:true",
				"updater_timeout:600",
			},
			RunControl:  RunControl_ALWAYS_RUN,
			ExecTimeout: &durationpb.Duration{Seconds: 900},
		},
		"Update FW from fw-image by servo and wait for boot": {
			Docs: []string{
				"This action will repair the firmware on the DUT, and ",
				"then reboot and wait for the DUT to again become ",
				"available. This action exists to wrap these component ",
				"actions into a single repair action.",
			},
			Conditions: []string{
				"Can become ChromeOS-based",
				"Is a Chromebook",
				"Is recovery-version has firmware image path",
				"Is servod running",
			},
			Dependencies: []string{
				"Flash EC (FW) by servo (allowed failed)",
				"Sleep 120 seconds",
				"Disable software write protection via servo",
				"Flash AP (FW) by servo with GBB (dev mode + USB boot)",
				"Wait to be pingable (normal boot)",
				"Remove REFLASH_FW repair-request",
			},
			ExecName: "sample_pass",
		},
		"Boot DUT in recovery and install from USB-drive": {
			Docs: []string{
				"This action installs the test image on DUT utilizing ",
				"the features of servo. DUT will be booted in recovery ",
				"mode. In some cases RO FW is not allowed to boot in ",
				"recovery mode with active PD, so we will change it to ",
				"sink-mode if required.",
			},
			ExecName: "cros_install_in_recovery_mode",
			ExecExtraArgs: []string{
				"run_custom_commands:false",
				"set_dev_default_boot:true",
				"run_tpm_reset:true",
				"run_os_install:true",
				"boot_timeout:480",
				"boot_interval:10",
				"boot_retry:2",
				"halt_timeout:120",
				"install_timeout:1200",
				"tpm_reset_timeout:60",
				"post_install_boot_time:15",
				"ignore_reboot_failure:true",
				"badblocks_mode:auto",
				"rw_badblocks_timeout:5400",
				"ro_badblocks_timeout:3600",
				"after_reboot_check:true",
				"after_reboot_timeout:150",
				"after_reboot_allow_use_servo_reset:true",
			},
			ExecTimeout: &durationpb.Duration{Seconds: 8000},
			RunControl:  RunControl_ALWAYS_RUN,
		},
		"Boot DUT in recovery and install from USB-drive (no storage check)": {
			Docs: []string{
				"This action installs the test image on DUT utilizing ",
				"the features of servo. DUT will be booted in recovery ",
				"mode. In some cases RO FW is not allowed to boot in ",
				"recovery mode with active PD, so we will change it to ",
				"sink-mode if required.",
			},
			ExecName: "cros_install_in_recovery_mode",
			ExecExtraArgs: []string{
				"run_custom_commands:false",
				"set_dev_default_boot:true",
				"run_tpm_reset:true",
				"run_os_install:true",
				"boot_timeout:480",
				"boot_interval:10",
				"boot_retry:2",
				"halt_timeout:120",
				"install_timeout:1200",
				"tpm_reset_timeout:60",
				"post_install_boot_time:15",
				"ignore_reboot_failure:true",
				"run_storage_checks:false",
				"badblocks_mode:not",
				"rw_badblocks_timeout:5400",
				"ro_badblocks_timeout:3600",
				"after_reboot_check:true",
				"after_reboot_timeout:150",
				"after_reboot_allow_use_servo_reset:true",
			},
			ExecTimeout: &durationpb.Duration{Seconds: 8000},
			RunControl:  RunControl_ALWAYS_RUN,
		},
		"Restore FW from USB drive": {
			Docs: []string{
				"The goal to force update DUT fw when devices booted in the recovery mode from USB-stick",
			},
			ExecName: "cros_install_in_recovery_mode",
			ExecExtraArgs: []string{
				"run_custom_commands:true",
				"custom_command_allowed_to_fail:true",
				"custom_command_timeout:60",
				"custom_commands:chromeos-firmwareupdate --mode=recovery",
				"set_dev_default_boot:true",
				"run_tpm_reset:false",
				"run_os_install:false",
				"boot_timeout:480",
				"boot_interval:10",
				"boot_retry:1",
				"halt_timeout:120",
				"badblocks_mode:not",
				"ignore_reboot_failure:true",
				"after_reboot_check:true",
				"after_reboot_timeout:150",
				"after_reboot_allow_use_servo_reset:true",
			},
			ExecTimeout: &durationpb.Duration{Seconds: 1000},
			RunControl:  RunControl_ALWAYS_RUN,
		},
		"Update fingerpprint FW from USB drive": {
			Docs: []string{
				// The action runs USB-drive as devices with fp issue usually in reboot loop.
				// The action can also run when boot from USB drive with ctrl+U.
				"The goal to force update fingerprint fw when devices booted from USB-stick",
			},
			Conditions: []string{
				"Can become ChromeOS-based",
				"Is a Chromebook",
				"Is servod running",
				"Is servo USB key detected",
			},
			Dependencies: []string{
				"Set fw_wp_state to force_off",
			},
			ExecName: "cros_install_in_recovery_mode",
			ExecExtraArgs: []string{
				"run_custom_commands:true",
				"custom_command_allowed_to_fail:true",
				"custom_command_timeout:60",
				"custom_commands:flash_fp_mcu /opt/google/biod/fw/$(cros_config /fingerprint board)*.bin",
				"set_dev_default_boot:true",
				"run_tpm_reset:false",
				"run_os_install:false",
				"boot_timeout:480",
				"boot_interval:10",
				"boot_retry:1",
				"halt_timeout:120",
				"badblocks_mode:not",
				"ignore_reboot_failure:true",
				"after_reboot_check:true",
				"after_reboot_timeout:150",
				"after_reboot_allow_use_servo_reset:true",
			},
			ExecTimeout: &durationpb.Duration{Seconds: 1000},
			RunControl:  RunControl_ALWAYS_RUN,
		},
		"Set fw_wp_state to force_off": {
			Docs: []string{
				"Force disable WP of FW by servo.",
			},
			Dependencies: []string{
				"Verify servod is responsive",
			},
			ExecName: "servo_set",
			ExecExtraArgs: []string{
				"command:fw_wp_state",
				"string_value:force_off",
			},
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_UPLOAD_ON_ERROR},
		},
		"Boot DUT from USB in DEV mode": {
			Docs: []string{
				"Restart and try to boot from USB-drive",
				"First boot in dev mode can take time so set boot time to 10 minutes.",
			},
			Dependencies: []string{
				"Setup has servo info",
			},
			ExecName: "cros_dev_mode_boot_from_servo_usb_drive",
			ExecExtraArgs: []string{
				"boot_retry:2",
				"boot_timeout:600",
				"retry_interval:1",
				"verify_usbkey_boot:true",
			},
			ExecTimeout: &durationpb.Duration{Seconds: 650},
			RunControl:  RunControl_ALWAYS_RUN,
		},
		"Run install after boot from USB-drive": {
			Docs: []string{
				"Perform install process when device booted from USB-drive.",
			},
			ExecName:    "cros_run_chromeos_install_command_after_boot_usbdrive",
			ExecTimeout: &durationpb.Duration{Seconds: 1200},
			RunControl:  RunControl_ALWAYS_RUN,
		},
		"DUT not on stable version": {
			Docs: []string{
				"Confirm that DUT does not have stable version.",
			},
			ExecName: "cros_not_on_stable_version",
		},
		"RPM set state: MISSING_CONFIG": {
			ExecName: "set_rpm_state",
			ExecExtraArgs: []string{
				"device_type:dut",
				"state:MISSING_CONFIG",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"RPM set state: WRONG_CONFIG": {
			ExecName: "set_rpm_state",
			ExecExtraArgs: []string{
				"device_type:dut",
				"state:WRONG_CONFIG",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Setup has Servo PD control": {
			Docs: []string{
				"Verify that servo has build in PD control.",
			},
			Conditions: []string{
				"Setup has servo info",
			},
			ExecName: "servo_build_in_pd_present",
		},
		"Audit RPM config (without battery)": {
			Docs: []string{
				"Verify RPM configs and set RPM state for DUT without battery",
			},
			Conditions: []string{
				"rpm_action_enabled",
				"No Battery is present on device",
			},
			Dependencies: []string{
				"RPM set state: WRONG_CONFIG",
			},
			ExecName:    "rpm_audit_without_battery",
			ExecTimeout: &durationpb.Duration{Seconds: 600},
		},
		"Audit RPM config (with battery)": {
			Docs: []string{
				"Verify RPM when battery is present on the DUT.",
			},
			Conditions: []string{
				"rpm_action_enabled",
				"Battery is present on device",
			},
			Dependencies: []string{
				"RPM set state: WRONG_CONFIG",
			},
			ExecName:    "rpm_audit_with_battery",
			ExecTimeout: &durationpb.Duration{Seconds: 600},
		},
		"Verify RPM config": {
			Docs: []string{
				"Verify RPM config of DUT.",
				"Not applicable for cr50 servos based on b/205728276",
				"Action is not critical as it updates own state.",
			},
			Conditions: []string{
				"Is ChromeOS based?",
				// If rpm info is not provided then we just want to set a state and skip verification.
				"RPM set state: MISSING_CONFIG",
				"RPM config present",
			},
			Dependencies: []string{
				"Device is SSHable (simple)",
				"Audit RPM config (with battery)",
				"Audit RPM config (without battery)",
			},
			ExecName:      "sample_pass",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Is servod running": {
			Docs: []string{
				"Verify that servo host specified in setup and servod is running.",
			},
			Dependencies: []string{
				"Setup has servo info",
				"Verify servod is responsive",
			},
			ExecName:      "sample_pass",
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Setup has servo info": {
			ExecName: "dut_servo_host_present",
			MetricsConfig: &MetricsConfig{
				UploadPolicy: MetricsConfig_SKIP_ALL,
			},
		},
		"Verify servod is responsive": {
			Dependencies: []string{
				"Setup has servo info",
			},
			ExecName:    "servod_echo",
			ExecTimeout: &durationpb.Duration{Seconds: 10},
			ExecExtraArgs: []string{
				"ssh_check:false",
			},
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_UPLOAD_ON_ERROR},
		},
		"Verify RO_VPD dsm_calib": {
			Docs: []string{
				"Verify that dsm_calib is present in RO_VPD, if required.",
			},
			Conditions: []string{
				"Is a Chromebook",
				"RO_VPD dsm_calib is required",
			},
			ExecName: "cros_verify_ro_vpd_dsm_calib",
			RecoveryActions: []string{
				"Set fake RO_VPD dsm_calib",
			},
			AllowFailAfterRecovery: true,
		},
		"RO_VPD dsm_calib is required": {
			Docs: []string{
				"Verifies if RO_VPD dsm_calib is required on this device.",
			},
			ExecName: "cros_is_ro_vpd_dsm_calib_required",
		},
		"Set fake RO_VPD dsm_calib": {
			Docs: []string{
				"Set a fake dsm_calib in RO_VPD",
			},
			ExecName:               "cros_set_fake_ro_vpd_dsm_calib",
			AllowFailAfterRecovery: true,
		},
		"Verify RO_VPD sku_number": {
			Docs: []string{
				"Verify that sku_number is present in RO_VPD, if required.",
			},
			Conditions: []string{
				"Is a Chromebook",
				"RO_VPD sku_number is required",
			},
			ExecName: "cros_verify_ro_vpd_sku_number",
			RecoveryActions: []string{
				"Set fake RO_VPD sku_number",
			},
			AllowFailAfterRecovery: true,
		},
		"RO_VPD sku_number is required": {
			Docs: []string{
				"Verifies if RO_VPD sku_number is required on this device.",
			},
			ExecName: "cros_is_ro_vpd_sku_number_required",
		},
		"Set fake RO_VPD sku_number": {
			Docs: []string{
				"Set a fake sku_number in RO_VPD",
			},
			ExecName: "cros_set_fake_ro_vpd_sku_number",
			ExecExtraArgs: []string{
				"sku_number:FAKE-SKU",
			},
			AllowFailAfterRecovery: true,
		},
		"Read RO_VPD from DUT": {
			Docs: []string{
				"Record data that is present in RO_VPD of DUT to inventory.",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_update_ro_vpd_inventory",
		},
		"Update RO_VPD from DUT to Inventory": {
			Docs: []string{
				"Record data that is present in RO_VPD of DUT to inventory.",
				"Skip update values if already known by inventory.",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_update_ro_vpd_inventory",
			ExecExtraArgs: []string{
				"allow_override:false",
			},
		},
		"Verify RO_VPD data on DUT": {
			Docs: []string{
				"Verify if data from RO_VPD key: wifi_sar that was present on deploy is still present.",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_match_ro_vpd_inventory",
			RecoveryActions: []string{
				"Restore RO_VPD on DUT",
			},
		},
		"Restore RO_VPD on DUT": {
			Docs: []string{
				"Restore data from RO_VPD key: wifi_sar that was present on deploy.",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_set_ro_vpd",
		},
		"Check if OS on required version for camerabox tablet": {
			Docs: []string{
				"Check if the camerabox tablet os is the same as required version",
			},
			Dependencies: []string{
				"Internal storage is responsive",
			},
			Conditions: []string{
				"is_camerabox_tablet_pool",
			},
			ExecName: "is_camerabox_tablet_on_os_version",
			RecoveryActions: []string{
				"provision camerabox tablet",
			},
		},
		"provision camerabox tablet": {
			Docs: []string{
				"Provision camerabox tablet",
			},
			ExecName:    "provision_camerabox_tablet",
			ExecTimeout: &durationpb.Duration{Seconds: 3600},
		},
		"is_camerabox_tablet_pool": {
			Docs: []string{
				"Verify device is in camerabox_tablet pool.",
			},
			ExecName: "dut_is_in_pool",
			ExecExtraArgs: []string{
				"camerabox_tablet",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Audit camera": {
			Docs: []string{
				"Check the cameras on DUT and update camera state accordingly",
				"Timeout is 10 mins.",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_audit_camera",
			ExecTimeout: &durationpb.Duration{
				Seconds: 600,
			},
			ExecExtraArgs: []string{
				"audit_interval_hours:168", // 7*24 hours
			},
			RecoveryActions: []string{
				"Cold reset by servo and wait for ping",
			},
			AllowFailAfterRecovery: true,
		},
		"Check audio latency toolkit state": {
			Docs: []string{
				"Check the state of audio latency toolkit.",
			},
			ExecName:               "cros_update_audio_latency_toolkit_state",
			AllowFailAfterRecovery: true,
		},
		"Collect audio beamforming type": {
			Docs: []string{
				"Identify the type of beamforming in for audio",
			},
			Conditions: []string{
				"Is a Chromebook",
			},
			ExecName:               "cros_update_audio_beamforming_type",
			AllowFailAfterRecovery: true,
		},
		"Place reboot request for labstation": {
			Docs: []string{
				"Check if there's a need to reboot the connected labstation.",
				"Request labstation reboot may bring DUT back but it happens",
				"in an asynchronous task so we make this action always fail",
				"to avoid an unnecessary retry of critical actions that triggered",
				"this action, e.g. 'Device is pingable' will get retried.",
			},
			Conditions: []string{
				"Check if servo is not connected by hub",
			},
			Dependencies: []string{
				"Create request to reboot labstation",
			},
			ExecName: "sample_fail",
		},
		"Check if servo is not connected by hub": {
			Docs: []string{
				"Check if the servo is not connected by hub.",
			},
			ExecName: "servo_not_connected_by_hub",
		},
		"Create request to reboot labstation": {
			Docs: []string{
				"Try to create reboot flag file request in the connected labstation.",
			},
			ExecName:   "labstation_create_reboot_request",
			RunControl: RunControl_RUN_ONCE,
		},
		"Verify rootfs is on fs-verity": {
			Docs: []string{
				"Run rootdev to check rootfs",
			},
			Conditions: []string{
				"Is a Chromebook",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName:   "cros_verify_rootfs_verity",
			RunControl: RunControl_ALWAYS_RUN,
			RecoveryActions: []string{
				"Quick provision OS",
				"Install ChromeOS by booting from servo USB-drive",
				"Install OS in recovery mode by booting from servo USB-drive",
				"Install OS in DEV mode, with force to DEV-mode",
			},
		},
		"Set dev_boot_usb is enabled": {
			Docs: []string{
				"Set dev_boot_usb=1 to enable booting from USB drive in DEV mode.",
			},
			Conditions: []string{
				"Is a Chromebook",
			},
			ExecName: "cros_update_crossystem",
			ExecExtraArgs: []string{
				"command:dev_boot_usb",
				"value:1",
				"check_after_update:true",
			},
			RunControl:             RunControl_RUN_ONCE,
			AllowFailAfterRecovery: true,
		},
		"Collect logs and crashinfo": {
			Docs: []string{
				"We collect any pre-existing logs from before deletes such logs ",
				"on the DUT. Any logs collection are not critical, and we marks ",
				"that action attempt to perform to avoid repeating it.",
			},
			Conditions: []string{
				"Device is SSHable",
				"Confirm log collection info does not exist",
			},
			Dependencies: []string{
				"Create log collection info",
				"Collect logs from DUT on /var/log/*",
				"Collect logs from DUT on /sys/fs/pstore/*",
				"Collect dmesg",
				"Collect crash dumps",
			},
			ExecName:               "sample_pass",
			RunControl:             RunControl_RUN_ONCE,
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Collect logs from DUT on /var/log/*": {
			Docs: []string{
				"We collect any pre-existing logs before executing repairs on ",
				"the DUT. Any failures with this initial log collection are ",
				"not critical.",
			},
			Conditions: []string{
				"Device is SSHable",
			},
			ExecName: "cros_copy_to_logs",
			ExecExtraArgs: []string{
				"src_host_type:dut",
				"src_path:/var/log",
				"src_type:dir",
				"use_host_dir:true",
				"dest_suffix:prior_logs",
			},
			RunControl:             RunControl_RUN_ONCE,
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Collect logs from DUT on /sys/fs/pstore/*": {
			Docs: []string{
				"We collect kernel persistent storage file system before ",
				"executing repairs on the DUT. Any failures with this initial ",
				"log collection are not critical.",
			},
			Conditions: []string{
				"Device is SSHable",
			},
			ExecName: "cros_copy_to_logs",
			ExecExtraArgs: []string{
				"src_host_type:dut",
				"src_path:/sys/fs/pstore",
				"src_type:dir",
				"use_host_dir:true",
				"dest_suffix:prior_pstore",
			},
			RunControl:             RunControl_RUN_ONCE,
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Collect var/log/messages from DUT": {
			Docs: []string{
				"Try to copy /var/log/messages from DUT in order to monitor ",
				"system messages logged during the repair process to help ",
				"retain context even when a repair was successful.",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_copy_to_logs",
			ExecExtraArgs: []string{
				"src_host_type:dut",
				"src_path:/var/log/messages",
				"src_type:file",
				"use_host_dir:true",
			},
			RunControl:             RunControl_RUN_ONCE,
			AllowFailAfterRecovery: true,
			MetricsConfig: &MetricsConfig{
				UploadPolicy: MetricsConfig_SKIP_ALL,
			},
		},
		"Collect var/log/net.log from DUT": {
			Docs: []string{
				"Try to copy /var/log/net.log from DUT in order to monitor ",
				"system messages logged during the repair process to help ",
				"retain context even when a repair was successful.",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_copy_to_logs",
			ExecExtraArgs: []string{
				"src_host_type:dut",
				"src_path:/var/log/net.log",
				"src_type:file",
				"use_host_dir:true",
			},
			RunControl:             RunControl_RUN_ONCE,
			AllowFailAfterRecovery: true,
			MetricsConfig: &MetricsConfig{
				UploadPolicy: MetricsConfig_SKIP_ALL,
			},
		},
		"Is shill debug CLI present": {
			Docs: []string{
				"Checks if shill debug utility can be found in DUT cli.",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_is_tool_present",
			ExecExtraArgs: []string{
				"tools:ff_debug",
			},
			MetricsConfig: &MetricsConfig{
				UploadPolicy: MetricsConfig_SKIP_ALL,
			},
		},
		"Is modem CLI present": {
			Docs: []string{
				"Checks if modem utility can be found in DUT cli.",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_is_tool_present",
			ExecExtraArgs: []string{
				"tools:modem",
			},
			MetricsConfig: &MetricsConfig{
				UploadPolicy: MetricsConfig_SKIP_ALL,
			},
		},
		"Enable verbose shill logs": {
			Docs: []string{
				"Enables verbose logging of shill network manager.",
			},
			Dependencies: []string{
				"Device is SSHable",
				"Is shill debug CLI present",
			},
			ExecName: "cros_set_verbose_shill_logs",
			ExecExtraArgs: []string{
				"is_enabled:true",
			},
			AllowFailAfterRecovery: true,
			MetricsConfig: &MetricsConfig{
				UploadPolicy: MetricsConfig_SKIP_ALL,
			},
		},
		"Enable verbose ModemManager logs": {
			Docs: []string{
				"Enables verbose logging of modem manager.",
			},
			Dependencies: []string{
				"Device is SSHable",
				"Is modem CLI present",
			},
			ExecName: "cros_set_verbose_mm_logs",
			ExecExtraArgs: []string{
				"is_enabled:true",
			},
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Disable verbose shill logs": {
			Docs: []string{
				"Enables verbose logging of shill network manager.",
			},
			Dependencies: []string{
				"Device is SSHable",
				"Is shill debug CLI present",
			},
			ExecName: "cros_set_verbose_shill_logs",
			ExecExtraArgs: []string{
				"is_enabled:false",
			},
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Disable verbose ModemManager logs": {
			Docs: []string{
				"Enables verbose logging of modem manager.",
			},
			Dependencies: []string{
				"Device is SSHable",
				"Is modem CLI present",
			},
			ExecName: "cros_set_verbose_mm_logs",
			ExecExtraArgs: []string{
				"is_enabled:false",
			},
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Enable verbose network logging for cellular DUTs": {
			Docs: []string{
				"Enables verbose logging of networking daemons.",
			},
			Conditions: []string{
				"Is in cellular pool",
				"Device is SSHable",
			},
			Dependencies: []string{
				"Enable verbose shill logs",
				"Enable verbose ModemManager logs",
			},
			ExecName:               "sample_pass",
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Disable verbose network logging for cellular DUTs": {
			Docs: []string{
				"Disables verbose logging of networking daemons.",
			},
			Conditions: []string{
				"Is in cellular pool",
			},
			Dependencies: []string{
				"Disable verbose shill logs",
				"Disable verbose ModemManager logs",
			},
			ExecName:               "sample_pass",
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Collect dmesg": {
			Docs: []string{
				"We collect the dmesg output.",
			},
			Conditions: []string{
				"Device is SSHable",
			},
			ExecName: "cros_dmesg",
			ExecExtraArgs: []string{
				"human_readable:false",
				"device_type:dut",
				"create_crashinfo_dir:true",
			},
			RunControl:             RunControl_RUN_ONCE,
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Collect crash dumps": {
			Docs: []string{
				"We collect the crash dumps on the DUT. Additionally, the files ",
				"on the source are deleted, irrespective of whether the ",
				"copy-attempt completes with success or not.",
			},
			Conditions: []string{
				"Device is SSHable",
			},
			ExecName: "cros_collect_crash_dumps",
			ExecExtraArgs: []string{
				"clean:true",
				"cleanup_timeout:10",
			},
			RunControl:             RunControl_RUN_ONCE,
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Create log collection info": {
			Docs: []string{
				"When the log collection completes, we create an info file that ",
				"indicates the successful completion of the collection process.",
			},
			Conditions: []string{
				"Confirm log collection info does not exist",
			},
			ExecName: "cros_create_log_collection_info",
			ExecExtraArgs: []string{
				"info_file:log_collection_info",
			},
			RunControl:    RunControl_RUN_ONCE,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Confirm log collection info does not exist": {
			Docs: []string{
				"We need to check whether the log collection info file already ",
				"exists in the file system. A pre-existing file indicates that ",
				"the collection of any pre-existing logs has already been ",
				"tried to be collected.",
			},
			ExecName: "cros_confirm_file_not_exists",
			ExecExtraArgs: []string{
				"target_file:log_collection_info",
			},
		},
		"Battery cut-off by servo EC console": {
			Docs: []string{
				"Try to cut-off battery by servo EC console.",
				"It will force to look only to PD on the servo.",
			},
			ExecName:   "servo_set_ec_uart_cmd",
			RunControl: RunControl_ALWAYS_RUN,
			ExecExtraArgs: []string{
				"wait_timeout:1",
				"value:cutoff",
			},
		},
		"Sleep 60 seconds": {
			ExecName: "sample_sleep",
			ExecExtraArgs: []string{
				"sleep:60",
			},
			ExecTimeout:            &durationpb.Duration{Seconds: 70},
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Sleep 120 seconds": {
			ExecName: "sample_sleep",
			ExecExtraArgs: []string{
				"sleep:120",
			},
			ExecTimeout:            &durationpb.Duration{Seconds: 125},
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Sleep 1 seconds": {
			ExecName: "sample_sleep",
			ExecExtraArgs: []string{
				"sleep:1",
			},
			ExecTimeout:            &durationpb.Duration{Seconds: 2},
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Sleep 10 seconds": {
			ExecName: "sample_sleep",
			ExecExtraArgs: []string{
				"sleep:10",
			},
			ExecTimeout:            &durationpb.Duration{Seconds: 11},
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Disable software write protection via servo": {
			Docs: []string{
				"Disable software write protection(for flash firmware) via servo.",
			},
			Dependencies: []string{
				"Is servod running",
			},
			ExecName:               "cros_disable_software_write_protection_by_servo",
			ExecTimeout:            &durationpb.Duration{Seconds: 60},
			AllowFailAfterRecovery: true,
			RunControl:             RunControl_ALWAYS_RUN,
		},
		"Has a stable-version service": {
			Docs: []string{
				"Verify if we have access to the service provided access to the stable version",
			},
			ExecName:      "has_stable_version_service_path",
			RunControl:    RunControl_RUN_ONCE,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Verify if booted from priority kernel": {
			Docs: []string{
				"Kernel can wait for reboot as the last provisioning waiting for some issue.",
				"Verified if DUT's kernel doesn't waiting for update.",
			},
			Conditions: []string{
				"Device is SSHable",
			},
			ExecName: "cros_kernel_priority_has_not_changed",
			RecoveryActions: []string{
				"Simple reboot to right kernel",
				"Cold reset by servo and wait for ping",
			},
		},
		"Simple reboot to right kernel": {
			Docs: []string{
				"Simple reboot DUT to boot from right kernel.",
				"Reboot initiated from DUT side.",
			},
			Conditions: []string{
				"Device is SSHable",
			},
			Dependencies: []string{
				"Simple reboot",
				"Wait to be SSHable (normal boot)",
			},
			ExecName: "cros_kernel_priority_has_not_changed",
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
				"Is a Chromebook",
				"Device is SSHable",
				"Is crosid present",
			},
			ExecName: "cros_run_command",
			ExecExtraArgs: []string{
				"host:",
				"command:crosid",
			},
			RecoveryActions: []string{
				// First we try to reboot something already recovered but not applied yet as required reboot.
				"Repair by Reboot",
				"Quick provision OS",
				"Remove whitelabel_tag from vpd and reboot",
			},
		},
		"Remove whitelabel_tag from vpd and reboot": {
			Docs: []string{
				"Remove whitelabel_tag from vpd and reboot to apply change.",
			},
			Conditions: []string{
				"Device is SSHable",
				"Is a Chromebook",
			},
			Dependencies: []string{
				"Delete whitelabel_tag from vpd",
				"Simple reboot",
			},
			ExecName:    "cros_ssh",
			ExecTimeout: &durationpb.Duration{Seconds: 150},
			RunControl:  RunControl_ALWAYS_RUN,
		},
		"Delete whitelabel_tag from vpd": {
			Docs: []string{
				"Remove whitelabel_tagfrom vpd as it can cause issue related to crosid readability.",
			},
			Dependencies: []string{
				"Is a Chromebook",
			},
			ExecName: "cros_run_command",
			ExecExtraArgs: []string{
				"host:dut",
				"command:vpd -d whitelabel_tag",
				"background:false",
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Battery cut-off by servo and wait for SSH": {
			Docs: []string{
				"This repair action will use EC console to cut battery power and then try to restore power which force to reset the board.",
				"Logic establishe from b/277637455.",
			},
			Conditions: []string{
				"Is a Chromebook",
				"Is servod running",
				"is_servo_type_ccd",
				"DUT is G3/S5 powerstate",
			},
			Dependencies: []string{
				"Battery cut-off by servo EC console",
				"Sleep 10 seconds",
				"Try fake disconnect",
				"Sleep 60 seconds",
				"Wait to be SSHable (normal boot)",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Try fake disconnect and wait to be access": {
			Docs: []string{
				"This action will virtually disconnect servo from DUT and plug it back.",
			},
			Conditions: []string{
				"Is a Chromebook",
				"Is servod running",
				"is_servo_type_ccd",
				"DUT is G3/S5 powerstate",
			},
			Dependencies: []string{
				"Try fake disconnect",
				"Wait to be SSHable (short waiting)",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"DUT is G3/S5 powerstate": {
			Docs: []string{
				"Check if the DUT powerstate is S5 or G3 (device is off).",
			},
			ExecName: "servo_power_state_match",
			ExecExtraArgs: []string{
				"power_states:G3,S5",
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Try fake disconnect": {
			Docs: []string{
				"Run servo fake disconnect type-c connector from the DUT side.",
				"The action mimic unplug-plug connection to the DUT.",
			},
			Conditions: []string{
				"is_servo_type_ccd",
			},
			ExecName: "servo_fake_disconnect_dut",
			ExecExtraArgs: []string{
				"delay_in_ms:100",
				"timeout_in_ms:2000",
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Read bootId": {
			Docs: []string{
				"Read bootid and public to config scope.",
			},
			ExecName: "cros_read_bootid",
			ExecExtraArgs: []string{
				"skip_empty:true",
				"publish:true",
			},
			RecoveryActions: []string{
				// Sleep works as retry.
				"Sleep 1s",
			},
		},
		"Verify bootId and compare": {
			Docs: []string{
				"Read bootid and compare with bootId when we started this plan.",
			},
			ExecName: "cros_read_bootid",
			ExecExtraArgs: []string{
				"skip_empty:true",
				"compare:true",
			},
			RecoveryActions: []string{
				"Quick provision OS",
				"Install ChromeOS by booting from servo USB-drive",
				"Install OS in recovery mode by booting from servo USB-drive",
				"Install OS in DEV mode, with force to DEV-mode",
			},
		},
		"Reset servo_v4.1 ethernet and wait for ping": {
			Docs: []string{
				"This repair action will reset servo ethernet power and wait for ssh, applicable to servo_v4.1 only",
			},
			Conditions: []string{
				"Is servod running",
				"is_servo_v4p1_by_serial_number",
			},
			Dependencies: []string{
				"Reset servo_v4.1 ethernet power",
				"Wait to be pingable (normal boot)",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Reset servo_v4.1 ethernet power": {
			Docs: []string{
				"Reset servo_v4.1 ethernet power via built-in control",
			},
			Dependencies: []string{
				"Is servod running",
			},
			ExecExtraArgs: []string{
				"reset_timeout:1",
			},
			ExecName:   "servo_v4p1_network_reset",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Deep-repair ChromeOS DUT": {
			Docs: []string{
				"Force repair DUT with FW flash by servo and reimage from USB-drive in dev mode.",
				"The action doesn't use recovery boot.",
			},
			Conditions: []string{
				"Recovery version has OS image path",
				"Is recovery-version has firmware image path",
				"Is a Chromebook",
			},
			Dependencies: []string{
				"Mark labstation as servod is in-use",
				"Flash EC (FW) by servo (allowed failed)",
				"Sleep 120 seconds",
				"Disable software write protection via servo",
				"Flash AP (FW) by servo with GBB (dev mode + USB boot) (allowed failed)",
				"Download stable version OS image to servo usbkey if necessary (allow fail)",
				"Install OS in DEV mode by USB-drive",
				"Remove REFLASH_FW repair-request",
			},
			ExecName: "sample_pass",
		},
		"Deep-repair Flex DUT": {
			Docs: []string{
				"Force repair DUT with reimage from USB-drive.",
			},
			Conditions: []string{
				"Is Flex device",
			},
			Dependencies: []string{
				"Install OS in recovery mode by booting from servo USB-drive (Flex with AMT)",
			},
			ExecName:      "sample_pass",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"RPM config present": {
			Docs: []string{
				"Verifies that the RPM configuration provides some data for the hostname and outlet.",
				"This action does not verify the correctness of the data.",
			},
			ExecName: "device_has_rpm_info",
			ExecExtraArgs: []string{
				"device_type:dut",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Verify access to cache": {
			Docs: []string{
				"Verifies connection to the cache service.",
				"Connection checked from the DUT.",
			},
			Conditions: []string{
				"Not Satlab device",
			},
			ExecName:               "cache_download_check",
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
		},
		"DUT has correct cros image version": {
			Docs: []string{
				"Check whether the cros image version on the device is match expected.",
			},
			Conditions: []string{
				"Is it first deployment task",
				"Recovery version has OS image path",
				"Has a stable-version service",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName: "cros_is_on_stable_version",
			RecoveryActions: []string{
				"Quick provision OS",
				"Install ChromeOS by booting from servo USB-drive",
				"Install OS in DEV mode, with force to DEV-mode",
				"Install OS in DEV mode with fresh image",
				"Install OS in DEV mode, with force to DEV-mode with test firmware",
				"Install OS in recovery mode by booting from servo USB-drive",
			},
		},
		"Install OS in DEV mode with fresh image": {
			Docs: []string{
				"Download fresh usb image and Install OS from it in DEV-mode.",
			},
			Conditions: []string{
				"Is servod running",
				"Is a Chromebook",
				"Is servo USB key detected",
				"Recovery version has OS image path",
			},
			Dependencies: []string{
				"Download stable image to USB-key",
				"Install OS in DEV mode by USB-drive",
			},
			ExecName: "sample_pass",
		},
		"Install OS in DEV mode, with force to DEV-mode with test firmware": {
			Docs: []string{
				"Second attempt to install image in DEV mode",
			},
			Conditions: []string{
				"Is servod running",
				"Is a Chromebook",
				"Is servo USB key detected",
				"Recovery version has OS image path",
				"Is recovery-version has firmware image path",
			},
			Dependencies: []string{
				"Flash EC (FW) by servo (allowed failed)",
				"Sleep 120 seconds",
				"Disable software write protection via servo",
				"Flash AP (FW) by servo with GBB (dev mode + USB boot)",
				"Sleep 120 seconds",
				"Install OS in DEV mode by USB-drive",
				"Remove REFLASH_FW repair-request",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Is it first deployment task": {
			Docs: []string{
				"Check if deployment check not need to be run.",
				"If HWID or serial-number already collected from DUT then we already test it before.",
			},
			Conditions: []string{
				"Is HWID known",
				"Is serial-number known",
			},
			ExecName:   "sample_fail",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Collect firmware target": {
			Docs: []string{
				"Collect firmware target from DUT to inventory.",
			},
			Conditions: []string{
				"Is a Chromebook",
			},
			Dependencies: []string{
				"Device is SSHable",
				"Is crosid readable",
			},
			ExecName: "cros_collect_firmware_target",
			ExecExtraArgs: []string{
				"force_override:false",
			},
			RecoveryActions: []string{
				// Just try again as we are awaiting health data.
				"Sleep 1s",
				"Quick provision OS",
			},
			AllowFailAfterRecovery: true,
			RunControl:             RunControl_ALWAYS_RUN,
		},
		"Collect gpu_id": {
			Docs: []string{
				"Collect gpu_id from DUT with graphics hardware_probe to UFS and recoveries's label GpuId.",
			},
			Dependencies: []string{
				"Device is SSHable",
			},
			ExecName:               "cros_collect_gpu_id",
			AllowFailAfterRecovery: true,
			RunControl:             RunControl_ALWAYS_RUN,
		},
		"Collect fingerprint": {
			Docs: []string{
				"Collect fingerprintBoard, fingerprintMcu, and fingerprintSensor, verify they match cros_config fingerprint board, and add to UFS and recoveries's fingerprint labels.",
			},
			ExecName:               "cros_collect_fingerprint",
			AllowFailAfterRecovery: true,
		},
		"Has valid Fingerprint board value": {
			Docs: []string{
				"Validate fingerprint board.",
			},
			ExecName:      "cros_is_valid_fingerprint_board",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			RunControl:    RunControl_ALWAYS_RUN,
		},
		"Power-off the DUT using AMT": {
			Docs: []string{
				"Ensure the DUT is powered-off using Intel AMT.",
			},
			Conditions: []string{
				"Device has Intel AMT",
				"Intel AMT is available",
			},
			ExecName: "cros_flex_set_amt_power_state",
			ExecExtraArgs: []string{
				"state:off",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			RunControl:    RunControl_ALWAYS_RUN,
		},
		"Power-on the DUT using AMT": {
			Docs: []string{
				"Ensure the DUT is powered-on using Intel AMT.",
			},
			Conditions: []string{
				"Device has Intel AMT",
				"Intel AMT is available",
			},
			ExecName: "cros_flex_set_amt_power_state",
			ExecExtraArgs: []string{
				"state:on",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			RunControl:    RunControl_ALWAYS_RUN,
		},
		"Are AMT details known": {
			Docs: []string{
				"Check if the DUT has the AMT manager peripheral.",
				"Does not verify the health of AMT itself.",
			},
			Conditions: []string{
				"Is Flex device",
			},
			ExecName:      "cros_flex_amt_known",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			RunControl:    RunControl_ALWAYS_RUN,
		},
		"Device has Intel AMT": {
			Docs: []string{
				"DUT is a Flex device with the AMT manager peripheral.",
			},
			Dependencies: []string{
				"Is Flex device",
				"Are AMT details known",
			},
			ExecName:      "sample_pass",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Device does not have Intel AMT": {
			Docs: []string{
				"DUT is Flex device without the AMT manager peripheral.",
			},
			Dependencies: []string{
				"Is Flex device",
				"Are AMT details known",
			},
			ExecName:      "sample_fail",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Intel AMT is available": {
			Docs: []string{
				"Ping the AMT interface until it responds to pings.",
			},
			Dependencies: []string{
				"Device has Intel AMT",
			},
			ExecName:      "cros_flex_amt_available",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			RunControl:    RunControl_ALWAYS_RUN,
			ExecTimeout:   &durationpb.Duration{Seconds: 91},
		},
		"FPMCU is working": {
			Docs: []string{
				"Check FPMCU is working and can provide version info.",
			},
			Conditions: []string{
				"Is a Chromebook",
				"Device not in MP Signed AP FW pool",
				"Has valid Fingerprint board value",
			},
			ExecName: "cros_run_command",
			ExecExtraArgs: []string{
				"host:dut",
				"command:ectool --name=cros_fp version",
				"background:false",
			},
			RecoveryActions: []string{
				"Provision OS if needed",
				"Reflash FP MCU from installed OS",
			},
			AllowFailAfterRecovery: true,
		},
		"Reflash FP MCU from installed OS": {
			Docs: []string{
				"Reflash FPMCU from existing OS on the DUT.",
				"The action is expected to be finished in a few minutes.",
			},
			Dependencies: []string{
				"Set fw_wp_state to force_off",
			},
			ExecName: "cros_run_command",
			ExecExtraArgs: []string{
				"host:dut",
				"command:flash_fp_mcu /opt/google/biod/fw/$(cros_config /fingerprint board)*.bin",
				"background:false",
			},
			ExecTimeout: &durationpb.Duration{Seconds: 600},
			RunControl:  RunControl_ALWAYS_RUN,
		},
		"FW WP is disabled": {
			Docs: []string{
				"Validate if WP is enabled or not.",
			},
			Conditions: []string{
				"Is a Chromebook",
				"Device not in MP Signed AP FW pool",
			},
			ExecName: "servo_check_servod_control",
			ExecExtraArgs: []string{
				"command:fw_wp_state",
				"expected_string_value:force_off",
			},
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
		},
		"Boot DUT in recovery and reset firmware config in CBI": {
			Docs: []string{
				"This boot DUT into recovery mode, and compare firmware config value ",
				"between OS image and CBI content. It will reset value in CBI based on ",
				"value from OS image if a mismatch is detected.",
			},
			ExecName: "cros_reset_firmware_config_in_cbi",
			ExecExtraArgs: []string{
				"boot_timeout:480",
				"boot_interval:10",
				"boot_retry:2",
				"post_reset_boot_time:15",
				"after_reboot_check:true",
				"after_reboot_timeout:150",
				"after_reboot_allow_use_servo_reset:true",
			},
			ExecTimeout: &durationpb.Duration{Seconds: 3000},
			RunControl:  RunControl_ALWAYS_RUN,
		},
		"Install ChromeOS by booting from servo USB-drive": {
			Docs: []string{
				"Use provision image on USB-key to boot and get SSH access.",
				"Run specific install command to install ChromeOS.",
				"The logic is copy from cros-provision",
			},
			Conditions: []string{
				"Is a Chromebook",
				"Is servod running",
				"Can become ChromeOS-based",
				"Is servo USB key detected",
			},
			Dependencies: []string{
				"Mark as ChromeOS based",
				"Detect CacheService address",
				"Call servod to download provision image to USB-key",
				"Boot on USB-key and install ChroemOS",
				"Wait to be SSHable (normal boot)",
				"Remove REIMAGE_BY_USBKEY repair-request",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Boot on USB-key and install ChroemOS": {
			Docs: []string{
				"This action installs the test image on DUT utilizing ",
				"the features of servo. DUT will be booted in recovery ",
				"mode. In some cases RO FW is not allowed to boot in ",
				"recovery mode with active PD, so we will change it to ",
				"sink-mode if required.",
			},
			ExecName: "cros_provision_actions_from_recovery_mode",
			ExecExtraArgs: []string{
				"run_android_install:false",
				"run_cros_install:true",
				"reset_tpm:true",
				"boot_timeout:150",
				"boot_interval:10",
				"boot_retry:1",
				"ignore_reboot_failure:true",
				"after_reboot_check:true",
				// Increase from 150 to 240 see b/389769971#comment3
				"after_reboot_timeout:240",
				"after_reboot_allow_use_servo_reset:true",
			},
			ExecTimeout: &durationpb.Duration{Seconds: 10000},
			RunControl:  RunControl_ALWAYS_RUN,
		},
	}

	androidActions(actions)
	addCrosCftContainers(actions)
	addEnvActions(actions)
	addStateActions(actions)
	return actions
}
