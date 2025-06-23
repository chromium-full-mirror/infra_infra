// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	bt "go.chromium.org/infra/cros/recovery/internal/components/cros/bluetooth"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/internal/retry"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// auditBluetoothExec will verify bluetooth on the host is detected correctly.
//
// Check if bluetooth adapter on the host is responding to a status check.
func auditBluetoothExec(ctx context.Context, info *execs.ExecInfo) error {
	r := info.DefaultRunner()
	bluetooth := info.GetChromeos().GetBluetooth()
	if bluetooth == nil {
		return errors.Reason("audit bluetooth: data is not present in dut info").Err()
	}

	argsMap := info.GetActionArgs(ctx)
	cmdTimeout := argsMap.AsDuration(ctx, "cmd_timeout", 30, time.Second)

	var hasAdapterFunc func(context.Context, components.Runner, time.Duration) (bool, error)
	var hasBluetooth bool

	if bt.FlossEnabled(ctx, r, cmdTimeout) {
		hasAdapterFunc = bt.HasAdapterFloss
	} else {
		hasAdapterFunc = bt.HasAdapterBlueZ
	}

	// Retry 8 times with timeout of 4 seconds.
	err := retry.LimitCount(ctx, 8, 4*time.Second, func() error {
		present, err := hasAdapterFunc(ctx, r, cmdTimeout)
		if err != nil {
			return err
		}
		if !present {
			return errors.Reason("bluetooth adapter not found").Err()
		}
		// bluetooth adapter found. Stop retry.
		hasBluetooth = present
		return nil

	}, "check bluetooth adapter status")

	if err == nil && hasBluetooth {
		bluetooth.State = tlw.HardwareState_HARDWARE_NORMAL
		log.Infof(ctx, "set bluetooth state to be: %s", tlw.HardwareState_HARDWARE_NORMAL)
		return nil
	}

	if execs.SSHErrorInternal.In(err) || execs.SSHErrorCLINotFound.In(err) {
		bluetooth.State = tlw.HardwareState_HARDWARE_UNSPECIFIED
		log.Infof(ctx, "set bluetooth state to be: %s", tlw.HardwareState_HARDWARE_UNSPECIFIED)
		return errors.Fmt("audit bluetooth: %w", err)
	}
	if bluetooth.GetExpected() {
		// If bluetooth is not detected, but was expected by setup info
		// then we set needs_replacement as it is probably a hardware issue.
		bluetooth.State = tlw.HardwareState_HARDWARE_NEED_REPLACEMENT
		log.Infof(ctx, "set bluetooth state to be: %s", tlw.HardwareState_HARDWARE_NEED_REPLACEMENT)
		return errors.Fmt("audit bluetooth: %w", err)
	}
	// the bluetooth state cannot be determined due to cmd failed
	// therefore, set it to HardwareStateNotDetected.
	bluetooth.State = tlw.HardwareState_HARDWARE_NOT_DETECTED
	log.Infof(ctx, "set bluetooth state to be: %s", tlw.HardwareState_HARDWARE_NOT_DETECTED)
	return errors.Fmt("audit bluetooth: %w", err)
}

func hasBtPeers(ctx context.Context, info *execs.ExecInfo) error {
	btpeers := info.GetChromeos().GetBluetoothPeers()
	if btpeers == nil {
		return errors.Reason("has_btpeers check: btpeer data is not present in dut info").Err()
	}
	if len(btpeers) == 0 {
		return errors.Reason("has_btpeers check: no btpeers found").Err()
	}
	return nil
}

func init() {
	execs.Register("cros_audit_bluetooth", auditBluetoothExec)
	execs.Register("cros_has_btpeers", hasBtPeers)
}
