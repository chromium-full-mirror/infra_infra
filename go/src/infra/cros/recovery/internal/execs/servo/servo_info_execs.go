// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	components_cros "go.chromium.org/infra/cros/recovery/internal/components/cros"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/execs/servo/topology"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

const (
	rebootRequestCreateSingleGlob = "touch /var/lib/servod/%d_reboot"
)

// servoVerifyPortNumberExec verifies that the servo host attached to
// the DUT has a port number configured for running servod daemon on
// the servo host.
func servoVerifyPortNumberExec(ctx context.Context, info *execs.ExecInfo) error {
	if port := info.NewServod().Port(); port > 9000 {
		log.Debugf(ctx, "Servo Verify Port Number Exec: %d", port)
		return nil
	}
	return errors.Reason("servo verify port number: port number is not available").Err()
}

// servoNotConnectedByHubExec checks if the servo is not connected by hub.
func servoNotConnectedByHubExec(ctx context.Context, info *execs.ExecInfo) error {
	servoInfo := info.GetChromeos().GetServo()
	if servoInfo == nil {
		return errors.Reason("servo does not exist").Err()
	}
	if err := components_cros.IsPingable(ctx, 1, info.NewPinger(servoInfo.GetName())); err != nil {
		return errors.Reason("servo is not pingable").Err()
	}
	run := info.NewRunner(servoInfo.GetName())
	rootServoPath, err := topology.GetRootServoPath(ctx, run, servoInfo.SerialNumber)
	if err != nil {
		return errors.Annotate(err, "cannot get servo usb path").Err()
	}
	if len(strings.Split(filepath.Base(rootServoPath), ".")) > 2 {
		return errors.Reason("servo is connected to labstation via hub").Err()
	}
	return nil
}

// createRebootRequestExec creates reboot flag file request in labstation.
func createRebootRequestExec(ctx context.Context, info *execs.ExecInfo) error {
	run := info.NewRunner(info.GetChromeos().GetServo().GetName())
	_, err := run(ctx, time.Minute, fmt.Sprintf(rebootRequestCreateSingleGlob, info.GetChromeos().GetServo().GetServodPort()))
	if err != nil {
		// Print finish result as we ignore any errors.
		log.Debugf(ctx, "Create the reboot request in labstation: %s", err)
	}
	return nil
}

func init() {
	execs.Register("servo_servod_port_present", servoVerifyPortNumberExec)
	execs.Register("servo_not_connected_by_hub", servoNotConnectedByHubExec)
	execs.Register("labstation_create_reboot_request", createRebootRequestExec)
}
