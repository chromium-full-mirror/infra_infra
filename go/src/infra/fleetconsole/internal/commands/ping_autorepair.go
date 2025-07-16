// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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
)

// PingAutorepairCommand contains ping-autorepair command specification
var PingAutorepairCommand = &subcommands.Command{
	UsageLine: "ping-autorepair <dut_name...>",
	ShortDesc: "Ping the ScheduleAutorepair RPC.",
	LongDesc:  "Pings the ScheduleAutorepair RPC endpoint by scheduling a repair for the provided DUTs.",
	CommandRun: func() subcommands.CommandRun {
		c := &pingAutorepairCommand{}
		c.Init()
		return c
	},
}

type pingAutorepairCommand struct {
	site.Subcommand
}

func (c *pingAutorepairCommand) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	if len(args) == 0 {
		fmt.Fprintf(a.GetErr(), "no DUT name provided - at least one DUT name is required\n")
		return 1
	}

	err := c.innerRun(ctx, a, args, env)
	return c.Done(ctx, err)
}

func (c *pingAutorepairCommand) innerRun(ctx context.Context, a subcommands.Application, dutNames []string, env subcommands.Env) error {
	host, err := c.CommonFlags.Host()
	if err != nil {
		return errors.Annotate(err, "ping-autorepair - getting host").Err()
	}

	client, err := consoleClient(ctx, host, c.AuthFlags, c.CommonFlags.HTTP(), 30*time.Second)
	if err != nil {
		return errors.Annotate(err, "ping-autorepair - getting console client").Err()
	}

	req := &fleetconsolerpc.ScheduleAutorepairRequest{
		UnitNames: dutNames,
	}

	resp, err := client.ScheduleAutorepair(ctx, req)
	if err != nil {
		return errors.Annotate(err, "ping-autorepair - scheduling autorepair").Err()
	}

	showProto(a.GetOut(), resp)
	return nil
}
