// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
)

func btpeerRepairPlan() *Plan {
	return &Plan{
		CriticalActions: []string{
			"Set state: BROKEN",
			"Device is pingable",
			"Device is SSHable",
			"Fetch btpeer image release config from GCS",
			"OS image is up to date",
			"Chameleond version is up to date",
			"Chameleond service is running",
			"Device has been rebooted recently",
			"Set state: WORKING",
		},
		Actions: map[string]*Action{
			"Set state: BROKEN": {
				Docs: []string{
					"The device state BROKEN.",
				},
				ExecName:      "btpeer_state_broken",
				RunControl:    RunControl_RUN_ONCE,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state: WORKING": {
				Docs: []string{
					"The device state WORKING.",
				},
				ExecName:      "btpeer_state_working",
				RunControl:    RunControl_RUN_ONCE,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Device is pingable": {
				Docs: []string{
					"Wait device to be pingable.",
					"Waiting time 15 seconds.",
				},
				ExecName:    "cros_ping",
				ExecTimeout: &durationpb.Duration{Seconds: 15},
				RunControl:  RunControl_ALWAYS_RUN,
			},
			"Device is SSHable": {
				Docs: []string{
					"Try to wait device to be sshable.",
					"Waiting time 150 seconds.",
				},
				ExecName:   "cros_ssh",
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Device has been rebooted recently": {
				Docs: []string{
					"Checks the device's uptime and fails if it is not less than one day.",
					"Recovers by rebooting the device.",
				},
				ExecName: "btpeer_assert_uptime_is_less_than_duration",
				ExecExtraArgs: []string{
					"duration_min:1440",
				},
				RecoveryActions: []string{
					"Reboot device and wait for SSH",
				},
			},
			"Reboot device and wait for SSH": {
				Docs: []string{
					"Reboots the device over ssh and waits for the device to become ssh-able again.",
				},
				ExecName:    "btpeer_reboot",
				ExecTimeout: durationpb.New(5 * time.Minute),
				RunControl:  RunControl_ALWAYS_RUN,
			},
			"Chameleond service is running": {
				Docs: []string{
					"Checks the status of the chameleond service on the device to see if it is running.",
					"Fails if the service is not running and attempts recovery by rebooting.",
				},
				ExecName: "btpeer_assert_chameleond_service_is_running",
				RecoveryActions: []string{
					"Reboot device and wait for SSH",
				},
				RunControl: RunControl_ALWAYS_RUN,
			},

			// Chameleond release process actions.
			"Chameleond version is up to date": {
				Docs: []string{
					"Checks if the chameleond version on the btpeer is outdated and updates it if it is.",
				},
				Conditions: []string{
					"Btpeer release process should be chameleond-based",
				},
				Dependencies: []string{
					"Fetch btpeer chameleond release config from GCS",
					"Identify expected chameleond release bundle",
					"Fetch installed chameleond bundle commit from btpeer",
					"Btpeer has expected chameleond bundle installed",
				},
				ExecName:      "sample_pass",
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Fetch btpeer chameleond release config from GCS": {
				Docs: []string{
					"Retrieves the production btpeer chameleond config from GCS and stores it in the exec state for later reference.",
				},
				ExecName:   "btpeer_fetch_btpeer_chameleond_release_config",
				RunControl: RunControl_RUN_ONCE,
			},
			"Identify expected chameleond release bundle": {
				Docs: []string{
					"Identifies the expected chameleond release bundle based off of the config and DUT host.",
					"Note: For now this step ignores the DUT host and always selects the latest, non-next bundle.",
				},
				ExecName:   "btpeer_identify_expected_chameleond_release_bundle",
				RunControl: RunControl_RUN_ONCE,
			},
			"Fetch installed chameleond bundle commit from btpeer": {
				Docs: []string{
					"Retrieves the chameleond commit of the currently installed chameleond version from a log file on the btpeer.",
					"If it fails to retrieve the commit we assume the chameleond version is too old to have the needed log file and we recover by installing the expected chameleond bundle.",
				},
				ExecName: "btpeer_fetch_installed_chameleond_bundle_commit",
				RecoveryActions: []string{
					"Install expected chameleond release bundle and then reboot device",
				},
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Btpeer has expected chameleond bundle installed": {
				Docs: []string{
					"Checks if the installed chameleond commit matches the expected chameleond bundle commit.",
					"If the check fails, it attempts to recover by updating chameleond with the expected bundle.",
				},
				ExecName: "btpeer_assert_btpeer_has_expected_chameleond_release_bundle_installed",
				RecoveryActions: []string{
					"Install expected chameleond release bundle and then reboot device",
				},
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Install expected chameleond release bundle and then reboot device": {
				Docs: []string{
					"Installs/updates chameleond on the btpeer and then reboots the device.",
				},
				Dependencies: []string{
					"Install expected chameleond release bundle",
					"Reboot device and wait for SSH",
				},
				ExecName:   "sample_pass",
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Install expected chameleond release bundle": {
				Docs: []string{
					"Installs/updates chameleond on the btpeer with the expected chameleond bundle.",
					"The expected bundle archive is downloaded from GCS to the btpeer through the cache, extracted, and installed via make.",
				},
				ExecName:    "btpeer_install_expected_chameleond_release_bundle",
				ExecTimeout: durationpb.New(15 * time.Minute),
				RecoveryActions: []string{
					"Reboot device and wait for SSH",
				},
			},

			// OS image release process actions.
			"Fetch btpeer image release config from GCS": {
				Docs: []string{
					"Retrieves the production btpeer image config from GCS and stores it in the exec state for later reference.",
				},
				ExecName:   "btpeer_image_fetch_release_config",
				RunControl: RunControl_RUN_ONCE,
			},
			"Btpeer release process should be image-based": {
				Docs: []string{
					"Passes if the release process this specific btpeer use is image-based.",
					"The btpeer will be chosen to use the image-based process if it currently has a custom image installed (i.e. existence of an image UUID in scope) or the hostname of the primary dut in this testbed is present in the image release config's NextImageVerificationDutPool.",
				},
				ExecName: "btpeer_assert_release_process_matches",
				ExecExtraArgs: []string{
					"expected_release_process:image",
				},
				RunControl: RunControl_RUN_ONCE,
			},
			"Btpeer release process should be chameleond-based": {
				Docs: []string{
					"Passes if the release process this specific btpeer use is image-based.",
					"The btpeer will be chosen to use the chameleond-based process if it is not chosen to use the new image-based release process.",
				},
				ExecName: "btpeer_assert_release_process_matches",
				ExecExtraArgs: []string{
					"expected_release_process:chameleond",
				},
				RunControl: RunControl_RUN_ONCE,
			},
			"OS image is up to date": {
				Docs: []string{
					"Ensures the OS image installed on the btpeer is the expected OS image release.",
					"Checks the btpeer scope state for the expected image UUID and installed image UUID and fails if they differ.",
					"Recovers by installing the expected image release.",
				},
				Conditions: []string{
					"Btpeer release process should be image-based",
				},
				Dependencies: []string{
					"Identify expected image release",
					"Identify installed image release",
				},
				ExecName: "btpeer_image_assert_expected_installed",
				RecoveryActions: []string{
					"Provision OS to expected image release",
				},
			},
			"Identify expected image release": {
				Docs: []string{
					"Identifies which btpeer image this specific btpeer should have installed based on the release config and the primary dut's hostname, and then stores it in the scope.",
				},
				ExecName:   "btpeer_image_identify_expected",
				RunControl: RunControl_RUN_ONCE,
			},
			"Identify installed image release": {
				Docs: []string{
					"Reads the image UUID from the image build info file present on all ChromeOS Raspberry Pi btpeer OS image installations and stores it in the scope.",
					"Still passes if the image build info file is not present (legacy image assumed), leaving the image UUID unset so that 'Btpeer has expected image release installed' fails and installs the expected image.",
				},
				ExecName: "btpeer_image_fetch_installed_uuid",
				ExecExtraArgs: []string{
					"allow_legacy_image:true",
				},
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Provision OS to expected image release": {
				Docs: []string{
					"Installs the expected OS image release onto the btpeer.",
					"This process can take about 30 minutes and includes many device reboots.",
					"If this fails, a manual recovery of flashing the SD card with a known good image is likely necessary.",
				},
				Dependencies: []string{
					"AB Partition device",
					"Temp Boot into A partition",
					"Set permanent boot partition as A",
					"Download expected OS image release to device",
					"Flash B partitions with downloaded OS image",
					"Temp Boot into B partition",
					"Set permanent boot partition as B",
					"Verify newly installed image has an image UUID",
				},
				ExecName:   "sample_pass",
				RunControl: RunControl_ALWAYS_RUN,
			},
			"AB Partition device": {
				Docs: []string{
					"Takes a device with the default partitioning scheme and re-partitions it.",
					"This action is skipped if the device is already AB partitioned.",
				},
				Conditions: []string{
					"Device is not AB partitioned",
				},
				Dependencies: []string{
					"Verify device has space for new B partitions",
					"Verify device has standard partition scheme",
					"Enable initrd",
					"Shrink rootfs",
					"Disable initrd",
					"Create new AB Partitions",
				},
				ExecName:   "sample_pass",
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Device is not AB partitioned": {
				Docs: []string{
					"Checks that the raspberry pi is not AB partitioned",
				},
				ExecName: "btpeer_has_partitions_with_labels",
				ExecExtraArgs: []string{
					"labels:BOOT_A,ROOT_A,BOOT_B,ROOT_B",
					"expect_match:false",
				},
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Verify device has space for new B partitions": {
				Docs: []string{
					"Verifies that the device has space to add the new",
					"BOOT_B/ROOT_B partitions",
				},
				ExecName:   "btpeer_has_partition_room",
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Verify device has standard partition scheme": {
				Docs: []string{
					"Verifies that the device has the default raspberry PI partitioning scheme",
					"consisting of 1 FAT32 boot partition and 1 EXT4 rootfs partition.",
				},
				ExecName:   "btpeer_device_has_standard_partitions",
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Enable initrd": {
				Docs: []string{
					"Enable initrd on the btpeers so we can add a pre-mount hook for rootfs.",
				},
				ExecName:    "btpeer_enable_initrd",
				RunControl:  RunControl_ALWAYS_RUN,
				ExecTimeout: &durationpb.Duration{Seconds: 600},
			},
			"Shrink rootfs": {
				Docs: []string{
					"Shrinks the device rootfs so it can be partitioned.",
				},
				ExecName:    "btpeer_shrink_rootfs",
				RunControl:  RunControl_ALWAYS_RUN,
				ExecTimeout: &durationpb.Duration{Seconds: 1000},
			},
			"Disable initrd": {
				Docs: []string{
					"Disables use of initrd on the btpeer.",
				},
				ExecName:    "btpeer_disable_initrd",
				RunControl:  RunControl_ALWAYS_RUN,
				ExecTimeout: &durationpb.Duration{Seconds: 600},
			},
			"Create new AB Partitions": {
				Docs: []string{
					"Takes a normal-partitioned raspberry PI partitions it for AB booting.",
				},
				ExecName:    "btpeer_partition_device",
				ExecTimeout: &durationpb.Duration{Seconds: 1000},
				RunControl:  RunControl_ALWAYS_RUN,
			},
			"Enable temp booting": {
				Docs: []string{
					"Applies necessary configurations to ensure temp booting is",
					"supported on the device.",
				},
				Conditions: []string{
					"btpeer_is_raspi_4b_less_than_revision_14",
				},
				Dependencies: []string{
					"btpeer_enable_debug_quirks",
					"Reboot device and wait for SSH",
				},
				ExecName:   "sample_pass",
				RunControl: RunControl_ALWAYS_RUN,
			},
			"Temp Boot into A partition": {
				Docs: []string{
					"Temporarily boots into the A partition on the device so we can ",
					"verify it's ok before setting it as the permanent boot device.",
				},
				ExecName: "btpeer_temp_boot_into_partition",
				Dependencies: []string{
					"Enable temp booting",
				},
				ExecExtraArgs: []string{
					"boot_partition_label:BOOT_A",
				},
				RunControl:  RunControl_ALWAYS_RUN,
				ExecTimeout: &durationpb.Duration{Seconds: 600},
			},
			"Set permanent boot partition as A": {
				Docs: []string{
					"Sets boot partition A as the default boot partition to use on reboot.",
				},
				ExecName: "btpeer_set_permanent_boot_partition",
				ExecExtraArgs: []string{
					"boot_partition_label:BOOT_A",
				},
				RunControl:  RunControl_ALWAYS_RUN,
				ExecTimeout: &durationpb.Duration{Seconds: 600},
			},
			"Download expected OS image release to device": {
				Docs: []string{
					"Downloads and decompresses the expected OS image release to the device.",
					"The overall timeout is set to 25m, with a 10m timeout for the download and a 10m timeout for decompression.",
				},
				ExecName:    "btpeer_download_image",
				RunControl:  RunControl_ALWAYS_RUN,
				ExecTimeout: &durationpb.Duration{Seconds: 1500}, // 25m
			},
			"Flash B partitions with downloaded OS image": {
				Docs: []string{
					"Flashes a new OS onto the BOOT_B/ROOT_B partitions.",
					"The overall timeout is set to 40m, with a 5m timeout for the BOOT_B partition flash and a 30m timeout for the ROOT_B partition flash.",
				},
				ExecName:    "btpeer_provision_device",
				RunControl:  RunControl_ALWAYS_RUN,
				ExecTimeout: &durationpb.Duration{Seconds: 2400}, // 40m
			},
			"Temp Boot into B partition": {
				Docs: []string{
					"Temporarily boots into the B partition on the device so we can ",
					"verify it's ok before setting it as the permanent boot device.",
				},
				Dependencies: []string{
					"Enable temp booting",
				},
				ExecName: "btpeer_temp_boot_into_partition",
				ExecExtraArgs: []string{
					"boot_partition_label:BOOT_B",
				},
				RunControl:  RunControl_ALWAYS_RUN,
				ExecTimeout: &durationpb.Duration{Seconds: 600},
			},
			"Set permanent boot partition as B": {
				Docs: []string{
					"Sets boot partition B as the default boot partition to use on reboot.",
				},
				ExecName: "btpeer_set_permanent_boot_partition",
				ExecExtraArgs: []string{
					"boot_partition_label:BOOT_B",
				},
				RunControl:  RunControl_ALWAYS_RUN,
				ExecTimeout: &durationpb.Duration{Seconds: 600},
			},
			"Verify newly installed image has an image UUID": {
				Docs: []string{
					"Reads the image UUID from the image build info file present on all ChromeOS Raspberry Pi btpeer OS image installations and stores it in the scope.",
					"Fails if unable to get the image UUID from the newly installed image release, as all image releases are expected to have UUIDs.",
				},
				ExecName: "btpeer_image_fetch_installed_uuid",
				ExecExtraArgs: []string{
					"allow_legacy_image:false",
				},
				RunControl: RunControl_ALWAYS_RUN,
			},
		},
	}
}
