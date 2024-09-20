// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/flag/fixflagpos"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/hardcoded/chromeinfra"
	"go.chromium.org/luci/luciexe/build"
)

type luciexeGenerateRun struct {
	generateRun
}

type generateRun struct {
	subcommands.CommandRunBase
	logCfg   gologger.LoggerConfig
	authOpts auth.Options
}

func main() {
	authOpts := chromeinfra.DefaultAuthOptions()
	authOpts.Scopes = []string{
		auth.OAuthScopeEmail,
		"https://www.googleapis.com/auth/bigquery",
		"https://www.googleapis.com/auth/cloud-platform",
	}

	cliApp := &cli.Application{
		Name:  "builder-value",
		Title: "Builder Value",
		Commands: []*subcommands.Command{
			{
				UsageLine: `luciexe`,
				ShortDesc: "Run as a luciexe and do what generate does",
				LongDesc:  "Run as a luciexe and do what generate does",
				CommandRun: func() subcommands.CommandRun {
					r := &luciexeGenerateRun{
						generateRun{
							authOpts: authOpts,
						},
					}
					r.logCfg = gologger.LoggerConfig{Out: os.Stderr}
					return r
				},
			},
			{
				UsageLine: `generate`,
				ShortDesc: "Generate builder value data",
				LongDesc: `Read builder value data from different source and write it to BigQuery
				Required ACLs: BigQuery read and write permissions in cr-builder-value.
				`,
				CommandRun: func() subcommands.CommandRun {
					r := &generateRun{
						authOpts: authOpts,
					}
					r.logCfg = gologger.LoggerConfig{Out: os.Stderr}

					return r
				},
			},

			{}, // spacer

			authcli.SubcommandLogin(authOpts, "auth-login", false),
			authcli.SubcommandLogout(authOpts, "auth-logout", false),
			authcli.SubcommandInfo(authOpts, "auth-info", false),
		},
	}

	os.Exit(subcommands.Run(cliApp, fixflagpos.FixSubcommands(os.Args[1:])))
}

func Run(ctx context.Context) error {
	if err := generate(ctx); err != nil {
		return err
	}

	return nil
}

// Called by bb invocation
func (r *luciexeGenerateRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	build.Main(func(ctx context.Context, userArgs []string, state *build.State) error {
		return Run(ctx)
	})

	return 0
}

// Called by cmdline invocation
func (r *generateRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	// Setup
	ctx := r.logCfg.Use(cli.GetContext(a, r, env))

	// Run
	var err = Run(ctx)
	if err != nil {
		logging.Errorf(ctx, "Error in Run: %v", err)
		return 1
	}

	return 0
}
