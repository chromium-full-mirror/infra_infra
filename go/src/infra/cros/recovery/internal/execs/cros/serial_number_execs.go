// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

const (
	readSerialNumberCommand = "vpd -g serial_number"
)

// updateSerialNumberToInvExec updates serial number in DUT-info.
func updateSerialNumberToInvExec(ctx context.Context, info *execs.ExecInfo) error {
	run := info.DefaultRunner()
	sn, err := run(ctx, time.Minute, readSerialNumberCommand)
	if err != nil {
		return errors.Annotate(err, "update serial number in DUT-info").Err()
	}
	if sn == "" {
		return errors.Reason("update serial number in DUT-info: is empty").Err()
	}
	log.Debugf(ctx, "Update serial_number %q in DUT-info.", sn)
	info.GetChromeos().SerialNumber = sn
	return nil
}

// matchSerialNumberToInvExec matches serial number from the resource to value in the Inventory.
func matchSerialNumberToInvExec(ctx context.Context, info *execs.ExecInfo) error {
	run := info.DefaultRunner()
	actualSerialNumber, err := run(ctx, time.Minute, readSerialNumberCommand)
	if err != nil {
		return errors.Annotate(err, "match serial number to inventory").Err()
	}
	expectedSerialNumber := info.GetChromeos().GetSerialNumber()
	if actualSerialNumber != expectedSerialNumber {
		return errors.Reason("match serial number to inventory: failed, expected: %q, but got %q", expectedSerialNumber, actualSerialNumber).Err()
	}
	return nil
}

// restoreSerialNumberFromInvExec restores serial number of device in device VPD.
func restoreSerialNumberFromInvExec(ctx context.Context, info *execs.ExecInfo) error {
	invSerialNumber := info.GetChromeos().GetSerialNumber()
	if invSerialNumber == "" {
		return errors.Reason("restore serial number from inventory: inventory is empty").Err()
	}
	run := info.DefaultRunner()
	if _, err := run(ctx, info.GetExecTimeout(), fmt.Sprintf("vpd -s serial_number=%s", invSerialNumber)); err != nil {
		return errors.Annotate(err, "restore serial number from inventory").Err()
	}
	// TODO(b/77594752): dump_vpd_log was removed in M-127. Remove when we're done testing pre-M-127.
	if _, err := run(ctx, info.GetExecTimeout(), "dump_vpd_log --force"); err != nil {
		log.Debugf(ctx, "Restore serial-number %q: fail to dump vpd of the host", invSerialNumber)
	}
	return nil
}

func init() {
	execs.Register("cros_update_serial_number_inventory", updateSerialNumberToInvExec)
	execs.Register("cros_match_serial_number_inventory", matchSerialNumberToInvExec)
	execs.Register("cros_restore_serial_number", restoreSerialNumberFromInvExec)
}
