// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmds

import (
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"

	"go.chromium.org/infra/cmd/shivas/site"
)

// SyncDUTInfo subcommand: One round data sync between bot and UFS.
var SyncDUTInfo = &subcommands.Command{
	UsageLine: "internal-sync-dut-info",
	ShortDesc: "sync DUT info between the client and UFS",
	LongDesc: `Sync DUT info between the client and UFS.

	For internal use only.
	The client can upload/download specific data to/from UFS. Either direction is
  optional. If both direction are specified, we do upload first, then download.`,
	CommandRun: func() subcommands.CommandRun {
		c := &syncDUTInfoRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.commonFlags.Register(&c.Flags)

		c.Flags.BoolVar(&c.byHostname, "by-hostname", false, "Lookup by hostname instead of ID/Asset tag.")

		c.Flags.StringVar(&c.uploadHealthStatus, "upload-health-status", "", "Upload the health status of the DUT to UFS.")
		c.Flags.BoolVar(&c.downloadDUTInfo, "download-dut-info", false, "Download DUT info from UFS.")

		return c
	},
}

type syncDUTInfoRun struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	byHostname bool

	uploadHealthStatus string
	downloadDUTInfo    bool
}

func (c *syncDUTInfoRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

func (c *syncDUTInfoRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	return nil
}
