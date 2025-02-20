// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cli

import (
	"context"
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	kartepb "go.chromium.org/infra/cros/karte/api"
	"go.chromium.org/infra/cros/karte/client"
	"go.chromium.org/infra/cros/karte/internal/site"
)

// CreateObservation is a CLI command that creates an observation on the Karte server.
var CreateObservation = &subcommands.Command{
	UsageLine: `create-observation`,
	ShortDesc: "create observation",
	LongDesc:  "Create an observation on the karte server.",
	CommandRun: func() subcommands.CommandRun {
		r := &createObservationRun{}
		r.authFlags.Register(&r.Flags, site.DefaultAuthOptions)
		r.Flags.StringVar(&r.actionName, "action-name", "", "the action name")
		r.Flags.StringVar(&r.metricKind, "metric-kind", "", "the metric kind")
		r.Flags.StringVar(&r.value, "value", "", "value")
		return r
	},
}

// createObservationRun runs create-action.
type createObservationRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags

	actionName string
	metricKind string
	value      string
}

// Run creates an observation and returns an exit status.
func (c *createObservationRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	if err := c.innerRun(ctx, a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

// innerRun creates an observation and returns an error.
func (c *createObservationRun) innerRun(ctx context.Context, a subcommands.Application, args []string, env subcommands.Env) error {
	if len(args) != 0 {
		return errors.Reason("positional arguments are not accepted").Err()
	}
	authOptions, err := c.authFlags.Options()
	if err != nil {
		return errors.Annotate(err, "create action").Err()
	}
	kClient, err := client.NewClient(ctx, client.DevConfig(authOptions))
	if err != nil {
		return errors.Annotate(err, "create action").Err()
	}
	observation := &kartepb.Observation{
		ActionName: c.actionName,
		MetricKind: c.metricKind,
		Value: &kartepb.Observation_ValueString{
			ValueString: c.value,
		},
	}
	out, err := kClient.CreateObservation(ctx, &kartepb.CreateObservationRequest{Observation: observation})
	if err != nil {
		return errors.Annotate(err, "create action").Err()
	}
	fmt.Fprintf(a.GetOut(), "%s\n", out.String())
	return nil
}
