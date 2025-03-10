// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gofrs/flock"

	"go.chromium.org/chromiumos/config/go/test/api"
)

// DockerRun represents `docker run` and is an alias to StartContainerRequest
type DockerRun struct {
	*api.StartContainerRequest
}

// compose implements argumentsComposer
func (c *DockerRun) compose() ([]string, error) {
	if c.ContainerImage == "" {
		return nil, errors.New("ContainerImage is mandatory")
	}
	args := []string{"run", "-d", "--rm", "--cap-add=NET_RAW"}
	if strings.Contains(c.ContainerImage, "foil-test") || strings.Contains(c.ContainerImage, "tradefed") {
		args = append(args, "--security-opt", "seccomp=unconfined")
	}
	if c.Name != "" {
		args = append(args, "--name", c.Name)
	}
	if c.AdditionalOptions != nil {
		options := c.AdditionalOptions
		if options.Network != "" {
			args = append(args, "--network", options.Network)
		}
		// TODO: pass port exposing through request object.
		// Reneable expose options.
		if options.Volume != nil {
			for _, volume := range options.Volume {
				if volume == "" {
					continue
				}
				args = append(args, "--volume", volume)
			}
		}
		if options.Env != nil {
			for _, env := range options.Env {
				if env == "" {
					continue
				}
				args = append(args, "--env", env)
			}
		}
	}
	args = append(args, c.ContainerImage)
	args = append(args, "timeout", "30h")
	args = append(args, c.StartCommand...)
	return args, nil
}

func (c *DockerRun) Execute(ctx context.Context) (string, string, error) {
	args, err := c.compose()
	if err != nil {
		return "", "", err
	}
	// Normally docker run -d should return a container ID instantly. However, on
	// Drone, I/O constraints may delay execution significantly. See discussion in
	// b/238684062
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	startTime := time.Now()
	stdout, stderr, err := execute(ctx, dockerCmd, args)
	status := statusPass
	if err != nil {
		status = statusFail
	}
	monitorTime(c, startTime)
	monitorStatus(c, status)
	return stdout, stderr, err
}

// DockerPull represents `docker pull`
type DockerPull struct {
	ContainerImage string // ContainerImage is the full location of an image that can directly pulled by docker
}

func (c *DockerPull) Execute(ctx context.Context) (string, string, error) {
	args := []string{"pull", c.ContainerImage}
	startTime := time.Now()
	stdout, stderr, err := execute(ctx, dockerCmd, args)
	if err == nil {
		monitorTime(c, startTime)
	}
	return stdout, stderr, err
}

// DockerLogin represents `docker login` and is an alias to LoginRegistryRequest
type DockerLogin struct {
	*api.LoginRegistryRequest
}

func (c *DockerLogin) Execute(ctx context.Context) (string, string, error) {
	args := []string{"login", "-u", c.Username, "--password-stdin", c.Registry}
	return executeWithStdin(ctx, dockerCmd, args, c.Password)
}

// GcloudAuthTokenPrint represents `gcloud auth print-access-token`
type GcloudAuthTokenPrint struct {
}

func (c *GcloudAuthTokenPrint) Execute(ctx context.Context) (string, string, error) {
	args := []string{"auth", "print-access-token"}
	// TODO(mingkong) refactor commands package to make command unit testable
	fileLock := flock.New(lockFile)
	err := fileLock.Lock()
	if err != nil {
		return "", "failed to get FLock prior to gcloud auth print-access-token call", err
	}
	defer fileLock.Unlock()
	return execute(ctx, "gcloud", args)
}

// GcloudAuthServiceAccount represents `gcloud auth activate-service-account`
type GcloudAuthServiceAccount struct {
	Args []string
}

func (c *GcloudAuthServiceAccount) Execute(ctx context.Context) (string, string, error) {
	args := []string{"auth", "activate-service-account"}
	args = append(args, c.Args...)
	fileLock := flock.New(lockFile)
	err := fileLock.Lock()
	if err != nil {
		return "", "failed to get FLock prior to gcloud auth activate-service-account call", err
	}
	defer fileLock.Unlock()
	return execute(ctx, "gcloud", args)
}
