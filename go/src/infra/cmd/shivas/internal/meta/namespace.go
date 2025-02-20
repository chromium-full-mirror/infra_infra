// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"

	"go.chromium.org/infra/cmd/shivas/site"
)

// GetNamespace is a diagnostic utility that shows the current namespace based on the environment
// and other factors.
var GetNamespace = &subcommands.Command{
	UsageLine: "get-namespace",
	ShortDesc: "print the namespace",
	LongDesc:  `Print the namespace.`,
	CommandRun: func() subcommands.CommandRun {
		c := &getNamespaceRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		return c
	},
}

type getNamespaceRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	envFlags  site.EnvFlags
}

// Run runs the get-namespace command.
func (c *getNamespaceRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	if err := c.innerRun(ctx, a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

func (c *getNamespaceRun) innerRun(ctx context.Context, a subcommands.Application, args []string, env subcommands.Env) error {
	ns, err := c.envFlags.Namespace(nil, "")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(a.GetOut(), "%s\n", ns)
	return err
}
