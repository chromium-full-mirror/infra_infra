// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"strings"
)

type DockerInspect struct {
	Name   string
	Format string
}

func (c *DockerInspect) Execute(ctx context.Context) (string, string, error) {
	args := []string{"inspect"}
	if strings.TrimSpace(c.Format) != "" {
		args = append(args, "-f", c.Format)
	}
	args = append(args, c.Name)
	return execute(ctx, dockerCmd, args)
}
