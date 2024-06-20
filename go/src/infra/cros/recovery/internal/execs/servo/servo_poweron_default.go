// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
)

func servoSetPoweronDefaultsExec(ctx context.Context, info *execs.ExecInfo) error {

	dolos := info.GetChromeos().GetDolos()
	isDolos := dolos != nil
	direction := "dut_sees_usbkey"

	if isDolos {
		direction = "servo_sees_usbkey"
		log.Debugf(ctx, "Set bottom USB mux: set %q as dolos detected", direction)
	}

	servod := info.NewServod()

	if err := servod.Set(ctx, "bottom_usb_mux_sel", direction); err != nil {
		return errors.Annotate(err, "set bottom USB mux").Err()
	}

	if isDolos {
		if err := servod.Set(ctx, "bottom_usb_mux_sel_at_poweron", direction); err != nil {
			return errors.Annotate(err, "set bottom USB default power on mux").Err()
		}
	} else {
		if err := servod.Set(ctx, "servo_poweron_conf_default", ""); err != nil {
			return errors.Annotate(err, "reset all servo power on defaults").Err()
		}
	}
	return nil
}

func init() {
	execs.Register("servo_set_poweron_defaults", servoSetPoweronDefaultsExec)
}
