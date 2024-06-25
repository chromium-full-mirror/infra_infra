// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/logging/gologger"

	"infra/cros/fleetcost/internal/commands"
	"infra/cros/fleetcost/internal/site"
)

// getApplication returns the fleetcost command line application.
func getApplication() *cli.Application {
	return &cli.Application{
		Name:  "fleet cost",
		Title: "fleet cost command line tool",
		Context: func(ctx context.Context) context.Context {
			return gologger.StdConfig.Use(ctx)
		},
		Commands: []*subcommands.Command{
			subcommands.CmdHelp,
			subcommands.Section("Cost Indicators"),
			commands.GetCostIndicatorCommand,
			commands.CreateCostIndicatorCommand,
			commands.UpdateCostIndicatorCommand,
			commands.DeleteCostIndicatorCommand,
			commands.RepopulateCacheCommand,
			commands.PersistToBigqueryCommand,
			subcommands.Section("Cost Results"),
			commands.GetCostResultCommand,
			subcommands.Section("Debugging"),
			commands.PingCommand,
			commands.PingUFSCommand,
			commands.BatchCreateCICommand,
			subcommands.Section("Authentication"),
			authcli.SubcommandInfo(site.DefaultAuthOptions, "whoami", false),
			authcli.SubcommandLogin(site.DefaultAuthOptions, "login", false),
			authcli.SubcommandLogout(site.DefaultAuthOptions, "logout", false),
		},
	}
}

// main is the entrypoint to the fleet command line application.
func main() {
	os.Exit(subcommands.Run(getApplication(), nil))
}
