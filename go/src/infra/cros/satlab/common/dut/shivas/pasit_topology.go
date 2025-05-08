// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shivas

import (
	"context"
	"os/exec"

	"go.chromium.org/infra/cros/satlab/common/commands"
	"go.chromium.org/infra/cros/satlab/common/paths"
	"go.chromium.org/infra/cros/satlab/common/site"
	"go.chromium.org/infra/cros/satlab/common/utils/errors"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
)

type PASITTopology struct {
	Hostname     string
	TopologyPath string
	Executor     executor.IExecCommander
}

// Add executes "shivas add peripheral-pasit-host" command.
func (c *PASITTopology) Add(ctx context.Context) error {
	args := (&commands.CommandWithFlags{
		Commands: []string{paths.ShivasCLI, "add", "peripheral-pasit-host"},
		Flags: map[string][]string{
			"namespace": {site.GetNamespace("")},
			"dut":       {c.Hostname},
			"f":         {c.TopologyPath},
		},
		AuthRequired: true,
	}).ToCommand()
	command := exec.CommandContext(ctx, args[0], args[1:]...)
	_, err := c.Executor.Output(command)
	return errors.HandleExitError(err)
}

// Delete executes "shivas delete peripheral-pasit-host" command.
func (c *PASITTopology) Delete(ctx context.Context) error {
	args := (&commands.CommandWithFlags{
		Commands: []string{paths.ShivasCLI, "delete", "peripheral-pasit-host"},
		Flags: map[string][]string{
			"namespace": {site.GetNamespace("")},
			"dut":       {c.Hostname},
		},
		AuthRequired: true,
	}).ToCommand()
	command := exec.CommandContext(ctx, args[0], args[1:]...)
	_, err := c.Executor.Output(command)
	return errors.HandleExitError(err)
}
