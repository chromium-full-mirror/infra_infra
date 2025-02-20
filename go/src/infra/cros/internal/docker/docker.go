// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package docker provides helper methods for ChromeOS usage of Docker.
package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/pkg/errors"

	"go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cros/internal/cmd"
)

func generateMountArgs(mounts []mount.Mount) ([]string, error) {
	var args []string

	for _, m := range mounts {
		mountStrParts := []string{
			fmt.Sprintf("source=%s", m.Source),
			fmt.Sprintf("target=%s", m.Target),
		}

		switch m.Type {
		case mount.TypeBind:
			mountStrParts = append(mountStrParts, "type=bind")
		default:
			return nil, fmt.Errorf("mount type %s not supported", m.Type)
		}

		if m.ReadOnly {
			mountStrParts = append(mountStrParts, "readonly")
		}

		args = append(args, fmt.Sprintf("--mount=%s", strings.Join(mountStrParts, ",")))
	}

	return args, nil
}

// dockerLogin generates an access token with 'gcloud auth print-access-token'
// and then runs 'docker login'.
//
// TODO(b/201431966): Remove this when it is not necessary, e.g. when
// 'gcloud auth configure-docker' is run in the environment setup.
func dockerLogin(ctx context.Context, runner cmd.CommandRunner, registry string) error {
	if err := runner.RunCommand(
		ctx, os.Stdout, os.Stderr, "",
		"sudo", "gcloud", "auth", "activate-service-account",
		"--key-file=/creds/service_accounts/skylab-drone.json",
	); err != nil {
		return errors.Wrap(err, "failed running 'gcloud auth activate-service-account'")
	}

	var stdoutBuf bytes.Buffer

	err := runner.RunCommand(
		ctx, &stdoutBuf, os.Stderr, "",
		"sudo", "gcloud", "auth", "print-access-token",
	)

	if err != nil {
		return errors.Wrap(err, "failed running 'gcloud auth print-access-token'")
	}

	accessToken := stdoutBuf.String()

	err = runner.RunCommand(
		ctx, os.Stdout, os.Stderr, "",
		"sudo", "docker", "login", "-u", "oauth2accesstoken",
		"-p", accessToken, registry,
	)

	if err != nil {
		return errors.Wrap(err, "failed running 'docker login'")
	}

	return nil
}

// RuntimeOptions configures how docker and related auth commands are run.
type RuntimeOptions struct {
	// If true, use `gcloud auth configure-docker` instead of `docker login` to
	// authenticate with the registry. Note that this is never run with sudo.
	UseConfigureDocker bool
	// If true, don't use sudo when running docker. Note this doesn't affect the
	// authentication commands.
	NoSudo bool
	// Writer to send stdout from the docker command to, nil is valid (stdout
	// isn't captured or printed).
	StdoutBuf io.Writer
	// Writer to send stderr from the docker command to, nil is valid (stderr
	// isn't captured or printed).
	StderrBuf io.Writer
}

// ContainerRunner runs Docker containers, handling authentication, mounting,
// etc.
type ContainerRunner struct {
	runner          cmd.CommandRunner
	gcloudMutex     sync.Mutex
	configuredHosts map[string]bool
}

// NewContainerRunner returns a ContainerRunner that uses runner to run `docker`
// commands and other required commands (such as authenticating with `gcloud`).
func NewContainerRunner(runner cmd.CommandRunner) *ContainerRunner {
	return &ContainerRunner{
		runner:          runner,
		gcloudMutex:     sync.Mutex{},
		configuredHosts: make(map[string]bool),
	}
}

// RunContainer runs a container with `docker run`.
func (c *ContainerRunner) RunContainer(
	ctx context.Context,
	containerConfig *container.Config,
	hostConfig *container.HostConfig,
	containerImageInfo *api.ContainerImageInfo,
	runtimeOptions *RuntimeOptions,
) error {
	if runtimeOptions.UseConfigureDocker {
		c.gcloudMutex.Lock()
		defer c.gcloudMutex.Unlock()

		host := containerImageInfo.GetRepository().GetHostname()
		if _, configured := c.configuredHosts[host]; !configured {
			logging.Debugf(ctx, "host %q not configured, running `gcloud auth configure-docker`", host)
			var stderrBuf bytes.Buffer
			if err := c.runner.RunCommand(ctx, io.Discard, &stderrBuf, "", "gcloud", "auth", "configure-docker", containerImageInfo.GetRepository().GetHostname(), "--quiet"); err != nil {
				logging.Errorf(ctx, "gcloud auth configure-docker failed, stderr: %s", &stderrBuf)
				return err
			}

			c.configuredHosts[host] = true
		} else {
			logging.Debugf(ctx, "host %q already configured", host)
		}
	} else {
		if err := dockerLogin(
			ctx, c.runner,
			fmt.Sprintf(
				"%s/%s",
				containerImageInfo.GetRepository().GetHostname(),
				containerImageInfo.GetRepository().GetProject(),
			),
		); err != nil {
			return err
		}
	}

	args := []string{"run"}

	if containerConfig.User != "" {
		args = append(args, "--user", containerConfig.User)
	}

	if hostConfig.NetworkMode != "" {
		args = append(args, "--network", string(hostConfig.NetworkMode))
	}

	mountArgs, err := generateMountArgs(hostConfig.Mounts)
	if err != nil {
		return err
	}

	args = append(args, mountArgs...)
	args = append(args, containerConfig.Image)
	args = append(args, containerConfig.Cmd...)

	logging.Debugf(ctx, "Running docker cmd: %q", args)

	var cmd string
	if runtimeOptions.NoSudo {
		cmd = "docker"
	} else {
		cmd = "sudo"
		args = append([]string{"docker"}, args...)
	}
	return c.runner.RunCommand(ctx, runtimeOptions.StdoutBuf, runtimeOptions.StderrBuf, "", cmd, args...)
}
