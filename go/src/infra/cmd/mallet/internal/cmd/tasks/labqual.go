// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tasks

import (
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"

	"infra/cmd/mallet/internal/site"
	"infra/cmdsupport/cmdlib"
)

var Labqual = &subcommands.Command{
	UsageLine: "labqual [FLAGS...] -board BOARD -pool POOL HOSTNAME [HOSTNAME...]",
	ShortDesc: "Run a lab qualification on given host(s) using the board's stable build",
	CommandRun: func() subcommands.CommandRun {
		c := &LabqualRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.Flags.StringVar(&c.stableConfigPath, "stable-config", "", "Path to stable firmware config JSON")
		c.Flags.StringVar(&c.board, "board", "", "Board of device(s) (Required)")
		c.Flags.StringVar(&c.pool, "pool", "", "Pool to schedule qualification jobs in (Required)")
		return c
	},
}

type LabqualRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	envFlags  site.EnvFlags

	board            string
	pool             string
	stableConfigPath string
}

func (c *LabqualRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *LabqualRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {

	if c.board == "" {
		return cmdlib.NewUsageError(c.Flags, "Flag '-board' is required")
	}

	if c.pool == "" {
		return cmdlib.NewUsageError(c.Flags, "Flag '-pool' is required")
	}

	if len(args) == 0 {
		return cmdlib.NewUsageError(c.Flags, "Expected at least 1 hostname after flags")
	}

	for _, host := range args {
		fmt.Printf("Starting labqual on host %q\n", host)
	}

	return nil
}
