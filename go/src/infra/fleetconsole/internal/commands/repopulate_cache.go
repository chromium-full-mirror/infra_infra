// Copyright 2025 The Chromium Authors
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

// RepopulateCacheCommand manually triggers a job to repopulate the cache.
var RepopulateCacheCommand *subcommands.Command = &subcommands.Command{
	UsageLine: "repopulate-cache [options...]",
	ShortDesc: "repopulate the cache",
	LongDesc:  "repopulate the cache",
	CommandRun: func() subcommands.CommandRun {
		c := &repopulateCacheCommand{}
		c.Init()
		return c
	},
}

type repopulateCacheCommand struct {
	site.Subcommand
}

// Run is the main entrypoint to repopulating the cache.
func (c *repopulateCacheCommand) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	err := c.innerRun(ctx, a, args, env)
	return c.Done(ctx, err)
}

func (c *repopulateCacheCommand) innerRun(ctx context.Context, a subcommands.Application, _ []string, _ subcommands.Env) error {
	host, err := c.CommonFlags.Host()
	if err != nil {
		return errors.Annotate(err, "repopulate cache").Err()
	}
	client, err := consoleClient(ctx, host, c.AuthFlags, c.CommonFlags.HTTP(), 30*time.Minute)
	if err != nil {
		return errors.Annotate(err, "repopulate cache").Err()
	}
	resp, err := client.RepopulateCache(ctx, &fleetconsolerpc.RepopulateCacheRequest{})
	if err != nil {
		return errors.Annotate(err, "repopulate cache").Err()
	}
	showProto(a.GetOut(), resp)
	return errors.Annotate(err, "repopulate cache").Err()
}
