// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package commands hosts all CLI commands CTRv2 interacts with. Each command is
// a struct that represents a CLI command, holds caller-specified arguments, and
// implements an API to uniformly execute itself.
package commands

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/common"
)

const (
	// dockerCmd is the name of docker command. To mimic drone environment locally
	// the value can be changed to podman, which is the underlying command on drones
	// (docker is an alias to podman on drones).
	dockerCmd = "docker"

	// lockFile is the lock file to control one gcloud command executed at a time:
	// gcloud doesn't support parallel execution: b/132194628
	lockFile = "/var/lock/go-lock.lock"
)

// Command is the interface of the command pattern. Only support blocking
// execution for now.
type Command interface {
	Execute(context.Context) (string, string, error)
}

// argumentsComposer is the interface to be implemented by more complicated
// commands to separate composing command from execution.
type argumentsComposer interface {
	// compose returns an array of arguments, error is returned if not composable
	compose() ([]string, error)
}

// execute runs blocking command and returns stdout and stderr as strings.
func execute(ctx context.Context, name string, args []string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	// TODO(mingkong) update RunWithTimeout since timeout is part of ctx
	return common.RunWithTimeout(ctx, cmd, time.Minute, true)
}

// executeWithStdin accepts a censored string for logging to protect secrets, and a stdin input
func executeWithStdin(ctx context.Context, name string, args []string, stdin string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	return common.RunWithTimeout(ctx, cmd, time.Minute, true)
}

// sensitiveExecute accepts a censored string for logging to protect secrets.
func sensitiveExecute(ctx context.Context, name string, args []string, censored string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return common.RunWithTimeoutSpecialLog(ctx, cmd, time.Minute, true, censored)
}

// ContextualExecutor executes a command using the provided context.
type ContextualExecutor struct{}

func (*ContextualExecutor) Execute(ctx context.Context, cmd Command) (string, string, error) {
	return cmd.Execute(ctx)
}
