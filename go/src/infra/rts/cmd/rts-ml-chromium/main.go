// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/storage"
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/api/gerrit"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/flag/fixflagpos"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/hardcoded/chromeinfra"

	"go.chromium.org/infra/rts/internal/chromium"
)

var logCfg = gologger.LoggerConfig{
	Format: `%{message}`,
	Out:    os.Stderr,
}

func main() {
	authOpt := chromeinfra.DefaultAuthOptions()
	authOpt.Scopes = append(authOpt.Scopes, bigquery.Scope, gerrit.OAuthScope, storage.ScopeReadOnly)

	app := &cli.Application{
		Name:  "rts-ml-chromium",
		Title: "RTS for Chromium.",
		Context: func(ctx context.Context) context.Context {
			return logCfg.Use(ctx)
		},
		Commands: []*subcommands.Command{
			cmdFileGraph(&authOpt),
			cmdCreateModel(&authOpt),
			cmdSelect(),
			cmdGenTrainingData(&authOpt),

			{}, // a separator
			chromium.SubcommandCommandFetchRejections(&authOpt),
			chromium.SubcommandCommandFetchDurations(&authOpt),

			{}, // a separator
			authcli.SubcommandLogin(authOpt, "auth-login", false),
			authcli.SubcommandLogout(authOpt, "auth-logout", false),
			authcli.SubcommandInfo(authOpt, "auth-info", false),

			{}, // a separator
			subcommands.CmdHelp,
		},
	}

	os.Exit(subcommands.Run(app, fixflagpos.FixSubcommands(os.Args[1:])))
}

type baseCommandRun struct {
	subcommands.CommandRunBase
}

func (r *baseCommandRun) done(err error) int {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
