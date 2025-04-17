// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"log"

	"google.golang.org/protobuf/types/known/durationpb"
)

func crosDeployPlan() *Plan {
	return &Plan{
		CriticalActions: []string{
			"Set state: needs_deploy",
			"Check stable versions exist",
			"Download stable version OS image to servo usbkey if necessary",
			"Device is pingable before deploy",
			"DUT is on test channel OS",
			"Collect firmware target",
			"DUT has correct cros image version",
			"Set dev_boot_usb is enabled",
			"DUT has expected dev firmware",
			"DUT has expected firmware version",
			"Deployment checks",
			"Collect DUT labels",
		},
		Actions: crosDeployAndRepairActions(),
	}
}

func mhDeployPlan() *Plan {
	return &Plan{
		CriticalActions: []string{
			"Mark as Android based",
			"Set CacheService address",
			"Check stable versions exist",
			"Set GBB flags to enable dev mode and boot from usb by servo",
			"ADB Connect DUT",
			"Android is accessable",
			"ADB set Android as always awake",
			// TODO(b/410571779): Slowly verify and enable actions below.
			// "DUT is on test channel OS",
			// "Collect firmware target",  Blocked by b/374944007
			// "DUT has correct cros image version",
			"DUT has expected dev firmware",
			// "DUT has expected firmware version",
			// "Deployment checks",
			// "Collect DUT labels",
		},
		Actions: crosDeployAndRepairActions(),
	}
}

func deployActions() map[string]*Action {
	return map[string]*Action{
		"Device is pingable before deploy": {
			Docs: []string{
				"Verify that device is present in setup.",
				"All devices is pingable by default even they have prod images on them.",
				"If device is not pingable then device is off on not connected",
			},
			ExecName:    "cros_ping",
			ExecTimeout: &durationpb.Duration{Seconds: 15},
			RecoveryActions: []string{
				"Cold reset DUT by servo and wait to boot",
				"Power cycle DUT by RPM and wait for ping",
			},
		},
		"DUT is on test channel OS": {
			Docs: []string{
				"Verify that device has OS version from test channel, if not then install it.",
			},
			Dependencies: []string{
				"Device is pingable before deploy",
				"Device NOT booted from USB-drive",
			},
			ExecName: "cros_is_os_test_channel",
			RecoveryActions: []string{
				"Quick provision OS",
				"Install OS in DEV mode",
				"Install OS in DEV mode, with force to DEV-mode",
				"Install OS in DEV mode with fresh image",
				"Install OS in DEV mode, with force to DEV-mode with test firmware",
			},
		},
		"DUT has expected dev firmware": {
			Docs: []string{
				"Verify that FW on the DUT has dev keys.",
			},
			Conditions: []string{
				"Is a Chromebook",
				"Device not in MP Signed AP FW pool",
			},
			ExecName:    "cros_has_dev_signed_firmware",
			ExecTimeout: &durationpb.Duration{Seconds: 600},
			RecoveryActions: []string{
				"Update FW from fw-image by servo and wait for boot",
				"Update firmware with factory mode by host",
				// IF DUT failed too boot after reboot then hard rebboot it.
				"Cold reset DUT by servo and wait to boot",
				"Update firmware with factory mode from host OS",
			},
		},
		"DUT has expected firmware version": {
			Docs: []string{
				"Verify that FW on the DUT has dev keys.",
			},
			Conditions: []string{
				"Is it first deployment task",
				"Is a Chromebook",
				"Device not in MP Signed AP FW pool",
				"Has a stable-version service",
				"Check stable firmware version exists",
				"Is recovery-version has firmware image path",
				// Some model depends on hwid to differentiate firmware target, so we need collect this info before firmware update.
				"Collect HWID into inventory",
			},
			Dependencies: []string{
				"DUT has expected RO firmware version",
				"DUT has expected RW firmware version",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"DUT has expected RO firmware version": {
			Docs: []string{
				"Verify that RO FW on the DUT matches stable version.",
			},
			ExecName: "cros_is_on_stable_firmware_version",
			ExecExtraArgs: []string{
				"target:ro",
			},
			RecoveryActions: []string{
				"Fix FW on the DUT to match stable-version and wait to boot",
				"Update FW from fw-image by servo and wait for boot",
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"DUT has expected RW firmware version": {
			Docs: []string{
				"Verify that RW FW on the DUT matches stable version.",
			},
			ExecName: "cros_is_on_stable_firmware_version",
			ExecExtraArgs: []string{
				"target:rw",
			},
			RecoveryActions: []string{
				"Fix FW on the DUT to match stable-version and wait to boot",
				"Update FW from fw-image by servo and wait for boot",
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Update firmware with factory mode by host": {
			Docs: []string{
				"Force update FW on the DUT by factory mode. Access to the ",
				"device under test is required to collect HWID information, ",
				"which is critical to finding the firmware targets for flash.",
			},
			Conditions: []string{
				"Is a Chromebook",
				"Is recovery-version has firmware image path",
				"Device is SSHable",
			},
			Dependencies: []string{
				"Disable software-controlled write-protect for 'internal'",
				"Disable software-controlled write-protect for 'ec'",
				"Update FW from fw-image with factory mode from DUT",
				"Remove REFLASH_FW repair-request",
				"Simple reboot",
				"Wait to be accessable",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Update firmware with factory mode from host OS": {
			Docs: []string{
				"Force update FW on the DUT by factory mode. Access to the ",
				"device under test is required to collect HWID information, ",
				"which is critical to finding the firmware targets for flash.",
			},
			Conditions: []string{
				"Is a Chromebook",
				"Is recovery-version has firmware image path",
				"Device is SSHable",
			},
			Dependencies: []string{
				"Disable software-controlled write-protect for 'internal'",
				"Disable software-controlled write-protect for 'ec'",
				"Update FW from host OS image with factory mode",
				"Remove REFLASH_FW repair-request",
				"Simple reboot",
				"Wait to be accessable",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Deployment checks": {
			Docs: []string{
				"Run some special checks as part of deployment.",
			},
			Conditions: []string{
				"Not Satlab device",
				"Is it first deployment task",
			},
			Dependencies: []string{
				"Verify battery charging level",
				"Verify boot in recovery mode",
				"Wait to be accessable",
				"Verify RPM config",
				"Wait to be accessable",
			},
			ExecName: "sample_pass",
		},
		"Verify battery charging level": {
			Docs: []string{
				"Battery will be checked that it can be charged to the 80% as if device cannot then probably device is not fully prepared for deployment.",
				"If battery is not charged, then we will re-check every 15 minutes for 8 time to allows to charge the battery.",
				"Dues overheat battery in audio boxes mostly it deployed ",
			},
			Conditions: []string{
				"Is not in audio box",
				"Battery is expected on device",
				"Battery is present on device",
			},
			Dependencies: []string{
				"Wait to be accessable",
			},
			ExecName: "cros_battery_changable_to_expected_level",
			ExecExtraArgs: []string{
				"charge_retry_count:8",
				"charge_retry_interval:900",
			},
			ExecTimeout: &durationpb.Duration{Seconds: 9000},
		},
		"Verify boot in recovery mode": {
			Docs: []string{
				"Devices deployed with servo in the pools required secure mode need to be able to be boot in recovery mode.",
			},
			Conditions: []string{
				"Setup has servo info",
			},
			Dependencies: []string{
				"Is servod running",
				"Wait to be accessable",
			},
			ExecName: "cros_install_in_recovery_mode",
			ExecExtraArgs: []string{
				"run_custom_commands:false",
				"set_dev_default_boot:true",
				"run_tpm_reset:true",
				"tpm_reset_timeout:60",
				"run_os_install:false",
				"boot_timeout:480",
				"boot_retry:1",
				"boot_interval:10",
				"halt_timeout:120",
				"ignore_reboot_failure:true",
				"badblocks_mode:not",
				"after_reboot_check:true",
				"after_reboot_timeout:150",
				"after_reboot_allow_use_servo_reset:true",
			},
			ExecTimeout: &durationpb.Duration{Seconds: 1500},
			RecoveryActions: []string{
				"Cold reset DUT by servo and wait to boot",
				// The other reason why it fail on good DUT is that USB-key has not good image.
				"Download stable image to USB-key",
			},
		},
		"Install OS in DEV mode": {
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
				"Install OS in DEV mode by USB-drive",
			},
			ExecName:      "sample_pass",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Collect cellular labels": {
			Docs: []string{
				"Collect device labels on cellular DUTs",
			},
			Conditions: []string{
				"Is in cellular pool",
			},
			Dependencies: []string{
				"Update cellular modem labels",
				"Update cellular sim labels",
			},
			ExecName: "sample_pass",
			// Do not block deployment on cellular label detection.
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Collect DUT labels": {
			Docs: []string{
				"Updating device info in inventory.",
			},
			Dependencies: []string{
				// Collect HWID may not get tiggered before this if a DUT already have DEV signed firmware.
				// Invoke it here to ensure we have fwid updated, and the action has RunControl_RUN_ONCE so
				// the actual exectution won't happen twice.
				"Collect HWID into inventory",
				"Read DUT serial-number from DUT",
				"Read DUT serial-number from DUT (Satlab)",
				"Read storage type from DUT",
				"Read device SKU",
				"servo_type_label",
				"Read RO_VPD from DUT",
				"Collect cellular labels",
			},
			ExecName:      "sample_pass",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"servo_type_label": {
			Docs: []string{
				"Update the servo type label for the DUT info.",
			},
			ExecName:               "servo_update_servo_type_label",
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Check stable versions exist": {
			Docs: []string{
				"Check the DUT has model specific cros, firmware and faft stable_version configured.",
			},
			Conditions: []string{
				"Has a stable-version service",
			},
			Dependencies: []string{
				"Recovery version has OS image path",
				"Check stable firmware version exists",
			},
			ExecName:      "sample_pass",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Collect HWID into inventory": {
			Docs: []string{
				"Collect DUT hwid and update it into inventory info.",
			},
			Dependencies: []string{
				"Read HWID from DUT",
				"Read HWID from DUT (Satlab)",
			},
			RunControl:    RunControl_RUN_ONCE,
			ExecName:      "sample_pass",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
	}
}

func crosDeployAndRepairActions() map[string]*Action {
	combo := deployActions()
	for name, action := range crosRepairActions() {
		if _, ok := combo[name]; ok {
			log.Fatalf("duplicate name in crosDeploy and crosRepair plan actions: %s", name)
		}
		combo[name] = action
	}
	return combo
}
