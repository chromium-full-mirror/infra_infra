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
				"Is Android based",
			},
			Dependencies: []string{
				"Android is accessable",
				"ADB set Android as always awake",
				"Read bootId",
				"Device Uptime",
				"Has repair-request for re-provision",
				"Reset provisioned info",
				"Verify that DUT is not in DEV mode",
				"Verify that DUT has default GBB flags",
				"Missing HWID",
				"Match HWID",
			},
			ExecName:      "sample_pass",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_UPLOAD_ON_ERROR},
		},
		"Android is accessable": {
			Docs: []string{
				"Validate is Andoid OS is accessable by reading data from the host.",
			},
			Conditions: []string{
				"Is Android based",
			},
			ExecName:    "cros_ssh",
			ExecTimeout: &durationpb.Duration{Seconds: 15},
			RunControl:  RunControl_ALWAYS_RUN,
			RecoveryActions: []string{
				"Reboot by ADB",
				"Cold reset by servo and wait for ping",
				"Provision Android OS",
				"Install Android OS by booting from servo USB-drive",
				"Reset servo_v4.1 ethernet and wait for ping",
				"Power cycle DUT by RPM and wait for ping",
				"Force reimage to ChromeOS in DEV mode",
				"Install OS in recovery mode by booting from servo USB-drive",
			},
		},
		"ADB set Android as always awake": {
			Docs: []string{
				"Set Android to be awake always.",
			},
			Conditions: []string{
				"Is Android based",
			},
			ExecName:   "ctr_make_awake_always",
			RunControl: RunControl_ALWAYS_RUN,
			RecoveryActions: []string{
				"Reboot by ADB",
				"Cold reset by servo and wait for ping",
				"Provision Android OS",
				"Install Android OS by booting from servo USB-drive",
				"Reset servo_v4.1 ethernet and wait for ping",
				"Power cycle DUT by RPM and wait for ping",
				"Force reimage to ChromeOS in DEV mode",
				"Install OS in recovery mode by booting from servo USB-drive",
			},
		},
		"Is Android based": {
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
				"retry_interval:3",
				"timeout:5",
			},
			RecoveryActions: []string{
				"Cold reset by servo and wait for ping",
				"Install Android OS by booting from servo USB-drive",
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"ADB reconnect": {
			Docs: []string{
				"Connect to DUT by ADB if not connected.",
				"Only executed if DUT is Android based.",
			},
			Conditions: []string{
				"Is Android based",
			},
			ExecName: "ctr_adb_connect",
			ExecExtraArgs: []string{
				"retry_count:3",
				"retry_interval:3",
				"timeout:5",
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Reboot by ADB": {
			Docs: []string{
				"Reboot by ADB util.",
			},
			Conditions: []string{
				"Is Android based",
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
		"Provision Android OS": {
			Docs: []string{
				"The install performs real install Android on the DUT.",
			},
			Conditions: []string{
				"Execution not on Mobile Harness box",
				"Is Android based by ADB or provision-info",
			},
			Dependencies: []string{
				"Detect CacheService address",
				"Start Foil-provision",
				"Foil-provision Setup service",
			},
			ExecName: "ctr_foil_provision_install",
			ExecTimeout: &durationpb.Duration{
				// The provision now requires USB-drive by default!
				// Set 1 hour just in case.
				Seconds: 3600,
			},
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Install Android OS by booting from servo USB-drive": {
			Docs: []string{
				"Use provision image on USB-key to boot and get SSH access.",
				"Run specific install command to install Android OS.",
				"The logic is copy from foil-provision",
			},
			Conditions: []string{
				"Execution on Mobile Harness box",
				"Is a Chromebook",
				"Is servod running",
				"Is Android based by ADB or provision-info",
				"Is servo USB key detected",
			},
			Dependencies: []string{
				"Mark as Android based",
				"Set CacheService address",
				"Call servod to download provision image to USB-key",
				"Boot on USB-key and install AndroidOS",
				"Wait to be SSHable (normal boot)",
				"ADB reconnect",
				"ADB set Android as always awake",
				"Remove REIMAGE_BY_USBKEY repair-request",
			},
			ExecName:   "sample_pass",
			RunControl: RunControl_ALWAYS_RUN,
		},
		"Set CacheService address": {
			Docs: []string{
				"MH box does not have labservice, so we hard-code caceh addrress.",
				"Remove when prototype developing finished",
			},
			ExecName: "cache_service_address_detection",
			ExecExtraArgs: []string{
				"cache_address:10.128.176.210:8082",
			},
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
				"boot_timeout:150",
				"boot_interval:10",
				"boot_retry:1",
				"ignore_reboot_failure:true",
				"after_reboot_check:true",
				"after_reboot_timeout:150",
				"after_reboot_allow_use_servo_reset:true",
			},
			ExecTimeout: &durationpb.Duration{Seconds: 8000},
			RunControl:  RunControl_ALWAYS_RUN,
		},
	}
	for k, v := range am {
		if _, ok := actions[k]; ok {
			panic("duplicate key:" + k + " in actions map")
		}
		actions[k] = v
	}
}
