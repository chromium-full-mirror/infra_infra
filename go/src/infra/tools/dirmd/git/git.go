// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package git encapsulates git command interactions for dirmd.
package git

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"
	"strings"

	"go.chromium.org/luci/common/errors"
)

// Binary is the platform aware git binary name.
var Binary string

func init() {
	if runtime.GOOS != "windows" {
		Binary = "git"
		return
	}

	Binary = "git.exe"
	if _, err := exec.LookPath("git.bat"); err == nil {
		// git.bat is available. Prefer git.bat instead.
		Binary = "git.bat"
	}
	// Note that this function does not raise errors (by panicking).
	// Instead, if code execution needs git indeed, then it will fail with a nice
	// error message (as opposed to a stack trace from panic).
}

// FindRepoRoot returns the root dir of the repo which contains the provided dir
func FindRepoRoot(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, Binary, "-C", dir, "rev-parse", "--show-toplevel")
	stdout, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", errors.Reason("failed to call %q: %s", cmd.Args, exitErr.Stderr).Err()
		}
		return "", errors.Annotate(err, "failed to call %q", cmd.Args).Err()
	}
	return string(bytes.TrimSpace(stdout)), nil
}

// IsInGitRepo returns true if the provided dir is in a git repository.
func IsInGitRepo(dir string) bool {
	cmd := exec.Command(Binary, "-C", dir, "rev-parse", "--is-inside-work-tree")
	switch stdout, err := cmd.Output(); {
	case err != nil:
		// error will be returned instead of "false" if dir is outside of a git
		// repository.
		return false
	case strings.ToLower(string(bytes.TrimSpace(stdout))) == "true":
		return true
	default:
		return false
	}
}
