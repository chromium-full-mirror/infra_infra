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

	"infra/fleetconsole/api/fleetconsolerpc"
	"infra/fleetconsole/internal/site"
)

// PingDBCommand tries to ping the database.
var PingDBCommand *subcommands.Command = &subcommands.Command{
	UsageLine: "ping-db [options...]",
	ShortDesc: "ping the db",
	LongDesc:  "ping the db",
	CommandRun: func() subcommands.CommandRun {
		c := &pingDBCommand{}
		c.Init()
		return c
	},
}

type pingDBCommand struct {
	site.Subcommand
}

// Run is the main entrypoint to pinging the DB.
func (c *pingDBCommand) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	err := c.innerRun(ctx, a, args, env)
	return c.Done(ctx, err)
}

func (c *pingDBCommand) innerRun(ctx context.Context, a subcommands.Application, _ []string, _ subcommands.Env) error {
	host, err := c.CommonFlags.Host()
	if err != nil {
		return errors.Annotate(err, "ping db").Err()
	}
	client, err := consoleClient(ctx, host, c.AuthFlags, c.CommonFlags.HTTP())
	if err != nil {
		return errors.Annotate(err, "ping db").Err()
	}
	resp, err := client.PingDB(ctx, &fleetconsolerpc.PingDBRequest{})
	if err != nil {
		return errors.Annotate(err, "ping db").Err()
	}
	_, err = showProto(a.GetOut(), resp)
	return errors.Annotate(err, "ping db").Err()
}
