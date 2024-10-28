// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package commands contains the fleet console CLI.
package commands

import (
	"context"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"infra/fleetconsole/api/fleetconsolerpc"
	"infra/fleetconsole/internal/site"
)

// PingDeviceManagerCommand pings device manager, via the Console UI server by default.
var PingDeviceManagerCommand *subcommands.Command = &subcommands.Command{
	UsageLine: "ping-dm [options...]",
	ShortDesc: "ping device manager through a fleet console instance",
	LongDesc:  "Ping device manager through a fleet console instance",
	CommandRun: func() subcommands.CommandRun {
		c := &pingDeviceManagerCommand{}
		c.Init()
		return c
	},
}

type pingDeviceManagerCommand struct {
	site.Subcommand
}

// Run is the main entrypoint to the ping.
func (c *pingDeviceManagerCommand) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	err := c.innerRun(ctx, a, args, env)
	return c.Done(ctx, err)
}

func (c *pingDeviceManagerCommand) innerRun(ctx context.Context, a subcommands.Application, args []string, env subcommands.Env) error {
	host, err := c.CommonFlags.Host()
	if err != nil {
		return errors.Annotate(err, "ping").Err()
	}
	client, err := consoleClient(ctx, host, c.AuthFlags, c.CommonFlags.HTTP())
	if err != nil {
		return err
	}
	resp, err := client.PingDeviceManager(ctx, &fleetconsolerpc.PingDeviceManagerRequest{})
	if err != nil {
		return errors.Annotate(err, "ping").Err()
	}
	_, err = showProto(a.GetOut(), resp)
	return errors.Annotate(err, "ping").Err()
}
