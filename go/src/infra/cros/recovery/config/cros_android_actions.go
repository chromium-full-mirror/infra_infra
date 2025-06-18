// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import "google.golang.org/protobuf/types/known/durationpb"

func androidActions(actions map[string]*Action) {
	am := map[string]*Action{
		"Android OS checks": {
			Docs: []string{
				"Run DUT readiness checks for Android based DUTs.",
			},
			Conditions: []string{
				"Is Android based?",
			},
			Dependencies: []string{
				"ADB reconnect",
				"Android is accessible",
				"ADB set Android as always awake",
				"Android: Collect logs",
				"Read bootId",
				"Device Uptime",
				"Has repair-request for re-provision",
				"Reset provisioned info",
				"Verify by host that DUT has default GBB flags",
				"Verify that DUT is not in DEV mode",
				"Missing HWID",
				"Match HWID",
				"Missing serial-number",
				"Match serial-number",
			},
			ExecName:      "sample_pass",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_UPLOAD_ON_ERROR},
		},
		"Android is accessible": {
			Docs: []string{
				"Validate is Andoid OS is accessible by reading data from the host.",
			},
			ExecName:    "cros_access",
			ExecTimeout: &durationpb.Duration{Seconds: 15},
			RunControl:  RunControl_ALWAYS_RUN,
			RecoveryActions: []string{
				"Reboot by ADB",
				"Cold reset by servo and wait for ping",
				"Install Android OS by booting from servo USB-drive",
				"Foil: Install Android OS from servo USB-drive",
				"Reset servo_v4.1 ethernet and wait for ping",
				"Power cycle DUT by RPM and wait for ping",
				"Try fake disconnect and wait to be access",
				"Force reimage to ChromeOS in DEV mode",
				"Install OS in recovery mode by booting from servo USB-drive",
				"Place reboot request for labstation",
			},
		},
		"ADB set Android as always awake": {
			Docs: []string{
				"Set Android to be awake always.",
			},
			ExecName:   "ctr_make_awake_always",
			RunControl: RunControl_ALWAYS_RUN,
			RecoveryActions: []string{
				"Reboot by ADB",
				"Cold reset by servo and wait for ping",
				"Install Android OS by booting from servo USB-drive",
				"Foil: Install Android OS from servo USB-drive",
				"Force reimage to ChromeOS in DEV mode",
				"Install OS in recovery mode by booting from servo USB-drive",
			},
		},
		"Is Android based?": {
			ExecName:      "cros_is_android_based",
			RunControl:    RunControl_ALWAYS_RUN,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_UPLOAD_ON_ERROR},
		},
		"ADB Connect DUT": {
			Docs: []string{
				"Exec ADB connect to the DUT by ethernet.",
			},
			ExecName: "ctr_adb_connect",
			ExecExtraArgs: []string{
				"retry_count:3",
				"retry_interval:1",
				"timeout:10",
				"skip_when_connected:true",
			},
			RecoveryActions: []string{
				"Cold reset by servo and wait for ping",
				"Install Android OS by booting from servo USB-drive",
				"Update FW and install Android OS from servo USB-drive",
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"ADB reconnect": {
			Docs: []string{
				"Connect to DUT by ADB if not connected.",
				"Only executed if DUT is Android based.",
			},
			Conditions: []string{
				"Is Android based on previous DUT OS",
			},
			ExecName: "ctr_adb_connect",
			ExecExtraArgs: []string{
				"retry_count:3",
				"retry_interval:1",
				"timeout:10",
				"skip_when_connected:true",
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Reboot by ADB": {
			Docs: []string{
				"Reboot by ADB util.",
			},
			Conditions: []string{
				"Is Android based on previous DUT OS",
			},
			ExecName: "ctr_adb_command",
			ExecExtraArgs: []string{
				"command:reboot",
			},
			RunControl:             RunControl_ALWAYS_RUN,
			AllowFailAfterRecovery: true,
		},
		"Is Android based on previous DUT OS": {
			Docs: []string{
				"Validate that OS on the DUT was provisioned with Android.",
			},
			ExecName: "cros_is_previous_android_os_type",
		},
		"Is Android based by ADB or provision-info": {
			Docs: []string{
				"Validate that DUT has AndroidOS based on ADB connection or provisioned info.",
			},
			ExecName:   "cros_is_previous_android_based_or_os_type",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Foil-provision Setup service": {
			Docs: []string{
				"The setup method needs to be called once before performing install.",
			},
			ExecName: "ctr_foil_provision_setup_service",
		},
		"Foil-provision install OS": {
			Docs: []string{
				"Run install scrpt on the DUT.",
			},
			ExecName: "ctr_foil_provision_install",
			ExecTimeout: &durationpb.Duration{
				// The provision now requires USB-drive by default!
				// Set 1 hour just in case.
				Seconds: 3600,
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Foil: Install Android OS from servo USB-drive": {
			Docs: []string{
				"The install performs real install Android on the DUT.",
			},
			Conditions: []string{
				"Do not run on Mobile Harness box",
				"Is Android based by ADB or provision-info",
			},
			Dependencies: []string{
				"Detect CacheService address",
				"Start Foil-provision",
				"Foil-provision Setup service",
				"Foil-provision install OS",
				"Remove REIMAGE_BY_USBKEY repair-request",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Update FW and install Android OS from servo USB-drive": {
			Docs: []string{
				"Use provision image on USB-key to boot and get SSH access.",
				"Run specific install command to install Android OS.",
				"The logic is copy from foil-provision",
			},
			Conditions: []string{
				"Has light-provision image",
				"Run only on ChromeOS hardware",
				"Is servod running",
				"Is Android based by ADB or provision-info",
				"Is servo USB key detected",
				"Is recovery-version has firmware image path",
			},
			Dependencies: []string{
				"Mark as Android based",
				"Detect CacheService address",
				"Call servod to download provision image to USB-key",
				// Update FW.
				"Flash EC (FW) by servo (allowed failed)",
				"Sleep 120 seconds",
				"Disable software write protection via servo",
				"Flash AP (FW) by servo with GBB (dev mode + USB boot)",
				"Sleep 120 seconds",
				// Reimage the DUT
				"Boot on USB-key and install AndroidOS",
				// Re-ping and reconnect as the device may have gone to sleep.
				"Wait to be pingable (normal boot)",
				"ADB reconnect",
				"ADB set Android as always awake",
				"Remove REIMAGE_BY_USBKEY repair-request",
				"Remove REFLASH_FW repair-request",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Install Android OS by booting from servo USB-drive": {
			Docs: []string{
				"Use provision image on USB-key to boot and get SSH access.",
				"Run specific install command to install Android OS.",
				"The logic is copy from foil-provision",
			},
			Conditions: []string{
				"Has light-provision image",
				"Run only on ChromeOS hardware",
				"Is servod running",
				"Is Android based by ADB or provision-info",
				"Run for Android boards",
				"Is servo USB key detected",
			},
			Dependencies: []string{
				"Mark as Android based",
				"Detect CacheService address",
				"Call servod to download provision image to USB-key",
				"Boot on USB-key and install AndroidOS",
				// Re-ping and reconnect as the device may have gone to sleep.
				"Wait to be pingable (long boot)",
				"ADB reconnect",
				"ADB set Android as always awake",
				"Remove REIMAGE_BY_USBKEY repair-request",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Run for Android boards": {
			Docs: []string{
				"Check that board support AndroidOS",
			},
			ExecName: "dut_check_board",
			ExecExtraArgs: []string{
				"string_values:brya,corsola,dedede,fatcat,nissa,rauru",
			},
			RunControl:    RunControl_RUN_ONCE,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Has light-provision image": {
			Docs: []string{
				"Check that board has light-provision image.",
			},
			ExecName: "dut_check_board",
			ExecExtraArgs: []string{
				"string_values:brya,corsola,dedede,fatcat,nissa,rauru",
			},
			RunControl:    RunControl_RUN_ONCE,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Set CacheService address": {
			Docs: []string{
				"MH box does not have labservice, so we hard-code caceh addrress.",
				"Remove when prototype developing finished",
			},
			Conditions: []string{
				"Execution on Mobile Harness box",
			},
			ExecName: "cache_service_address_detection",
			ExecExtraArgs: []string{
				"cache_address:10.128.176.210:8082",
			},
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			RunControl:    RunControl_RUN_ONCE,
		},
		"Boot on USB-key and install AndroidOS": {
			Docs: []string{
				"This action installs the test image on DUT utilizing ",
				"the features of servo. DUT will be booted in recovery ",
				"mode. In some cases RO FW is not allowed to boot in ",
				"recovery mode with active PD, so we will change it to ",
				"sink-mode if required.",
			},
			ExecName: "cros_provision_actions_from_recovery_mode",
			ExecExtraArgs: []string{
				"run_android_install:true",
				"run_cros_install:false",
				"reset_tpm:true",
				"boot_timeout:150",
				"boot_interval:10",
				"boot_retry:1",
				"install_timeout:1200",
				"ignore_reboot_failure:true",
				"after_reboot_check:true",
				// Increase from 150 to 240 see b/389769971#comment3
				"after_reboot_timeout:240",
				"after_reboot_allow_use_servo_reset:true",
			},
			ExecTimeout: &durationpb.Duration{Seconds: 10000},
			RunControl:  RunControl_ALWAYS_RUN,
		},
		"Android: verify boot in recovery mode": {
			Docs: []string{
				"Verify that devices can boot from USB drive in recovery mode.",
			},
			Conditions: []string{
				"Has light-provision image",
				"Setup has servo info",
			},
			Dependencies: []string{
				"Is servod running",
				"Is servo USB key detected",
				"Detect CacheService address",
				"Call servod to download provision image to USB-key",
			},
			ExecName: "cros_provision_actions_from_recovery_mode",
			ExecExtraArgs: []string{
				"run_android_install:false",
				"run_cros_install:false",
				"reset_tpm:false",
				"boot_timeout:150",
				"boot_interval:10",
				"boot_retry:2",
				"ignore_reboot_failure:true",
				"after_reboot_check:true",
				// Increase from 150 to 240 see b/389769971#comment3
				"after_reboot_timeout:240",
				"after_reboot_allow_use_servo_reset:true",
			},
			ExecTimeout: &durationpb.Duration{Seconds: 1500},
			RecoveryActions: []string{
				"Cold reset DUT by servo and wait to boot",
			},
		},
		"Provision to stable-version if required": {
			Docs: []string{
				"Provision to stable-version for stable-version required pools.",
			},
			Conditions: []string{
				"Is Android based?",
				"Is in stable version required pools",
			},
			ExecName: "android_is_on_os_stable_version",
			RecoveryActions: []string{
				"Install Android OS by booting from servo USB-drive",
				"Foil: Install Android OS from servo USB-drive",
				"Update FW and install Android OS from servo USB-drive",
			},
		},
		"Android: Verify OS is on stable-version": {
			Docs: []string{
				"Verify that Android OS is on stable version.",
			},
			Conditions: []string{
				"Is Android based?",
			},
			ExecName: "android_is_on_os_stable_version",
			RecoveryActions: []string{
				"Install Android OS by booting from servo USB-drive",
				"Foil: Install Android OS from servo USB-drive",
				"Update FW and install Android OS from servo USB-drive",
			},
		},
		"Is in stable version required pools": {
			Docs: []string{
				"Verify device is in pools which require to be on stable version.",
			},
			ExecName: "dut_is_in_pool",
			ExecExtraArgs: []string{
				"chrome",
				"chrome.tests",
				"chrome.tests.gpu",
				"chrome.tests.osnext",
				"chrome.tests.perf",
			},
			RunControl:    RunControl_RUN_ONCE,
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Android: Collect logs": {
			Docs: []string{
				"We collect any pre-existing logs from before deletes such logs ",
				"on the DUT. Any logs collection are not critical, and we marks ",
				"that action attempt to perform to avoid repeating it.",
			},
			Conditions: []string{
				"Android:Device is pingable",
				"Android is accessible",
				"Confirm log collection info does not exist",
			},
			Dependencies: []string{
				"Create log collection info",
				"Android: Collect dmesg logs from DUT",
			},
			ExecName:               "sample_pass",
			RunControl:             RunControl_RUN_ONCE,
			AllowFailAfterRecovery: true,
			MetricsConfig:          &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
		},
		"Android: Collect dmesg logs from DUT": {
			Docs: []string{
				"Collect the entire output of dmesg",
			},
			ExecName: "cros_dmesg",
			ExecExtraArgs: []string{
				"device_type:dut",
				"human_readable:false",
			},
			AllowFailAfterRecovery: true,
		},
	}
	for k, v := range am {
		if _, ok := actions[k]; ok {
			panic("duplicate key:" + k + " in actions map")
		}
		actions[k] = v
	}
}
