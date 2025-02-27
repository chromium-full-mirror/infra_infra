// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package commands contains the fleet console CLI.
package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/maruel/subcommands"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/devicemanagerclient"
	"go.chromium.org/infra/fleetconsole/internal/site"
)

// PingDeviceManagerCommand pings device manager, via the Console UI server by default.
var PingDeviceManagerCommand *subcommands.Command = &subcommands.Command{
	UsageLine: "ping-dm [options...]",
	ShortDesc: "ping device manager through a fleet console instance",
	LongDesc:  "Ping device manager through a fleet console instance",
	CommandRun: func() subcommands.CommandRun {
		c := &pingDeviceManagerCommand{}
		c.Init()
		c.Flags.StringVar(&c.mode, "mode", "default", `how to ping DM {"default", "direct"}`)
		return c
	},
}

type pingDeviceManagerCommand struct {
	site.Subcommand
	// Possible modes are:
	// 1) "default"   <- default, through console server.
	// 2) "direct"    <- ping DM without an intermediary.
	mode string
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
	switch c.mode {
	case "default":
		client, err := consoleClient(ctx, host, c.AuthFlags, c.CommonFlags.HTTP(), 30*time.Second)
		if err != nil {
			return errors.Annotate(err, "ping (default)").Err()
		}
		resp, err := client.PingDeviceManager(ctx, &fleetconsolerpc.PingDeviceManagerRequest{})
		if err != nil {
			return errors.Annotate(err, "ping (default)").Err()
		}
		showProto(a.GetOut(), resp)
		return errors.Annotate(err, "ping (default)").Err()
	case "direct":
		client, err := dmClient(ctx, devicemanagerclient.DMProdURL, c.AuthFlags)
		if err != nil {
			return errors.Annotate(err, "ping (direct)").Err()
		}
		resp, err := client.ListDevices(ctx, &api.ListDevicesRequest{
			PageSize: 3,
		})
		if err != nil {
			return errors.Annotate(err, "ping (direct)").Err()
		}
		showProto(a.GetOut(), resp)
		return nil
	}
	return fmt.Errorf("bad mode %q", c.mode)
}
