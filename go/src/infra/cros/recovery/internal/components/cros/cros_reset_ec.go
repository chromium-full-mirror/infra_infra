// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// RebootECByEcTool finds the command to reboot ec and executes it.
func RebootECByEcTool(ctx context.Context, r components.Runner) error {
	ecRebootCmds := []string{"ec_reboot", "reboot_ec"}

	var rebootCmd = ""
	for _, cmd := range ecRebootCmds {
		fmtCmd := fmt.Sprintf("ectool help | grep %s", cmd)
		output, err := r(ctx, 5*time.Second, fmtCmd)
		log.Debugf(ctx, "RebootECByEcTool EC Reboot command help output : %s", output)
		if err != nil {
			errorCode, ok := components.ErrCodeTag.Value(err)
			if !ok {
				return errors.Annotate(err, "RebootECByEcTool: cannot find error code").Err()
			}
			if errorCode == 1 {
				continue
			}
		}
		rebootCmd = cmd
		break
	}
	if rebootCmd == "" {
		return errors.New("RebootECByEcTool: cannot find ectool reboot command through ectool help command")
	}
	log.Infof(ctx, "Found EC Reboot command : %s", rebootCmd)

	// Command to reset EC from DUT side.
	ecResetCmd := fmt.Sprintf("ectool %s cold && exit", rebootCmd)
	if out, err := r(ctx, 30*time.Second, ecResetCmd); err != nil {
		// Client closed connected as rebooting.
		log.Debugf(ctx, "Client exit as device rebooted: %s", err)
		return errors.Annotate(err, "RebootECByEcTool").Err()
	} else {
		log.Debugf(ctx, "Stdout: %s", out)
	}
	return nil
}
