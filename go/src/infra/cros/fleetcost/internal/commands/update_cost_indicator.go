// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"net/http"
	"time"

	"github.com/maruel/subcommands"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"
	prpc "go.chromium.org/luci/grpc/prpc"

	"infra/cmdsupport/cmdlib"
	fleetcostpb "infra/cros/fleetcost/api/models"
	fleetcostAPI "infra/cros/fleetcost/api/rpc"
	"infra/cros/fleetcost/internal/site"
)

var UpdateCostIndicatorCommand *subcommands.Command = &subcommands.Command{
	UsageLine: "update-ci [options...]",
	ShortDesc: "update a cost indicator",
	LongDesc:  "Update a cost indicator",
	CommandRun: func() subcommands.CommandRun {
		c := &updateCostIndicatorCommand{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.authFlags.RegisterIDTokenFlags(&c.Flags)
		c.commonFlags.Register(&c.Flags)
		c.Flags.StringVar(&c.name, "name", "", "name of cost indicator")
		c.Flags.Func("type", "name of cost indicator", makeTypeRecorder(&c.typ))
		c.Flags.StringVar(&c.primary, "primary", "", "primary")
		c.Flags.StringVar(&c.secondary, "secondary", "", "secondary")
		c.Flags.StringVar(&c.tertiary, "tertiary", "", "tertiary")
		c.Flags.Func("cost", "cost", makeMoneyRecorder(&c.cost))
		c.Flags.Func("cadence", "cost-cadence", makeCostCadenceRecorder(&c.costCadence))
		c.Flags.Float64Var(&c.burnoutRate, "burnout", 0, "device burnout rate")
		c.Flags.Func("location", "where the device is located", makeLocationRecorder(&c.location))
		c.Flags.Float64Var(&c.amortizationInYears, "am", 0, "amortization time in years")
		return c
	},
}

type updateCostIndicatorCommand struct {
	subcommands.CommandRunBase
	authFlags           authcli.Flags
	commonFlags         site.CommonFlags
	name                string
	typ                 fleetcostpb.IndicatorType
	primary             string
	secondary           string
	tertiary            string
	cost                *money.Money
	costCadence         fleetcostpb.CostCadence
	burnoutRate         float64
	location            fleetcostpb.Location
	description         string
	amortizationInYears float64
}

// Run is the main entrypoint to update-ci.
func (c *updateCostIndicatorCommand) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	if err := c.innerRun(ctx, a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *updateCostIndicatorCommand) getFieldMaskPaths() []string {
	var out []string
	if c.name != "" {
		out = append(out, "name")
	}
	if int(c.typ) != 0 {
		out = append(out, "type")
	}
	if c.primary != "" {
		out = append(out, "primary")
	}
	if c.secondary != "" {
		out = append(out, "secondary")
	}
	if c.tertiary != "" {
		out = append(out, "tertiary")
	}
	if c.cost != nil {
		out = append(out, "cost")
	}
	if int(c.costCadence) != 0 {
		out = append(out, "cost_cadence")
	}
	if c.burnoutRate != 0 {
		out = append(out, "burnout_rate")
	}
	if int(c.location) != 0 {
		out = append(out, "location")
	}
	if c.description != "" {
		out = append(out, "description")
	}
	if c.amortizationInYears != 0 {
		out = append(out, "amortization_in_years")
	}
	return out
}

func (c *updateCostIndicatorCommand) innerRun(ctx context.Context, a subcommands.Application, args []string, env subcommands.Env) error {
	host, err := c.commonFlags.Host()
	if err != nil {
		return errors.Annotate(err, "update cost indicator command").Err()
	}
	var httpClient *http.Client
	if !c.commonFlags.HTTP() {
		var err error
		httpClient, err = getSecureClient(ctx, host, c.authFlags)
		if err != nil {
			return errors.Annotate(err, "update cost indicator command").Err()
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
	resp, err := fleetCostClient.UpdateCostIndicator(ctx, &fleetcostAPI.UpdateCostIndicatorRequest{
		CostIndicator: &fleetcostpb.CostIndicator{
			Name:                c.name,
			Type:                c.typ,
			Primary:             c.primary,
			Secondary:           c.secondary,
			Tertiary:            c.tertiary,
			Cost:                c.cost,
			CostCadence:         c.costCadence,
			BurnoutRate:         c.burnoutRate,
			Location:            c.location,
			Description:         c.description,
			AmortizationInYears: c.amortizationInYears,
		},
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: c.getFieldMaskPaths(),
		},
	})
	if err != nil {
		return errors.Annotate(err, "update cost indicator command").Err()
	}
	_, err = showProto(a.GetOut(), resp)
	return errors.Annotate(err, "update cost indicator command").Err()
}
