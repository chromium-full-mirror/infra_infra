// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package commands contains the fleet console CLI.
package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/site"
	"go.chromium.org/infra/fleetconsole/internal/ufsclient"
	"go.chromium.org/infra/unifiedfleet/api/ufsclients"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

// PingUFSCommand pings ufs, via the Console UI server by default.
var PingUFSCommand *subcommands.Command = &subcommands.Command{
	UsageLine: "ping-ufs [options...]",
	ShortDesc: "ping UFS through a fleet console instance",
	LongDesc:  "Ping UFS through a fleet console instance",
	CommandRun: func() subcommands.CommandRun {
		c := &pingUFSCommand{}
		c.Init()
		c.Flags.StringVar(&c.mode, "mode", "default", `how to ping UFS {"default", "direct"}`)
		return c
	},
}

type pingUFSCommand struct {
	site.Subcommand
	// Possible modes are:
	// 1) "default"   <- default, through console server.
	// 2) "direct"    <- ping UFS without an intermediary.
	mode string
}

// Run is the main entrypoint to the ping.
func (c *pingUFSCommand) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	err := c.innerRun(ctx, a, args, env)
	return c.Done(ctx, err)
}

func (c *pingUFSCommand) innerRun(ctx context.Context, a subcommands.Application, _ []string, _ subcommands.Env) error {
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
		resp, err := client.PingUfs(ctx, &fleetconsolerpc.PingUfsRequest{})
		if err != nil {
			return errors.Annotate(err, "ping (default)").Err()
		}
		showProto(a.GetOut(), resp)
		return nil
	case "direct":
		ufsClient, err := ufsclients.NewUFSClientFromCLI(ctx, ufsclient.UfsProdURL, &c.AuthFlags, nil)
		if err != nil {
			return err
		}
		resp, err := ufsClient.ListMachineLSEs(ctx, &ufsAPI.ListMachineLSEsRequest{
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
