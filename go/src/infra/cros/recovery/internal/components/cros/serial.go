// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// HostSerial read serial-number from the DUT.
func HostSerial(ctx context.Context, dut *tlw.Dut, timeout time.Duration, run components.Runner) (string, error) {
	if dut.GetChromeos().GetIsAndroidBased() {
		hwid, err := run(ctx, timeout, "getprop", "ro.serialno")
		return hwid, errors.Annotate(err, "host serial-number").Err()
	}
	hwid, err := run(ctx, timeout, "vpd -g serial_number")
	return hwid, errors.Annotate(err, "host serial-number").Err()
}
