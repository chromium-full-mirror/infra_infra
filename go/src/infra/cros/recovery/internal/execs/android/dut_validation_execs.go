// Copyright 2022 The ChromiumOS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package android

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// hasDutBoardExec verifies that attached DUT provides board name.
func hasDutBoardExec(ctx context.Context, info *execs.ExecInfo) error {
	b := info.GetAndroid().GetBoard()
	log.Debugf(ctx, "Attached DUT board name: %q", b)
	if b != "" {
		return nil
	}
	return errors.Reason("attached dut board name is empty").Err()
}

// hasDutModelExec verifies that attached DUT provides model name.
func hasDutModelExec(ctx context.Context, info *execs.ExecInfo) error {
	m := info.GetAndroid().GetModel()
	log.Debugf(ctx, "Attached DUT model name: %q", m)
	if m != "" {
		return nil
	}
	return errors.Reason("attached dut model name is empty").Err()
}

// hasDutSerialNumberExec verifies that attached DUT has serial number.
func hasDutSerialNumberExec(ctx context.Context, info *execs.ExecInfo) error {
	s := info.GetAndroid().GetSerialNumber()
	log.Debugf(ctx, "Attached DUT serial number: %q", s)
	if s != "" {
		return nil
	}
	return errors.Reason("attached dut serial number is empty").Err()
}

// hasDutAssociatedHostExec verifies that attached DUT has associated host.
func hasDutAssociatedHostExec(ctx context.Context, info *execs.ExecInfo) error {
	h := info.GetAndroid().GetAssociatedHostname()
	log.Debugf(ctx, "Attached DUT associated host: %q", h)
	if h != "" {
		return nil
	}
	return errors.Reason("attached dut associated host is empty").Err()
}

func init() {
	execs.Register("android_dut_has_board_name", hasDutBoardExec)
	execs.Register("android_dut_has_model_name", hasDutModelExec)
	execs.Register("android_dut_has_serial_number", hasDutSerialNumberExec)
	execs.Register("android_dut_has_associated_host", hasDutAssociatedHostExec)
}
