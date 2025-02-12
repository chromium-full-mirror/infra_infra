// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Satlab is a wrapper around shivas.

package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"

	"infra/cros/satlab/common/site"
	"infra/cros/satlab/common/utils/misc"
	"infra/cros/satlab/satlab/internal/components/run"
	"infra/cros/satlab/satlab/internal/meta"
	"infra/cros/satlab/satlab/internal/stableversion"
	"infra/cros/satlab/satlab/internal/subcmds"
)

const StandaloneSatlabEnvVar = "STANDALONE_SATLAB"

// GetApplication returns the main application.
func getApplication() *cli.Application {
	return &cli.Application{
		Name:  site.AppPrefix,
		Title: `Satlab DUT Management Tool`,
		Context: func(ctx context.Context) context.Context {
			return ctx
		},
		Commands: []*subcommands.Command{
			subcommands.CmdHelp,
			subcommands.Section("Meta"),
			meta.Version,
			meta.Update,
			subcommands.Section("Authentication"),
			authcli.SubcommandInfo(site.DefaultAuthOptions, "whoami", false),
			authcli.SubcommandLogin(site.DefaultAuthOptions, "login", false),
			authcli.SubcommandLogout(site.DefaultAuthOptions, "logout", false),
			subcommands.Section("Resource Management"),
			subcmds.AddCmd,
			subcmds.DeleteCmd,
			subcmds.GetCmd,
			subcmds.UpdateCmd,
			subcmds.RepairCmd,
			subcmds.SetupCmd,
			subcommands.Section("Run"),
			run.RunCmd,
			subcommands.Section("Stable Version"),
			stableversion.GetStableVersionCmd,
			stableversion.SetStableVersionCmd,
			stableversion.DeleteStableVersionCmd,
			subcommands.Section("Utils"),
			subcmds.IsSatlabRemoteAccessCmd,
			subcmds.ServodCmd,
			subcmds.PruneCmd,
			subcmds.CCDCmd,
			subcmds.SupportCmd,
		},
	}
}

// isSatlabConfigured checks if Satlab is configured.
// It can be omitted with the STANDALONE_SATLAB environment variable.
func isSatlabConfigured() bool {
	if misc.BoolVal(os.Getenv(StandaloneSatlabEnvVar)) {
		return true
	}
	if _, err := os.Stat(site.GetServiceAccountPath()); errors.Is(err, os.ErrNotExist) {
		fmt.Println("Please set up your satlab first")
		return false
	}
	return true
}

// Main is the entrypoint for "satlab".
func main() {
	if !isSatlabConfigured() {
		os.Exit(1)
	}
	os.Exit(meta.UpdateThenRun(getApplication()))
}
