// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// To generate the runner mock, use:
// mockgen -source=internal/execs/wifirouter/ssh/runner.go -destination internal/execs/wifirouter/ssh/mocks/runner.go -package mocks
// Then re-add the copyright to the top.

package ssh

import (
	"context"
	"time"

	"go.chromium.org/infra/cros/recovery/tlw"
)

// Access provides a single method for executing ssh commands on devices.
type Access interface {
	// Run executes command on device by SSH related to resource name.
	Run(ctx context.Context, request *tlw.RunRequest) *tlw.RunResult
}

// RunResult provides functions for retrieving the result of an ssh command call.
type RunResult interface {
	// GetCommand returns the full command executed on the resource over ssh.
	GetCommand() string
	// GetStdout returns the stdout of the command execution.
	GetStdout() string
	// GetStderr returns the stderr of the command execution.
	GetStderr() string
	// GetExitCode returns the exit code of the command execution.
	GetExitCode() int32
}

// Runner provides functions for executing ssh commands on a host.
type Runner interface {
	// Run executes the given command with its arguments on a host over ssh. The
	// stdout of the command execution is returned. If the command returns a
	// non-zero exit code, a non-nil error is returned.
	//
	// Can be used as a components.Runner.
	Run(ctx context.Context, timeout time.Duration, cmd string, args ...string) (string, error)

	// RunInBackground starts to execute the given command with its arguments on a
	// host over ssh, but does so in a background process and does not wait for it
	// to finish before returning. A non-nil error is returned when it fails to
	// start the command execution.
	RunInBackground(ctx context.Context, timeout time.Duration, cmd string, args ...string) error

	// RunForResult executes the given command with its arguments on a host over
	// ssh. A RunResult is returned, which can be used to determine the success
	// of the command execution.
	RunForResult(ctx context.Context, timeout time.Duration, inBackground bool, cmd string, args ...string) RunResult
}
