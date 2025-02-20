// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"net/http"
	"time"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"
	prpc "go.chromium.org/luci/grpc/prpc"

	"go.chromium.org/infra/cmdsupport/cmdlib"
	fleetcostAPI "go.chromium.org/infra/cros/fleetcost/api/rpc"
	"go.chromium.org/infra/cros/fleetcost/internal/site"
)

// RepopulateCacheCommand repopulates the cache.
var RepopulateCacheCommand *subcommands.Command = &subcommands.Command{
	UsageLine: "repopulate-cache [options...]",
	ShortDesc: "Repopulate the cache",
	LongDesc:  "Repopulate the cache",
	CommandRun: func() subcommands.CommandRun {
		c := &repopulateCacheCommand{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.authFlags.RegisterIDTokenFlags(&c.Flags)
		c.commonFlags.Register(&c.Flags)
		c.Flags.BoolVar(&c.forgiveMissingEntries, "forgive", false, "forgive missing entries")
		return c
	},
}

type repopulateCacheCommand struct {
	subcommands.CommandRunBase
	authFlags             authcli.Flags
	commonFlags           site.CommonFlags
	forgiveMissingEntries bool
}

// Run is the main entrypoint for calling the RepopulateCache RPC.
func (c *repopulateCacheCommand) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	if err := c.innerRun(ctx, a); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *repopulateCacheCommand) innerRun(ctx context.Context, a subcommands.Application) error {
	host, err := c.commonFlags.Host()
	if err != nil {
		return errors.Annotate(err, "get cost result command").Err()
	}
	var httpClient *http.Client
	if !c.commonFlags.HTTP() {
		var err error
		httpClient, err = getSecureClient(ctx, host, c.authFlags)
		if err != nil {
			return errors.Annotate(err, "get cost result").Err()
		}
	}
	prpcClient := &prpc.Client{
		C:    httpClient,
		Host: host,
		Options: &prpc.Options{
			Insecure:      c.commonFlags.HTTP(),
			PerRPCTimeout: 60 * time.Minute,
		},
	}
	fleetCostClient := fleetcostAPI.NewFleetCostPRPCClient(prpcClient)
	request := &fleetcostAPI.RepopulateCacheRequest{
		ForgiveMissingEntries: c.forgiveMissingEntries,
	}
	resp, err := fleetCostClient.RepopulateCache(ctx, request)
	if err != nil {
		return errors.Annotate(err, "repopulate cache").Err()
	}
	_, err = showProto(a.GetOut(), resp)
	return errors.Annotate(err, "repopulate cache").Err()
}
