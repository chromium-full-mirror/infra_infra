// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"fmt"
	"os/exec"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"

	"go.chromium.org/infra/cros/satlab/common/site"
)

// CipdRoot is the path to root directory for CIPD.
var CipdRoot = "/cipd"

// CipdEnsureFile is the path to the ensure file.
var CipdEnsureFile = "/cipd/spec"

// Update subcommand: Update satlab tool.
var Update = &subcommands.Command{
	UsageLine: "upgrade", // we have a separate update cmd under UFS, so changing this to upgrade.
	ShortDesc: "upgrade satlab CIPD packages",
	LongDesc: `Upgrade satlab CIPD packages.

This is a thin wrapper around CIPD. Wil`,
	CommandRun: func() subcommands.CommandRun {
		c := &updateRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)

		c.Flags.BoolVar(&c.silent, "silent", false, "Whether to run silently.")
		return c
	},
}

// UpdateRun is the update command for satlab.
type updateRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags

	silent bool
}

// Run updates satlab and returns an exit status.
func (c *updateRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		if !c.silent {
			fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		}
		return 1
	}
	return 0
}

// InnerRun is the implementation of the run command.
func (c *updateRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	if err := c.ensure(a, CipdRoot, CipdEnsureFile); err != nil {
		return err
	}
	if !c.silent {
		fmt.Fprintf(a.GetErr(), "%s: You may need to run satlab login again after the upgrade\n", a.GetName())
		fmt.Fprintf(a.GetErr(), "%s: Run satlab whoami to check login status\n", a.GetName())
	}
	return nil
}

// ensure wraps a cipd ensure call.
func (c *updateRun) ensure(a subcommands.Application, root string, ensureFile string) error {
	cmd := exec.Command("sudo", "cipd", "ensure", "-root", root, "-ensure-file", ensureFile)
	if !c.silent {
		cmd.Stdout = a.GetOut()
		cmd.Stderr = a.GetErr()
	}

	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
