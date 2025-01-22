// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package clilib contains the shivas application.
package clilib

import (
	"context"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"

	"infra/cmd/shivas/internal/experimental"
	"infra/cmd/shivas/internal/meta"
	queen_cmds "infra/cmd/shivas/internal/queen/cmds"
	sw_cmds "infra/cmd/shivas/internal/swarming/cmds"
	bot_cmds "infra/cmd/shivas/internal/ufs/cmds/bot"
	"infra/cmd/shivas/internal/ufs/cmds/operations"
	"infra/cmd/shivas/internal/ufs/cmds/state"
	"infra/cmd/shivas/site"
)

func Application() *cli.Application {
	return &cli.Application{
		Name: "shivas",
		Title: `Unified Fleet System Management

Tool uses a default RPC retry strategy with five attempts and exponential backoff.
Full documentation http://go/shivas-cli`,
		Context: func(ctx context.Context) context.Context {
			return ctx
		},
		Commands: []*subcommands.Command{
			subcommands.CmdHelp,
			subcommands.Section("Meta"),
			meta.Version,
			meta.Update,
			meta.GetNamespace,
			subcommands.Section("Authentication"),
			authcli.SubcommandInfo(site.DefaultAuthOptions, "whoami", false),
			authcli.SubcommandLogin(site.DefaultAuthOptions, "login", false),
			authcli.SubcommandLogout(site.DefaultAuthOptions, "logout", false),
			subcommands.Section("Resource Management"),
			operations.AddCmd,
			operations.UpdateCmd,
			operations.DeleteCmd,
			operations.GetCmd,
			operations.RenameCmd,
			operations.ReplaceCmd,
			subcommands.Section("Repair"),
			sw_cmds.RepairDutsCmd,
			sw_cmds.AuditDutsCmd,
			subcommands.Section("State"),
			sw_cmds.ReserveDutsCmd,
			state.DutStateCmd,
			subcommands.Section("Drone Queen Inspection"),
			queen_cmds.InspectDuts,
			queen_cmds.InspectDrones,
			queen_cmds.PushDuts,
			subcommands.Section("Internal use"),
			bot_cmds.PrintBotInfo,
			operations.AdminCmd,
			experimental.VerifyBotStatusCmd,
			experimental.DumpNlyteCmd,
			experimental.RepairProfileCmd,
			experimental.ModelAnalysisCmd,
			experimental.DUTAvailabilityDiffCmd,
			experimental.GetDutsForLabstationCmd,
			experimental.ImportOSNicsCmd,
			experimental.ChangelogCmd,
		},
	}
}
