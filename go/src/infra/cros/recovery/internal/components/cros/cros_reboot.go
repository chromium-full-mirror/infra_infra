// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/internal/retry"
)

const (
	// Default reboot command for ChromeOS devices.
	// Each command set sleep 1 second to wait for reaction of the command from left part.
	rebootCommand = "(echo begin 1; sync; echo end 1 \"$?\")& sleep 1;" +
		"(echo begin 2; reboot; echo end 2 \"$?\")& sleep 1;" +
		// Force reboot is not calling shutdown.
		"(echo begin 3; reboot -f; echo end 3 \"$?\")& sleep 1;" +
		// Force reboot without sync.
		"(echo begin 4; reboot -nf; echo end 4 \"$?\")& sleep 1;" +
		// telinit 6 sets run level for process initialized, which is equivalent to reboot.
		"(echo begin 5; telinit 6; echo end 5 \"$?\")"
)

// Reboot executes the reboot command using a command runner for a
// DUT.
//
// This function executes an ellaborate reboot sequence that includes
// executing sync and then attempting forcible reboot etc.
func Reboot(ctx context.Context, run components.Runner, timeout time.Duration) error {
	log.Debugf(ctx, "Reboot Helper : %s", rebootCommand)
	out, err := run(ctx, timeout, rebootCommand)
	if components.NoExitStatusErrorInternal.In(err) {
		// Client closed connected as rebooting.
		log.Debugf(ctx, "Client exit as device rebooted: %s", err)
	} else if err != nil {
		return errors.Annotate(err, "reboot helper").Err()
	}
	log.Debugf(ctx, "Stdout: %s", out)
	return nil
}

const (
	// WaitTimeToDownAtRestart is the time for the device to be down at reboot.
	WaitTimeToDownAtRestart = 120 * time.Second
	// WaitTimeToBoot is the time for the device to be up after reboot.
	WaitTimeToBootAfterRestart = 240 * time.Second
)

// RebootWithCheck executes a simple reboot and check that host goes down and up.
func RebootWithCheck(ctx context.Context, ha components.HostAccess, timeToDown, timeToUp time.Duration) error {
	log.Debugf(ctx, "Requested reboot with timeouts down:%v and up:%v", timeToDown, timeToUp)
	if _, err := ha.RunBackground(ctx, 10*time.Second, "reboot"); err != nil {
		return errors.Annotate(err, "reboot with check").Err()
	}
	// wait for it to be down.
	log.Debugf(ctx, "Wait for device to lost a ping %s.", timeToUp)
	if err := retry.WithTimeout(ctx, PingRetryInterval, timeToDown, func() error {
		return IsNotPingable(ctx, DefaultPingCount, ha.Ping)
	}, "wait till device is down"); err != nil {
		return errors.Annotate(err, "reboot with check").Err()
	}
	// wait down for servo device is successful, then wait for device
	// up.
	log.Debugf(ctx, "Wait for device to be pingable %s.", timeToUp)
	if err := retry.WithTimeout(ctx, PingRetryInterval, timeToUp, func() error {
		return IsPingable(ctx, DefaultPingCount, ha.Ping)
	}, "wait until device is up"); err != nil {
		return errors.Annotate(err, "reboot with check").Err()
	}
	log.Infof(ctx, "Device successfully rebooted, it is up and pingable.")
	return nil
}
