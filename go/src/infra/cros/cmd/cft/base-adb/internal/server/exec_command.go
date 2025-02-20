// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package server

import (
	"context"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/cft/base-adb/internal/runner"
)

// ExecCommand executes blocking ADB commands.
func (s *adbService) ExecCommand(ctx context.Context, req *api.ADBCommandRequest) (*api.ADBCommandResponse, error) {
	s.logger.Println("Received request: ", req)
	var args []string
	if req.GetCommand() != "" {
		args = append(args, req.GetCommand())
	}
	if len(req.GetArgs()) != 0 {
		args = append(args, req.GetArgs()...)
	}
	if len(args) == 0 {
		return nil, errors.Reason("exec command: ADB command or args is not specified").Err()
	}
	a, err := runner.New(s.logger)
	if err != nil {
		return nil, errors.Annotate(err, "exec command").Err()
	}
	r, err := a.Run(ctx, args...)
	if err != nil {
		return nil, errors.Annotate(err, "exec command").Err()
	}
	return &api.ADBCommandResponse{
		Stdout:   r.Stdout,
		Stderr:   r.Stderr,
		ExitCode: r.ExitCode,
	}, nil
}
