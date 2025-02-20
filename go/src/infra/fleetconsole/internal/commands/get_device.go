// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"infra/fleetconsole/internal/devicemanagerclient"
	"infra/fleetconsole/internal/site"
)

var GetDeviceCommand *subcommands.Command = &subcommands.Command{
	UsageLine: "get-device [options...]",
	ShortDesc: "",
	LongDesc:  "",
	CommandRun: func() subcommands.CommandRun {
		c := &getDeviceCommand{}
		c.Init()
		c.Flags.StringVar(&c.name, "name", "", "name of the device to ping")
		c.Flags.StringVar(&c.mode, "mode", "default", `how to ping DM {"default", "direct"}`)
		return c
	},
}

type getDeviceCommand struct {
	site.Subcommand

	name string

	// Possible modes are:
	// 1) "default"   <- default, through console server.
	// 2) "direct"    <- ping DM without an intermediary.
	mode string
}

// Run is the main entrypoint to the ping.
func (c *getDeviceCommand) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	err := c.innerRun(ctx, a, args, env)
	return c.Done(ctx, err)
}

func (c *getDeviceCommand) innerRun(ctx context.Context, a subcommands.Application, args []string, env subcommands.Env) error {
	switch c.mode {
	case "default":
		return errors.New("not implemented")
	case "direct":
		client, err := dmClient(ctx, devicemanagerclient.DMProdURL, c.AuthFlags)
		if err != nil {
			return errors.Annotate(err, "ping (direct)").Err()
		}

		resp, err := client.GetDevice(ctx, &api.GetDeviceRequest{
			Name: c.name,
		})
		if err != nil {
			return err
		}

		showProto(a.GetOut(), resp)
		return nil
	}
	return fmt.Errorf("bad mode %q", c.mode)
}
