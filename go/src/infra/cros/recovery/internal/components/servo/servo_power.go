// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components"
	"infra/cros/recovery/internal/log"
)

// TODO(b/222941834): These constants are repeated from the package
// "internal/execs/servo". Over a period of time, all such utilities
// that help exec functions will be migrated within components.
const (
	// servodPdRoleCmd is the servod command for
	servodPdRoleCmd = "servo_pd_role"
	// servodPdRoleValueSnk is the one of the two values for servodPdRoleCmd
	// snk is the state that servo in normal mode and not passes power to DUT.
	servodPdRoleValueSnk = "snk"
	// servodPdRoleValueSrc is the one of the two values for servodPdRoleCmd
	// src is the state that servo in power delivery mode and passes power to the DUT.
	servodPdRoleValueSrc = "src"
	// batteryChargePercentCmd is the servod command to determine the
	// battery charge percentage.
	batteryChargePercentCmd = "battery_charge_percent"
)

func ServodPdRoleCmd() string {
	return servodPdRoleCmd
}

func ServodPdRoleValueSnk() string {
	return servodPdRoleValueSnk
}

func ServodPdRoleValueSrc() string {
	return servodPdRoleValueSrc
}

// BatteryChargePercent obtains the current battery charge percentage
// using servod controls, and returns the charge value or any error
// encountered while obtaining the value of servod control.
func BatteryChargePercent(ctx context.Context, servod components.Servod) (int32, error) {
	batteryLevel, err := GetInt(ctx, servod, batteryChargePercentCmd)
	if err != nil {
		return 0, errors.Annotate(err, "battery charge percent").Err()
	}
	return batteryLevel, nil
}

type PDRole struct {
	role string
}

var (
	PD_ON  = PDRole{ServodPdRoleValueSrc()}
	PD_OFF = PDRole{ServodPdRoleValueSnk()}
)

// SetPDRole sets the power-delivery role for servo to the passed
// role-value if the power-delivery control is supported by servod.
func SetPDRole(ctx context.Context, servod components.Servod, role PDRole) error {
	currentRole, err := GetString(ctx, servod, ServodPdRoleCmd())
	if err != nil {
		log.Debugf(ctx, "Set PD Role: could not determine current PD role")
		return errors.Annotate(err, "set PD role").Err()
	}
	currentPDRole := PDRole{currentRole}
	if currentPDRole == role {
		log.Debugf(ctx, "Set PD Role: PD role is already %q", role)
		return nil
	}
	if err := servod.Set(ctx, ServodPdRoleCmd(), role.role); err != nil {
		log.Debugf(ctx, "Set PD Role: %q", err.Error())
		return errors.Annotate(err, "set PD role").Err()
	}
	return nil
}
