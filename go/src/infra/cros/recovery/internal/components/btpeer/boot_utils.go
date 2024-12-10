// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package btpeer

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components"
	"infra/cros/recovery/internal/components/cros"
	"infra/cros/recovery/internal/log"
)

// TempBootIntoPartition temporarily boots into a specific partition.
//
// This is done by passing an additional argument to the "reboot" command which tells the system to
// use a different partition on next boot. This is temporary and only applies to the next boot.
func TempBootIntoPartition(ctx context.Context, runner components.Runner, bootPartitionLabel string, rebootTime time.Duration) error {
	// Get the partition with the matching label.
	bootPart, err := GetFSWithLabel(ctx, runner, bootPartitionLabel)
	if err != nil {
		return errors.Annotate(err, "set temp boot partition: failed to find boot partition device").Err()
	}

	// Get the partition number of the filesystem that we want to boot from.
	bootPartNumber, err := getPartitionNumber(ctx, runner, bootPart)
	if err != nil {
		return errors.Annotate(err, "set temp boot partition: failed to get boot partition number").Err()
	}

	// Reboot the device from the requested partition.
	cmd := fmt.Sprintf("reboot %d &", bootPartNumber)
	if _, err := runner(ctx, 15*time.Second, cmd); err != nil {
		return errors.Annotate(err, "set tmp boot partition: failed to reboot from partition %d", bootPartNumber).Err()
	}

	if err := waitForBootPartition(ctx, runner, bootPart, rebootTime); err != nil {
		return errors.Annotate(err, "set perm boot partition: failed to wait for the device to boot into the requested partition").Err()
	}
	return nil
}

// GetCurrentBootPath gets the mounted boot directory that the OS is using.
//
// Older versions of raspberry pi use /boot/, while newer versions use /boot/firmware.
// See: https://github.com/raspberrypi/documentation/blob/develop/documentation/asciidoc/computers/configuration/boot_folder.adoc
func GetCurrentBootPath(ctx context.Context, runner components.Runner) (string, error) {
	if _, err := runner(ctx, 15*time.Second, "test -d /boot/firmware/"); err == nil {
		return "/boot/firmware/", nil
	}
	log.Debugf(ctx, "/boot/firmware does not exist, checking for /boot/")

	if _, err := runner(ctx, 15*time.Second, "test -d /boot/"); err == nil {
		return "/boot/", nil
	}

	return "", errors.Reason("get current booth path: failed to find a mounted boot path").Err()
}

// PermBootIntoPartition sets a partition as the permenent boot partition.
//
// This is accomplished by writing a file "autoboot.txt" to the recovery partition which instructs the
// device as to which partition to boot from.
func PermBootIntoPartition(ctx context.Context, runner components.Runner, recoveryPartitionLabel, bootPartitionLabel string, rebootTime time.Duration) error {
	// Get the recovery partition which houses the autoboot.txt file.
	recoveryPart, err := GetFSWithLabel(ctx, runner, recoveryPartitionLabel)
	if err != nil {
		return errors.Annotate(err, "set perm boot partition: failed to find %q partition", recoveryPartitionLabel).Err()
	}

	bootPart, err := GetFSWithLabel(ctx, runner, bootPartitionLabel)
	if err != nil {
		return errors.Annotate(err, "set perm boot partition: failed to find %q partition", bootPartitionLabel).Err()
	}

	bootPartNumber, err := getPartitionNumber(ctx, runner, bootPart)
	if err != nil {
		return errors.Annotate(err, "set perm boot partition: failed to get partition number for partition: %q", bootPart).Err()
	}

	bootPath, err := GetCurrentBootPath(ctx, runner)
	if err != nil {
		return errors.Annotate(err, "set perm boot partition: failed to get current boot path").Err()
	}

	// Ensure that the partition that we're setting as the perm partition is the one we're
	// currently booted into so there's no risk of booting into the wrong partition.
	if dev, err := getMountDeviceForPath(ctx, runner, bootPath); err != nil {
		return errors.Annotate(err, "set perm boot partition: failed to get current boot device.").Err()
	} else if dev != bootPart {
		return errors.Reason("set perm boot partition: cannot change boot partition, booted into %q, expected %q.", dev, bootPart).Err()
	}

	// Mount the recovery device so we can write the autoboot.txt file.
	imgBootMount, err := MountDevice(ctx, runner, recoveryPart)
	if err != nil {
		return errors.Annotate(err, "set perm boot partition: failed to mount image boot partition").Err()
	}

	// Write the current boot partition to autoboot.txt to ensure we will reboot from this partition on next restart.
	tryBootText := fmt.Sprintf("[all]\ntryboot_a_b=1\nboot_partition=%d", bootPartNumber)
	filePath := imgBootMount + "/autoboot.txt"
	cmd := fmt.Sprintf("cat > %s <<\"EOF\"\n%s\nEOF", filePath, tryBootText)
	if _, err := runner(ctx, time.Minute, "sudo", "bash", "-c", cmd); err != nil {
		return errors.Annotate(err, "set perm boot partition: failed to write autoboot.txt file").Err()
	}

	// Only unmount if these are two separate partitions, otherwise we will be unmounting our current boot partition.
	if bootPartitionLabel != recoveryPartitionLabel {
		if err := unmountDevice(ctx, runner, recoveryPart); err != nil {
			// Don't error out since this step is non-critical as we will unmount after rebooting.
			log.Infof(ctx, "Failed to unmount partition %q", recoveryPart)
		}
	}

	// Reboot the device and ensure that we're using the correct partition.
	if _, err := runner(ctx, time.Minute, "reboot &"); err != nil {
		return errors.Annotate(err, "set tmp boot partition: failed to reboot from partition %d", bootPartNumber).Err()
	}

	if err := waitForBootPartition(ctx, runner, bootPart, rebootTime); err != nil {
		return errors.Annotate(err, "set perm boot partition: failed to wait for the device to boot into the requested partition").Err()
	}
	return nil
}

// waitForBootPartition waits for the device to reconnect after rebooting and verifies that it's booted from the expected partition.
func waitForBootPartition(ctx context.Context, runner components.Runner, bootPart string, timeout time.Duration) error {
	log.Debugf(ctx, "Waiting %s for host to start rebooting before reconnecting", 10*time.Second)
	time.Sleep(10 * time.Second)
	if err := cros.WaitUntilSSHable(ctx, timeout, 10*time.Second, runner); err != nil {
		return errors.Annotate(err, "wait for boot partition: device did not come back up after temp booting into partition").Err()
	}

	bootPath, err := GetCurrentBootPath(ctx, runner)
	if err != nil {
		return errors.Annotate(err, "set perm boot partition: failed to get current boot path").Err()
	}

	// Verify after rebooting that we're booted into the expected partition
	if dev, err := getMountDeviceForPath(ctx, runner, bootPath); err != nil {
		return errors.Annotate(err, "wait for boot partition: failed to get current boot device.").Err()
	} else if dev != bootPart {
		return errors.Reason("wait for boot partition: failed to verify boot partition, got %q, expected %q.", dev, bootPart).Err()
	}
	return nil
}
