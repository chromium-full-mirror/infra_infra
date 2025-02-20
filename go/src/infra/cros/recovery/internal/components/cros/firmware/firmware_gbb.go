// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
)

// ReadGBBByServo read GBB flags from DUT.
func ReadGBBByServo(ctx context.Context, timeout time.Duration, run components.Runner, servod components.Servod) (string, error) {
	if run == nil || servod == nil {
		return "", errors.Reason("read GBB by servo: run or servod is not provided").Err()
	}
	r, err := regexp.Compile(`flags: ([0-9x]*)$`)
	if err != nil {
		return "", errors.Annotate(err, "read GBB by servo").Err()
	}
	const readGbbCmd = "futility gbb --servo_port %d --get --flags"
	out, err := run(ctx, timeout, fmt.Sprintf(readGbbCmd, servod.Port()))
	if err != nil {
		return "", errors.Annotate(err, "read GBB by servo").Err()
	}
	matches := r.FindAllStringSubmatch(out, -1)
	if len(matches) == 0 || len(matches[0]) < 2 {
		return "", errors.Reason("read GBB by servo: gbb not found").Err()
	}
	return matches[0][1], nil
}

// SetGBBByServo updates GBB flags on DUT.
//
// GBB expected in the format of 0x18 or 0x0.
func SetGBBByServo(ctx context.Context, gbb string, timeout time.Duration, run components.Runner, servod components.Servod) error {
	if run == nil || servod == nil {
		return errors.Reason("set GBB by servo: run or servod is not provided").Err()
	}
	const setGbbCmd = "futility gbb --servo_port %d --set --flags %s"
	_, err := run(ctx, timeout, fmt.Sprintf(setGbbCmd, servod.Port(), gbb))
	return errors.Annotate(err, "set GBB by servo").Err()
}
