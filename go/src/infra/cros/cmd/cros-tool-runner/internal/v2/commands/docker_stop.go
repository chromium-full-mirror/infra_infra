// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"time"
)

// DockerStop represents `docker stop`
type DockerStop struct {
	ContainerName string
}

func (c *DockerStop) Execute(ctx context.Context) (string, string, error) {
	args := []string{"stop", c.ContainerName}
	startTime := time.Now()
	stdout, stderr, err := execute(ctx, dockerCmd, args)
	if err == nil {
		monitorTime(c, startTime)
	}
	return stdout, stderr, err
}
