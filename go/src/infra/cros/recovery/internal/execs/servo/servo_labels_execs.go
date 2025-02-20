// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/servo"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// updateServoTypeLabelExec updates DUT's servo type to the correct servo type string.
func updateServoTypeLabelExec(ctx context.Context, info *execs.ExecInfo) error {
	sh := info.GetChromeos().GetServo()
	if sh.GetName() == "" {
		return errors.Reason("update servo type: servo is not present as part of dut info").Err()
	}
	servoType, err := servo.GetServoType(ctx, info.NewServod())
	if err != nil {
		return errors.Annotate(err, "update servo type label").Err()
	}
	sh.ServodType = servoType.String()
	log.Infof(ctx, "Set DUT's servo type to be: %s", servoType)
	return nil
}

func init() {
	execs.Register("servo_update_servo_type_label", updateServoTypeLabelExec)
}
