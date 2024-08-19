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

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"
	prpc "go.chromium.org/luci/grpc/prpc"

	fleetcostpb "infra/cros/fleetcost/api/models"
	fleetcostAPI "infra/cros/fleetcost/api/rpc"
	"infra/cros/fleetcost/internal/site"
	"infra/cros/fleetcost/internal/utils"
	"infra/cros/fleetcost/internal/validation"
)

var CreateCostIndicatorCommand *subcommands.Command = &subcommands.Command{
	UsageLine: "create-ci [options...]",
	ShortDesc: "create a cost indicator",
	LongDesc:  "Create a cost indicator",
	CommandRun: func() subcommands.CommandRun {
		c := &createCostIndicatorCommand{}
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

type createCostIndicatorCommand struct {
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

// Run is the main entrypoint to the create-ci.
func (c *createCostIndicatorCommand) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	if err := c.innerRun(ctx, a, args, env); err != nil {
		utils.PrintMultiError(a, err)
		return 1
	}
	return 0
}

func (c *createCostIndicatorCommand) innerRun(ctx context.Context, a subcommands.Application, args []string, env subcommands.Env) error {
	host, err := c.commonFlags.Host()
	if err != nil {
		return errors.Annotate(err, "create cost indicator command").Err()
	}
	var httpClient *http.Client
	if !c.commonFlags.HTTP() {
		var err error
		httpClient, err = getSecureClient(ctx, host, c.authFlags)
		if err != nil {
			return errors.Annotate(err, "create cost indicator command").Err()
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
	request := &fleetcostAPI.CreateCostIndicatorRequest{
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
	}
	if err := validation.ValidateCreateCostIndicatorRequest(request); err != nil {
		return errors.Annotate(err, "create cost indicator command").Err()
	}
	resp, err := fleetCostClient.CreateCostIndicator(ctx, request)
	if err != nil {
		return errors.Annotate(err, "create cost indicator command").Err()
	}
	_, err = showProto(a.GetOut(), resp)
	return errors.Annotate(err, "create cost indicator command").Err()
}
