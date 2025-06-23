// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	kartepb "go.chromium.org/infra/cros/karte/api"
	"go.chromium.org/infra/cros/karte/client"
	"go.chromium.org/infra/cros/karte/internal/scalars"
	"go.chromium.org/infra/cros/karte/internal/site"
)

// PersistActionRange is a command that persists a range of actions to BigQuery.
// Since this command is destructive, it only affects the dev instance of Karte.
var PersistActionRange = &subcommands.Command{
	UsageLine: `persist-action-range -hour`,
	ShortDesc: `Persist a single action to BigQuery in an ad hoc way`,
	LongDesc:  ``,
	CommandRun: func() subcommands.CommandRun {
		r := &persistActionRangeRun{}
		r.authFlags.Register(&r.Flags, site.DefaultAuthOptions)
		r.Flags.IntVar(&r.hours, "hours", 1, "hours before present that the query window begins")
		return r
	},
}

// persistActionRangeRun runs the command.
type persistActionRangeRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags

	hours int
}

// Run logs an error message if there is one and returns an exit status.
func (c *persistActionRangeRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	if err := c.innerRun(ctx, a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

// innerRun validates arguments and performs the RPC call.
func (c *persistActionRangeRun) innerRun(ctx context.Context, a subcommands.Application, args []string, env subcommands.Env) error {
	if len(args) != 0 {
		return errors.Reason("persist action range does not take positional args").Err()
	}
	authOptions, err := c.authFlags.Options()
	if err != nil {
		return errors.Annotate(err, "persist action range").Err()
	}
	kClient, err := client.NewClient(ctx, client.DevConfig(authOptions))
	if err != nil {
		return errors.Annotate(err, "persist action range").Err()
	}
	marshalIndent := jsonpb.Marshaler{
		Indent: "  ",
	}
	now := time.Now().UTC()
	req := &kartepb.PersistActionRangeRequest{
		// Karte identifier version: see internal/idstrategy/interface.go for details.
		StartVersion: "zzzz",
		StartTime:    scalars.ConvertTimeToTimestampPtr(now.Add(-1 * time.Hour)),
		StopVersion:  "zzzz",
		StopTime:     scalars.ConvertTimeToTimestampPtr(now),
	}
	marshalIndent.Marshal(a.GetErr(), req)
	res, err := kClient.PersistActionRange(ctx, req)
	if err != nil {
		return errors.Annotate(err, "persist action range").Err()
	}
	return errors.WrapIf(marshalIndent.Marshal(a.GetErr(), res), "marshal JSON")
}
