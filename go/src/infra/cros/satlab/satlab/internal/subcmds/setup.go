// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subcmds

import (
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"

	"go.chromium.org/infra/cmdsupport/cmdlib"
	commonSetup "go.chromium.org/infra/cros/satlab/common/setup"
)

var SetupCmd = &subcommands.Command{
	UsageLine: "setup [options ...]",
	ShortDesc: "",
	CommandRun: func() subcommands.CommandRun {
		c := &setupCmd{}
		c.Flags.StringVar(&c.Bucket, "bucket", "", "Bucket contain the key")
		c.Flags.StringVar(&c.GSAccessKeyId, "gcs_key_id", "", "GCS bucket key ID")
		c.Flags.StringVar(&c.GSSecretAccessKey, "gcs_key_secret", "", "GCS Bucket key Secret")
		return c
	},
}

// SetupRun is the placeholder satlab setup command.
type setupCmd struct {
	subcommands.CommandRunBase
	commonSetup.Setup
}

// Run runs the command, prints the error if there is one, and returns an exit status.
func (c *setupCmd) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *setupCmd) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	return c.StartSetup(ctx)
}
