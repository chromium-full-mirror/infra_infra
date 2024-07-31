// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"fmt"
)

// ProvisionBtpeerConfig creates configuration to provision a btpeer with a new OS.
//
// btpeerProvisionPlan provisions a btpeer with a new OS. The call flow is as followed:
//
//  1. If the device is not already partitioned for AB updates, then re-partition is by:
//     a. Verify device has space for new B partitions
//     b. Verify Verify device has standard partition scheme
//     c. Enable initrd
//     d. Shrink rootfs
//     e. Disable initrd
//     f. Create new AB Partitions
//
//  2. If the device is A/B partitioned
//     a. Temp Boot into A partition
//     b. Set permanent boot partition as A
//     c. Download OS Image to  device
//     d. Flash B partitions with new OS Image
//     e. Temp Boot into B partition
//     f. Set permanent boot partition as B
func ProvisionBtpeerConfig(imagePath string) *Configuration {
	const downloadAction = "Download expected OS image release to device"

	plan := btpeerRepairPlan()
	plan.CriticalActions = []string{"Provision OS to expected image release"}
	plan.GetActions()[downloadAction].ExecExtraArgs = []string{fmt.Sprintf("image_path:%s", imagePath)}
	return &Configuration{
		PlanNames: []string{
			PlanBluetoothPeer,
		},
		Plans: map[string]*Plan{
			PlanBluetoothPeer: plan,
		},
	}
}
