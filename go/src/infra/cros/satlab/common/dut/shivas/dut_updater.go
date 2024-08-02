// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shivas

import (
	"context"
	"os/exec"

	"infra/cros/satlab/common/commands"
	"infra/cros/satlab/common/paths"
	e "infra/cros/satlab/common/utils/errors"
	"infra/cros/satlab/common/utils/executor"
)

// DUTUpdater updates a DUT with the given name.
type DUTUpdater struct {
	// Name is a hostname indicate which DUT we want to update
	Name string
	// Executor is a command execuotr
	Executor executor.IExecCommander
}

// Update invokes shivas with the required arguments to update information
// about a DUT.
func (u *DUTUpdater) Update(ctx context.Context, args map[string][]string) error {
	flags := make(map[string][]string)

	for k, v := range args {
		flags[k] = v
	}

	flags["name"] = []string{u.Name}

	command_args := (&commands.CommandWithFlags{
		Commands:     []string{paths.ShivasCLI, "update", "dut"},
		Flags:        flags,
		AuthRequired: true,
	}).ToCommand()
	// We ignore the output here because we don't need any information from
	// the output now.
	_, err := u.Executor.CombinedOutput(exec.CommandContext(ctx, command_args[0], command_args[1:]...))
	return e.HandleExitError(err)
}
