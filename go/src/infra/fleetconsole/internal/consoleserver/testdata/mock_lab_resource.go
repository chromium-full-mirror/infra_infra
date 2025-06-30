// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testdata

import (
	omnilab_pubsub "go.chromium.org/infra/fleetconsole/omnilab/omnilab-pubsub"
)

var MockLabResource = &omnilab_pubsub.MonitoredRecord{
	HostEntry: &omnilab_pubsub.MonitoredEntry{
		Identifier: map[string]string{
			"lab_name":     "atc",
			"test_harness": "MOBILEHARNESS",
			"hostname":     "at4-ao4.atc.google.com",
		},
		Attribute: []*omnilab_pubsub.Attribute{
			{
				Name:  "os",
				Value: "Linux",
			},
			{
				Name:  "host_os",
				Value: "Linux",
			},
			{
				Name:  "host_os_version",
				Value: "Ubuntu 22.04.4 LTS",
			},
			{
				Name:  "host_location",
				Value: "atc",
			},
			{
				Name:  "lab_type",
				Value: "Satellite",
			},
			{
				Name:  "harness_version",
				Value: "4.313.1",
			},
			{
				Name:  "tradefed_version",
				Value: "13358221",
			},
			{
				Name:  "java_version",
				Value: "21.0.6",
			},
			{
				Name:  "total_mem",
				Value: "62.66GB",
			},
			{
				Name:  "root_disk_space",
				Value: "455G",
			},
			{
				Name:  "host_group",
				Value: "atc:ate-main",
			},
			{
				Name:  "daemon_server_status",
				Value: "STOPPED",
			},
			{
				Name:  "status",
				Value: "ONLINE",
			},
		},
	},
	DeviceEntry: []*omnilab_pubsub.MonitoredEntry{
		{
			Identifier: map[string]string{
				"device_serial": "3A060DLJG00458",
			},
			Attribute: []*omnilab_pubsub.Attribute{
				{
					Name:  "run_target",
					Value: "husky",
				},
				{
					Name:  "hostname",
					Value: "at4-ao4.atc.google.com",
				},
				{
					Name:  "pool",
					Value: "android-test-executor",
				},
				{
					Name:  "cluster",
					Value: "platinum",
				},
				{
					Name:  "cluster",
					Value: "presubmit",
				},
				{
					Name:  "cluster",
					Value: "cts",
				},
				{
					Name:  "cluster",
					Value: "vts",
				},
				{
					Name:  "cluster",
					Value: "asit",
				},
				{
					Name:  "cluster",
					Value: "apct",
				},
				{
					Name:  "cluster",
					Value: "docker",
				},
				{
					Name:  "cluster",
					Value: "fuse-zip",
				},
				{
					Name:  "owner",
					Value: "android-hammock",
				},
				{
					Name:  "device_type",
					Value: "AndroidRealDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidFlashableDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidOnlineDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidDevice",
				},
				{
					Name:  "property",
					Value: "sdk_version=36",
				},
				{
					Name:  "property",
					Value: "model=pixel 8 pro",
				},
				{
					Name:  "property",
					Value: "revision=mp1.0",
				},
				{
					Name:  "hardware",
					Value: "husky",
				},
				{
					Name:  "build_type",
					Value: "userdebug",
				},
				{
					Name:  "internet",
					Value: "false",
				},
				{
					Name:  "brand",
					Value: "google",
				},
				{
					Name:  "platform",
					Value: "ANDROID",
				},
				{
					Name:  "status",
					Value: "ONLINE",
				},
			},
		},
		{
			Identifier: map[string]string{
				"device_serial": "37150DLJG0001A",
			},
			Attribute: []*omnilab_pubsub.Attribute{
				{
					Name:  "run_target",
					Value: "husky",
				},
				{
					Name:  "hostname",
					Value: "at4-ao4.atc.google.com",
				},
				{
					Name:  "pool",
					Value: "android-test-executor",
				},
				{
					Name:  "cluster",
					Value: "platinum",
				},
				{
					Name:  "cluster",
					Value: "presubmit",
				},
				{
					Name:  "cluster",
					Value: "cts",
				},
				{
					Name:  "cluster",
					Value: "vts",
				},
				{
					Name:  "cluster",
					Value: "asit",
				},
				{
					Name:  "cluster",
					Value: "apct",
				},
				{
					Name:  "cluster",
					Value: "docker",
				},
				{
					Name:  "cluster",
					Value: "fuse-zip",
				},
				{
					Name:  "owner",
					Value: "android-hammock",
				},
				{
					Name:  "device_type",
					Value: "AndroidRealDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidFlashableDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidOnlineDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidDevice",
				},
				{
					Name:  "property",
					Value: "sdk_version=36",
				},
				{
					Name:  "property",
					Value: "model=pixel 8 pro",
				},
				{
					Name:  "property",
					Value: "revision=mp1.0",
				},
				{
					Name:  "hardware",
					Value: "husky",
				},
				{
					Name:  "build_type",
					Value: "userdebug",
				},
				{
					Name:  "internet",
					Value: "false",
				},
				{
					Name:  "brand",
					Value: "google",
				},
				{
					Name:  "platform",
					Value: "ANDROID",
				},
				{
					Name:  "status",
					Value: "ONLINE",
				},
			},
		},
		{
			Identifier: map[string]string{
				"device_serial": "3A060DLJG003ML",
			},
			Attribute: []*omnilab_pubsub.Attribute{
				{
					Name:  "run_target",
					Value: "husky",
				},
				{
					Name:  "hostname",
					Value: "at4-ao4.atc.google.com",
				},
				{
					Name:  "pool",
					Value: "android-test-executor",
				},
				{
					Name:  "cluster",
					Value: "platinum",
				},
				{
					Name:  "cluster",
					Value: "presubmit",
				},
				{
					Name:  "cluster",
					Value: "cts",
				},
				{
					Name:  "cluster",
					Value: "vts",
				},
				{
					Name:  "cluster",
					Value: "asit",
				},
				{
					Name:  "cluster",
					Value: "apct",
				},
				{
					Name:  "cluster",
					Value: "docker",
				},
				{
					Name:  "cluster",
					Value: "fuse-zip",
				},
				{
					Name:  "owner",
					Value: "android-hammock",
				},
				{
					Name:  "device_type",
					Value: "AndroidRealDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidFlashableDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidOnlineDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidDevice",
				},
				{
					Name:  "property",
					Value: "sdk_version=36",
				},
				{
					Name:  "property",
					Value: "model=pixel 8 pro",
				},
				{
					Name:  "property",
					Value: "revision=mp1.0",
				},
				{
					Name:  "hardware",
					Value: "husky",
				},
				{
					Name:  "build_type",
					Value: "userdebug",
				},
				{
					Name:  "internet",
					Value: "false",
				},
				{
					Name:  "brand",
					Value: "google",
				},
				{
					Name:  "platform",
					Value: "ANDROID",
				},
				{
					Name:  "status",
					Value: "ONLINE",
				},
			},
		},
		{
			Identifier: map[string]string{
				"device_serial": "3B020DLJG00109",
			},
			Attribute: []*omnilab_pubsub.Attribute{
				{
					Name:  "run_target",
					Value: "husky",
				},
				{
					Name:  "hostname",
					Value: "at4-ao4.atc.google.com",
				},
				{
					Name:  "pool",
					Value: "android-test-executor",
				},
				{
					Name:  "cluster",
					Value: "platinum",
				},
				{
					Name:  "cluster",
					Value: "presubmit",
				},
				{
					Name:  "cluster",
					Value: "cts",
				},
				{
					Name:  "cluster",
					Value: "vts",
				},
				{
					Name:  "cluster",
					Value: "asit",
				},
				{
					Name:  "cluster",
					Value: "apct",
				},
				{
					Name:  "cluster",
					Value: "docker",
				},
				{
					Name:  "cluster",
					Value: "fuse-zip",
				},
				{
					Name:  "owner",
					Value: "android-hammock",
				},
				{
					Name:  "device_type",
					Value: "AndroidRealDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidFlashableDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidOnlineDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidDevice",
				},
				{
					Name:  "property",
					Value: "sdk_version=34",
				},
				{
					Name:  "property",
					Value: "model=pixel 8 pro",
				},
				{
					Name:  "property",
					Value: "revision=mp1.0",
				},
				{
					Name:  "hardware",
					Value: "husky",
				},
				{
					Name:  "build_type",
					Value: "userdebug",
				},
				{
					Name:  "internet",
					Value: "false",
				},
				{
					Name:  "brand",
					Value: "google",
				},
				{
					Name:  "platform",
					Value: "ANDROID",
				},
				{
					Name:  "status",
					Value: "ALLOCATED",
				},
			},
		},
		{
			Identifier: map[string]string{
				"device_serial": "38290DLJG001QU",
			},
			Attribute: []*omnilab_pubsub.Attribute{
				{
					Name:  "run_target",
					Value: "husky",
				},
				{
					Name:  "hostname",
					Value: "at4-ao4.atc.google.com",
				},
				{
					Name:  "pool",
					Value: "android-test-executor",
				},
				{
					Name:  "cluster",
					Value: "platinum",
				},
				{
					Name:  "cluster",
					Value: "presubmit",
				},
				{
					Name:  "cluster",
					Value: "cts",
				},
				{
					Name:  "cluster",
					Value: "vts",
				},
				{
					Name:  "cluster",
					Value: "asit",
				},
				{
					Name:  "cluster",
					Value: "apct",
				},
				{
					Name:  "cluster",
					Value: "docker",
				},
				{
					Name:  "cluster",
					Value: "fuse-zip",
				},
				{
					Name:  "owner",
					Value: "android-hammock",
				},
				{
					Name:  "device_type",
					Value: "AndroidRealDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidFlashableDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidOnlineDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidDevice",
				},
				{
					Name:  "property",
					Value: "sdk_version=34",
				},
				{
					Name:  "property",
					Value: "model=pixel 8 pro",
				},
				{
					Name:  "property",
					Value: "revision=mp1.0",
				},
				{
					Name:  "hardware",
					Value: "husky",
				},
				{
					Name:  "build_type",
					Value: "userdebug",
				},
				{
					Name:  "internet",
					Value: "false",
				},
				{
					Name:  "brand",
					Value: "google",
				},
				{
					Name:  "platform",
					Value: "ANDROID",
				},
				{
					Name:  "status",
					Value: "ALLOCATED",
				},
			},
		},
		{
			Identifier: map[string]string{
				"device_serial": "3A060DLJG001SU",
			},
			Attribute: []*omnilab_pubsub.Attribute{
				{
					Name:  "run_target",
					Value: "mustang",
				},
				{
					Name:  "hostname",
					Value: "at4-ao4.atc.google.com",
				},
				{
					Name:  "pool",
					Value: "android-test-executor",
				},
				{
					Name:  "cluster",
					Value: "platinum",
				},
				{
					Name:  "cluster",
					Value: "presubmit",
				},
				{
					Name:  "cluster",
					Value: "cts",
				},
				{
					Name:  "cluster",
					Value: "vts",
				},
				{
					Name:  "cluster",
					Value: "asit",
				},
				{
					Name:  "cluster",
					Value: "apct",
				},
				{
					Name:  "cluster",
					Value: "docker",
				},
				{
					Name:  "cluster",
					Value: "fuse-zip",
				},
				{
					Name:  "owner",
					Value: "android-hammock",
				},
				{
					Name:  "device_type",
					Value: "AndroidRealDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidFlashableDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidOnlineDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidDevice",
				},
				{
					Name:  "property",
					Value: "sdk_version=36",
				},
				{
					Name:  "property",
					Value: "model=pixel 8 pro",
				},
				{
					Name:  "property",
					Value: "revision=mp1.0",
				},
				{
					Name:  "hardware",
					Value: "mustang",
				},
				{
					Name:  "build_type",
					Value: "userdebug",
				},
				{
					Name:  "internet",
					Value: "false",
				},
				{
					Name:  "brand",
					Value: "google",
				},
				{
					Name:  "platform",
					Value: "ANDROID",
				},
				{
					Name:  "status",
					Value: "OFFLINE",
				},
			},
		},
		{
			Identifier: map[string]string{
				"device_serial": "37290DLJG001S5",
			},
			Attribute: []*omnilab_pubsub.Attribute{
				{
					Name:  "run_target",
					Value: "mustang",
				},
				{
					Name:  "hostname",
					Value: "at4-ao4.atc.google.com",
				},
				{
					Name:  "pool",
					Value: "android-test-executor",
				},
				{
					Name:  "cluster",
					Value: "platinum",
				},
				{
					Name:  "cluster",
					Value: "presubmit",
				},
				{
					Name:  "cluster",
					Value: "cts",
				},
				{
					Name:  "cluster",
					Value: "vts",
				},
				{
					Name:  "cluster",
					Value: "asit",
				},
				{
					Name:  "cluster",
					Value: "apct",
				},
				{
					Name:  "cluster",
					Value: "docker",
				},
				{
					Name:  "cluster",
					Value: "fuse-zip",
				},
				{
					Name:  "owner",
					Value: "android-hammock",
				},
				{
					Name:  "device_type",
					Value: "AndroidRealDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidFlashableDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidOnlineDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidDevice",
				},
				{
					Name:  "property",
					Value: "sdk_version=36",
				},
				{
					Name:  "property",
					Value: "model=pixel 8 pro",
				},
				{
					Name:  "property",
					Value: "revision=mp1.0",
				},
				{
					Name:  "hardware",
					Value: "mustang",
				},
				{
					Name:  "build_type",
					Value: "userdebug",
				},
				{
					Name:  "internet",
					Value: "false",
				},
				{
					Name:  "brand",
					Value: "google",
				},
				{
					Name:  "platform",
					Value: "ANDROID",
				},
				{
					Name:  "status",
					Value: "OFFLINE",
				},
			},
		},
		{
			Identifier: map[string]string{
				"device_serial": "3A060DLJG003WA",
			},
			Attribute: []*omnilab_pubsub.Attribute{
				{
					Name:  "run_target",
					Value: "mustang",
				},
				{
					Name:  "hostname",
					Value: "at4-ao4.atc.google.com",
				},
				{
					Name:  "pool",
					Value: "android-test-executor",
				},
				{
					Name:  "cluster",
					Value: "platinum",
				},
				{
					Name:  "cluster",
					Value: "presubmit",
				},
				{
					Name:  "cluster",
					Value: "cts",
				},
				{
					Name:  "cluster",
					Value: "vts",
				},
				{
					Name:  "cluster",
					Value: "asit",
				},
				{
					Name:  "cluster",
					Value: "apct",
				},
				{
					Name:  "cluster",
					Value: "docker",
				},
				{
					Name:  "cluster",
					Value: "fuse-zip",
				},
				{
					Name:  "owner",
					Value: "android-hammock",
				},
				{
					Name:  "device_type",
					Value: "AndroidRealDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidFlashableDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidOnlineDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidDevice",
				},
				{
					Name:  "property",
					Value: "sdk_version=34",
				},
				{
					Name:  "property",
					Value: "model=pixel 8 pro",
				},
				{
					Name:  "property",
					Value: "revision=mp1.0",
				},
				{
					Name:  "hardware",
					Value: "mustang",
				},
				{
					Name:  "build_type",
					Value: "userdebug",
				},
				{
					Name:  "internet",
					Value: "false",
				},
				{
					Name:  "brand",
					Value: "google",
				},
				{
					Name:  "platform",
					Value: "ANDROID",
				},
				{
					Name:  "status",
					Value: "MISSING",
				},
			},
		},
		{
			Identifier: map[string]string{
				"device_serial": "3B020DLJG000VK",
			},
			Attribute: []*omnilab_pubsub.Attribute{
				{
					Name:  "run_target",
					Value: "husky",
				},
				{
					Name:  "hostname",
					Value: "at4-ao4.atc.google.com",
				},
				{
					Name:  "pool",
					Value: "android-test-executor",
				},
				{
					Name:  "cluster",
					Value: "platinum",
				},
				{
					Name:  "cluster",
					Value: "presubmit",
				},
				{
					Name:  "cluster",
					Value: "cts",
				},
				{
					Name:  "cluster",
					Value: "vts",
				},
				{
					Name:  "cluster",
					Value: "asit",
				},
				{
					Name:  "cluster",
					Value: "apct",
				},
				{
					Name:  "cluster",
					Value: "docker",
				},
				{
					Name:  "cluster",
					Value: "fuse-zip",
				},
				{
					Name:  "owner",
					Value: "android-hammock",
				},
				{
					Name:  "device_type",
					Value: "AndroidRealDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidFlashableDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidOnlineDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidDevice",
				},
				{
					Name:  "property",
					Value: "sdk_version=36",
				},
				{
					Name:  "property",
					Value: "model=pixel 8 pro",
				},
				{
					Name:  "property",
					Value: "revision=mp1.0",
				},
				{
					Name:  "hardware",
					Value: "husky",
				},
				{
					Name:  "build_type",
					Value: "userdebug",
				},
				{
					Name:  "internet",
					Value: "false",
				},
				{
					Name:  "brand",
					Value: "google",
				},
				{
					Name:  "platform",
					Value: "ANDROID",
				},
				{
					Name:  "status",
					Value: "MISSING",
				},
			},
		},
		{
			Identifier: map[string]string{
				"device_serial": "3A070DLJG0005J",
			},
			Attribute: []*omnilab_pubsub.Attribute{
				{
					Name:  "run_target",
					Value: "husky",
				},
				{
					Name:  "hostname",
					Value: "at4-ao4.atc.google.com",
				},
				{
					Name:  "pool",
					Value: "android-test-executor",
				},
				{
					Name:  "cluster",
					Value: "platinum",
				},
				{
					Name:  "cluster",
					Value: "presubmit",
				},
				{
					Name:  "cluster",
					Value: "cts",
				},
				{
					Name:  "cluster",
					Value: "vts",
				},
				{
					Name:  "cluster",
					Value: "asit",
				},
				{
					Name:  "cluster",
					Value: "apct",
				},
				{
					Name:  "cluster",
					Value: "docker",
				},
				{
					Name:  "cluster",
					Value: "fuse-zip",
				},
				{
					Name:  "owner",
					Value: "android-hammock",
				},
				{
					Name:  "device_type",
					Value: "AndroidRealDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidFlashableDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidOnlineDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidDevice",
				},
				{
					Name:  "property",
					Value: "sdk_version=36",
				},
				{
					Name:  "property",
					Value: "model=pixel 8 pro",
				},
				{
					Name:  "property",
					Value: "revision=mp1.0",
				},
				{
					Name:  "hardware",
					Value: "husky",
				},
				{
					Name:  "build_type",
					Value: "userdebug",
				},
				{
					Name:  "internet",
					Value: "false",
				},
				{
					Name:  "brand",
					Value: "google",
				},
				{
					Name:  "platform",
					Value: "ANDROID",
				},
				{
					Name:  "status",
					Value: "OFFLINE",
				},
			},
		},
		{
			Identifier: map[string]string{
				"device_serial": "3B020DLJG00029",
			},
			Attribute: []*omnilab_pubsub.Attribute{
				{
					Name:  "run_target",
					Value: "mustang",
				},
				{
					Name:  "hostname",
					Value: "at4-ao4.atc.google.com",
				},
				{
					Name:  "pool",
					Value: "android-test-executor",
				},
				{
					Name:  "cluster",
					Value: "platinum",
				},
				{
					Name:  "cluster",
					Value: "presubmit",
				},
				{
					Name:  "cluster",
					Value: "cts",
				},
				{
					Name:  "cluster",
					Value: "vts",
				},
				{
					Name:  "cluster",
					Value: "asit",
				},
				{
					Name:  "cluster",
					Value: "apct",
				},
				{
					Name:  "cluster",
					Value: "docker",
				},
				{
					Name:  "cluster",
					Value: "fuse-zip",
				},
				{
					Name:  "owner",
					Value: "android-hammock",
				},
				{
					Name:  "device_type",
					Value: "AndroidRealDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidFlashableDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidOnlineDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidDevice",
				},
				{
					Name:  "property",
					Value: "sdk_version=36",
				},
				{
					Name:  "property",
					Value: "model=pixel 8 pro",
				},
				{
					Name:  "property",
					Value: "revision=mp1.0",
				},
				{
					Name:  "hardware",
					Value: "mustang",
				},
				{
					Name:  "build_type",
					Value: "userdebug",
				},
				{
					Name:  "internet",
					Value: "false",
				},
				{
					Name:  "brand",
					Value: "google",
				},
				{
					Name:  "platform",
					Value: "ANDROID",
				},
				{
					Name:  "status",
					Value: "FASTBOOT",
				},
			},
		},
		{
			Identifier: map[string]string{
				"device_serial": "3A060DLJG00269",
			},
			Attribute: []*omnilab_pubsub.Attribute{
				{
					Name:  "run_target",
					Value: "mustang",
				},
				{
					Name:  "hostname",
					Value: "at4-ao4.atc.google.com",
				},
				{
					Name:  "pool",
					Value: "android-test-executor",
				},
				{
					Name:  "cluster",
					Value: "platinum",
				},
				{
					Name:  "cluster",
					Value: "presubmit",
				},
				{
					Name:  "cluster",
					Value: "cts",
				},
				{
					Name:  "cluster",
					Value: "vts",
				},
				{
					Name:  "cluster",
					Value: "asit",
				},
				{
					Name:  "cluster",
					Value: "apct",
				},
				{
					Name:  "cluster",
					Value: "docker",
				},
				{
					Name:  "cluster",
					Value: "fuse-zip",
				},
				{
					Name:  "owner",
					Value: "android-hammock",
				},
				{
					Name:  "device_type",
					Value: "AndroidRealDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidFlashableDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidOnlineDevice",
				},
				{
					Name:  "device_type",
					Value: "AndroidDevice",
				},
				{
					Name:  "property",
					Value: "sdk_version=36",
				},
				{
					Name:  "property",
					Value: "model=pixel 8 pro",
				},
				{
					Name:  "property",
					Value: "revision=mp1.0",
				},
				{
					Name:  "hardware",
					Value: "mustang",
				},
				{
					Name:  "build_type",
					Value: "userdebug",
				},
				{
					Name:  "internet",
					Value: "false",
				},
				{
					Name:  "brand",
					Value: "google",
				},
				{
					Name:  "platform",
					Value: "ANDROID",
				},
				{
					Name:  "status",
					Value: "Unavailable",
				},
			},
		},
	},
}
