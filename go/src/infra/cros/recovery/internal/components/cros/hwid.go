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

// HostHWID read HWID from the DUT.
func HostHWID(ctx context.Context, dut *tlw.Dut, timeout time.Duration, run components.Runner) (string, error) {
	if dut.GetChromeos().GetIsAndroidBased() {
		hwid, err := run(ctx, timeout, "getprop", "ro.boot.product.hardware.id")
		return hwid, errors.WrapIf(err, "host HWID")
	}
	hwid, err := run(ctx, timeout, "crossystem hwid")
	return hwid, errors.WrapIf(err, "host HWID")
}
