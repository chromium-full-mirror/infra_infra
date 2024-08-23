// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
	"sort"

	"go.chromium.org/luci/auth"

	"infra/cros/satlab/common/site"
)

// CommandWithFlags is a representation of a command with subcommands that takes a combination
// of flags, some of which have arguments and some of which don't. It also takes positional
// parameters, which are inserted after the subcommands and flags when serialized.
type CommandWithFlags struct {
	Commands       []string
	Flags          map[string][]string
	PositionalArgs []string
	AuthRequired   bool
}

// ToCommand produces a list of arguments given a representation of a command with flags.
func (c *CommandWithFlags) ToCommand() []string {
	if c == nil {
		return nil
	}
	var out []string
	for _, s := range c.Commands {
		out = append(out, s)
	}
	if c.AuthRequired {
		a := auth.NewAuthenticator(context.Background(), auth.SilentLogin, site.DefaultAuthOptions)
		if err := a.CheckLoginRequired(); err != nil {
			c.Flags["service-account-json"] = []string{site.GetServiceAccountPath()}
		}
	}
	// ToCommand must be deterministic, sort the keys before iterating.
	var keys []string
	for k := range c.Flags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		items := c.Flags[k]
		if len(items) == 0 {
			out = append(out, fmt.Sprintf("-%s", k))
			continue
		}
		for _, item := range items {
			out = append(out, fmt.Sprintf("-%s", k))
			out = append(out, item)
		}
	}
	for _, s := range c.PositionalArgs {
		out = append(out, s)
	}
	return out
}
