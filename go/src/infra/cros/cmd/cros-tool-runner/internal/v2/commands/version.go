// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
)

// DockerVersion represents `docker -v`.
type DockerVersion struct{}

func (c *DockerVersion) Execute(ctx context.Context) (string, string, error) {
	args := []string{"-v"}
	return execute(ctx, dockerCmd, args)
}
