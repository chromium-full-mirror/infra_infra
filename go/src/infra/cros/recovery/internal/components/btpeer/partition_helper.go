// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package btpeer

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// GetUsedFSSpace returns the amount of used space on the filesystem.
func GetUsedFSSpace(ctx context.Context, runner components.Runner, path string) (int64, error) {
	cmd := fmt.Sprintf("df %q --block-size=1 --output=used | sed 1d", path)
	if out, err := runner(ctx, time.Minute, cmd); err != nil {
		return 0, errors.Annotate(err, "get used space: failed to get filesystem usage stats using df").Err()
	} else if bytes, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64); err != nil {
		return 0, errors.Annotate(err, "get used space: failed to parse filesystem useage stats using df").Err()
	} else {
		return bytes, nil
	}
}

// FlashImage writes an image to a device using dd.
func FlashImage(ctx context.Context, runner components.Runner, timeout time.Duration, input string, outputDev string) error {
	// Ensure the output device is unmounted before we attempt to flash it as the machine may automatically
	// mount the drive even when it's not the boot/root device.
	if err := unmountDevice(ctx, runner, outputDev); err != nil {
		return errors.Annotate(err, "flash image: failed to unmount destination device before flashing").Err()
	}

	cmd := fmt.Sprintf("dd if=%q of=%q bs=4M", input, outputDev)
	if _, err := runner(ctx, timeout, cmd); err != nil {
		return errors.Annotate(err, "flash image: failed to flash image with dd").Err()
	}
	return nil
}

// LoadImageAsLoopbackDevice loads an image as a loopback device and returns the device.
func LoadImageAsLoopbackDevice(ctx context.Context, runner components.Runner, image string) (string, error) {
	cmd := fmt.Sprintf("losetup -f --show -P %s", image)
	dev, err := runner(ctx, 2*time.Minute, cmd)
	if err != nil {
		return "", errors.Annotate(err, "loop image: failed to load image to loopback device").Err()
	}
	return strings.TrimSpace(dev), nil
}

// getMountPoints returns the mount point of a device or an empty string if it is not mounted.
func getMountPoints(ctx context.Context, runner components.Runner, device string) ([]string, error) {
	// findmnt will return all listed mount points, but fall back to lsblk in event of an error since
	// findmnt does not differentiate between unknown device and mountpoint not found.
	cmd := fmt.Sprintf("findmnt %q -o target -n || lsblk -no mountpoint %q", device, device)
	out, err := runner(ctx, 15*time.Second, cmd)
	if err != nil {
		return nil, errors.Annotate(err, "get mount points: failed to get mount points for device %q", device).Err()
	}
	res := make([]string, 0)
	for _, mnt := range strings.Split(out, "\n") {
		mnt = strings.TrimSpace(mnt)
		if mnt != "" {
			res = append(res, mnt)
		}
	}
	return res, nil
}

// MountDevice returns a devices mount point if it's already mounted or otherwise mounts it and returns the new mount path.
func MountDevice(ctx context.Context, runner components.Runner, device string) (string, error) {
	if mounts, err := getMountPoints(ctx, runner, device); err == nil && len(mounts) > 0 {
		log.Infof(ctx, "Found existing points for device %q: %v", device, mounts)
		return mounts[0], nil
	}

	mount, err := runner(ctx, time.Minute, "mktemp -d")
	if err != nil {
		return "", errors.Annotate(err, "flash image: failed to flash image with dd").Err()
	}

	mount = strings.TrimSpace(mount)
	if _, err := runner(ctx, 2*time.Minute, "mount", device, mount); err != nil {
		return "", errors.Annotate(err, "flash image: failed to flash image with dd").Err()
	}
	return mount, nil
}

// unmountDevice unmounts a device if it is mounted.
func unmountDevice(ctx context.Context, runner components.Runner, device string) error {
	mounts, err := getMountPoints(ctx, runner, device)
	if err != nil {
		return errors.Annotate(err, "unmount device: failed to get mount point for device %q", device).Err()
	}

	log.Infof(ctx, "Found mount points for device %q: %v", device, mounts)
	for _, mount := range mounts {
		cmd := fmt.Sprintf("umount %q", mount)
		if _, err := runner(ctx, 2*time.Minute, cmd); err != nil {
			return errors.Annotate(err, "unmount device: failed to unmount device %q", device).Err()
		}
	}

	return nil
}

// GetFSWithLabel returns a device whose FS has the matching label.
func GetFSWithLabel(ctx context.Context, runner components.Runner, label string) (string, error) {
	// Use lsblk to list devices rather than findfs since we can filter out loopback devices.
	// Example output:
	/*
		PATH           LABEL
		/dev/ram15
		/dev/mmcblk0
		/dev/mmcblk0p1 boot
		/dev/mmcblk0p2 rootfs
	*/
	cmd := fmt.Sprintf("lsblk -l -o path,label -e 7 | grep %q | cut -d ' ' -f1", label)
	dev, err := runner(ctx, time.Minute, cmd)
	if err != nil {
		return "", errors.Annotate(err, "get fs with label: failed to find device with label %q", label).Err()
	} else if strings.TrimSpace(dev) == "" {
		return "", errors.Reason("get fs with label: failed to find device with label %q", label).Err()
	} else if len(strings.Split(strings.TrimSpace(dev), "\n")) > 1 {
		return "", errors.Reason("get fs with label: more than one device with label %q exists", label).Err()
	}

	return strings.TrimSpace(dev), nil
}

// SetLabelForEXTPath sets a label for a EXT file system given a path.
func SetLabelForEXTPath(ctx context.Context, runner components.Runner, path string, label string) error {
	device, err := getMountDeviceForPath(ctx, runner, path)
	if err != nil {
		return errors.Annotate(err, "set ext fs label: failed to get current root device").Err()
	}

	if err := SetEXTLabel(ctx, runner, device, label); err != nil {
		return errors.Annotate(err, "create partitions: failed to label ROOT_A").Err()
	}

	return nil
}

// SetLabelForFAT32Path sets a label for a FAT32 file system given a path.
func SetLabelForFAT32Path(ctx context.Context, runner components.Runner, path string, label string) error {
	device, err := getMountDeviceForPath(ctx, runner, path)
	if err != nil {
		return errors.Annotate(err, "create partitions: failed to get current boot device").Err()
	}

	if err := SetFAT32Label(ctx, runner, device, label); err != nil {
		return errors.Annotate(err, "create partitions: failed to label ROOT_A").Err()
	}

	return nil
}

// SetFAT32Label sets a label on a FAT32 FS.
func SetFAT32Label(ctx context.Context, runner components.Runner, device string, label string) error {
	cmd := fmt.Sprintf("fatlabel %q %q", device, label)
	if _, err := runner(ctx, 3*time.Minute, cmd); err != nil {
		return errors.Annotate(err, "set fat32 label: failed to set label via fatlabel").Err()
	}
	return nil
}

// SetEXTLabel sets a label on an EXT FS.
func SetEXTLabel(ctx context.Context, runner components.Runner, device string, label string) error {
	cmd := fmt.Sprintf("e2label %q %q", device, label)
	if _, err := runner(ctx, 10*time.Minute, cmd); err != nil {
		return errors.Annotate(err, "set ext label: failed to set label via e2label").Err()
	}
	return nil
}

// InitFAT32FS initializes a FAT32 FS on a new partition.
func InitFAT32FS(ctx context.Context, runner components.Runner, device, label string) error {
	cmd := fmt.Sprintf("mkfs.fat %s -F 32 -n %s", device, label)
	if _, err := runner(ctx, 6*time.Minute, cmd); err != nil {
		return errors.Annotate(err, "make ext4 fs: failed to initialize fs").Err()
	}
	return nil
}

// InitEXT4FS initializes an EXT4 FS on a new partition.
func InitEXT4FS(ctx context.Context, runner components.Runner, device, label string) error {
	cmd := fmt.Sprintf("mkfs.ext4 %s -F -L %s", device, label)
	if _, err := runner(ctx, 6*time.Minute, cmd); err != nil {
		return errors.Annotate(err, "make ext4 fs: failed to initialize fs").Err()
	}
	return nil
}

// getPartitionID gets the UUID of a partition/device.
func getPartitionID(ctx context.Context, runner components.Runner, device string) (string, error) {
	cmd := fmt.Sprintf("blkid %q -s PARTUUID -o value", device)
	dev, err := runner(ctx, time.Minute, cmd)
	if err != nil {
		return "", errors.Annotate(err, "get device for path: failed to get mount with findmnt").Err()
	}
	return strings.TrimSpace(dev), nil
}

// ReplacePartID replaces the old partiion ID with the new one in the specified file.
func ReplacePartID(ctx context.Context, runner components.Runner, oldDev, newDev, file string) error {
	newID, err := getPartitionID(ctx, runner, newDev)
	if err != nil {
		return errors.Annotate(err, "update partition id: failed to get BOOT_B partition ID").Err()
	} else if newID == "" {
		return errors.Reason("update partition id: partition ID for %q is empty", newDev).Err()
	}

	oldID, err := getPartitionID(ctx, runner, oldDev)
	if err != nil {
		return errors.Annotate(err, "update partition id: failed to get image root partition ID").Err()
	} else if oldID == "" {
		return errors.Reason("update partition id: partition ID for %q is empty", oldDev).Err()
	}

	// Find replace the requested text in the file.
	cmd := fmt.Sprintf(`sed -i "s/%s/%s/g" %q`, oldID, newID, file)
	if _, err := runner(ctx, time.Minute, cmd); err != nil {
		return errors.Annotate(err, "update partition id: failed to replace old image root PART ID").Err()
	}
	return nil
}

// getPartitionNumber gets the partition number of a device.
func getPartitionNumber(ctx context.Context, runner components.Runner, device string) (int, error) {
	cmd := fmt.Sprintf("partx -g -o NR %q", device)
	out, err := runner(ctx, time.Minute, cmd)
	if err != nil {
		return 0, errors.Annotate(err, "get partition number: failed to get partition number with partx").Err()
	}

	id, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, errors.Annotate(err, "get partition number: failed to parse partition number from: %q", out).Err()
	}
	return id, nil
}

// getMountDeviceForPath returns the device/partition that a specific file is associated with.
func getMountDeviceForPath(ctx context.Context, runner components.Runner, path string) (string, error) {
	cmd := fmt.Sprintf("findmnt %q -o source -n", path)
	dev, err := runner(ctx, time.Minute, cmd)
	if err != nil {
		return "", errors.Annotate(err, "get device for path: failed to get mount with findmnt").Err()
	}
	return strings.TrimSpace(dev), nil
}

// getRootDeviceForPath returns the root device that a specific file is associated with.
//
// This differs from GetMountDeviceForPath since it will return the parent device and not the individual partition.
func getRootDeviceForPath(ctx context.Context, runner components.Runner, path string) (string, error) {
	mountDev, err := getMountDeviceForPath(ctx, runner, path)
	if err != nil {
		return "", errors.Annotate(err, "get device for path: failed to get mounted device").Err()
	}

	cmd := fmt.Sprintf("lsblk -pno pkname %q", mountDev)
	rootDev, err := runner(ctx, time.Minute, cmd)
	if err != nil {
		return "", errors.Annotate(err, "get device for path: failed to get root device").Err()
	}
	return strings.TrimSpace(rootDev), nil
}

// partitionHelper is a helper utility to query and interact with device partitions.
type partitionHelper struct {
	runner components.Runner
	device string
}

// partitionInfo is information about a single device partition.
type partitionInfo struct {
	Number int
	Start  float64
	End    float64
	Size   float64
	Type   string
}

// deviceInfo contains basic information about a block device as reported by parted.
type deviceInfo struct {
	// The name of the block device.
	Name string
	// The size in Bytes of the block device.
	Size int64
}

// NewPartitionHelperForPath creates a new helper for the device that owns the requested path.
func NewPartitionHelperForPath(ctx context.Context, runner components.Runner, path string) (*partitionHelper, error) {
	rootDev, err := getRootDeviceForPath(ctx, runner, path)
	if err != nil {
		return nil, errors.Annotate(err, "new partition helper: failed to get root device for path %q", path).Err()
	}

	return &partitionHelper{
		runner: runner,
		device: rootDev,
	}, nil
}

// CreateNewPrimaryPartitionBytes creates a new primary partiion by providing the size in bytes.
func (p *partitionHelper) CreateNewPrimaryPartitionBytes(ctx context.Context, fsType string, size int64) (string, error) {
	pSize, err := p.getSizeAsPercent(ctx, size)
	if err != nil {
		return "", errors.Annotate(err, "create partition: failed to calculate disk size").Err()
	}

	start, err := p.getLastPartitionEndPercent(ctx)
	if err != nil {
		return "", errors.Annotate(err, "create partition: failed to get last partition end location").Err()
	}

	if (start + pSize) > 100.0 {
		return "", errors.Reason("create partition: not enough free space on device").Err()
	}

	return p.CreateNewPrimaryPartitionPercent(ctx, fsType, start+pSize)
}

// CreateNewPrimaryPartitionPercent creates a new primary partition by providing the partition end point in percent.
func (p *partitionHelper) CreateNewPrimaryPartitionPercent(ctx context.Context, fsType string, end float64) (string, error) {
	start, err := p.getLastPartitionEndPercent(ctx)
	if err != nil {
		return "", errors.Annotate(err, "create partition: failed to get last partition end location").Err()
	}

	cmd := fmt.Sprintf(`parted %s unit %s --align optimal mkpart primary %s %f %f`, p.device, "%", fsType, start, end)
	if _, err := p.runner(ctx, 2*time.Minute, cmd); err != nil {
		return "", errors.Annotate(err, "create partition: failed to create new partition with parted").Err()
	}

	// Get the newly created partition name and return it.
	// Example output:
	/*
		PATH
		/dev/mmcblk0
		/dev/mmcblk0p1
		/dev/mmcblk0p2
	*/
	cmd = fmt.Sprintf("lsblk -l -o path %q | tail -1", p.device)
	if newPart, err := p.runner(ctx, time.Minute, cmd); err != nil {
		return "", errors.Annotate(err, "create partition: failed to find new partition name").Err()
	} else {
		return strings.TrimSpace(newPart), nil
	}
}

// GetPartitionSize gets the partition size of the mounted device.
func (p *partitionHelper) GetPartitionSize(ctx context.Context, path string) (int64, error) {
	device, err := getMountDeviceForPath(ctx, p.runner, path)
	if err != nil {
		return 0, errors.Annotate(err, "get partition size: failed to get mount device").Err()
	}

	partID, err := getPartitionNumber(ctx, p.runner, device)
	if err != nil {
		return 0, errors.Annotate(err, "get partition size: failed to get partition id").Err()
	}

	partitions, err := p.GetPartitionInfo(ctx, "B", true)
	if err != nil {
		return 0, errors.Annotate(err, "get partition size: failed to get partition id").Err()
	}

	for _, partition := range partitions {
		if partition.Number == partID {
			return int64(partition.Size), nil
		}
	}

	return 0, errors.Reason("get partition size: could not find partition info for partition: %d", partID).Err()
}

// getSizeAsPercent converts a size in bytes to a percent of the total disk space.
func (p *partitionHelper) getSizeAsPercent(ctx context.Context, size int64) (float64, error) {
	deviceInfo, err := p.getDeviceInfo(ctx)
	if err != nil {
		return 0, errors.Annotate(err, "create primary partition: failed to get device information").Err()
	}

	return 100.0 * (float64(size) / float64(deviceInfo.Size)), nil
}

// getLastPartitionEnd returns the end location of the final used partition (in %).
func (p *partitionHelper) getLastPartitionEndPercent(ctx context.Context) (float64, error) {
	partitions, err := p.GetPartitionInfo(ctx, "%", true)
	if err != nil {
		return 0.0, errors.Annotate(err, "create primary partition: failed to get current partition information").Err()
	}

	if len(partitions) == 0 {
		return 0, nil
	}
	return partitions[len(partitions)-1].End, nil
}

// getDeviceInfo returns the high-level information about the device.
func (p *partitionHelper) getDeviceInfo(ctx context.Context) (*deviceInfo, error) {
	const unit = "B"
	// Example output:
	/*
		BYT;
		/dev/sda:100%:scsi:512:512:msdos: Patriot Memory:;
		1:0.01%:0.88%:0.87%:fat32::lba;
		2:0.88%:29.0%:28.1%:ext4::;
		3:29.0%:31.0%:1.97%:fat32::lba;
		4:31.0%:100%:69.0%:ext4::;
	*/
	cmd := fmt.Sprintf("parted -m %q unit %s print", p.device, unit)
	out, err := p.runner(ctx, time.Minute, cmd)
	if err != nil {
		return nil, errors.Annotate(err, "get device info: failed to get partition info from parted").Err()
	}

	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(line, ":")
		if len(fields) < 5 {
			continue
		}

		if !strings.EqualFold(fields[0], p.device) {
			continue
		}

		val := fields[1][:len(fields[1])-len(unit)]
		size, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil, errors.Reason("get device info: unable to parse size value: %q", val).Err()
		} else if size < 0 {
			return nil, errors.Reason("get device info: invalid size value: %d", size).Err()
		}

		return &deviceInfo{
			Name: p.device,
			Size: size,
		}, nil
	}

	return nil, errors.Reason("get device info: Failed to parse device info").Err()
}

// GetFreeSpace returns the amount of free space at the end of the partitions.
func (p *partitionHelper) GetFreeSpace(ctx context.Context) (int64, error) {
	partitions, err := p.GetPartitionInfo(ctx, "B", false)
	if err != nil {
		return 0, errors.Annotate(err, "get free space: failed to get partition info").Err()
	}

	if len(partitions) == 0 {
		return 0, errors.Annotate(err, "get free space: no partitions found").Err()
	}

	last := partitions[len(partitions)-1]
	if strings.EqualFold(last.Type, "free") {
		return int64(last.Size), nil
	}
	return 0, nil
}

// GetPartitionInfo returns all of the device partitions.
func (p *partitionHelper) GetPartitionInfo(ctx context.Context, unit string, excludeFree bool) ([]*partitionInfo, error) {
	/*
		BYT;
		/dev/sda:100%:scsi:512:512:msdos: Patriot Memory:;
		1:0.01%:0.88%:0.87%:fat32::lba;
		2:0.88%:29.0%:28.1%:ext4::;
		3:29.0%:31.0%:1.97%:fat32::lba;
		4:31.0%:100%:69.0%:ext4::;
	*/
	cmd := fmt.Sprintf("parted -m %q unit %s print free | tr -d ';'", p.device, unit)
	out, err := p.runner(ctx, time.Minute, cmd)
	if err != nil {
		return nil, errors.Annotate(err, "get partition info: failed to get partition info from parted").Err()
	}

	res := make([]*partitionInfo, 0)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(line, ":")
		if len(fields) < 5 {
			continue
		}

		if strings.EqualFold(fields[0], p.device) {
			continue
		}

		if excludeFree && strings.EqualFold(fields[4], "free") {
			continue
		}

		partNumber, err := strconv.Atoi(strings.TrimSpace(fields[0]))
		if err != nil {
			return nil, errors.Reason("get partition info: unable to parse partition value: %q", fields[0]).Err()
		}

		val := fields[1][:len(fields[1])-len(unit)]
		start, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return nil, errors.Reason("get partition info: unable to parse start value: %q", val).Err()
		} else if start < 0 {
			return nil, errors.Reason("get partition info: invalid start value: %f", start).Err()
		}

		val = fields[2][:len(fields[2])-len(unit)]
		end, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return nil, errors.Reason("get partition info: unable to parse end value: %q", val).Err()
		} else if end < 0 {
			return nil, errors.Reason("get partition info: invalid end value: %f", end).Err()
		}

		if end < start {
			return nil, errors.Reason("get partition info: invalid start/end values, end: %f,  must be > start: %f", end, start).Err()
		}

		res = append(res, &partitionInfo{
			Number: partNumber,
			Start:  start,
			End:    end,
			Size:   end - start,
			Type:   fields[4],
		})
	}
	return res, nil
}
