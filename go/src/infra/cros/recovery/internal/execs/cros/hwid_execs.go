// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/cros"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// updateHWIDToInvExec read HWID from the resource and update DUT info.
func updateHWIDToInvExec(ctx context.Context, info *execs.ExecInfo) error {
	run := info.DefaultRunner()
	actionArgs := info.GetActionArgs(ctx)
	allowedOverride := actionArgs.AsBool(ctx, "allowed_override", false)
	invHWID := info.GetChromeos().GetHwid()
	if invHWID != "" {
		if allowedOverride {
			log.Debugf(ctx, "Inventory have HWID %q and override is allowed.", invHWID)
		} else {
			log.Debugf(ctx, "Inventory have HWID %q and it is not allowed to override it.", invHWID)
			return nil
		}
	}
	hwid, err := cros.HostHWID(ctx, info.GetDut(), info.GetExecTimeout(), run)
	if err != nil {
		return errors.Annotate(err, "update HWID in DUT-info").Err()
	}
	if hwid == "" {
		return errors.Reason("update HWID in DUT-info: is empty").Err()
	}
	info.GetChromeos().Hwid = hwid
	log.Debugf(ctx, "Update HWID: %q", info.GetChromeos().GetHwid())
	return nil
}

// matchHWIDToInvExec matches HWID from the resource to value in the Inventory.
func matchHWIDToInvExec(ctx context.Context, info *execs.ExecInfo) error {
	run := info.DefaultRunner()
	actualHWID, err := cros.HostHWID(ctx, info.GetDut(), info.GetExecTimeout(), run)
	if err != nil {
		return errors.Annotate(err, "match HWID to inventory").Err()
	}
	expectedHWID := info.GetChromeos().GetHwid()
	if actualHWID != expectedHWID {
		return errors.Reason("match HWID to inventory: failed, expected: %q, but got %q", expectedHWID, actualHWID).Err()
	}
	return nil
}

// updateHWIDFromInvExec updates HWID from inventory to host.
//
// HWID can be checkd on the DUT bu futility.
// Crossystem represent cached data and will wait till reboot to update the value.
func updateHWIDFromInvExec(ctx context.Context, info *execs.ExecInfo) error {
	run := info.DefaultRunner()
	originalHwid := info.GetChromeos().GetHwid()
	// Update HWID to the AP firmware.
	log.Debugf(ctx, "Update HWID from host: update HWID %q in AP firmware.", originalHwid)
	out, err := run(ctx, 3*time.Minute, "futility", "gbb", "--flash --set", "--hwid", fmt.Sprintf("%q", originalHwid))
	if err != nil {
		return errors.Annotate(err, "update HWID from host").Err()
	}
	log.Debugf(ctx, "Update HWID from host: updated HWID in AP file (output): %s", out)
	return nil
}

func init() {
	execs.Register("cros_update_hwid_to_inventory", updateHWIDToInvExec)
	execs.Register("cros_match_hwid_to_inventory", matchHWIDToInvExec)
	execs.Register("cros_update_hwid_from_inventory_to_host", updateHWIDFromInvExec)
}
