// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package servod provides functions to manage connection and communication with servod daemon on servo-host.
package servod

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/dev"
	"infra/cros/recovery/docker"
	"infra/cros/recovery/internal/localtlw/localproxy"
	"infra/cros/recovery/internal/localtlw/ssh"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/logger/metrics"
	"infra/cros/recovery/tlw"
)

// StartServodRequest holds data to start servod container.
type StartServodRequest struct {
	Host        string
	Options     *tlw.ServodOptions
	SSHProvider ssh.SSHProvider
	// Containers info.
	ContainerName    string
	ContainerNetwork string
}

const (
	observationKindStartServodTimeoutFail    = "start_servod_timeout_fail"
	observationKindStartServodTimeoutSuccess = "start_servod_timeout_success"
)

// StartServod starts servod daemon on servo-host.
// Method detect and working with all type of hosts.
func StartServod(ctx context.Context, req *StartServodRequest) error {
	switch {
	case req.Host == "":
		return errors.Reason("start servod: host is ot specified").Err()
	case req.SSHProvider == nil:
		return errors.Reason("start servod: SSH provider is not specified").Err()
	case req.Options == nil:
		return errors.Reason("start servod: options is not specified").Err()
	case req.Options.GetServodPort() <= 0 && req.ContainerName == "":
		return errors.Reason("start servod: servod port is not specified").Err()
	case req.ContainerName == "":
		// regular labstation
		return startServodLabstation(ctx, req)
	case req.ContainerName == req.Host:
		return startServodOnLocalContainer(ctx, req)
	case req.ContainerName != req.Host:
		return startServodOnRemoteContainer(ctx, req)
	default:
		return errors.Reason("start servod: unsupported case").Err()
	}
}

func startServodOnLocalContainer(ctx context.Context, req *StartServodRequest) error {
	log.Debugf(ctx, "Start servod on local container with %#v", req.Options)
	d, err := newDockerClient(ctx)
	if err != nil {
		return errors.Annotate(err, "start servod container").Err()
	}
	// Print all containers to know if something wrong.
	d.PrintAll(ctx)
	log.Debugf(ctx, "Removing old container %q before start new one!", req.ContainerName)
	if err := d.Remove(ctx, req.ContainerName, true); err != nil {
		log.Debugf(ctx, "Fail to remove container (not-critical): %s", err)
	}
	// If a port is not specified then the request for a container without servod.
	startServod := req.Options.GetServodPort() > 0
	envVar := GenerateParams(req.Options)
	var exposePorts []string
	containerStartArgs := []string{"tail", "-f", "/dev/null"}
	if startServod {
		containerStartArgs = []string{"bash", "/start_servod.sh"}
		if dev.IsActive(ctx) {
			exposePorts = append(exposePorts, fmt.Sprintf("%d:%d/tcp", req.Options.GetServodPort(), req.Options.GetServodPort()))
		}
	}
	containerArgs := createServodContainerArgs(true, exposePorts, envVar, containerStartArgs)
	// Servod expected to start in less 1 minutes.
	// Image is small is expected to be download in less 1 minute.
	// To be safe we set 4 minutes to be sure everything will work.
	res, err := d.Start(ctx, req.ContainerName, containerArgs, 5*time.Minute)
	if err != nil {
		return errors.Annotate(err, "start servod container").Err()
	}
	log.Debugf(ctx, "Container started with id:%s\n with errout: %#v and code:%v", res.Stdout, res.Stderr, res.ExitCode)
	if startServod {
		// Waiting to finish servod initialization.
		// Wait 3 seconds as sometimes container is not fully initialized and fail
		// when start ing working with servod or tooling.
		time.Sleep(3 * time.Second)
		if err := dockerVerifyServodDaemonIsUp(ctx, d, req.ContainerName, req.Options.GetServodPort(), servodStartTimeoutSec); err != nil {
			return errors.Annotate(err, "start servod container").Err()
		}
	}
	log.Debugf(ctx, "Servod container %s started and up!", req.ContainerName)
	return nil

}

func startServodOnRemoteContainer(ctx context.Context, req *StartServodRequest) error {
	return errors.Reason("start servod on remote container: not implemented").Err()
}

func startServodLabstation(ctx context.Context, req *StartServodRequest) error {
	// Convert hostname to the proxy name used for local when called.
	host := localproxy.BuildAddr(req.Host)
	if stat, err := getServodStatus(ctx, host, req.Options.GetServodPort(), req.SSHProvider); err != nil {
		return errors.Annotate(err, "start servod on labstation").Err()
	} else if stat == servodRunning {
		// Servod is running already.
		return nil
	}
	if err := startServod(ctx, host, req.Options.GetServodPort(), GenerateParams(req.Options), req.SSHProvider); err != nil {
		return errors.Annotate(err, "start servod on labstation").Err()
	}
	return nil
}

// dockerServodImageName provides image for servod when use container.
func dockerServodImageName() string {
	label := getEnv("SERVOD_CONTAINER_LABEL", "release")
	registry := getEnv("REGISTRY_URI", "us-docker.pkg.dev/chromeos-partner-moblab/common-core")
	return fmt.Sprintf("%s/servod:%s", registry, label)
}

// getEnv retrieves the value of the environment variable named by the key.
// If retrieved value is empty return default value.
func getEnv(key, defaultvalue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultvalue
}

// defaultDockerNetwork provides network in which docker need to run.
func defaultDockerNetwork() string {
	return os.Getenv("DOCKER_DEFAULT_NETWORK")
}

// createServodContainerArgs creates default args for servodContainer.
func createServodContainerArgs(detached bool, exposePorts, envVar, cmd []string) *docker.ContainerArgs {
	return &docker.ContainerArgs{
		Detached:    detached,
		EnvVar:      envVar,
		ImageName:   dockerServodImageName(),
		Network:     defaultDockerNetwork(),
		Volumes:     []string{"/dev:/dev"},
		ExposePorts: exposePorts,
		Privileged:  true,
		Exec:        cmd,
	}
}

// dockerVerifyServodDaemonIsUp verifies servod is running on servod daemon is up in container.
func dockerVerifyServodDaemonIsUp(ctx context.Context, dc docker.Client, containerName string, servodPort int32, waitTime int) error {
	eReq := &docker.ExecRequest{
		Timeout: 2 * time.Minute,
		Cmd: []string{
			"servodtool",
			"instance",
			"wait-for-active",
			"-p",
			fmt.Sprintf("%d", servodPort),
			"--timeout",
			fmt.Sprintf("%d", waitTime),
		},
	}
	startTime := time.Now()
	res, err := dc.Exec(ctx, containerName, eReq)
	servodStartDuration := time.Since(startTime)
	if err != nil {
		metrics.DefaultActionAddObservations(ctx, metrics.NewFloat64Observation(observationKindStartServodTimeoutFail, servodStartDuration.Seconds()))
		return errors.Annotate(err, "docker verify servod daemon is up").Err()
	} else if res != nil && res.ExitCode != 0 {
		// When the wait time is less request timeout, the cmd can finish with nil error
		// exitcode can be 1 if the wait time is exceeded.
		log.Debugf(ctx, "servodtool did not response before %s", waitTime)
		metrics.DefaultActionAddObservations(ctx, metrics.NewFloat64Observation(observationKindStartServodTimeoutFail, servodStartDuration.Seconds()))
		return errors.Reason("docker verify servod daemon is up: %s", res.Stderr).Err()
	}
	// Servod process is up.
	metrics.DefaultActionAddObservations(ctx, metrics.NewFloat64Observation(observationKindStartServodTimeoutSuccess, servodStartDuration.Seconds()))
	return nil
}
