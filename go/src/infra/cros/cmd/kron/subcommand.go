// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"log"
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/api/gerrit"
	"go.chromium.org/luci/hardcoded/chromeinfra"

	"infra/cros/cmd/kron/common"
	kronSubCommands "infra/cros/cmd/kron/subcommands"
)

func getApplication(authOpts auth.Options) *subcommands.DefaultApplication {
	return &subcommands.DefaultApplication{
		Name:  "kron",
		Title: "Kron golang implementation",
		Commands: []*subcommands.Command{
			subcommands.CmdHelp,
			kronSubCommands.GetConfigParserCommand(authOpts),
			kronSubCommands.GetRunCommand(authOpts),
			kronSubCommands.GetFirestoreCommand(authOpts),
			authcli.SubcommandInfo(authOpts, "auth-info", false),
			authcli.SubcommandLogin(authOpts, "auth-login", false),
			authcli.SubcommandLogout(authOpts, "auth-logout", false),
		},
	}
}

type kronApplication struct {
	*subcommands.DefaultApplication
	stdoutLog *log.Logger
	stderrLog *log.Logger
}

func main() {
	opts := chromeinfra.DefaultAuthOptions()
	// Introduce gerrit scopes for accessing the config files
	gerritScopes := []string{
		gerrit.OAuthScope,
		auth.OAuthScopeEmail,
		auth.OAuthScopeIAM,
		// This scope is needed to access an internal repo. It does not mean
		// that the user is authenticated but it is the scope that is needed.
		"https://www.googleapis.com/auth/gerritcodereview",
	}
	opts.Scopes = append(opts.Scopes, gerritScopes...)

	authenticator := auth.NewAuthenticator(context.Background(), auth.SilentLogin, opts)
	_, err := authenticator.Client()
	if err != nil {
		common.Stderr.Println("please run kron auth-login")
		os.Exit(1)
	}

	s := &kronApplication{
		getApplication(opts),
		common.Stdout,
		common.Stderr}
	os.Exit(subcommands.Run(s, nil))
}
