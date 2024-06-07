// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package internal

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"
)

const (
	Oauth2Username  = "oauth2accesstoken"
	Oauth2Password  = "$(gcloud auth print-access-token)"
	Registry        = "us-docker.pkg.dev"
	ContainerFormat = "us-docker.pkg.dev/cros-registry/test-services/%s:%s"
)

// GcloudAuth signs into gcloud and attempts to activate the service account
// given by the dockerKeyFile.
func GcloudAuth(ctx context.Context, dockerKeyFile string) (err error) {
	step, ctx := build.StartStep(ctx, "Gcloud Auth")
	defer func() { step.End(err) }()

	_, _, err = activateServiceAccount(ctx, dockerKeyFile)
	if err != nil {
		err = errors.Annotate(err, "failed to activate service account").Err()
		return
	}

	password, _, err := getPassword(ctx)
	if err != nil {
		err = errors.Annotate(err, "failed to get password").Err()
		return
	}

	_, _, err = login(ctx, password)
	if err != nil {
		err = errors.Annotate(err, "failed to login").Err()
		return
	}

	return
}

// execCommand generically runs commands with options for stdin and directory.
func execCommand(ctx context.Context, displayName, command string, args []string, stdin, dir string) (stdout string, stderr string, err error) {
	step, ctx := build.StartStep(ctx, fmt.Sprintf("Run cmd: %s", displayName))
	defer func() { step.End(err) }()

	var se, so bytes.Buffer
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stderr = &se
	cmd.Stdout = &so
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	if dir != "" {
		cmd.Dir = dir
	}

	defer func() {
		stdout = so.String()
		stderr = se.String()

		logging.Infof(ctx, "STDOUT: %s", stdout)
		logging.Infof(ctx, "STDERR: %s", stderr)
	}()

	err = cmd.Run()

	return
}

// activateServiceAccount runs the `gcloud auth activate-service-account` command.
func activateServiceAccount(ctx context.Context, dockerKeyFile string) (stdout string, stderr string, err error) {
	if dockerKeyFile == "" {
		return
	}

	args := []string{"auth", "activate-service-account", "--key-file", dockerKeyFile}
	return execCommand(ctx, "Activate Service Account", "gcloud", args, "", "")
}

// getPassword runs the `glcoud auth print-access-token` command.
func getPassword(ctx context.Context) (stdout string, stderr string, err error) {
	args := []string{"auth", "print-access-token"}
	return execCommand(ctx, "Get Password", "gcloud", args, "", "")
}

// login runs the `docker login` command.
func login(ctx context.Context, password string) (stdout string, stderr string, err error) {
	args := []string{"login", "-u", Oauth2Username, "--password-stdin", Registry}
	return execCommand(ctx, "Login", "docker", args, password, "")
}

// buildImage runs the `docker build` command.
func buildImage(ctx context.Context, dir, dockerfile, name, tag string) (stdout string, stderr string, err error) {
	args := []string{"build", "-f", dockerfile, "-t", fmt.Sprintf(ContainerFormat, name, tag), "."}
	return execCommand(ctx, "Build Image", "docker", args, "", dir)
}

// pushImage runs the `docker push` command.
func pushImage(ctx context.Context, name, tag string) (stdout string, stderr string, err error) {
	args := []string{"push", fmt.Sprintf(ContainerFormat, name, tag)}
	return execCommand(ctx, "Push Image", "docker", args, "", "")
}

// buildAndPush builds and pushes the docker image to the artifact
// directory and returns the sha produced.
func buildAndPush(ctx context.Context, dir, dockerfile, name, tag string) (sha string, err error) {
	step, ctx := build.StartStep(ctx, "Build and Push")
	defer func() { step.End(err) }()

	_, _, err = buildImage(ctx, dir, dockerfile, name, tag)
	if err != nil {
		err = errors.Annotate(err, "failed to build image").Err()
		return
	}

	stdout, _, err := pushImage(ctx, name, tag)
	if err != nil {
		err = errors.Annotate(err, "failed to push image").Err()
		return
	}

	sha, err = extractDigestFromPushOutput(stdout)
	if err != nil {
		err = errors.Annotate(err, "name").Err()
		return
	}

	return
}

// extractDigestFromPushOutput grabs the sha256 from the output
// of the `docker push` command.
func extractDigestFromPushOutput(stdout string) (digest string, err error) {
	digestRegex, _ := regexp.Compile("sha256:[a-fA-F0-9]{64}")

	digest = digestRegex.FindString(stdout)

	if digest == "" {
		err = fmt.Errorf("failed to find digest from output")
	}

	return
}
