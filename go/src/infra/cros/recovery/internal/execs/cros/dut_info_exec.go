// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// isSmartHubExpectedExec checks if SmartHub is expected to be present in setup.
func isSmartHubExpectedExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	reserveAction := argsMap.AsBool(ctx, "reverse", false)
	smartUSBhubpresent := info.GetChromeos().GetServo().GetSmartUsbhubPresent()
	if reserveAction {
		if !smartUSBhubpresent {
			log.Debugf(ctx, "SmartHub expected: not specified in servo info.")
			return nil
		}
		return errors.Reason("smart-hub expected: specified in servo info").Err()
	} else {
		if smartUSBhubpresent {
			log.Debugf(ctx, "SmartHub expected: specified in servo info.")
			return nil
		}
		return errors.Reason("smart-hub expected: not specified in servo info").Err()
	}
}

func init() {
	execs.Register("servo_is_smarthub_expected", isSmartHubExpectedExec)
}
