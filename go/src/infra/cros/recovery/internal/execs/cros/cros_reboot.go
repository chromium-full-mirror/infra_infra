// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/cros/adb"
	"go.chromium.org/infra/cros/recovery/internal/execs"
)

// simpleRebootExec triggres reboot of the DUT without wait.
func simpleRebootExec(ctx context.Context, info *execs.ExecInfo) error {
	timeout := info.GetExecTimeout()
	if info.GetChromeos().GetIsAndroidBased() {
		// Trigger ADB reboot.
		_, err := adb.Exec(ctx, info.GetDut(), timeout, "reboot")
		return errors.WrapIf(err, "adb command")
	}
	// trigger rebot from the host for ChromeOS.
	run := info.NewBackgroundRunner(info.GetDut().Name)
	_, err := run(ctx, info.GetExecTimeout(), "reboot")
	return errors.WrapIf(err, "simple reboot")
}

func init() {
	execs.Register("cros_simple_reboot", simpleRebootExec)
}
