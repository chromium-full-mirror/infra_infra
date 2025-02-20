// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/cros"
	"go.chromium.org/infra/cros/recovery/internal/execs"
)

// servoHostIsLabstationExec confirms the servo host is a labstation
func servoHostIsLabstationExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	expected := argsMap.AsString(ctx, "board", "labstation")
	servo := info.GetChromeos().GetServo()
	if servo.GetName() == "" {
		return errors.Reason("servo host is labstation: servo is not present as part of dut info").Err()
	}
	run := info.NewRunner(servo.GetName())
	log := info.NewLogger()
	board, err := cros.ReleaseBoard(ctx, run, log)
	if err != nil {
		return errors.Annotate(err, "servo host is labstation").Err()
	}
	if !strings.Contains(board, expected) {
		return errors.Reason("servo host is not labstation").Err()
	}
	return nil
}

// servoUsesServodContainerExec checks if the servo uses a servod-container.
func servoUsesServodContainerExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	isContainer := IsContainerizedServoHost(ctx, info.GetChromeos().GetServo())
	reverse := argsMap.AsBool(ctx, "reverse", false)
	if reverse {
		if isContainer {
			return errors.Reason("servo uses servod container: container is specified").Err()
		}
	} else {
		if !isContainer {
			return errors.Reason("servo uses servod container: container is not specified").Err()
		}
	}
	return nil
}

func init() {
	execs.Register("servo_host_is_labstation", servoHostIsLabstationExec)
	execs.Register("servo_uses_servod_container", servoUsesServodContainerExec)
}
