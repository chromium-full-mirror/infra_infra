// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"google.golang.org/protobuf/types/known/durationpb"
)

func wifiRouterRepairPlan() *Plan {
	return &Plan{
		CriticalActions: []string{
			// Initial actions to prepare for device-specific actions.
			"Set WifiRouter state to BROKEN",
			"Device is ping-able",
			"Identify and set WifiRouter device_type",

			// Actions only executed for specific device types.
			"Check ChromeOS Gale device",
			"Check OpenWrt device",
			"Check AsusWrt device",
			"Check Ubuntu device",

			// General final actions done for all device types.
			"Reboot device",
			"Set WifiRouter state to WORKING",
		},
		Actions: map[string]*Action{
			// Generic AP actions.
			"Set WifiRouter state to BROKEN": {
				Docs: []string{
					"Set the WifiRouter state to BROKEN",
				},
				ExecName: "wifi_router_set_state",
				ExecExtraArgs: []string{
					"state:BROKEN",
				},
				RunControl:    RunControl_RUN_ONCE,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set WifiRouter state to WORKING": {
				Docs: []string{
					"Set the WifiRouter state to WORKING",
				},
				ExecName: "wifi_router_set_state",
				ExecExtraArgs: []string{
					"state:WORKING",
				},
				RunControl:    RunControl_RUN_ONCE,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Device is ping-able": {
				ExecName:    "cros_ping",
				ExecTimeout: &durationpb.Duration{Seconds: 15},
			},
			"Identify and set WifiRouter device_type": {
				Docs: []string{
					"Identifies the device type of the test AP by probing the device over ssh.",
					"APs that fail to be identified as one of the supported device types will be left in a broken state.",
				},
				ExecName: "wifi_router_identify_device_type",
			},
			"Reboot device": {
				Docs: []string{
					"Reboots the device over ssh and waits for it to be ssh-able again.",
				},
				ExecName:    "wifi_router_reboot",
				ExecTimeout: &durationpb.Duration{Seconds: 200},
				RunControl:  RunControl_ALWAYS_RUN,
			},

			// Device type conditions.
			"Is ChromeOS Gale": {
				Docs: []string{
					"Checks if the router's device type is WIFI_ROUTER_DEVICE_TYPE_CHROMEOS_GALE.",
				},
				ExecName: "wifi_router_device_type_in_list",
				ExecExtraArgs: []string{
					"device_types:WIFI_ROUTER_DEVICE_TYPE_CHROMEOS_GALE",
				},
				RunControl:    RunControl_RUN_ONCE,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Is OpenWrt": {
				Docs: []string{
					"Checks if the router's device type is WIFI_ROUTER_DEVICE_TYPE_OPENWRT.",
				},
				ExecName: "wifi_router_device_type_in_list",
				ExecExtraArgs: []string{
					"device_types:WIFI_ROUTER_DEVICE_TYPE_OPENWRT",
				},
				RunControl:    RunControl_RUN_ONCE,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Is AsusWrt": {
				Docs: []string{
					"Checks if the router's device type is WIFI_ROUTER_DEVICE_TYPE_ASUSWRT.",
				},
				ExecName: "wifi_router_device_type_in_list",
				ExecExtraArgs: []string{
					"device_types:WIFI_ROUTER_DEVICE_TYPE_ASUSWRT",
				},
				RunControl:    RunControl_RUN_ONCE,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Is Ubuntu": {
				Docs: []string{
					"Checks if the router's device type is WIFI_ROUTER_DEVICE_TYPE_UBUNTU.",
				},
				ExecName: "wifi_router_device_type_in_list",
				ExecExtraArgs: []string{
					"device_types:WIFI_ROUTER_DEVICE_TYPE_UBUNTU",
				},
				RunControl:    RunControl_RUN_ONCE,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},

			// ChromeOS Gale actions.
			"Check ChromeOS Gale device": {
				Docs: []string{
					"Recovery checks preformed only for ChromeOS Gale router devices.",
				},
				Conditions: []string{
					"Is ChromeOS Gale",
				},
				Dependencies: []string{
					"Set Gale WifiRouter model and features",
					"Device is on stable-version",
					"Device has required wifi tools",
					"Device has 50 percent tmp diskspace",
					"Device has 50 percent stateful partition diskspace",
				},
				ExecName: "sample_pass",
			},
			"Device is on stable-version": {
				Docs: []string{
					"Checks the ChromeOS image on the Gale device to see if it is on the stable-version.",
				},
				ExecName: "cros_is_on_stable_version",
				ExecExtraArgs: []string{
					"device_type:wifi_router",
				},
				RecoveryActions: []string{
					"Provision Gale device to stable version",
				},
			},
			"Set Gale WifiRouter model and features": {
				Docs: []string{
					"All ChromeOS Gale devices are expected to have the same model and ",
					"features, so we can set these as hardcoded values.",
				},
				ExecName: "wifi_router_update_model_and_features",
			},
			"Device has required wifi tools": {
				Docs: []string{
					"check whether wifirouter critical tools present: ",
					"tcpdump, hostapd, dnsmasq, netperf, iperf, iw",
				},
				Dependencies: []string{
					"cros_access",
				},
				ExecName: "cros_is_tool_present",
				ExecExtraArgs: []string{
					"tools:tcpdump,hostapd,dnsmasq,netperf,iperf,iw",
				},
				RecoveryActions: []string{
					"Provision Gale device to stable version",
				},
			},
			"Provision Gale device to stable version": {
				Docs: []string{
					"Install wifirouter stable os.",
					"Currently only has one version",
				},
				ExecName: "cros_provision",
				ExecExtraArgs: []string{
					"device_type:wifi_router",
				},
				ExecTimeout: &durationpb.Duration{Seconds: 3600},
			},
			"Device has 50 percent tmp diskspace": {
				Docs: []string{
					"Check if there are more than 50 percent of diskspace in /tmp",
				},
				ExecName: "cros_has_enough_storage_space_percentage",
				ExecExtraArgs: []string{
					"path:/tmp",
					"expected:50",
				},
				RecoveryActions: []string{
					"Clean up tmp space",
				},
			},
			"Device has 50 percent stateful partition diskspace": {
				Docs: []string{
					"Check if there are more than 50 percent of diskspace in /mnt/stateful_partition",
				},
				ExecName: "cros_has_enough_storage_space_percentage",
				ExecExtraArgs: []string{
					"path:/mnt/stateful_partition",
					"expected:50",
				},
				RecoveryActions: []string{
					"Clean up stateful sub space",
				},
			},
			"Clean up tmp space": {
				Docs: []string{
					"Clean up tmp space",
				},
				Dependencies: []string{
					"Log usage report of tmp space",
				},
				ExecName: "cros_run_shell_command",
				ExecExtraArgs: []string{
					"rm -Rf /tmp/*",
				},
			},
			"Clean up stateful sub space": {
				Docs: []string{
					"Remove unneeded files in /mnt/stateful_partition that grow over time.",
					"Specifically './home/.shadow', './dev_image/telemetry', and './var/log/metrics/*'.",
				},
				Dependencies: []string{
					"Log usage report of stateful partition diskspace",
				},
				ExecName: "cros_run_shell_command",
				ExecExtraArgs: []string{
					"rm -Rf " +
						"/mnt/stateful_partition/home/.shadow " +
						"/mnt/stateful_partition/dev_image/telemetry " +
						"/mnt/stateful_partition/var/log/metrics/*", // Every reboot adds a metric.
				},
			},
			"Log usage report of tmp space": {
				Docs: []string{
					"Logs a usage report of the /tmp directory",
				},
				ExecName: "wifi_router_log_usage_report_of_dir",
				ExecExtraArgs: []string{
					"path:/tmp",
				},
				RunControl:             RunControl_ALWAYS_RUN,
				AllowFailAfterRecovery: true,
			},
			"Log usage report of stateful partition diskspace": {
				Docs: []string{
					"Logs a usage report of the /mnt/stateful_partition directory",
				},
				ExecName: "wifi_router_log_usage_report_of_dir",
				ExecExtraArgs: []string{
					"path:/mnt/stateful_partition",
				},
				RunControl:             RunControl_ALWAYS_RUN,
				AllowFailAfterRecovery: true,
			},

			// OpenWrt actions.
			"Check OpenWrt device": {
				Docs: []string{
					"Recovery checks preformed only for OpenWrt router devices.",
				},
				Conditions: []string{
					"Is OpenWrt",
				},
				Dependencies: []string{
					"Fetch OpenWrt OS image build info from device",
					"Fetch OpenWrt image config from GCS",
					"Identify expected OS image for this OpenWrt device",
					"Device has expected OpenWrt OS image",
					"Set WifiRouter model and features based on this OpenWrt device",
				},
				ExecName: "sample_pass",
			},
			"Fetch OpenWrt OS image build info from device": {
				Docs: []string{
					"Retrieves the OpenWrt OS image build info from the device and stores it in the controller state for later reference.",
				},
				ExecName: "wifi_router_openwrt_fetch_build_info",
			},
			"Fetch OpenWrt image config from GCS": {
				Docs: []string{
					"Retrieves the production OpenWrt image config from GCS and stores it in the controller state for later reference.",
				},
				ExecName:   "wifi_router_openwrt_fetch_config",
				RunControl: RunControl_RUN_ONCE,
			},
			"Identify expected OS image for this OpenWrt device": {
				Docs: []string{
					"Identifies the expected OS image for this OpenWrt device based off of its image build info, the image config, and the host.",
					"The UUID of the expected image is stored in the controller state for later reference.",
				},
				ExecName:   "wifi_router_openwrt_identify_expected_image",
				RunControl: RunControl_RUN_ONCE,
			},
			"Device has expected OpenWrt OS image": {
				Docs: []string{
					"Checks if the UUID of the image installed on the device (from the image build info) matches the expected image UUID.",
					"If the check fails, it attempts to recover by updating the image to the expected image.",
				},
				ExecName: "wifi_router_openwrt_has_expected_image",
				RecoveryActions: []string{
					"Update OpenWrt OS image with expected image",
					"Reboot then update OpenWrt OS image with expected image",
				},
			},
			"Update OpenWrt OS image with expected image": {
				Docs: []string{
					"Updates the device to the expected image by preforming a sysupgrade with the expected OS image binary.",
					"The archive containing the expected image binary is downloaded from GCS to the OpenWrt device through the cache server and then extracted and used directly on the device.",
					"Once the new image is installed, the image build info is re-retrieved from the device and this new copy replaces the image build info in the controller state.",
				},
				ExecName:    "wifi_router_openwrt_update_to_expected_image",
				ExecTimeout: &durationpb.Duration{Seconds: 600},
				RunControl:  RunControl_ALWAYS_RUN,
			},
			"Reboot then update OpenWrt OS image with expected image": {
				Docs: []string{
					"Reboots the device before trying to update it.",
					"This allows the update to succeed even if the /tmp dir started off corrupted, since rebooting refreshes it.",
				},
				Dependencies: []string{
					"Reboot device",
					"Update OpenWrt OS image with expected image",
				},
				ExecName: "sample_pass",
			},
			"Set WifiRouter model and features based on this OpenWrt device": {
				Docs: []string{
					"Sets the WifiRouter model and features based on the image build info.",
				},
				ExecName: "wifi_router_update_model_and_features",
			},

			// AsusWrt actions.
			"Check AsusWrt device": {
				Docs: []string{
					"Recovery checks preformed only for AsusWrt router devices.",
				},
				Conditions: []string{
					"Is AsusWrt",
				},
				Dependencies: []string{
					"Fetch model from AsusWrt device",
					"Update model and features based on this AsusWrt device",
				},
				ExecName: "sample_pass",
			},
			"Update model and features based on this AsusWrt device": {
				Docs: []string{
					"Sets model based on data read from the AsusWrt device and sets the ",
					"features based on known, hardcoded values based on model.",
				},
				ExecName: "wifi_router_update_model_and_features",
			},
			"Fetch model from AsusWrt device": {
				Docs: []string{
					"Retrieves the AsusWrt device's model name from the device and stores it in the controller state for later reference.",
				},
				ExecName: "wifi_router_asuswrt_fetch_model",
			},

			// Ubuntu actions.
			"Check Ubuntu device": {
				Docs: []string{
					"Recovery checks preformed only for Ubuntu router devices.",
				},
				Conditions: []string{
					"Is Ubuntu",
				},
				Dependencies: []string{
					"Fetch system product name from Ubuntu device",
					"Fetch network controller name from Ubuntu device",
					"Update model and features based on this Ubuntu device",
				},
				ExecName: "sample_pass",
			},
			"Update model and features based on this Ubuntu device": {
				Docs: []string{
					"Sets model based on data read from the Ubuntu device and sets the ",
					"features based on known, hardcoded values based on model.",
				},
				ExecName: "wifi_router_update_model_and_features",
			},
			"Fetch system product name from Ubuntu device": {
				Docs: []string{
					"Retrieves the Ubuntu device's system product name from the device and stores it in the controller state for later reference.",
				},
				ExecName: "wifi_router_ubuntu_fetch_system_product_name",
			},
			"Fetch network controller name from Ubuntu device": {
				Docs: []string{
					"Retrieves the Ubuntu device's network controller name from the device and stores it in the controller state for later reference.",
				},
				ExecName: "wifi_router_ubuntu_fetch_network_controller_name",
			},
		},
	}
}
