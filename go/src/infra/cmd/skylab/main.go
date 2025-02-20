// Copyright 2018 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Command cros-admin is the Chrome OS infrastructure admin tool.
package main

import (
	"context"
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/logging/gologger"

	"go.chromium.org/infra/cmd/skylab/internal/cmd/internalcmds"
	"go.chromium.org/infra/cmd/skylab/internal/cmd/meta"
	"go.chromium.org/infra/cmd/skylab/internal/cmd/tasks"
	"go.chromium.org/infra/cmd/skylab/internal/site"
)

func getApplication() *cli.Application {
	return &cli.Application{
		Name: "skylab",
		Title: `[DEPRECATED; please use crosfleet (go/crosfleet-cli)]

Universal tool for Chrome OS Infra Skylab

Tool uses a default RPC retry strategy with five attempts and exponential backoff.
Full documentation http://go/skylab-cli`,
		Context: func(ctx context.Context) context.Context {
			return gologger.StdConfig.Use(ctx)
		},
		Commands: []*subcommands.Command{
			subcommands.CmdHelp,
			meta.Update,
			meta.Version,
			subcommands.Section("Authentication"),
			authcli.SubcommandLogin(site.DefaultAuthOptions, "login", false),
			authcli.SubcommandLogout(site.DefaultAuthOptions, "logout", false),
			authcli.SubcommandInfo(site.DefaultAuthOptions, "whoami", false),
			subcommands.Section("Tasks"),
			tasks.BackfillRequest,
			tasks.CreateTest,
			tasks.CreateSuite,
			tasks.CreateTestPlan,
			tasks.LeaseDut,
			tasks.ReleaseDuts,
			tasks.Repair,
			tasks.Verify,
			tasks.Audit,
			subcommands.Section("Internal use"),
			internalcmds.WaitTask,
			internalcmds.DutStableVersion,
		},
	}
}

func main() {
	os.Exit(subcommands.Run(getApplication(), nil))
}
