// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package btpeer

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs/wifirouter/ssh"
	"go.chromium.org/infra/cros/recovery/tlw"
)

type sshRunner struct {
	sshAccess ssh.Access
	resource  string
}

// NewSshRunner creates a normal ssh.Runner for a resource.
// TODO(jaredbennett) Move to common package when ssh package is refactored.
func NewSshRunner(sshAccess ssh.Access, resource string) ssh.Runner {
	r := &sshRunner{
		sshAccess: sshAccess,
		resource:  resource,
	}
	return r
}

// Run executes the given command with its arguments on a host over ssh. The
// stdout of the command execution is returned. If the command returns a
// non-zero exit code, a non-nil error is returned.
//
// Can be used as a components.Runner.
func (r *sshRunner) Run(ctx context.Context, timeout time.Duration, cmd string, args ...string) (string, error) {
	runResult := r.RunForResult(ctx, timeout, false, cmd, args...)
	if runResult.GetExitCode() != 0 {
		return "", errors.Reason("run of command %q result returned non-zero exit code: %s", runResult.GetCommand(), runResult.GetStderr()).Err()
	}
	return runResult.GetStdout(), nil
}

// RunInBackground starts to execute the given command with its arguments on a
// host over ssh, but does so in a background process and does not wait for it
// to finish before returning. A non-nil error is returned when it fails to
// start the command execution.
func (r *sshRunner) RunInBackground(ctx context.Context, timeout time.Duration, cmd string, args ...string) error {
	runResult := r.RunForResult(ctx, timeout, true, cmd, args...)
	if runResult.GetExitCode() != 0 {
		return errors.Reason("background run of command %q result returned non-zero exit code: %s", runResult.GetCommand(), runResult.GetStderr()).Err()
	}
	return nil
}

// RunForResult executes the given command with its arguments on a host over
// ssh. A RunResult is returned, which can be used to determine the success
// of the command execution. Timeout is only set if it is greater than zero.
func (r *sshRunner) RunForResult(ctx context.Context, timeout time.Duration, inBackground bool, cmd string, args ...string) ssh.RunResult {
	runRequest := &tlw.RunRequest{
		Resource:     r.resource,
		InBackground: inBackground,
		Command:      cmd,
		Args:         args,
	}
	if timeout > 0 {
		runRequest.Timeout = durationpb.New(timeout)
	}
	return r.sshAccess.Run(ctx, runRequest)
}
