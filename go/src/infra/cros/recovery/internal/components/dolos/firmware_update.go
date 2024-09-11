// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dolos

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/tlw"
)

const (
	dolosSubCmdUpdateFirmware     = "update-firmware"
	dolosSubCmdUpdateFirmwareGlob = dolosSubCmdUpdateFirmware + "--firmware_version %s "
)

// DolosUpdateFirmware - call doloscmd update-firmware on the host with the correct arguments to update the firmware
// to the version listed in UFS.
func DolosUpdateFirmware(ctx context.Context, run components.Runner, dolosInfo *tlw.Dolos, timeout time.Duration) error {
	log.Infof(ctx, "Update dolos firmware from to %s", dolosInfo.FwVersion)
	_, err := runDolosCommand(ctx, run, fmt.Sprintf(dolosSubCmdUpdateFirmwareGlob, dolosInfo.FwVersion), dolosInfo, timeout)
	if err != nil {
		return errors.Annotate(err, "unable to update dolos version").Err()
	}
	return nil
}
