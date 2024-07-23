// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/tlw"
)

// Detect if WiFiAdapter is present and initialized.
func hasWiFiAdapter(ctx context.Context, r components.Runner) (bool, error) {
	const wifiDetectCmd = `iw list`
	output, err := r(ctx, time.Minute, wifiDetectCmd)
	if err != nil {
		return false, errors.Annotate(err, "WiFi Adapter check failed").Err()
	}

	// if 'iw list' command enumerates any phy radio, there will
	// be a line starting with 'Wiphy phy'.
	for _, word := range strings.Fields(output) {
		if word == "Wiphy" {
			return true, nil
		}
	}

	return false, nil
}

// auditWiFiExec will validate wifi chip and update state.
//
// Detect if the DUT has wifi device listed in the output of 'lspci' command.
func auditWiFiExec(ctx context.Context, info *execs.ExecInfo) error {
	r := info.DefaultRunner()
	wifi := info.GetChromeos().GetWifi()
	if wifi == nil {
		return errors.Reason("audit wifi: data is not present in dut info").Err()
	}
	isWiFiDetected, err := hasWiFiAdapter(ctx, r)

	if err == nil && isWiFiDetected {
		// successfully detected
		wifi.State = tlw.HardwareState_HARDWARE_NORMAL
		log.Infof(ctx, "set wifi state to be: %s", tlw.HardwareState_HARDWARE_NORMAL)
		return nil
	}
	if execs.SSHErrorInternal.In(err) || execs.SSHErrorCLINotFound.In(err) {
		wifi.State = tlw.HardwareState_HARDWARE_UNSPECIFIED
		return errors.Annotate(err, "audit wifi").Err()
	}
	if wifi.GetChipName() != "" {
		// If wifi chip is not detected, but was expected by setup info then we
		// set needs_replacement as it is probably a hardware issue.
		wifi.State = tlw.HardwareState_HARDWARE_NEED_REPLACEMENT
	} else {
		// the wifi state cannot be determined due to cmd failed
		// therefore, set it to HardwareStateNotDetected
		wifi.State = tlw.HardwareState_HARDWARE_NOT_DETECTED
	}
	log.Infof(ctx, "set wifi state to be: %s", wifi.State)
	return errors.Annotate(err, "audit wifi").Err()
}

func init() {
	execs.Register("cros_audit_wifi", auditWiFiExec)
}
