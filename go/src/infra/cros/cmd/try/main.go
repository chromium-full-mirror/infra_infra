// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements the `cros try` CLI, which enables users to
// run ChromeOS builders with certain common configurations.
package main

import (
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/api/gerrit"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/hardcoded/chromeinfra"

	"go.chromium.org/infra/cros/cmd/try/try"
)

func newApplication(authOpts auth.Options) *cli.Application {
	return &cli.Application{
		Name:  "try",
		Title: "cros try CLI",
		Commands: []*subcommands.Command{
			subcommands.CmdHelp,
			try.GetCmdRelease(authOpts),
			try.GetCmdRetry(),
			try.GetCmdFactory(authOpts),
			try.GetCmdFirmware(authOpts),
			try.GetCmdChromiumOSSDK(authOpts),
			try.GetCmdCreatePreMPKeys(authOpts),
			try.GetCmdCreateAccessoryKeys(authOpts),
			authcli.SubcommandInfo(authOpts, "auth-info", false),
			authcli.SubcommandLogin(authOpts, "auth-login", false),
			authcli.SubcommandLogout(authOpts, "auth-logout", false),
		},
	}
}

// Main is the main entrypoint to the application.
func main() {
	opts := chromeinfra.DefaultAuthOptions()
	opts.Scopes = append(opts.Scopes, gerrit.OAuthScope)
	os.Exit(subcommands.Run(newApplication(opts), nil))
}
