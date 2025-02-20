// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"os"

	"cloud.google.com/go/bigquery"
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/api/gerrit"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/gcloud/gs"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/hardcoded/chromeinfra"

	testplancli "go.chromium.org/infra/cros/internal/testplan/cli"
)

var logCfg = gologger.LoggerConfig{
	Out: os.Stderr,
}

func app(authOpts auth.Options) *cli.Application {
	return &cli.Application{
		Name:    "test_plan",
		Title:   "A tool to work with SourceTestPlan protos in DIR_METADATA files.",
		Context: logCfg.Use,
		Commands: []*subcommands.Command{
			subcommands.CmdHelp,

			subcommands.Section("Test Planning"),
			testplancli.CmdGenerate(authOpts),
			testplancli.CmdGetTestable(authOpts),
			testplancli.CmdRelevantPlans(authOpts),
			testplancli.CmdValidate(authOpts),
			testplancli.CmdMigrationStatus(authOpts),

			subcommands.Section("Authentication"),
			authcli.SubcommandInfo(authOpts, "auth-info", false),
			authcli.SubcommandLogin(authOpts, "auth-login", false),
			authcli.SubcommandLogout(authOpts, "auth-logout", false),

			subcommands.Section("BigQuery Updates (Advanced, Internal use only)"),
			testplancli.CmdChromeosDirmdUpdateRun(authOpts),
			testplancli.CmdChromeosCoverageRulesUpdateRun(authOpts),
		},
	}
}

func main() {
	opts := chromeinfra.DefaultAuthOptions()
	opts.PopulateDefaults()
	opts.Scopes = append(opts.Scopes, gerrit.OAuthScope, bigquery.Scope)
	opts.Scopes = append(opts.Scopes, gs.ReadOnlyScopes...)
	os.Exit(subcommands.Run(app(opts), nil))
}
