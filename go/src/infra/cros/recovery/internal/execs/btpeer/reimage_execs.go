// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package btpeer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cros/recovery/dev"
	"go.chromium.org/infra/cros/recovery/internal/components/btpeer"
	"go.chromium.org/infra/cros/recovery/internal/components/btpeer/image"
	"go.chromium.org/infra/cros/recovery/internal/components/cache"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/execs/wifirouter/ssh"
	"go.chromium.org/infra/cros/recovery/internal/localtlw/localproxy"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/internal/retry"
	"go.chromium.org/infra/cros/recovery/tlw"
)

const (
	// rootASizeGB is the size of the new A partition rootFS after shrinking (in GB).
	rootASizeGB = 16
	// bPartitionsSizeGB is the combined space for the B root+boot partitions (in GB), this
	// is smaller than A to make sure that we can fit it all on a 32GB SD card.
	bPartitionsSizeGB = 12
	// localOSImageStorePath is the path to the local image file after downloading it to the device.
	localOSImageStorePath = "/tmp/rpi.img"
	// Filesystem labels.
	bootALabel = "BOOT_A"
	rootALabel = "ROOT_A"
	bootBLabel = "BOOT_B"
	rootBLabel = "ROOT_B"

	// The GCS bucket and folder which all btpeer images are stored under.
	imageBaseGCSPath     = "gs://chromeos-connectivity-test-artifacts/btpeer/raspios-cros-btpeer/"
	localImagePathPrefix = "file://"
)

// enableInitrdExec enables initrd on the btpeer.
func enableInitrdExec(ctx context.Context, info *execs.ExecInfo) error {
	runner := btpeer.NewSshRunner(info.GetAccess(), info.GetActiveResource())

	// Enable building initrd images in the kernel.
	if err := btpeer.AddLineToFile(ctx, runner.Run, "/etc/default/raspberrypi-kernel", "INITRD=Yes"); err != nil {
		return errors.Annotate(err, "enable initrd: failed to enable initrd building on Raspberry Pi").Err()
	}

	if err := btpeer.BuildInitrd(ctx, runner.Run); err != nil {
		return errors.Annotate(err, "enable initrd: failed to build initial initrd image").Err()
	}

	bootPath, err := btpeer.GetCurrentBootPath(ctx, runner.Run)
	if err != nil {
		return errors.Annotate(err, "enable initrd: failed to get current boot path").Err()
	}

	// Tell the kernel to use initrd image that we just built and renamed.
	if err := btpeer.AddLineToFile(ctx, runner.Run, bootPath+"config.txt", "initramfs initrd.img followkernel"); err != nil {
		return errors.Annotate(err, "enable initrd: failed to build initial initrd image").Err()
	}

	argsMap := info.GetActionArgs(ctx)
	rebootTime := argsMap.AsDuration(ctx, "wait_reboot", 300, time.Second)
	if err := ssh.Reboot(ctx, runner, 10*time.Second, 10*time.Second, rebootTime); err != nil {
		return errors.Annotate(err, "enable initrd: failed to reboot btpeer").Err()
	}

	// Check that we find initrd messages in the dmesg after rebooting.
	if _, err := runner.Run(ctx, 30*time.Second, "dmesg -T | grep \"initrd\""); err != nil {
		return errors.Annotate(err, "enable initrd: failed to verify initrd is enabled on device after reboot").Err()
	}

	return nil
}

// disableInitrdExec disables initrd/initramfs on the btpeer.
func disableInitrdExec(ctx context.Context, info *execs.ExecInfo) error {
	runner := btpeer.NewSshRunner(info.GetAccess(), info.GetActiveResource())

	bootPath, err := btpeer.GetCurrentBootPath(ctx, runner.Run)
	if err != nil {
		return errors.Annotate(err, "disable initrd: failed to get current boot path").Err()
	}

	// Remove initramfs line from boot config.
	if err := btpeer.RemoveLineFromFile(ctx, runner.Run, bootPath+"config.txt", "initramfs initrd.img followkernel"); err != nil {
		return errors.Annotate(err, "disable initrd: failed to build initial initrd image").Err()
	}

	argsMap := info.GetActionArgs(ctx)
	rebootTime := argsMap.AsDuration(ctx, "wait_reboot", 300, time.Second)
	if err := ssh.Reboot(ctx, runner, 10*time.Second, 10*time.Second, rebootTime); err != nil {
		return errors.Annotate(err, "disable initr: failed to reboot btpeer").Err()
	}

	// Check that the initrd message is not in dmesg.
	if _, err := runner.Run(ctx, 30*time.Second, "dmesg -T | grep \"initrd\""); err == nil {
		return errors.Annotate(err, "disable initrd: failed to verify initrd is disabled on device after reboot").Err()
	}

	return nil
}

// shrinkRootFSExec shrinks the root partition using a pre-mount hook that is executed
// prior to mounting the root partition.
func shrinkRootFSExec(ctx context.Context, info *execs.ExecInfo) error {
	runner := btpeer.NewSshRunner(info.GetAccess(), info.GetActiveResource())

	argsMap := info.GetActionArgs(ctx)
	rootSize := int64(argsMap.AsInt(ctx, "root_a_size", rootASizeGB) * 1e9)

	clean, err := btpeer.CreateShrinkInitrdHook(ctx, runner.Run, rootSize)
	if err != nil {
		return errors.Annotate(err, "shrink rootfs: failed to create shrink hook").Err()
	}

	// Reboot device to run hook.
	if err := ssh.Reboot(ctx, runner, 10*time.Second, 10*time.Second, 10*time.Minute); err != nil {
		return errors.Annotate(err, "shrink rootfs: failed to reboot device").Err()
	}

	if err := clean(ctx); err != nil {
		return errors.Annotate(err, "shrink rootfs: failed to clean up shrink hook").Err()
	}
	return nil
}

// hasRoomToPartition verifies that the device has enough unused space to create the A/B partitioning.
func hasRoomToPartitionExec(ctx context.Context, info *execs.ExecInfo) error {
	runner := btpeer.NewSshRunner(info.GetAccess(), info.GetActiveResource())

	argsMap := info.GetActionArgs(ctx)
	rootSize := int64(argsMap.AsInt(ctx, "root_a_size", rootASizeGB) * 1e9)

	usedFSSpace, err := btpeer.GetUsedFSSpace(ctx, runner.Run, "/")
	if err != nil {
		return errors.Reason("has room to partition: failed to get current used bytes").Err()
	} else if usedFSSpace > rootSize {
		return errors.Reason("has room to partition: current file system %dB is too large to resize, need %dB", usedFSSpace, rootSize).Err()
	}
	log.Infof(ctx, "ROOT_A used space: %dB", usedFSSpace)

	helper, err := btpeer.NewPartitionHelperForPath(ctx, runner.Run, "/")
	if err != nil {
		return errors.Annotate(err, "has room to partition: failed to create partition helper").Err()
	}

	freeSpace, err := helper.GetFreeSpace(ctx)
	if err != nil {
		return errors.Annotate(err, "has room to partition: failed to create partition helper").Err()
	}
	log.Infof(ctx, "ROOT_A partition free space: %dB", freeSpace)

	partitionSize, err := helper.GetPartitionSize(ctx, "/")
	if err != nil {
		return errors.Annotate(err, "has room to partition: failed to get current partition size").Err()
	}
	log.Infof(ctx, "ROOT_A partition size: %dB", partitionSize)

	// Calculate final space available for the new partition.
	newPartitionSize := int64(argsMap.AsInt(ctx, "b_partition_size", bPartitionsSizeGB) * 1e9)
	partitionSpace := partitionSize + freeSpace - rootSize
	if partitionSpace < newPartitionSize {
		return errors.Reason("has room to partition: not enough room for new partitions, have %dB space available, want %dB", partitionSpace, newPartitionSize).Err()
	}

	return nil
}

// hasStandardPartitioningExec checks that the device has the normal OS partition scheme consisting of
// 1 boot (fat32) and 1 rootfs (ext4) partition.
func hasStandardPartitioningExec(ctx context.Context, info *execs.ExecInfo) error {
	runner := btpeer.NewSshRunner(info.GetAccess(), info.GetActiveResource())
	helper, err := btpeer.NewPartitionHelperForPath(ctx, runner.Run, "/")
	if err != nil {
		return errors.Annotate(err, "has standard partitioning: failed to create partition helper").Err()
	}

	parts, err := helper.GetPartitionInfo(ctx, "%", true)
	if err != nil {
		return errors.Annotate(err, "has standard partitioning: failed to get partition info").Err()
	}

	if len(parts) != 2 {
		return errors.Reason("has standard partitioning: expected 2 partitions, got %d", len(parts)).Err()
	}
	if !strings.EqualFold(parts[0].Type, "fat32") {
		return errors.Reason("has standard partitioning: expected first partition type fat32, got %q", parts[0].Type).Err()
	}
	if !strings.EqualFold(parts[1].Type, "ext4") {
		return errors.Reason("has standard partitioning: expected second partition type ext4, got %q", parts[1].Type).Err()
	}

	return nil
}

// createPartitionsExec creates two new "B"  partitions in the free space on the
// btpeer so the device can be AB updated.
//
// The final partioning looks like:
//
//	BOOT_A(FAT32) - existing partition
//	ROOT_A(EXT4)  - existing partition
//	BOOT_B(FAT32) - newly created
//	ROOT_B(EXT4)  - newly created
//
// The A partitions correspond to the "recovery" partitions while the B partitions
// are the ones that the B new OS versions will actually be flashed onto.
func createPartitionsExec(ctx context.Context, info *execs.ExecInfo) error {
	runner := btpeer.NewSshRunner(info.GetAccess(), info.GetActiveResource())
	helper, err := btpeer.NewPartitionHelperForPath(ctx, runner.Run, "/")
	if err != nil {
		return errors.Annotate(err, "create partitions: failed to create partition helper").Err()
	}

	bootPath, err := btpeer.GetCurrentBootPath(ctx, runner.Run)
	if err != nil {
		return errors.Annotate(err, "createn partitions: failed to get current boot path").Err()
	}

	// Add label to existing BOOT_A partition.
	if err := btpeer.SetLabelForFAT32Path(ctx, runner.Run, bootPath, bootALabel); err != nil {
		return errors.Annotate(err, "create partitions: failed to set BOOT_A label").Err()
	}

	// Add label to existing ROOT_A partition.
	if err := btpeer.SetLabelForEXTPath(ctx, runner.Run, "/", rootALabel); err != nil {
		return errors.Annotate(err, "create partitions: failed to set ROOT_A label").Err()
	}

	// Create the B boot partition with a fixed size of 1GB.
	if bootB, err := helper.CreateNewPrimaryPartitionBytes(ctx, "fat32", int64(1e9)); err != nil {
		return errors.Annotate(err, "create partitions: failed to create boot partition").Err()
		// Initialize an empty filesystem on the newly created boot partition and add the BOOT_B label.
	} else if err := btpeer.InitFAT32FS(ctx, runner.Run, bootB, bootBLabel); err != nil {
		return errors.Annotate(err, "create partitions: failed initialize ROOT_B fs").Err()
	}

	// Create the B root partition using all of the remaining free space.
	if rootB, err := helper.CreateNewPrimaryPartitionPercent(ctx, "ext4", 100); err != nil {
		return errors.Annotate(err, "create partitions: failed to create root partition").Err()
		// Initialize an empty filesystem on the newly created root partition and add the ROOT_B label.
	} else if err := btpeer.InitEXT4FS(ctx, runner.Run, rootB, rootBLabel); err != nil {
		return errors.Annotate(err, "create partitions: failed initialize ROOT_B fs").Err()
	}

	return nil
}

// downloadImageExec downloads an OS Image to the btpeer from GCS through the
// cache server and decompresses it. Once complete, the decompressed image will
// be located at localOSImageStorePath.
//
// Supports xz (*.img.xz) and gz (*.img.gz) compressed images based on file
// extension.
//
// Image file must be stored in GCS under imageBaseGCSPath.
//
// If the image_path exec arg is unset and the expected image config is
// specified in the exec scope, the expected image's path will be used. Will
// fail if neither the image_path is set nor expected image config is specified
// in the exec scope.
func downloadImageExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	runner := info.DefaultRunner()
	externalDownloadURL := argsMap.AsString(ctx, "image_path", "")
	downloadTimeout := argsMap.AsDuration(ctx, "download_timeout", 600, time.Second)
	decompressTimeout := argsMap.AsDuration(ctx, "decompress_timeout", 600, time.Second)

	// Validate/parse image URL.
	if externalDownloadURL == "" {
		btpeerScopeState, err := getBtpeerScopeState(ctx, info)
		if err != nil {
			return errors.Annotate(err, "download image: failed to get btpeer scope state").Err()
		}
		expectedImageConfig := btpeerScopeState.GetRaspiosCrosBtpeerImage().GetExpectedImageConfig()
		if expectedImageConfig == nil {
			return errors.Reason("download image: required image_path argument missing and no expected image config specified in btpeer scope state").Err()
		}
		if expectedImageConfig.GetUuid() == "" || expectedImageConfig.GetPath() == "" {
			return errors.Reason("download image: expected image present in btpeer scope state, but is invalid: %s", expectedImageConfig).Err()
		}
		logging.Infof(
			ctx,
			"Download image exec arg image_path unset, downloading expected image from btpeer scope state with UUID %q and path %q",
			expectedImageConfig.GetUuid(),
			expectedImageConfig.GetPath(),
		)
		externalDownloadURL = expectedImageConfig.GetPath()
	}

	var xzCompression bool
	downloadDst := localOSImageStorePath
	if strings.HasSuffix(externalDownloadURL, ".img.xz") {
		xzCompression = true
		downloadDst += ".xz"
	} else if strings.HasSuffix(externalDownloadURL, ".img.gz") {
		xzCompression = false
		downloadDst += ".gz"
	} else {
		return errors.Reason("download image: image %q not identified as having xz or gz compression", externalDownloadURL).Err()
	}

	switch {
	case strings.HasPrefix(externalDownloadURL, localImagePathPrefix):
		log.Infof(ctx, "Using image from local system, path: %q", externalDownloadURL)
		if !dev.IsActive(ctx) {
			return errors.Reason("download image: can only use local images when running in dev mode").Err()
		}
		// Local file, copy it to the device.
		req := &tlw.CopyRequest{
			PathSource:      strings.TrimPrefix(externalDownloadURL, localImagePathPrefix),
			Resource:        localproxy.BuildAddr(info.GetActiveResource()),
			PathDestination: downloadDst,
		}
		if err := info.GetAccess().CopyFileTo(ctx, req); err != nil {
			return errors.Annotate(err, "download image: failed to copy local image to device").Err()
		}
	case strings.HasPrefix(externalDownloadURL, imageBaseGCSPath):
		log.Infof(ctx, "Using image from GCS bucket, path: %q", externalDownloadURL)
		// Download compressed image through cache server.
		cacheDownloadURL, err := info.GetAccess().GetCacheUrl(ctx, info.GetDut().Name, externalDownloadURL)
		if err != nil {
			return errors.Annotate(err, "failed to get download URL from cache server for file path %q", externalDownloadURL).Err()
		}
		if _, err := cache.CurlFile(ctx, runner, cacheDownloadURL, downloadDst, downloadTimeout); err != nil {
			return errors.Annotate(err, "failed to download image %q to btpeer at %q", externalDownloadURL, downloadDst).Err()
		}
	default:
		return errors.Reason("download image: image_path expected to be located in: %q or %q, got %q",
			imageBaseGCSPath, localImagePathPrefix, externalDownloadURL).Err()
	}

	// Decompress image in-place (removes compression file extension).
	if xzCompression {
		if _, err := runner(ctx, decompressTimeout, "unxz", downloadDst); err != nil {
			return errors.Annotate(err, "download image: failed to extract xz image").Err()
		}
	} else {
		// Is gz compression.
		if _, err := runner(ctx, decompressTimeout, "gzip", "-d", downloadDst); err != nil {
			return errors.Annotate(err, "download image: failed to extract gz image").Err()
		}
	}

	return nil
}

// hasPartitionsWithLabelsExec verifies that partitions can be found with the
// requested labels.
//
// Use required exec arg "labels" to specify the expected labels.
//
// Use optional exec arg "expect_match" to specify if the partitions are
// expected to exist or not (default is true). If expect_match is false, this
// exec will pass if it fails to confirm that the device has partitions with all
// the labels.
func hasPartitionsWithLabelsExec(ctx context.Context, info *execs.ExecInfo) error {
	runner := btpeer.NewSshRunner(info.GetAccess(), info.GetActiveResource())
	argsMap := info.GetActionArgs(ctx)
	labels := argsMap.AsStringSlice(ctx, "labels", []string{})
	expectMatch := argsMap.AsBool(ctx, "expect_match", true)
	if len(labels) == 0 {
		return errors.Reason("has partitions with labels: required argument: 'labels' not provided").Err()
	}
	var device string
	var err error
	for _, label := range labels {
		device, err = btpeer.GetFSWithLabel(ctx, runner.Run, label)
		if err != nil {
			err = errors.Annotate(err, "failed to find partition with label: %q", label).Err()
			break
		}
		log.Infof(ctx, "Found device: %q with label: %q", label, device)
	}
	if expectMatch {
		return errors.Annotate(err, "has partitions with labels: failed to confirm that partitions with all labels exist as expected (expect_match=true)").Err()
	}
	if err == nil {
		return errors.Reason("has partitions with labels: device not expected to have partitions with all labels (expect_match=false)").Err()
	}
	logging.Infof(ctx, "Successfully failed to confirm that partitions with all labels exist as expected (expect_match=false): %v", err)
	return nil
}

// provisionExec flashes a raspberry pi image to the ROOT_B/BOOT_B partitions.
func provisionExec(ctx context.Context, info *execs.ExecInfo) error {
	runner := btpeer.NewSshRunner(info.GetAccess(), info.GetActiveResource())
	argsMap := info.GetActionArgs(ctx)
	flashBootTimeout := argsMap.AsDuration(ctx, "flash_boot_timeout", 300, time.Second)
	flashRootTimeout := argsMap.AsDuration(ctx, "flash_root_timeout", 1800, time.Second)

	// First find the partitions we are going to flash.
	rootB, err := btpeer.GetFSWithLabel(ctx, runner.Run, rootBLabel)
	if err != nil {
		return errors.Annotate(err, "provision: failed to find ROOT_B partition").Err()
	}
	bootB, err := btpeer.GetFSWithLabel(ctx, runner.Run, bootBLabel)
	if err != nil {
		return errors.Annotate(err, "provision: failed to find BOOT_B partition").Err()
	}

	// Load the image loopback device so we can interact with it as if it's just a normal device.
	device, err := btpeer.LoadImageAsLoopbackDevice(ctx, runner.Run, localOSImageStorePath)
	if err != nil {
		return errors.Annotate(err, "provision: failed to get and associate image").Err()
	}

	imgBootDev := fmt.Sprintf("%sp1", device)
	imgRootDev := fmt.Sprintf("%sp2", device)

	// Change partition labels on image before flashing.
	// Do this here so if something goes wrong during the flashing the partition
	// The labels will still exist so we can find the correct partition next time.
	if err := btpeer.SetEXTLabel(ctx, runner.Run, imgRootDev, rootBLabel); err != nil {
		return errors.Annotate(err, "provision: failed to label ROOT_B").Err()
	}

	if err := btpeer.SetFAT32Label(ctx, runner.Run, imgBootDev, bootBLabel); err != nil {
		return errors.Annotate(err, "provision: failed to label BOOT_B").Err()
	}

	// Flash boot partition.
	if err := btpeer.FlashImage(ctx, runner.Run, flashBootTimeout, imgBootDev, bootB); err != nil {
		return errors.Annotate(err, "provision: failed to flash BOOT_B parition").Err()
	}

	// Flash root partition.
	if err := btpeer.FlashImage(ctx, runner.Run, flashRootTimeout, imgRootDev, rootB); err != nil {
		return errors.Annotate(err, "provision: failed to flash ROOT_B parition").Err()
	}

	// Finally, update the partition IDs to match the new device.
	// Without this, the raspberry PI will search for partitions that match the UUID in the image since it's expecting
	// that we dd the entire .img to the device rather than individual partitions.
	// This is similar to what is already done in the default raspberry pi OS during first boot:
	// https://github.com/RPi-Distro/raspi-config/blob/bookworm/usr/lib/raspi-config/init_resize.sh#L90-L91

	// Mount the newly flashed partitions so we can update the files.
	imgBootMount, err := btpeer.MountDevice(ctx, runner.Run, bootB)
	if err != nil {
		return errors.Annotate(err, "update partition id: failed to mount image boot partition").Err()
	}

	imgRootMount, err := btpeer.MountDevice(ctx, runner.Run, rootB)
	if err != nil {
		return errors.Annotate(err, "update partition id: failed to mount image boot partition").Err()
	}

	// Replace old boot and root partition IDs in fstab.
	if err := btpeer.ReplacePartID(ctx, runner.Run, imgRootDev, rootB, imgRootMount+"/etc/fstab"); err != nil {
		return errors.Annotate(err, "update partition id: failed to replace old image root PART ID").Err()
	}

	if err := btpeer.ReplacePartID(ctx, runner.Run, imgBootDev, bootB, imgRootMount+"/etc/fstab"); err != nil {
		return errors.Annotate(err, "update partition id: failed to replace old image root PART ID").Err()
	}

	// Replace old root partition ID in cmdline.txt.
	if err := btpeer.ReplacePartID(ctx, runner.Run, imgRootDev, rootB, imgBootMount+"/cmdline.txt"); err != nil {
		return errors.Annotate(err, "update partition id: failed to replace old image root PART ID").Err()
	}

	return nil
}

// isRaspi4BLessThanRevision14Exec checks that the raspberry PI is a Model 4B with revision < 1.4.
func isRaspi4BLessThanRevision14Exec(ctx context.Context, info *execs.ExecInfo) error {
	runner := btpeer.NewSshRunner(info.GetAccess(), info.GetActiveResource())

	model, revision, err := btpeer.GetHWInfo(ctx, runner.Run)
	if err != nil {
		return errors.Annotate(err, "is less than revision 1.4: failed to get revision information").Err()
	}
	if model != btpeer.Model4B {
		return errors.Reason("is less than revision 1.4, unexpected model %v", model).Err()
	}
	if revision >= 4 {
		return errors.Reason("is less than revision 1.4, revision: %d is not less than 4", revision).Err()
	}
	log.Infof(ctx, "Btpeer revision %d", revision)
	return nil
}

// enableDebugQuirks sets the 'sdhci.debug_quirks2=4' kernel option which is required on older
// Raspberry Pi devices (Rev < 1.4) to allow temp booting into other partitions (b/336878507).
//
// Without this option, the temp boot bit does not persist after power down so the device
// will continue to boot into the same partition.
func enableDebugQuirksExec(ctx context.Context, info *execs.ExecInfo) error {
	runner := btpeer.NewSshRunner(info.GetAccess(), info.GetActiveResource())

	bootPath, err := btpeer.GetCurrentBootPath(ctx, runner.Run)
	if err != nil {
		return errors.Annotate(err, "enable debug quirks: failed to get current boot path").Err()
	}

	// cmdline.txt is a file with a single line that lists the args to be passed
	// to the kerenel on startup.
	filePath := bootPath + "cmdline.txt"

	// Check if the 'sdhci.debug_quirks2=4' option already exists in cmdline.txt.
	// If it's not present grep will fail with non-zero exit code.
	checkCmd := fmt.Sprintf("cat %q | grep 'sdhci.debug_quirks2=4'", filePath)
	if _, err := runner.Run(ctx, 15*time.Second, checkCmd); err == nil {
		log.Infof(ctx, "option already exists, no change applied")
		return nil
	}

	// If option doesn't already exist, append ' sdhci.debug_quirks2=4' to the first line in the file.
	optionCmd := fmt.Sprintf("sed -i ' 1 s/.*/& sdhci.debug_quirks2=4/' %q", filePath)
	if _, err := runner.Run(ctx, 15*time.Second, optionCmd); err != nil {
		return errors.Annotate(err, "enable debug quirks: failed to add debug line to: %q", filePath).Err()
	}
	return nil
}

// setTempBootPartitionExec temporarily boots into the specified partition.
//
// This is done prior to changing the permanent boot partition to verify that the
// partition is valid and bootable before making any permanent changes.
func setTempBootPartitionExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	partition := argsMap.AsString(ctx, "boot_partition_label", "")
	if partition == "" {
		return errors.Reason("set temp boot partition: required arg boot_partition_label not provided.").Err()
	}

	runner := btpeer.NewSshRunner(info.GetAccess(), info.GetActiveResource())
	rebootTime := argsMap.AsDuration(ctx, "wait_reboot", 300, time.Second)
	tempBoot := func() error {
		if err := btpeer.TempBootIntoPartition(ctx, runner.Run, partition, rebootTime); err != nil {
			return errors.Annotate(err, "failed to boot into requested partition").Err()
		}
		return nil
	}

	// In some cases, the device may reboot as part of the first-boot script and
	// end up booting back into the previous partition. Allow up to two attempts to boot
	// into the expected partition. This is mainly an issue on earlier revisions as the
	// tryboot bit will be cleared on restart.
	attempts := argsMap.AsInt(ctx, "retries", 2)
	if err := retry.LimitCount(ctx, attempts, time.Second, tempBoot, "boot in recovery mode"); err != nil {
		return errors.Annotate(err, "set temp boot partition:").Err()
	}
	return nil
}

// setPermanentBootPartitionExec permanently boots into the requested partition.
//
// See: https://github.com/raspberrypi/documentation/blob/develop/documentation/asciidoc/computers/config_txt/autoboot.adoc
func setPermanentBootPartitionExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	partition := argsMap.AsString(ctx, "boot_partition_label", "")
	if partition == "" {
		return errors.Reason("set temp boot partition: required arg boot_partition_label not provided.").Err()
	}

	rebootTime := argsMap.AsDuration(ctx, "wait_reboot", 300, time.Second)
	runner := btpeer.NewSshRunner(info.GetAccess(), info.GetActiveResource())
	// Raspberry PI bootloader will search the first FAT32 partition for
	// an autoboot.txt file which points to which partition contains the boot partition.
	// This is sometimes a separate RECOVERY partition, but for us it is just BOOT_A partition
	if err := btpeer.PermBootIntoPartition(ctx, runner.Run, bootALabel, partition, rebootTime); err != nil {
		return errors.Reason("set perm boot partition: failed to set perm boot partition.").Err()
	}
	return nil
}

// fetchImageReleaseConfigExec downloads the current production btpeer image
// release config from GCS through the btpeer host and stores it in the scope.
func fetchImageReleaseConfigExec(ctx context.Context, info *execs.ExecInfo) error {
	btpeerScopeState, err := getBtpeerScopeState(ctx, info)
	if err != nil {
		return errors.Annotate(err, "fetch image release config: failed to get btpeer scope state").Err()
	}
	if btpeerScopeState.GetRaspiosCrosBtpeerImage() == nil {
		return errors.Reason("identify expected btpeer image: invalid btpeer scope state: RaspiosCrosBtpeerImage is nil").Err()
	}
	releaseConfig, err := image.FetchBtpeerImageReleaseConfig(ctx, info.DefaultRunner())
	if err != nil {
		return errors.Annotate(err, "fetch image release config").Err()
	}
	configJSON, err := image.MarshalBtpeerImageReleaseConfig(releaseConfig)
	if err != nil {
		return errors.Annotate(err, "fetch image release config").Err()
	}
	log.Infof(ctx, "Successfully retrieved btpeer image release config:\n%s", configJSON)
	btpeerScopeState.GetRaspiosCrosBtpeerImage().ReleaseConfig = releaseConfig
	return nil
}

// fetchInstalledImageUUIDExec reads the image UUID from the image
// build info file present on all ChromeOS Raspberry Pi btpeer OS image
// installations and stores it in the scope. Will fail if no build info file
// is present unless the "allow_legacy_image" arg is true.
func fetchInstalledImageUUIDExec(ctx context.Context, info *execs.ExecInfo) error {
	const allowLegacyImageArg = "allow_legacy_image"
	argsMap := info.GetActionArgs(ctx)
	allowLegacyImage := argsMap.AsBool(ctx, allowLegacyImageArg, false)
	btpeerScopeState, err := getBtpeerScopeState(ctx, info)
	if err != nil {
		return errors.Annotate(err, "fetch installed btpeer image UUID: failed to get btpeer scope state").Err()
	}
	if btpeerScopeState.GetRaspiosCrosBtpeerImage() == nil {
		return errors.Reason("fetch installed btpeer image UUID: invalid btpeer scope state: RaspiosCrosBtpeerImage is nil").Err()
	}
	sshRunner := btpeer.NewSshRunner(info.GetAccess(), info.GetActiveResource())
	hasImageBuildInfoFile, err := image.BtpeerHasImageBuildInfoFile(ctx, sshRunner)
	if err != nil {
		return errors.Annotate(err, "fetch installed btpeer image UUID").Err()
	}
	if !hasImageBuildInfoFile {
		// No image file, assume that a legacy image is installed without an image UUID.
		logging.Infof(ctx, "Btpeer resource %q identified as having a legacy OS image and allow_legacy_image exec arg is %t", info.GetActiveResource(), allowLegacyImage)
		if allowLegacyImage {
			logging.Infof(ctx, "Skipping fetch of installed btpeer image UUID as legacy OS images do not have UUIDs (allow_legacy_image=true)")
			return nil
		}
		return errors.Reason("fetch installed btpeer image UUID: no image build info file found on host (allow_legacy_image=false)").Err()
	}
	buildInfo, err := image.FetchBtpeerImageBuildInfo(ctx, sshRunner)
	if err != nil {
		return errors.Annotate(err, "fetch installed btpeer image UUID: image build info file exists, but failed to read it").Err()
	}
	if buildInfo.GetImageUuid() == "" {
		return errors.Reason("fetch installed btpeer image UUID: image build info file exists, but is missing its ImageUuid").Err()
	}
	logging.Infof(ctx, "Btpeer resource %q installed OS image UUID is %q", info.GetActiveResource(), buildInfo.GetImageUuid())
	btpeerScopeState.GetRaspiosCrosBtpeerImage().InstalledImageUuid = buildInfo.GetImageUuid()
	return nil
}

// identifyExpectedImageExec identifies which btpeer image this
// specific btpeer should have installed based on the release config and the
// primary dut's hostname, and then stores it in the scope.
func identifyExpectedImageExec(ctx context.Context, info *execs.ExecInfo) error {
	btpeerScopeState, err := getBtpeerScopeState(ctx, info)
	if err != nil {
		return errors.Annotate(err, "identify expected btpeer image: failed to get btpeer scope state").Err()
	}
	releaseConfig := btpeerScopeState.GetRaspiosCrosBtpeerImage().GetReleaseConfig()
	if releaseConfig == nil {
		return errors.Reason("identify expected btpeer image: invalid btpeer scope state: RaspiosCrosBtpeerImage.ReleaseConfig is nil").Err()
	}
	dut := info.GetDut()
	if dut == nil {
		return errors.Reason("identify expected btpeer image: dut is nil").Err()
	}
	expectedImageConfig, err := image.SelectBtpeerImageForDut(ctx, releaseConfig, dut.Name)
	if err != nil {
		return errors.Annotate(err, "identify expected btpeer image: failed to select image for btpeer with primary dut hostname %q", dut.Name).Err()
	}
	if expectedImageConfig.GetUuid() == "" || expectedImageConfig.GetPath() == "" {
		return errors.Reason("identify expected btpeer image: image selected for btpeer with primary dut hostname %q, but is invalid: %s", dut.Name, expectedImageConfig).Err()
	}
	var expectedImageType string
	if expectedImageConfig.GetUuid() == releaseConfig.GetCurrentImageUuid() {
		expectedImageType = "current"
	} else {
		expectedImageType = "next"
	}
	log.Infof(
		ctx,
		"Selected %s image with UUID %q for btpeer resource %q with primary dut hostname %q",
		expectedImageType,
		expectedImageConfig.GetUuid(),
		info.GetActiveResource(),
		dut.Name,
	)
	btpeerScopeState.GetRaspiosCrosBtpeerImage().ExpectedImageConfig = expectedImageConfig
	return nil
}

// assertExpectedAndInstalledImageUUIDsMatchExec checks the btpeer scope state for
// the expected image UUID and installed image UUID and fails if they differ.
func assertExpectedAndInstalledImageUUIDsMatchExec(ctx context.Context, info *execs.ExecInfo) error {
	btpeerScopeState, err := getBtpeerScopeState(ctx, info)
	if err != nil {
		return errors.Annotate(err, "assert btpeer has expected image installed: failed to get btpeer scope state").Err()
	}
	actual := btpeerScopeState.GetRaspiosCrosBtpeerImage().GetInstalledImageUuid()
	if actual == "" {
		return errors.Reason("assert btpeer has expected image installed: invalid btpeer scope state: RaspiosCrosBtpeerImage.InstalledImageUuid is empty").Err()
	}
	expected := btpeerScopeState.GetRaspiosCrosBtpeerImage().GetExpectedImageConfig().GetUuid()
	if expected == "" {
		return errors.Reason("assert btpeer has expected image installed: invalid btpeer scope state: RaspiosCrosBtpeerImage.ExpectedImageConfig.Uuid is empty").Err()
	}
	if actual != expected {
		return errors.Reason("assert btpeer has expected image installed: expected %q != actual %q", expected, actual).Err()
	}
	return nil
}

func init() {
	execs.Register("btpeer_enable_initrd", enableInitrdExec)
	execs.Register("btpeer_disable_initrd", disableInitrdExec)
	execs.Register("btpeer_has_partition_room", hasRoomToPartitionExec)
	execs.Register("btpeer_shrink_rootfs", shrinkRootFSExec)
	execs.Register("btpeer_partition_device", createPartitionsExec)
	execs.Register("btpeer_has_partitions_with_labels", hasPartitionsWithLabelsExec)
	execs.Register("btpeer_device_has_standard_partitions", hasStandardPartitioningExec)
	execs.Register("btpeer_download_image", downloadImageExec)
	execs.Register("btpeer_set_permanent_boot_partition", setPermanentBootPartitionExec)
	execs.Register("btpeer_temp_boot_into_partition", setTempBootPartitionExec)
	execs.Register("btpeer_is_raspi_4b_less_than_revision_14", isRaspi4BLessThanRevision14Exec)
	execs.Register("btpeer_enable_debug_quirks", enableDebugQuirksExec)
	execs.Register("btpeer_provision_device", provisionExec)
	execs.Register("btpeer_image_fetch_release_config", fetchImageReleaseConfigExec)
	execs.Register("btpeer_image_fetch_installed_uuid", fetchInstalledImageUUIDExec)
	execs.Register("btpeer_image_identify_expected", identifyExpectedImageExec)
	execs.Register("btpeer_image_assert_expected_installed", assertExpectedAndInstalledImageUUIDsMatchExec)
}
