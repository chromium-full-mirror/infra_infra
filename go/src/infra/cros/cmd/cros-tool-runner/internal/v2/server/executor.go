// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package server

import (
	"context"
	"strings"

	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/v2/commands"
)

// CommandExecutor proxies command execution to provide an abstraction layer for interception.
type CommandExecutor interface {
	// Execute returns the same output as the original command.
	Execute(context.Context, commands.Command) (string, string, error)
}

// DefaultCommandExecutor enforces rules on stdout so that the commands can only
// be used in a granular way: retrieve one piece of information at a time. The
// motivation is to make it easier to support both docker and podman commands
// which have subtle differences in stdout and data model.
type DefaultCommandExecutor struct{}

// Execute of DefaultCommandExecutor executes the command as is and processes
// the stdout to extract only the first line (without the newline character).
func (*DefaultCommandExecutor) Execute(ctx context.Context, cmd commands.Command) (string, string, error) {
	stdout, stderr, err := cmd.Execute(ctx)
	return strings.Trim(stdout, "\n"), stderr, err
}

// compatibleLookupNetworkIdCommand returns a command that supports both docker
// and podman. (podman network create/inspect does not return id.)
func compatibleLookupNetworkIdCommand(name string) commands.Command {
	return &commands.NetworkList{
		Names:  []string{name},
		Format: "{{.ID}}",
	}
}
