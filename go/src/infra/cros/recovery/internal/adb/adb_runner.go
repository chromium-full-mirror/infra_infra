// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package adb

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/log"
)

type runnerResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int32
}

func (r *runnerResult) GetStderr() []byte {
	return r.Stderr
}

func (r *runnerResult) GetStdout() []byte {
	return r.Stdout
}

func (r *runnerResult) GetExitCode() int32 {
	return r.ExitCode
}

// Run runs blocking command by ADB.
// Do not use for non-blocking commands (like: logcat).
func Run(ctx context.Context, args ...string) (*runnerResult, error) {
	if len(args) == 0 {
		return nil, errors.Reason("run adb: command is not provided").Err()
	}
	adb := ADBPath(ctx)
	fullCommand := adb + " " + strings.Join(args, " ")
	log.Debugf(ctx, "ADB running try to run: %q", fullCommand)
	cmd := exec.CommandContext(ctx, adb, args...)
	var outB bytes.Buffer
	var errB bytes.Buffer
	cmd.Stdout = &outB
	cmd.Stderr = &errB
	// We always convert error to exit code.
	// Return an error only if issue in code.
	err := cmd.Run()
	res := &runnerResult{
		Stdout:   outB.Bytes(),
		Stderr:   errB.Bytes(),
		ExitCode: 0,
	}
	if err != nil {
		res.ExitCode = 1
		res.Stderr = []byte(err.Error())
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.ExitCode = int32(exitErr.ExitCode())
		}
	}
	log.Debugf(ctx, "ADB stdout: %s", string(res.Stdout))
	log.Debugf(ctx, "ADB stderr: %s", string(res.Stderr))
	log.Debugf(ctx, "ADB exit-code: %s", res.ExitCode)
	return res, nil
}
