// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package docker_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/strslice"

	"go.chromium.org/chromiumos/config/go/build/api"

	"go.chromium.org/infra/cros/internal/cmd"
	"go.chromium.org/infra/cros/internal/docker"
)

var containerImageInfo = &api.ContainerImageInfo{
	Repository: &api.GcrRepository{
		Hostname: "gcr.io",
		Project:  "testproject",
	},
}

var containerConfig = &container.Config{
	Cmd:   strslice.StrSlice{"ls", "-l"},
	User:  "testuser",
	Image: "testimage",
}

var hostConfig = &container.HostConfig{
	Mounts: []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: "/tmp/hostdir",
			Target: "/usr/local/containerdir",
		},
		{
			Type:     mount.TypeBind,
			Source:   "/othersource",
			Target:   "/othertarget",
			ReadOnly: true,
		},
	},
	NetworkMode: "host",
}

func TestRunContainer(t *testing.T) {
	ctx := context.Background()

	runtimeOptions := &docker.RuntimeOptions{
		UseConfigureDocker: false,
		NoSudo:             false,
		StdoutBuf:          os.Stdout,
		StderrBuf:          os.Stderr,
	}

	cmdRunner := &cmd.FakeCommandRunnerMulti{
		CommandRunners: []cmd.FakeCommandRunner{
			{
				ExpectedCmd: []string{
					"sudo", "gcloud", "auth", "activate-service-account",
					"--key-file=/creds/service_accounts/skylab-drone.json",
				},
			},
			{
				ExpectedCmd: []string{
					"sudo", "gcloud", "auth", "print-access-token",
				},
				Stdout: "abc123",
			},
			{
				ExpectedCmd: []string{
					"sudo", "docker", "login", "-u", "oauth2accesstoken",
					"-p", "abc123", "gcr.io/testproject",
				},
			},
			{
				ExpectedCmd: []string{
					"sudo", "docker", "run",
					"--user", "testuser",
					"--network", "host",
					"--mount=source=/tmp/hostdir,target=/usr/local/containerdir,type=bind",
					"--mount=source=/othersource,target=/othertarget,type=bind,readonly",
					"testimage",
					"ls", "-l",
				},
			},
		},
	}

	err := docker.NewContainerRunner(cmdRunner).RunContainer(ctx, containerConfig, hostConfig, containerImageInfo, runtimeOptions)
	if err != nil {
		t.Fatalf("RunContainer failed: %s", err)
	}
}

func TestRunContainer_WithConfigureDocker(t *testing.T) {
	ctx := context.Background()

	runtimeOptions := &docker.RuntimeOptions{
		UseConfigureDocker: true,
		NoSudo:             true,
		StdoutBuf:          os.Stdout,
		StderrBuf:          os.Stderr,
	}

	cmdRunner := &cmd.FakeCommandRunnerMulti{
		CommandRunners: []cmd.FakeCommandRunner{
			{
				ExpectedCmd: []string{
					"gcloud", "auth", "configure-docker",
					"gcr.io", "--quiet",
				},
			},
			{
				ExpectedCmd: []string{
					"docker", "run",
					"--user", "testuser",
					"--network", "host",
					"--mount=source=/tmp/hostdir,target=/usr/local/containerdir,type=bind",
					"--mount=source=/othersource,target=/othertarget,type=bind,readonly",
					"testimage",
					"ls", "-l",
				},
			},
			{
				ExpectedCmd: []string{
					"docker", "run",
					"--user", "testuser",
					"--network", "host",
					"--mount=source=/tmp/hostdir,target=/usr/local/containerdir,type=bind",
					"--mount=source=/othersource,target=/othertarget,type=bind,readonly",
					"testimage",
					"ls", "-l",
				},
			},
		},
	}

	containerRunner := docker.NewContainerRunner(cmdRunner)
	for i := 0; i < 2; i += 1 {
		err := containerRunner.RunContainer(ctx, containerConfig, hostConfig, containerImageInfo, runtimeOptions)
		if err != nil {
			t.Fatalf("RunContainer failed: %s", err)
		}
	}
}

func TestRunContainer_CmdError(t *testing.T) {
	ctx := context.Background()

	runtimeOptions := &docker.RuntimeOptions{
		UseConfigureDocker: false,
		NoSudo:             false,
		StdoutBuf:          os.Stdout,
		StderrBuf:          os.Stderr,
	}

	cmdRunner := cmd.FakeCommandRunner{}
	cmdRunner.FailCommand = true
	cmdRunner.FailError = errors.New("docker cmd failed.")

	err := docker.NewContainerRunner(cmdRunner).RunContainer(ctx, containerConfig, hostConfig, containerImageInfo, runtimeOptions)
	if err == nil {
		t.Errorf("RunContainer expected to fail")
	}
}

func TestRunContainer_ConfigureDockerError(t *testing.T) {
	ctx := context.Background()

	runtimeOptions := &docker.RuntimeOptions{
		UseConfigureDocker: true,
		NoSudo:             true,
		StdoutBuf:          os.Stdout,
		StderrBuf:          os.Stderr,
	}

	cmdRunner := cmd.FakeCommandRunner{}
	cmdRunner.FailCommand = true
	cmdRunner.FailError = errors.New("configure-docker cmd failed.")

	err := docker.NewContainerRunner(cmdRunner).RunContainer(ctx, containerConfig, hostConfig, containerImageInfo, runtimeOptions)
	if err == nil {
		t.Errorf("RunContainer expected to fail")
	}
}
