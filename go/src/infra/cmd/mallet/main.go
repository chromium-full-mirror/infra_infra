// Copyright 2020 The Chromium Authors
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

	"go.chromium.org/infra/cmd/mallet/internal/cmd/meta"
	"go.chromium.org/infra/cmd/mallet/internal/cmd/tasks"
	"go.chromium.org/infra/cmd/mallet/internal/site"
)

func getApplication() *cli.Application {
	return &cli.Application{
		Name:  "mallet",
		Title: `mallet command line tool`,
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
			subcommands.Section("Experiments"),
			tasks.Recovery,
			tasks.CustomProvision,
			tasks.DownloadToUsbDrive,
			tasks.EthernetHook,
			tasks.RecoveryHWID,
			tasks.RepairCBI,
			tasks.ParseStableVersion,
			tasks.BatteryCutOff,
			tasks.SerialConsole,
			tasks.TestStateChange,
			tasks.ProvisionBtpeers,
			tasks.Labqual,
			tasks.SetFwTarget,
		},
	}
}

func main() {
	os.Exit(subcommands.Run(getApplication(), nil))
}
