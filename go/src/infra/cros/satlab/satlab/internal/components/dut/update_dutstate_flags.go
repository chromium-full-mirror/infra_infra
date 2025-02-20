// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"

	"go.chromium.org/infra/cros/satlab/common/site"
)

// updateDUTStateFlags is a command that contains the arguments that "shivas update" understands.
type updateDUTStateFlags struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	hostname string
	state    string
	force    bool
}

// registerUpdateDutStateFlags registers the flags inherited from shivas.
func registerUpdateDutStateFlags(c *updateDUTState) {
	c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
	c.envFlags.Register(&c.Flags)
	c.commonFlags.Register(&c.Flags)

	c.Flags.StringVar(&c.hostname, "hostname", "", "hostname of the DUT.")
	c.Flags.StringVar(&c.state, "state", "", "target state for the DUT.")
	c.Flags.BoolVar(&c.force, "force", false, "force dutstate update regardless of checks.")
}
