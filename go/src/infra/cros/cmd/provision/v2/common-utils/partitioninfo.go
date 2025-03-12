// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Partition constants and helpers
package common_utils

import (
	"fmt"
	"strings"
)

const (
	PartitionNumStateful = "1"
	PartitionNumKernelA  = "2"
	PartitionNumKernelB  = "4"
	PartitionNumRootA    = "3"
	PartitionNumRootB    = "5"
	PartitionNumMiniOSA  = "9"
	PartitionNumMiniOSB  = "10"
)

const (
	MiniOSUnsupportedGUIDPartition = "09845860-705F-4BB5-B16C-8A8A099CAF52"
)

const (
	StatefulPath           = "/mnt/stateful_partition"
	UpdateStatefulFilePath = StatefulPath + "/.update_available"
	// ProvisionMarker - This file acts as a flag to signal failed provisions.
	// As we create the file in stateful, that means that if provision
	// is successful it will be overwritten, meaning that the fact it exists
	// beyond a provision run means it must have failed.
	// This file should be created on every OS provision start.
	ProvisionMarker = "/var/tmp/provision_failed"
)

// partitionsInfo holds active/inactive root + kernel partition information.
type PartitionInfo struct {
	// The active + inactive kernel device partitions (e.g. /dev/nvme0n1p2).
	ActiveKernel      string
	ActiveKernelNum   string
	InactiveKernel    string
	InactiveKernelNum string
	// The active + inactive root device partitions (e.g. /dev/nvme0n1p3).
	ActiveRoot   string
	InactiveRoot string
	// The A + B miniOS device partitions.
	MiniOSA string
	MiniOSB string
	// The stateful partition path.
	Stateful string
}

// GetPartitionInfo retrieves relevant kernel and root info for a specific root
func GetPartitionInfo(root string, rootDisk string, rootPartNum string) PartitionInfo {
	// Determine the next kernel and root.
	rootDiskPartDelim := rootDisk + strings.TrimSuffix(strings.TrimPrefix(root, rootDisk), rootPartNum)
	switch rootPartNum {
	case PartitionNumRootA:
		return PartitionInfo{
			ActiveKernel:    rootDiskPartDelim + PartitionNumKernelA,
			ActiveKernelNum: PartitionNumKernelA,
			InactiveKernel:  rootDiskPartDelim + PartitionNumKernelB,
			ActiveRoot:      rootDiskPartDelim + PartitionNumRootA,
			InactiveRoot:    rootDiskPartDelim + PartitionNumRootB,
			MiniOSA:         rootDiskPartDelim + PartitionNumMiniOSA,
			MiniOSB:         rootDiskPartDelim + PartitionNumMiniOSB,
			Stateful:        rootDiskPartDelim + PartitionNumStateful,
		}
	case PartitionNumRootB:
		return PartitionInfo{
			ActiveKernel:    rootDiskPartDelim + PartitionNumKernelB,
			ActiveKernelNum: PartitionNumKernelB,
			InactiveKernel:  rootDiskPartDelim + PartitionNumKernelA,
			ActiveRoot:      rootDiskPartDelim + PartitionNumRootB,
			InactiveRoot:    rootDiskPartDelim + PartitionNumRootA,
			MiniOSA:         rootDiskPartDelim + PartitionNumMiniOSB,
			MiniOSB:         rootDiskPartDelim + PartitionNumMiniOSA,
			Stateful:        rootDiskPartDelim + PartitionNumStateful,
		}
	default:
		panic(fmt.Sprintf("Unexpected root partition number of %s", rootPartNum))
	}
}

// GetMiniOSPartitions returns the partition numbers for miniOS
func GetMiniOSPartitions() []string {
	return []string{PartitionNumMiniOSA, PartitionNumMiniOSB}
}
