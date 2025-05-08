// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package settings

import (
	"fmt"

	"github.com/maruel/subcommands"
)

// GetSettings is the command to print user settings.
var GetSettings = &subcommands.Command{
	UsageLine: "get [key]",
	ShortDesc: "get user settings",
	LongDesc:  "Get user settings",
	CommandRun: func() subcommands.CommandRun {
		c := &getSettings{}
		return c
	},
}

// getSettings struct contains the arguments needed to run GetSettings.
type getSettings struct {
	subcommands.CommandRunBase
}

// Run is what is called when a user inputs the getSettings command.
func (c *getSettings) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

// innerRun contains business logic.
func (c *getSettings) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	settings, err := readSettingsFromFile()
	if err != nil {
		return fmt.Errorf("unmarshalling JSON: %w", err)
	}

	if len(args) == 0 {
		return settings.PrintValues()
	}
	return settings.PrintValue(args[0])
}
