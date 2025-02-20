// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/cros"
	"go.chromium.org/infra/cros/recovery/internal/components/servo"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/internal/retry"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
)

// TODO(otabek@): Extract all commands to constants.
// NOTE: That is just fake execs for local testing during developing phase. The correct/final execs will be introduced later.

func servodEchoActionExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	sshCheck := argsMap.AsBool(ctx, "ssh_check", false)
	if sshCheck {
		name := info.GetChromeos().GetServo().GetName()
		if name == "" {
			return errors.Reason("servod echo exec: servod host is not specified").Err()
		}
		run := info.NewRunner(name)
		if err := cros.IsSSHable(ctx, run, info.GetExecTimeout()); err != nil {
			info.AddObservation(metrics.NewStringObservation("ssh_check", "fail"))
			log.Debugf(ctx, "servod echo exec: error %s while verifying connection to the servo host", err)
		} else {
			info.AddObservation(metrics.NewStringObservation("ssh_check", "pass"))
		}
	}
	res, err := servodGetString(ctx, info.NewServod(), "root.serialname")
	if err != nil {
		return errors.Annotate(err, "servod echo exec").Err()
	} else if res == "" {
		return errors.Reason("servod echo exec: received empty result").Err()
	}
	return nil
}

func servodLidopenActionExec(ctx context.Context, info *execs.ExecInfo) error {
	res, err := servodGetString(ctx, info.NewServod(), "lid_open")
	if err != nil {
		return errors.Annotate(err, "servod lid_open").Err()
	} else if res == "not_applicable" {
		log.Infof(ctx, "Device does not support this action. Skipping...")
	} else if res != "yes" {
		return errors.Reason("servod lid_open: received %q, expected 'yes'", res).Err()
	}
	return nil
}

const (
	// Time to allow for boot from power off. Among other things, this must account for the 30 second dev-mode
	// screen delay, time to start the network on the DUT, and the ssh timeout of 120 seconds.
	dutBootTimeout = 150 * time.Second
)

func servodDUTColdResetActionExec(ctx context.Context, info *execs.ExecInfo) error {
	if err := info.NewServod().Set(ctx, "power_state", "reset"); err != nil {
		return errors.Annotate(err, "servod cold_reset dut").Err()
	}
	return retry.WithTimeout(ctx, 5*time.Second, dutBootTimeout, func() error {
		return info.GetAccess().Ping(ctx, info.GetDut().Name, 2)
	}, "servod cold_reset dut: check ping access")
}

// servodHasExec verifies whether servod supports the command
// mentioned in action args.
func servodHasExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	command := argsMap.AsString(ctx, "command", "")
	if len(command) == 0 {
		return errors.Reason("servod has exec: no command is not provided").Err()
	}
	if err := info.NewServod().Has(ctx, command); err != nil {
		return errors.Annotate(err, "servod has exec").Err()
	}
	log.Debugf(ctx, "Servod Has Exec: Command %s is supported by servod", command)
	return nil
}

// servodCanReadAllExec verifies whether servod supports the list of
// commands mentioned in action args. The check can require all the
// commands be supported, or any one of them can be supported. This
// behavior is controlled by the value of 'any_one' extra arg in the
// config.
func servodCanReadAllExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	// The string 'commands' here is the token from config that
	// signifies the list of commands that servod may need to support.
	// TODO(vkjoshi): if more execs need this token, consider
	// extracting this out and creating a constant out of it.
	commands := argsMap.AsStringSlice(ctx, "commands", nil)
	// This token controls whether all the loaded servod commands need
	// to succeed, or can we greedily return as soon as any one
	// command succeeds.
	anyOne := argsMap.AsBool(ctx, "any_one", false)
	log.Debugf(ctx, "Servod Can Read All Exec: anyOne:%t.", anyOne)
	s := info.NewServod()
	for _, c := range commands {
		if err := s.Has(ctx, c); err != nil {
			log.Debugf(ctx, "Servod Can Read All Exec: control %q is not loaded, skipping this.", c)
			if !anyOne {
				return errors.Annotate(err, "servod can read all exec").Err()
			}
		} else {
			log.Debugf(ctx, "Servod Can Read All Exec: control %q is loaded.", c)
			if _, err = s.Get(ctx, c); err != nil {
				log.Debugf(ctx, "Servod Can Read All Exec: could not read the control %q.", c)
				if !anyOne {
					return errors.Annotate(err, "servod can read all exec").Err()
				}
			} else {
				log.Debugf(ctx, "Servod Can Read All Exec: %q was read successfully.", c)
				if anyOne {
					return nil
				}
			}
		}
	}
	if anyOne {
		return errors.Reason("servod can read all exec: no control could be read.").Err()
	}
	return nil
}

// servodSetActiveDutControllerExec sets the main servo device as the
// active DUT controller.
func servodSetActiveDutControllerExec(ctx context.Context, info *execs.ExecInfo) error {
	servod := info.NewServod()
	servoType, err := servo.GetServoType(ctx, servod)
	if err != nil {
		return errors.Annotate(err, "servod set active dut control").Err()
	}
	if !strings.Contains(servoType.String(), "_and_") {
		// We have only one child device it will be detect by it self.
		return nil
	}
	mainDevice := servoType.MainDevice()
	if mainDevice == "" {
		return errors.Reason("servod set active dut controller: main device is empty.").Err()
	}
	command := "active_dut_controller"
	if err = servod.Set(ctx, command, mainDevice); err != nil {
		return errors.Annotate(err, "servod set active dut controller").Err()
	}
	returnedMainDevice, err := servodGetString(ctx, servod, command)
	if err != nil {
		return errors.Annotate(err, "servod set active dut controller").Err()
	}
	if returnedMainDevice != mainDevice {
		return errors.Reason("servod set active dut controller: expected the main device to be %q, but found it to be %q", mainDevice, returnedMainDevice).Err()
	}
	log.Debugf(ctx, "Servod Set Active Dut Controller: the expected value of servod control %q matches the value returned.", command)
	return nil
}

func init() {
	execs.Register("servod_echo", servodEchoActionExec)
	execs.Register("servod_lidopen", servodLidopenActionExec)
	execs.Register("servod_dut_cold_reset", servodDUTColdResetActionExec)
	execs.Register("servod_has", servodHasExec)
	execs.Register("servod_can_read_all", servodCanReadAllExec)
	execs.Register("servod_set_main_device", servodSetActiveDutControllerExec)
}
