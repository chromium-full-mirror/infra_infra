// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows

// Package ccd contains functionality around management of the GSC console.
package ccd

import (
	"fmt"
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"

	"infra/cros/satlab/common/dut"
	"infra/cros/satlab/common/utils/executor"
)

// CCDOpenCmd contains the usage and implementation for the ccd open command
var CCDOpenCmd = &subcommands.Command{
	UsageLine: "open <args>",
	ShortDesc: `Opens CCD`,
	LongDesc: `Opens CCD, enables testlab mode and optionally resets CCD to factory settings.
	Providing servo serial or USB device is optional if only one device is connected to the host.
	Servo V4, V4p1 and SuzyQ cable are supported. You can provide either a Servo serial number
	or a path to a Cr50/Ti50 serial console USB device (/dev/ttyUSBy).`,
	CommandRun: func() subcommands.CommandRun {
		c := &ccdOpenRun{}
		c.Flags.StringVar(&c.ServoSerial, "servo-serial", "", "Servo serial number")
		c.Flags.StringVar(&c.UsbDevice, "usb-device", "", "Cr50/Ti50 serial console USB device")
		c.Flags.BoolVar(&c.UseRmaAuth, "rma-auth", false, "Use rma_auth to open CCD")
		c.Flags.BoolVar(&c.ResetFactory, "reset-factory", false, "Reset CCD to factory settings")
		return c
	},
}

// ccdOpenRun struct contains the arguments needed to run CCDOpenCmd and common fields for the methods
type ccdOpenRun struct {
	subcommands.CommandRunBase

	dut.CCDOpenRun
}

// Run is what is called when a user inputs the ccd open command
func (c *ccdOpenRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

// innerRun handles all orchestration needed to pass appropriate clients, contexts and commands into the application
func (c *ccdOpenRun) innerRun(a subcommands.Application, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	err := c.TriggerRun(ctx, &executor.ExecCommander{}, os.Stdout)
	return err
}
