// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package rpm wraps xmlrpc communications to rpm service.
package rpm

import (
	"context"
	"time"

	xmlrpc_value "go.chromium.org/chromiumos/config/go/api/test/xmlrpc"
	"go.chromium.org/luci/common/errors"

	"infra/cros/internal/env"
	"infra/cros/recovery/internal/localtlw/xmlrpc"
)

const (
	// A normal RPM request would take no longer than 10 seconds,
	// leave it at 60 seconds here for some buffer.
	setPowerTimeout = 60 * time.Second
	// The k8s cluster internal service name point to rpm frontend server.
	rpmServiceHost = "rpm-service"
	// The service port of rpm frontend server.
	rpmServicePort = 9999
)

// PowerState indicates a state we want to set for a outlet on powerunit.
type PowerState string

const (
	// Map to ON state on powerunit.
	PowerStateOn PowerState = "ON"
	// Map to OFF state on powerunit.
	PowerStateOff PowerState = "OFF"
	// CYCLE state will tell RPM server to set a outlet to OFF state and then ON (with necessary interval).
	PowerStateCycle PowerState = "CYCLE"
)

var sentryStateMap = map[PowerState]string{
	PowerStateOn:    "on",
	PowerStateOff:   "off",
	PowerStateCycle: "reboot",
}

// RPMType is the RPM model/protocol for the device
type RPMType string

const (
	// Sentry Switched CDU RPM
	RPMTypeSentry RPMType = "SENTRY"
	// IPPower 9850 RPM
	RPMTypeIP9850 RPMType = "IP9850"
)

// RPMPowerRequest holds data required from rpm service to perform a state change.
type RPMPowerRequest struct {
	// Hostname of the DUT.
	Hostname string
	// Hostname of the RPM power unit, e.g. "chromeos6-row13_14-rack15-rpm2".
	PowerUnitHostname string
	// Name to locate a specific outlet from a RPM power unit, e.g. ".A7".
	PowerunitOutlet string
	// Hostname of hydra if the power unit is connected via a hydra.
	HydraHostname string
	// The expecting new state to set power to.
	State PowerState
	// RPMType is the RPM model/protocol for the device
	Type RPMType
}

// SetPowerState talks to RPM service via xmltpc to set power state based on a RPMPowerRequest.
func SetPowerState(ctx context.Context, req *RPMPowerRequest) error {
	var err error
	if err := validateRequest(req); err != nil {
		return errors.Annotate(err, "set power state").Err()
	}
	c := xmlrpc.New(rpmServiceHost, rpmServicePort)
	// We need to convert PowerState type back to string here as xmlrpc.NewValue cannot recognize the customized type during unpack.
	call := xmlrpc.NewCallTimeout(setPowerTimeout, "set_power_via_rpm", req.Hostname, req.PowerUnitHostname, req.PowerunitOutlet, req.HydraHostname, string(req.State))
	result := &xmlrpc_value.Value{}
	if env.IsCloudBot() {
		err = c.RunOnCloudBots(ctx, call, result)
	} else {
		err = c.Run(ctx, call, result)
	}
	if err != nil {
		return errors.Annotate(err, "set power state").Err()
	}
	// We only expect a boolean response from rpm server to determine if the operation success or not.
	if !result.GetBoolean() {
		return errors.Reason("set power state: failed to change outlet status for host: %s to state: %s.", req.Hostname, req.State).Err()
	}
	return nil
}

// SetPowerStateHTTP talks to RPM directly over HTTPS to set power state based on RPMPowerRequest.
func SetPowerStateHTTP(ctx context.Context, req *RPMPowerRequest) error {
	if err := validateRequest(req); err != nil {
		return errors.Annotate(err, "set power state").Err()
	}
	switch req.Type {
	case RPMTypeSentry:
		return setPowerStateSentry(ctx, req)
	case RPMTypeIP9850:
		return setPowerStateIP9850(ctx, req)
	case "":
		return errors.Reason("SetPowerStateHTTP: RPMType cannot be empty.").Err()
	default:
		return errors.Reason("SetPowerStateHTTP: unknown RPMType %s.", req.Type).Err()
	}
}

// setPowerStateSentry sets power state for Sentry Switched CDU over HTTP based on RPMPowerRequest.
// http://www.servertech.com/products/switched-pdus/
// https://cdn10.servertech.com/assets/documents/documents/968/original/JSON_API_Web_Service_%28JAWS%29_V1.06.pdf?1641867726
func setPowerStateSentry(ctx context.Context, r *RPMPowerRequest) error {
	return errors.Reason("Not implemented").Err()
}

// setPowerStateSentry sets power state for IPPower 9850 RPM over HTTP based on RPMPowerRequest.
// https://www.aviosys.com/products/9850.php
func setPowerStateIP9850(ctx context.Context, req *RPMPowerRequest) error {
	return errors.Reason("Not implemented").Err()
}

// validateRequest validates args in a RPMPowerRequest.
func validateRequest(req *RPMPowerRequest) error {
	if req.Hostname == "" {
		return errors.Reason("validate request: Hostname cannot be empty.").Err()
	}
	if req.PowerUnitHostname == "" {
		return errors.Reason("validate request: PowerUnitHostname cannot be empty.").Err()
	}
	if req.PowerunitOutlet == "" {
		return errors.Reason("validate request: PowerUnitOutlet cannot be empty.").Err()
	}
	switch req.State {
	case PowerStateOn, PowerStateOff, PowerStateCycle:
		return nil
	case "":
		return errors.Reason("validate request: State cannot be empty.").Err()
	default:
		return errors.Reason("validate request: unknown State %s.", req.State).Err()
	}
}
