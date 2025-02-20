// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmds

import (
	"context"
	"errors"
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"

	"go.chromium.org/infra/appengine/crosskylabadmin/site"
)

var PushBotsForAdminTasks = &subcommands.Command{
	UsageLine: "push-bots-for-admin-tasks",
	ShortDesc: "Call the push bots for admin tasks RPC",
	CommandRun: func() subcommands.CommandRun {
		r := &pushBotsForAdminTasksRun{}
		r.crOSAdminRPCRun.Register(&r.Flags)
		r.authFlags.Register(&r.Flags, site.DefaultAuthOptions)
		return r
	},
}

type pushBotsForAdminTasksRun struct {
	crOSAdminRPCRun
	authFlags authcli.Flags
}

func (c *pushBotsForAdminTasksRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	if err := c.innerRun(ctx, a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

func (c *pushBotsForAdminTasksRun) innerRun(ctx context.Context, a subcommands.Application, args []string, env subcommands.Env) error {
	return errors.New("not yet implemented")
}
