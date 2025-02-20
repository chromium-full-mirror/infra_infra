// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"

	"go.chromium.org/infra/cros/satlab/common/site"
)

// ShivasRepairDUT holds the repair DUT flags inherited from shivas.
type shivasRepairDUT struct {
	subcommands.CommandRunBase

	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags
}

// registerRepairShivasFlags registers the shivas flags.
func registerRepairShivasFlags(c *repairDUTCmd) {
	c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
	c.envFlags.Register(&c.Flags)
	c.commonFlags.Register(&c.Flags)

	c.Flags.BoolVar(&c.Deep, "deep", false, "Use deep-repair task when scheduling a task.")
}
