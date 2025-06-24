// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package gclient is a package that enables performing gclient operations required by the chromium
// bootstrapper.
package gclient

import (
	"context"
	stderrors "errors"
	"os"
	"os/exec"
	"path"
	"strings"

	"go.chromium.org/luci/common/errors"
)

type Client struct {
	gclientPath string
}

// NewClient returns a new gclient client that uses the gclient binary at gclientPath.
func NewClient(gclientPath string) *Client {
	return &Client{gclientPath}
}

// NewClientForTesting returns a new gclient client that uses the gclient binary found on the
// machine's path.
func NewClientForTesting() (*Client, error) {
	gclientPath, err := exec.LookPath("gclient")
	if err != nil {
		return nil, errors.Annotate(err, "gclient not on $PATH, please install depot_tools").Err()
	}
	return &Client{gclientPath}, nil
}

func (c *Client) GetDep(ctx context.Context, depsContents, depPath string) (string, error) {
	d, err := os.MkdirTemp("", "")
	if err != nil {
		return "", err
	}

	f := path.Join(d, "DEPS")
	if err := os.WriteFile(f, []byte(depsContents), 0644); err != nil {
		return "", err
	}

	// --deps-file: The DEPS file to get dependency from
	// -r: get revision information about the dep at the given path
	cmd := exec.CommandContext(ctx, c.gclientPath, "getdep", "--deps-file", f, "-r", depPath)
	// Set DEPOT_TOOLS_UPDATE environment variable to 0 to prevent gclient from attempting to
	// update depot tools; just use the recipe bundle as-is (the recipe bundle also doesn't
	// contain the necessary update_depot_tools script)
	cmd.Env = append(os.Environ(), "DEPOT_TOOLS_UPDATE=0")
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if stderrors.As(err, &exitErr) {
			return "", errors.Annotate(err, "gclient failed with output:\n%s", exitErr.Stderr).Err()
		}
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}
