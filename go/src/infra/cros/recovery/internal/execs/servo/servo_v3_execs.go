// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/retry"
)

// Verify that the root of servo is enumerated/present on servo_v3 host.
func isServoV3RootPresentExec(ctx context.Context, info *execs.ExecInfo) error {
	sh := info.GetChromeos().GetServo()
	if sh.GetName() == "" {
		return errors.Reason("servo_v3 root present: servo is not present as part of dut info").Err()
	}
	run := info.NewRunner(sh.GetName())
	log := info.NewLogger()
	am := info.GetActionArgs(ctx)
	vidPidList := am.AsStringSlice(ctx, "vids_pids", []string{"18d1:5004", "0403:6014"})
	retryCount := am.AsInt(ctx, "retry_count", 3)
	retryTimeout := am.AsDuration(ctx, "retry_timeout", 30, time.Second)
	cmd := fmt.Sprintf("lsusb | grep %q", strings.Join(vidPidList, "\\|"))
	funcRootComponent := func() error {
		out, err := run(ctx, retryTimeout, cmd)
		log.Debugf("Servo_v3 root present check: output: %s", out)
		if err != nil || out == "" {
			log.Debugf("Servo_v3 root present check: fail with %s", err.Error())
			return err
		}
		log.Debugf("Servo_v3 root present check: board is detected on servo_v3!")
		return nil
	}
	if err := retry.LimitCount(ctx, retryCount, -1, funcRootComponent, "retry to detect servo_v3 root"); err != nil {
		return errors.Annotate(err, "is servo_v3 root present").Err()
	}
	return nil
}

func init() {
	execs.Register("servo_v3_root_present", isServoV3RootPresentExec)
}
