// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
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
	"go.chromium.org/infra/cros/fleetcost/internal/utils"
)

// GetCostResultCommand pings UFS via the fleet cost service.
var GetCostResultCommand *subcommands.Command = &subcommands.Command{
	UsageLine: "get-cost [options...]",
	ShortDesc: "Get cost result of a particular DUT",
	LongDesc:  "Get cost result of a particular DUT",
	CommandRun: func() subcommands.CommandRun {
		c := &getCostResultCommand{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.authFlags.RegisterIDTokenFlags(&c.Flags)
		c.commonFlags.Register(&c.Flags)
		c.Flags.StringVar(&c.name, "name", "", "hostname of a DUT. If hints are provided, then we want the cost of a hypothetical DUT and the hostname should not be provided.")
		c.Flags.BoolVar(&c.lax, "lax", false, "whether to forgive missing cost entries")
		c.Flags.StringVar(&c.hints, "hints", "", "if provided, do not talk to UFS and instead use hints to generate the cost estimate. Comma-delimited.")
		c.Flags.BoolVar(&c.forceUpdate, "forceupdate", false, "if provided, force update the cache entry (false by default)")
		return c
	},
}

type getCostResultCommand struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	commonFlags site.CommonFlags

	name        string
	lax         bool
	hints       string
	forceUpdate bool
}

// Run is the main entrypoint to the ping.
func (c *getCostResultCommand) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	if err := c.innerRun(ctx, a); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *getCostResultCommand) innerRun(ctx context.Context, a subcommands.Application) error {
	c.commonFlags.VerboseLog(a, "starting get-ci")
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
			PerRPCTimeout: 30 * time.Second,
		},
	}
	hints := utils.SplitComma(c.hints)
	if (len(hints) == 0) && (c.name == "") {
		return fmt.Errorf("at least one hint (%q given) or name (%q given) must be provided", c.hints, c.name)
	}
	if (len(hints) != 0) && (c.name != "") {
		return fmt.Errorf("at most one hint (%q given) or one name (%q given) can be provided", c.hints, c.name)
	}
	fleetCostClient := fleetcostAPI.NewFleetCostPRPCClient(prpcClient)
	resp, err := fleetCostClient.GetCostResult(ctx, &fleetcostAPI.GetCostResultRequest{
		Hostname:              c.name,
		ForgiveMissingEntries: c.lax,
		NoUfs:                 len(hints) != 0,
		AnalysisHint:          hints,
		ForceUpdate:           c.forceUpdate,
	})
	if err != nil {
		c.commonFlags.VerboseLog(a, "RPC call failed.")
		return errors.Annotate(err, "get cost result").Err()
	}
	c.commonFlags.VerboseLog(a, "RPC call succeeded")
	_, err = showProto(a.GetOut(), resp)
	return errors.Annotate(err, "get cost result").Err()
}
