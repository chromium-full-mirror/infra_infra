// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Command result_flow implements the result flow recipe's steps.
package main

import (
	"context"
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/logging/gologger"

	"go.chromium.org/infra/cros/cmd/result_flow/internal/cmd"
	"go.chromium.org/infra/cros/cmd/result_flow/internal/site"
)

func getApplication() *cli.Application {
	return &cli.Application{
		Name:  "result_flow",
		Title: "Binary steps implementations for result_flow recipe.",
		Context: func(ctx context.Context) context.Context {
			return gologger.StdConfig.Use(ctx)
		},
		Commands: []*subcommands.Command{
			subcommands.CmdHelp,

			subcommands.Section("Authentication"),
			authcli.SubcommandLogin(site.DefaultAuthOptions, "login", false),
			authcli.SubcommandLogout(site.DefaultAuthOptions, "logout", false),
			authcli.SubcommandInfo(site.DefaultAuthOptions, "whoami", false),

			subcommands.Section("Steps"),
			cmd.Publish,
			cmd.PipeCTPData,
			cmd.PipeTestRunnerData,
		},
	}
}

func main() {
	os.Exit(subcommands.Run(getApplication(), nil))
}
