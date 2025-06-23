// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cli

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/jsonpb"
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	kartepb "go.chromium.org/infra/cros/karte/api"
	"go.chromium.org/infra/cros/karte/client"
	"go.chromium.org/infra/cros/karte/internal/commonflags"
	"go.chromium.org/infra/cros/karte/internal/site"
)

// CronPersistToBigquery calls the persist-to-bigquery RPC on the karte cron service.
// This command is very similar to persist-action-range.
var CronPersistToBigquery = &subcommands.Command{
	UsageLine: `cron-persist-to-bigquery`,
	ShortDesc: `Call the cron-persist-to-bigquery cron job`,
	LongDesc:  `Call the cron-persist-to-bigquery cron job`,
	CommandRun: func() subcommands.CommandRun {
		r := &cronPersistToBigqueryRun{}
		r.commonFlags.Register(&r.Flags)
		r.authFlags.Register(&r.Flags, site.DefaultAuthOptions)
		return r
	},
}

// cronPersistToBigqueryRun runs the command.
type cronPersistToBigqueryRun struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	commonFlags commonflags.Flags
}

// Run logs an error message if there is one and returns an exit status.
func (c *cronPersistToBigqueryRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	if err := c.innerRun(ctx, a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

// innerRun validates arguments and performs the RPC call.
func (c *cronPersistToBigqueryRun) innerRun(ctx context.Context, a subcommands.Application, args []string, env subcommands.Env) error {
	if len(args) != 0 {
		return errors.Reason("persist action range does not take positional args").Err()
	}
	authOptions, err := c.authFlags.Options()
	if err != nil {
		return errors.Annotate(err, "persist action range").Err()
	}
	kClient, err := client.NewCronClient(ctx, c.commonFlags.MustSelectKarteConfig(authOptions))
	if err != nil {
		return errors.Annotate(err, "persist action range").Err()
	}
	marshalIndent := jsonpb.Marshaler{
		Indent: "  ",
	}
	req := &kartepb.PersistToBigqueryRequest{}
	marshalIndent.Marshal(a.GetErr(), req)
	res, err := kClient.PersistToBigquery(ctx, req)
	if err != nil {
		return errors.Annotate(err, "persist action range").Err()
	}
	return errors.WrapIf(marshalIndent.Marshal(a.GetErr(), res), "marshal JSON")
}
