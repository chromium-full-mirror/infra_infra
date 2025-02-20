// Copyright 2024 The Chromium Authors
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

	"go.chromium.org/infra/cros/cmd/paris/internal/site"
	"go.chromium.org/infra/cros/cmd/paris/internal/tasks"
)

func application() *cli.Application {
	return &cli.Application{
		Name:  "paris",
		Title: `paris command line tool`,
		Context: func(ctx context.Context) context.Context {
			return gologger.StdConfig.Use(ctx)
		},
		Commands: []*subcommands.Command{
			subcommands.CmdHelp,
			subcommands.Section("Authentication"),
			authcli.SubcommandLogin(site.DefaultAuthOptions, "login", false),
			authcli.SubcommandLogout(site.DefaultAuthOptions, "logout", false),
			authcli.SubcommandInfo(site.DefaultAuthOptions, "whoami", false),
			subcommands.Section("Experiments"),
			tasks.LocalRecovery,
			tasks.RecoveryConfig,
		},
	}
}

func main() {
	os.Exit(subcommands.Run(application(), nil))
}
