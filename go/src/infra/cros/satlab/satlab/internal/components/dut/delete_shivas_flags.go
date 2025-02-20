// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"

	"go.chromium.org/infra/cros/satlab/common/site"
)

// ShivasDeleteDUT holds the delete DUT flags inherited from shivas.
type shivasDeleteDUT struct {
	subcommands.CommandRunBase

	authFlags authcli.Flags
	envFlags  site.EnvFlags
}

// RegisterShivasFlags registers the shivas flags.
func registerShivasFlags(c *deleteDUTCmd) {
	c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
	c.envFlags.Register(&c.Flags)

	c.Flags.StringVar(&c.SatlabID, "satlab-id", "", "the ID for the satlab in question")
	c.Flags.BoolVar(&c.Full, "full", false, "whether to use a full/cascading delete for DUTs")
}
