// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package commands contains the fleet console CLI.
package commands

import (
	"context"
	"time"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/site"
)

// PingCommand pings the service.
var PingCommand *subcommands.Command = &subcommands.Command{
	UsageLine: "ping [options...]",
	ShortDesc: "ping a fleet console instance",
	LongDesc:  "Ping a fleet console instance",
	CommandRun: func() subcommands.CommandRun {
		c := &pingCommand{}
		c.Init()
		return c
	},
}

type pingCommand struct {
	site.Subcommand
}

// Run is the main entrypoint to the ping.
func (c *pingCommand) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	err := c.innerRun(ctx, a, args, env)
	return c.Done(ctx, err)
}

func (c *pingCommand) innerRun(ctx context.Context, a subcommands.Application, args []string, env subcommands.Env) error {
	host, err := c.CommonFlags.Host()
	if err != nil {
		return errors.Annotate(err, "ping").Err()
	}
	client, err := consoleClient(ctx, host, c.AuthFlags, c.CommonFlags.HTTP(), 30*time.Second)
	if err != nil {
		return err
	}
	resp, err := client.Ping(ctx, &fleetconsolerpc.PingRequest{})
	if err != nil {
		return errors.Annotate(err, "ping").Err()
	}
	showProto(a.GetOut(), resp)
	return nil
}
