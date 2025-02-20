// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package btpeer

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
)

// copyInitrdUtils is a script that copies the required utilities into the initrd image when it
// is built so they are available to be used by the shrinkRootFSCmd script.
const copyInitrdUtils = `#!/bin/sh

# Required preamble for initramfs to load hook.
PREREQ=""

prereqs() {
    echo "$PREREQ"
}

case "$1" in
    prereqs)
        prereqs
        exit 0
    ;;
esac

# Get access to hook utilities
. /usr/share/initramfs-tools/hook-functions

copy_exec /usr/bin/partx /usr/bin
copy_exec /bin/lsblk /bin
copy_exec /sbin/dumpe2fs /sbin
copy_exec /sbin/e2fsck /sbin
copy_exec /sbin/findfs /sbin
copy_exec /sbin/parted /sbin
copy_exec /sbin/resize2fs /sbin
`

// shrinkRootFSCmd is a script that is run prior to mounting the rootFS so we can
// safely reduce its size.
const shrinkRootFSCmd = `#!/bin/sh

# Required preamble for initramfs to load hook.
PREREQ=""

prereqs() {
    echo "$PREREQ"
}

case "$1" in
    prereqs)
        prereqs
        exit 0
    ;;
esac

set -x

# Wait for device to be enumerated if it's not already.
COUNT=0
while [ -z "${ROOT_DEV}" ] && [ $COUNT -lt 15 ]; do
	ROOT_DEV=$(findfs "$ROOT")
	COUNT=$(( $COUNT + 1 ))
	sleep 1
done

DEVICE=$(lsblk -pno pkname "$ROOT_DEV")
PART_NUMBER=$(partx -rgo NR "$ROOT_DEV")

# Perform the fs resize.
e2fsck -y -v -f "$ROOT_DEV" || exit 1
resize2fs -f "$ROOT_DEV" %dK

# Get current device partitioning info.
PART_TEXT=$(parted "$DEVICE" unit B -m print || exit 1)
CURRENT_START=$(echo "$PART_TEXT" | grep ^$PART_NUMBER: | cut -f 2 -d: | tr -d B)
CURRENT_END=$(echo "$PART_TEXT" | grep ^$PART_NUMBER: | cut -f 3 -d: | tr -d B)
if [ -z $CURRENT_START ] || [ -z $CURRENT_END ]; then
	echo "Error getting current partition info"
	exit 1
fi

# Get the size of the new FS
FS_TEXT=$(dumpe2fs "$ROOT_DEV" || exit 1)
FS_SIZE=$(echo "$FS_TEXT" | grep "Block count" | tr -d ' ' | cut -f2 -d:)
FS_BLOCK_SIZE=$(echo "$FS_TEXT" | grep "Block size" | tr -d ' ' | cut -f2 -d:)
if [ -z $FS_SIZE ] || [ -z $FS_BLOCK_SIZE ]; then
	echo "Error getting file system info"
	exit 1
fi

NEW_END=$((CURRENT_START + FS_SIZE * FS_BLOCK_SIZE))
echo Yes | parted "$DEVICE" ---pretend-input-tty resizepart $PART_NUMBER ${NEW_END}B

# Verify that the fs is ok after shrinking, else return to original size.
e2fsck -y -f "$ROOT_DEV" || echo Yes | parted "$DEVICE" ---pretend-input-tty resizepart $PART_NUMBER ${CURRENT_END}B
`

// CreateShrinkInitrdHook creates an initrd image that will shrink the filesystem on the next reboot.
func CreateShrinkInitrdHook(ctx context.Context, runner components.Runner, size int64) (func(context.Context) error, error) {
	// Create a hook to copy CLI utilities into initrd image during build so it can be used by premount hok.
	// Note: this can also be: "/etc/initramfs-tools/hooks/shrinkutils" but RPi OS 12 doesn't like to
	// pick up that directory.
	const shrinkUtilsFile = "/usr/share/initramfs-tools/hooks/shrinkutils"
	if err := createExecutableScript(ctx, runner, copyInitrdUtils, shrinkUtilsFile); err != nil {
		return nil, errors.Annotate(err, "create shrink hook: failed to create copy utility hook").Err()
	}

	// Minimum resize2fs size is in KB or 512 byte sectors, use Kb.
	if size%1000 != 0 {
		return nil, errors.Reason("invalid size %d, size must be an exact multiple of 1KB", size).Err()
	}

	// Create a hook to resize the partition before the rootfs is mounted.
	const resizePartitionFile = "/etc/initramfs-tools/scripts/init-premount/resizepart"
	shrinkCmd := fmt.Sprintf(shrinkRootFSCmd, size/1000)
	if err := createExecutableScript(ctx, runner, shrinkCmd, resizePartitionFile); err != nil {
		return nil, errors.Annotate(err, "create shrink hook: failed to create shrink hook").Err()
	}

	if err := BuildInitrd(ctx, runner); err != nil {
		return nil, errors.Annotate(err, "create shrink hook: failed to build initial initrd image").Err()
	}

	// Remove the scripts now that we built the image so they're not included next time an image is built.
	if _, err := runner(ctx, 30*time.Second, "rm", resizePartitionFile); err != nil {
		return nil, errors.Annotate(err, "create shrink hook: failed to remove resize hook").Err()
	}

	if _, err := runner(ctx, 30*time.Second, "rm", shrinkUtilsFile); err != nil {
		return nil, errors.Annotate(err, "create shrink hook: failed to remove copy utilities hook").Err()
	}

	return func(ctx context.Context) error {
		// Remove the hook by rebuilding initrd image without the hooks we injected
		if err := BuildInitrd(ctx, runner); err != nil {
			return errors.Annotate(err, "create shrink hook: failed to build initial initrd image").Err()
		}
		return nil
	}, nil
}
