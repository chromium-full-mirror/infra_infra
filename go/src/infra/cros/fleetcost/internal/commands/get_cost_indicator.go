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
	prpc "go.chromium.org/luci/grpc/prpc"

	"infra/cmdsupport/cmdlib"
	fleetcostpb "infra/cros/fleetcost/api/models"
	fleetcostAPI "infra/cros/fleetcost/api/rpc"
	"infra/cros/fleetcost/internal/site"
)

var GetCostIndicatorCommand *subcommands.Command = &subcommands.Command{
	UsageLine: "get-ci [options...]",
	ShortDesc: "get a cost indicator",
	LongDesc:  "Get a cost indicator",
	CommandRun: func() subcommands.CommandRun {
		c := &getCostIndicatorCommand{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.authFlags.RegisterIDTokenFlags(&c.Flags)
		c.commonFlags.Register(&c.Flags)
		c.Flags.StringVar(&c.primary, "primary", "", "primary")
		c.Flags.StringVar(&c.secondary, "secondary", "", "secondary")
		c.Flags.StringVar(&c.tertiary, "tertiary", "", "tertiary")
		c.Flags.Func("location", "where the device is located", makeLocationRecorder(&c.location))
		c.Flags.Func("type", "name of cost indicator", makeTypeRecorder(&c.typ))
		return c
	},
}

type getCostIndicatorCommand struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	commonFlags site.CommonFlags
	primary     string
	secondary   string
	tertiary    string
	location    fleetcostpb.Location
	typ         fleetcostpb.IndicatorType
}

// Run is the main entrypoint to the get-ci.
func (c *getCostIndicatorCommand) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	if err := c.innerRun(ctx, a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *getCostIndicatorCommand) innerRun(ctx context.Context, a subcommands.Application, args []string, env subcommands.Env) error {
	host, err := c.commonFlags.Host()
	if err != nil {
		return err
	}
	var httpClient *http.Client
	if !c.commonFlags.HTTP() {
		var err error
		httpClient, err = getSecureClient(ctx, host, c.authFlags)
		if err != nil {
			return err
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
	filter := &fleetcostAPI.ListCostIndicatorsFilter{
		Primary:   c.primary,
		Secondary: c.secondary,
		Tertiary:  c.tertiary,
	}
	if c.location != fleetcostpb.Location_LOCATION_UNKNOWN {
		filter.Location = c.location.String()
	}
	if c.typ != fleetcostpb.IndicatorType_INDICATOR_TYPE_UNKNOWN {
		filter.Type = c.typ.String()
	}
	resp, err := fleetCostClient.ListCostIndicators(
		ctx,
		&fleetcostAPI.ListCostIndicatorsRequest{Filter: filter},
	)
	if err != nil {
		return err
	}
	if _, err := showProto(a.GetOut(), resp); err != nil {
		return err
	}
	return nil
}
