// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package commands contains the fleet console CLI.
package commands

import (
	"context"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/site"
)

// PingBigQueryCommand pings the service.
var PingBigQueryCommand *subcommands.Command = &subcommands.Command{
	UsageLine: "ping-bq [options...]",
	ShortDesc: "ping a big query service",
	LongDesc:  "Ping a big query service",
	CommandRun: func() subcommands.CommandRun {
		c := &pingBigQueryQueryCommand{}
		c.Init()
		return c
	},
}

type pingBigQueryQueryCommand struct {
	site.Subcommand
}

// Run is the main entrypoint to the ping.
func (c *pingBigQueryQueryCommand) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	err := c.innerRun(ctx, a, args, env)
	return c.Done(ctx, err)
}

func (c *pingBigQueryQueryCommand) innerRun(ctx context.Context, a subcommands.Application, args []string, env subcommands.Env) error {
	host, err := c.CommonFlags.Host()
	if err != nil {
		return errors.Annotate(err, "ping big query").Err()
	}
	client, err := consoleClient(ctx, host, c.AuthFlags, c.CommonFlags.HTTP())
	if err != nil {
		return err
	}
	resp, err := client.PingBigQuery(ctx, &fleetconsolerpc.PingBigQueryRequest{})
	if err != nil {
		return errors.Annotate(err, "ping big query").Err()
	}
	showProto(a.GetOut(), resp)
	return nil
}
