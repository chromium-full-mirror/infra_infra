// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmds

import (
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"

	"go.chromium.org/infra/cmd/shivas/site"
)

// PrintBotInfo subcommand: Print Swarming dimensions for a DUT.
var PrintBotInfo = &subcommands.Command{
	UsageLine: "internal-print-bot-info DUT hostname/Asset tag",
	ShortDesc: "print Swarming bot info for a DUT",
	LongDesc: `Print Swarming bot info for a DUT.

For internal use only.`,
	CommandRun: func() subcommands.CommandRun {
		c := &printBotInfoRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.commonFlags.Register(&c.Flags)

		c.Flags.BoolVar(&c.byHostname, "by-hostname", false, "Lookup by hostname instead of ID/Asset tag.")
		return c
	},
}

type printBotInfoRun struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	byHostname bool
}

func (c *printBotInfoRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	alias := &syncDUTInfoRun{
		authFlags:       c.authFlags,
		envFlags:        c.envFlags,
		commonFlags:     c.commonFlags,
		byHostname:      c.byHostname,
		downloadDUTInfo: true,
	}
	return alias.Run(a, args, env)
}
