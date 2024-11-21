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
				"Is Andoid based",
			},
			Dependencies: []string{
				"Android is accessable",
				"ADB set Android as always awake",
				"Read bootId",
				"Device Uptime",
				"Has repair-request for re-provision",
				"Reset provisioned info",
			},
			ExecName:      "sample_pass",
			MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_UPLOAD_ON_ERROR},
		},
		"Android is accessable": {
			Docs: []string{
				"Validate is Andoid OS is accessable by reading data from the host.",
			},
			Conditions: []string{
				"Is Andoid based",
			},
			ExecName:    "cros_ssh",
			ExecTimeout: &durationpb.Duration{Seconds: 15},
			RunControl:  RunControl_ALWAYS_RUN,
			RecoveryActions: []string{
				"Reboot by ADB",
				"Cold reset by servo and wait for ping",
				"Provision Android OS",
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
				"Is Andoid based",
			},
			ExecName: "ctr_make_awake_always",
			RecoveryActions: []string{
				"Reboot by ADB",
				"Cold reset by servo and wait for ping",
				"Provision Android OS",
				"Reset servo_v4.1 ethernet and wait for ping",
				"Power cycle DUT by RPM and wait for ping",
				"Force reimage to ChromeOS in DEV mode",
				"Install OS in recovery mode by booting from servo USB-drive",
			},
		},
		"Is Andoid based": {
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
		},
		"Reboot by ADB": {
			Docs: []string{
				"Reboot by ADB util.",
			},
			Conditions: []string{
				"Is Andoid based",
			},
			ExecName: "ctr_adb_command",
			ExecExtraArgs: []string{
				"command:reboot",
			},
			AllowFailAfterRecovery: true,
		},
		"Is Android based on previous DUT OS": {
			Docs: []string{
				"Validate that OS on the DUT was provisioned with Android.",
			},
			ExecName: "cros_is_previous_android_os_type",
		},
		"Is Android based ADB or previous DUT OS": {
			Docs: []string{
				"Validate that OS on the DUT was provisioned with Android.",
			},
			ExecName: "cros_is_previous_android_based_or_os_type",
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
				"Is Android based ADB or previous DUT OS",
			},
			Dependencies: []string{
				"Detect CacheService address",
				"Start Foil-provision",
				"Foil-provision Setup service",
			},
			ExecName: "ctr_foil_provision_install",
			ExecTimeout: &durationpb.Duration{
				// The provisioning process may take not more than 15 minutes.
				Seconds: 900,
			},
		},
	}
	for k, v := range am {
		if _, ok := actions[k]; ok {
			panic("duplicate key:" + k + " in actions map")
		}
		actions[k] = v
	}
}
