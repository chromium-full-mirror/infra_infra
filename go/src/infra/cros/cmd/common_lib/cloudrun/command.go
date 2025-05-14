// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cloudrun

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"
)

func DeployToCloudRun(ctx context.Context, target *Target, container *Container) error {
	args := []string{"deploy", target.Name, "--no-allow-unauthenticated"}

	args = append(args, []string{
		"--project", target.Project,
		"--region", target.Region,
	}...)

	args = append(args, container.ToArgs()...)

	err := CloudRunCommand(ctx, args...)
	if err != nil {
		return err
	}

	// Ensure traffic is pointed to latest.
	if container.Tag == "" {
		err := EnforceTrafficToLatest(ctx, target)
		if err != nil {
			return err
		}
	}

	return nil
}

func EnforceTrafficToLatest(ctx context.Context, target *Target) error {
	args := []string{
		"services", "update-traffic", target.Name,
		"--project", target.Project,
		"--region", target.Region,
		"--to-latest",
	}
	err := CloudRunCommand(ctx, args...)
	if err != nil {
		return err
	}

	return nil
}

func CloudRunCommand(ctx context.Context, args ...string) (err error) {
	step, ctx := build.StartStep(ctx, "gcloud run")
	defer func() { step.End(err) }()

	var se, so bytes.Buffer
	cmd := exec.CommandContext(ctx, "gcloud", append([]string{"run"}, args...)...)
	cmd.Stderr = &se
	cmd.Stdout = &so

	defer func() {
		stdout := so.String()
		stderr := se.String()

		logging.Infof(ctx, "STDOUT: %s", stdout)
		logging.Infof(ctx, "STDERR: %s", stderr)
	}()

	logging.Infof(ctx, "Running command: gcloud run %s", strings.Join(cmd.Args, " "))
	err = cmd.Run()

	return
}
