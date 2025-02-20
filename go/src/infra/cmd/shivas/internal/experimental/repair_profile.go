// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package experimental

import (
	"fmt"

	"cloud.google.com/go/bigquery"
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"

	"go.chromium.org/infra/cmd/shivas/site"
)

// AuditDutsCmd contains audit-duts command specification
var RepairProfileCmd = &subcommands.Command{
	UsageLine: "get-repair-profile",
	ShortDesc: "get repair profile for a particular host in past 3 months",
	LongDesc: `get repair profile for a particular host in past 3 months.
	./shivas get-repair-profile -hostname ...`,
	CommandRun: func() subcommands.CommandRun {
		c := &repairProfileRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.Flags.StringVar(&c.hostname, "hostname", "", "a hostname to query")
		return c
	},
}

type repairProfileRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	envFlags  site.EnvFlags

	hostname string
}

func (c *repairProfileRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

func (c *repairProfileRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) (err error) {
	ctx := cli.GetContext(a, c, env)

	if len(c.hostname) == 0 {
		return fmt.Errorf("Must specify a hostname to query")
	}

	client, err := bigquery.NewClient(ctx, "unified-fleet-system")
	if err != nil {
		return err
	}

	if _, err := queryNeedsManualRepairBot(ctx, client, c.hostname); err != nil {
		return err
	}
	return nil
}
