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

	"infra/cmdsupport/cmdlib"
	fleetcostpb "infra/cros/fleetcost/api/models"
	fleetcostAPI "infra/cros/fleetcost/api/rpc"
	"infra/cros/fleetcost/internal/site"
)

// DeleteCostIndicatorCommand deletes a cost indicator.
var DeleteCostIndicatorCommand *subcommands.Command = &subcommands.Command{
	UsageLine: "delete-ci [options...]",
	ShortDesc: "delete a cost indicator",
	LongDesc:  "Delete a cost indicator",
	CommandRun: func() subcommands.CommandRun {
		c := &deleteCostIndicatorCommand{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.authFlags.RegisterIDTokenFlags(&c.Flags)
		c.commonFlags.Register(&c.Flags)
		c.Flags.StringVar(&c.primary, "primary", "", "the board of the indicator to delete")
		c.Flags.StringVar(&c.secondary, "secondary", "", "the model of the indicator to delete")
		c.Flags.StringVar(&c.tertiary, "tertiary", "", "the sku of the indicator to delete")
		c.Flags.Func("location", "the location of the thing to delete", makeLocationRecorder(&c.location))
		c.Flags.Func("type", "the type of the thing to delete", makeTypeRecorder(&c.typ))
		return c
	},
}

type deleteCostIndicatorCommand struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	commonFlags site.CommonFlags

	primary   string
	secondary string
	tertiary  string
	location  fleetcostpb.Location
	typ       fleetcostpb.IndicatorType
}

// Run is the main entrypoint to the deletion process.
func (c *deleteCostIndicatorCommand) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	if err := c.innerRun(ctx, a); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *deleteCostIndicatorCommand) innerRun(ctx context.Context, a subcommands.Application) error {
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
	fleetCostClient := fleetcostAPI.NewFleetCostPRPCClient(prpcClient)
	request := &fleetcostAPI.DeleteCostIndicatorRequest{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary:   c.primary,
			Secondary: c.secondary,
			Tertiary:  c.tertiary,
			Location:  c.location,
			Type:      c.typ,
		},
	}
	resp, err := fleetCostClient.DeleteCostIndicator(ctx, request)
	if err != nil {
		return errors.Annotate(err, "delete cost result").Err()
	}
	_, err = showProto(a.GetOut(), resp)
	return errors.Annotate(err, "delete cost result").Err()
}
