// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/logger"
)

const (
	servoV4p1NetResetControl = "dut_eth_pwr_en"
)

// ResetServoV4p1EthernetController resets the ethernet controller to servo_v4p1 device.
//
// The failures is hidden as off/on flow cannot be disturber and left device in middle stage.
func ResetServoV4p1EthernetController(ctx context.Context, servod components.Servod, log logger.Logger, timeout time.Duration) error {
	if err := servod.Has(ctx, servoV4p1NetResetControl); err != nil {
		log.Debugf("Reset servo_v4p1 network controller: control is not supported")
		return nil
	}
	log.Infof("Resetting servo's Ethernet controller...")
	defer func() {
		if err := servod.Set(ctx, servoV4p1NetResetControl, "on"); err != nil {
			log.Debugf("Reset servo_v4p1 network controller: fail to set 'on' value. Error: %q", err.Error())
		}
	}()
	if err := servod.Set(ctx, servoV4p1NetResetControl, "off"); err != nil {
		log.Debugf("Reset servo_v4p1 network controller: fail to set 'off' value. Error: %q", err.Error())
		return errors.Annotate(err, "set servo_v4.1 ethernet power").Err()
	}
	time.Sleep(timeout)
	return nil
}
