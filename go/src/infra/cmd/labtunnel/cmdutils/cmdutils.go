//go:build !linux

// Copyright 2025 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmdutils

import (
	"context"
	"fmt"
	"os/exec"
)

func CreateContextualCmd(ctx context.Context, command string, args ...string) (*exec.Cmd, error) {
	return nil, fmt.Errorf("not implemented")
}

func ExecuteAndWaitContextualCmd(ctx context.Context, cmd *exec.Cmd) error {
	return fmt.Errorf("not implemented")
}
