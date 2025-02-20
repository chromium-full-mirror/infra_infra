// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package commands contains the fleet console CLI.
package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/fleetconsole/internal/site"
)

// PingUICommand pings the service.
var PingUICommand *subcommands.Command = &subcommands.Command{
	UsageLine: "ping-ui [options...]",
	ShortDesc: "ping the UI",
	LongDesc:  "ping the UI",
	CommandRun: func() subcommands.CommandRun {
		c := &pingUICommand{}
		c.Init()
		return c
	},
}

type pingUICommand struct {
	site.Subcommand
}

// Run is the main entrypoint to the ping.
func (c *pingUICommand) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	err := c.innerRun(ctx, a, args, env)
	return c.Done(ctx, err)
}

func (c *pingUICommand) innerRun(ctx context.Context, a subcommands.Application, args []string, env subcommands.Env) error {
	host, err := c.CommonFlags.UIHost()
	if err != nil {
		return errors.Annotate(err, "ping").Err()
	}
	client, err := uiClient(ctx, host, c.AuthFlags)
	if err != nil {
		return err
	}
	resp, err := client.Get(fmt.Sprintf("https://%s/%s", host, "ui/fleet/labs/devices"))
	if err != nil {
		return err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(a.GetOut(), string(body))
	if err != nil {
		return err
	}
	return nil
}
