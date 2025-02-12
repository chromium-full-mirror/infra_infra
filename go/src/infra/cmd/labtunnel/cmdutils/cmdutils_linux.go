// Copyright 2025 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmdutils

import (
	"context"
	"os/exec"
	"syscall"
)

func CreateContextualCmd(ctx context.Context, command string, args ...string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,
		Pdeathsig: syscall.SIGKILL,
	}

	return cmd, nil
}

func ExecuteAndWaitContextualCmd(ctx context.Context, cmd *exec.Cmd) error {
	runChan := make(chan error)
	var runErr error
	go func() {
		runChan <- cmd.Run()
	}()
	select {
	case <-ctx.Done():
		runErr = ctx.Err()
		break
	case runErr = <-runChan:
		break
	}

	// Silently kill any subprocesses.
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}

	return runErr
}
