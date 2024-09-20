// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/maruel/subcommands"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/flag/fixflagpos"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/hardcoded/chromeinfra"
	"go.chromium.org/luci/luciexe/build"

	"infra/cr_builder_health/healthpb"
)

var iso8601Format = "2006-01-02"

type luciexeGenerateRun struct {
	generateRun
}

type generateRun struct {
	subcommands.CommandRunBase
	logCfg   gologger.LoggerConfig
	authOpts auth.Options

	// cmdline flags
	dateString string
	dryRun     bool
}

func main() {
	authOpts := chromeinfra.DefaultAuthOptions()
	authOpts.Scopes = []string{
		auth.OAuthScopeEmail,
		"https://www.googleapis.com/auth/bigquery",
		"https://www.googleapis.com/auth/cloud-platform",
	}

	cliApp := &cli.Application{
		Name:  "builder-health-indicators",
		Title: "Builder Health Indicators track Chromium builders' long term health",
		Commands: []*subcommands.Command{
			{
				UsageLine: `luciexe`,
				ShortDesc: "Run as a luciexe and do what generate_indicators does",
				LongDesc:  "Run as a luciexe and do what generate_indicators does",
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
				UsageLine: `generate_indicators`,
				ShortDesc: "Generate builder health indicators",
				LongDesc: `Takes metrics from cr-buildbucket BigQuery tables and processes them into health indicators stored back in cr-builder-health-indicators tables

				Also sends rpcs to Buildbucket to update live Builder Metadata, which is shown in Milo builder pages and consoles.

				Required ACLs: BigQuery read and write permissions in cr-builder-health-indicators.
				`,
				CommandRun: func() subcommands.CommandRun {
					r := &generateRun{
						authOpts: authOpts,
					}
					r.logCfg = gologger.LoggerConfig{Out: os.Stderr}
					r.Flags.StringVar(&r.dateString, "date", "", "The date to generate for in ISO 8601 (YYYY-MM-DD). The default date is yesterday.")
					r.Flags.BoolVar(&r.dryRun, "dry-run", false, "Calculate health and print but don't write to db and don't RPC.")

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

func Run(ctx context.Context, input *healthpb.InputParams) error {
	if err := generate(ctx, input); err != nil {
		return err
	}

	return nil
}

// TODO: note that luciexeGenerateRun.Run complete ignores these parameters.
// Likely a bug?
var inProps = build.RegisterInputProperty[*healthpb.InputParams]("")

// Called by bb invocation
func (r *luciexeGenerateRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	build.Main(func(ctx context.Context, userArgs []string, state *build.State) error {
		input, err := r.generateRun.ParseFlags(ctx)
		if err != nil {
			return err
		}
		return Run(ctx, input)
	})

	return 0
}

func (r *generateRun) ParseFlags(ctx context.Context) (*healthpb.InputParams, error) {
	input := &healthpb.InputParams{}
	input.DryRun = r.dryRun
	if r.dateString == "" {
		// The default date is yesterday
		yesterday := time.Now().Add(-24 * time.Hour)
		input.Date = timestamppb.New(yesterday)
		return input, nil
	}

	t, err := time.Parse(iso8601Format, r.dateString)
	if err != nil {
		return input, errors.New("Error parsing -date flag. Please specify date like YYYY-MM-DD")
	}
	input.Date = timestamppb.New(t)

	return input, nil
}

// Called by cmdline invocation
func (r *generateRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	// Setup
	ctx := r.logCfg.Use(cli.GetContext(a, r, env))
	input, err := r.ParseFlags(ctx)
	if err != nil {
		logging.Errorf(ctx, "Error parsing flags: %v", err)
		return 1
	}

	// Run
	err = Run(ctx, input)
	if err != nil {
		logging.Errorf(ctx, "Error in Run: %v", err)
		return 1
	}

	return 0
}
