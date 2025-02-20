// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// Prefix command to call storage commands.
const storageDetectionPrefix = ". /usr/sbin/write_gpt.sh; . /usr/share/misc/chromeos-common.sh; load_base_vars; %s"

// DetectInternalStorage detects internal storage based on present fixed disks.
func DetectInternalStorage(ctx context.Context, run components.Runner) string {
	searchFunctions := []string{
		"list_fixed_mmc_disks",
		"list_fixed_nvme_nss",
		"list_fixed_ufs_disks",
		"list_fixed_ata_disks",
	}
	for _, f := range searchFunctions {
		cmd := fmt.Sprintf(storageDetectionPrefix, f)
		disk, err := run(ctx, time.Minute, cmd)
		if err != nil {
			log.Debugf(ctx, "Fail to detect storage when uses %q function: %s", f, err)
			continue
		}
		disk = strings.TrimSpace(disk)
		if disk == "" {
			log.Infof(ctx, "Storage detected by function: %s is empty!", f)
			continue
		}
		// echo /dev/$(basename nvme0n1)
		return fmt.Sprintf("/dev/%s", disk)
	}
	return ""
}

// DeviceMainStoragePath returns the path of the main storage device
// on the DUT.
func DeviceMainStoragePath(ctx context.Context, run components.Runner) (string, error) {
	mainStorageCMD := fmt.Sprintf(storageDetectionPrefix, "get_fixed_dst_drive")
	mainStorage, err := run(ctx, time.Minute, mainStorageCMD)
	if err != nil {
		return "", errors.Annotate(err, "device storage path").Err()
	}
	return mainStorage, nil
}
