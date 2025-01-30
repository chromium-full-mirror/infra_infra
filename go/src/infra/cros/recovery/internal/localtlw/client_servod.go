// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package localtlw provides local implementation of TLW Access.
package localtlw

import (
	"context"
	"fmt"
	"os"

	"go.chromium.org/chromiumos/config/go/api/test/xmlrpc"
	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/docker"
	"infra/cros/recovery/internal/localtlw/servod"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/tlw"
)

// InitServod initiates servod daemon on servo-host.
func (c *tlwClient) InitServod(ctx context.Context, req *tlw.InitServodRequest) error {
	dut, err := c.getDevice(ctx, req.Resource)
	if err != nil {
		return errors.Annotate(err, "init servod %q", req.Resource).Err()
	}
	servoHost := dut.GetChromeos().GetServo()
	if servoHost.GetName() == "" {
		return errors.Reason("init servod %q: servo is not found", req.Resource).Err()
	}
	options := req.GetOptions()
	// Always override port by port specified in data.
	options.ServodPort = servoHost.GetServodPort()
	if req.GetNoServod() {
		// Set port 0 as it should be run.
		options.ServodPort = 0
	}
	startReq := &servod.StartServodRequest{
		Host:        servoHost.GetName(),
		SSHProvider: c.sshProvider,
		Options:     options,
		// Container info.
		ContainerName: servoHost.GetContainerName(),
	}
	switch {
	case startReq.ContainerName != "":
		fallthrough
	case startReq.ContainerName == "" && !req.GetNoServod():
		// Request to start.
		if err := servod.StartServod(ctx, startReq); err != nil {
			return errors.Annotate(err, "init servod %q", req.Resource).Err()
		}
	case startReq.ContainerName == "" && req.GetNoServod():
		// Just try to stop servod if it is running.
		if err := servod.StopServod(ctx, &servod.StopServodRequest{
			Host:        servoHost.GetName(),
			SSHProvider: c.sshProvider,
			Options: &tlw.ServodOptions{
				ServodPort: servoHost.GetServodPort(),
			},
		}); err != nil {
			log.Infof(ctx, "(Not critical) Fail to stop servod as requested to prepare servo-host without servod daemon: %s", err)
		}
	default:
		return errors.Reason("init servod %q: unexpected case", req.Resource).Err()
	}
	return nil
}

// dockerServodImageName provides image for servod when use container.
// TODO(b:260649824): move to servod package.
func dockerServodImageName() string {
	label := getEnv("SERVOD_CONTAINER_LABEL", "release")
	registry := getEnv("REGISTRY_URI", "us-docker.pkg.dev/chromeos-partner-moblab/common-core")
	return fmt.Sprintf("%s/servod:%s", registry, label)
}

// getEnv retrieves the value of the environment variable named by the key.
// If retrieved value is empty return default value.
// TODO(b:260649824): move to servod package.
func getEnv(key, defaultvalue string) string {
	if key != "" {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return defaultvalue
}

// createServodContainerArgs creates default args for servodContainer.
// TODO(b:260649824): move to servod package.
func createServodContainerArgs(detached bool, exposePorts, envVar, cmd []string) *docker.ContainerArgs {
	return &docker.ContainerArgs{
		Detached:   detached,
		EnvVar:     envVar,
		ImageName:  dockerServodImageName(),
		Network:    defaultDockerNetwork(),
		Volumes:    []string{"/dev:/dev"},
		Privileged: true,
		Exec:       cmd,
	}
}

// defaultDockerNetwork provides network in which docker need to run.
// TODO(b:260649824): move to servod package.
func defaultDockerNetwork() string {
	return os.Getenv("DOCKER_DEFAULT_NETWORK")
}

// StopServod stops servod daemon on servo-host.
func (c *tlwClient) StopServod(ctx context.Context, resourceName string) error {
	dut, err := c.getDevice(ctx, resourceName)
	if err != nil {
		return errors.Annotate(err, "stop servod %q", resourceName).Err()
	}
	servoHost := dut.GetChromeos().GetServo()
	if servoHost.GetName() == "" {
		return errors.Reason("stop servod %q: servo is not found", resourceName).Err()
	}
	stopReq := &servod.StopServodRequest{
		Host:        servoHost.GetName(),
		SSHProvider: c.sshProvider,
		Options: &tlw.ServodOptions{
			ServodPort: servoHost.GetServodPort(),
		},
		// Container info.
		ContainerName: servoHost.GetContainerName(),
	}
	if err := servod.StopServod(ctx, stopReq); err != nil {
		return errors.Annotate(err, "stop servod %q", resourceName).Err()
	}
	return nil
}

// CallServod executes a command on servod related to resource name.
// Commands will be run against servod on servo-host.
func (c *tlwClient) CallServod(ctx context.Context, req *tlw.CallServodRequest) *tlw.CallServodResponse {
	dut, err := c.getDevice(ctx, req.Resource)
	if err != nil {
		return generateFailCallServodResponse(ctx, req.GetResource(), err)
	}
	servoHost := dut.GetChromeos().GetServo()
	if servoHost.GetName() == "" {
		return generateFailCallServodResponse(ctx, req.GetResource(), errors.Reason("call servod %q: servo not found", req.GetResource()).Err())
	}
	callReq := &servod.ServodCallRequest{
		Host:        servoHost.GetName(),
		SSHProvider: c.sshProvider,
		Options: &tlw.ServodOptions{
			ServodPort: servoHost.GetServodPort(),
		},
		// Container info.
		ContainerName: servoHost.GetContainerName(),
		// Call details
		CallMethod:    req.GetMethod(),
		CallArguments: req.GetArgs(),
		CallTimeout:   req.GetTimeout().AsDuration(),
	}
	if val, err := servod.CallServod(ctx, callReq); err != nil {
		return generateFailCallServodResponse(ctx, req.GetResource(), err)
	} else {
		return &tlw.CallServodResponse{
			Value: val,
			Fault: false,
		}
	}
}

// generateFailCallServodResponse creates response for fail cases when call servod.
func generateFailCallServodResponse(ctx context.Context, resource string, err error) *tlw.CallServodResponse {
	log.Debugf(ctx, "Call servod fail with %s", err)
	return &tlw.CallServodResponse{
		Value: &xmlrpc.Value{
			ScalarOneof: &xmlrpc.Value_String_{
				String_: fmt.Sprintf("call servod %q: %s", resource, err),
			},
		},
		Fault: true,
	}
}

// isServoHost tells if host is servo-host.
func (c *tlwClient) isServoHost(host string) bool {
	if v, ok := c.hostTypes[host]; ok {
		return v == hostTypeServo
	}
	return false
}

// dockerClient provides docker client for target container by expected name of container.
func (c *tlwClient) dockerClient(ctx context.Context) (docker.Client, error) {
	d, err := docker.NewClient(ctx)
	return d, errors.Annotate(err, "docker client").Err()
}

// isServodContainer checks if DUT using servod-container.
// For now just simple check if servod container is provided.
// Later need distinguish when container running on the same host or remove one.
func isServodContainer(d *tlw.Dut) bool {
	return servoContainerName(d) != ""
}

// servoContainerName returns container name specified for servo-host.
func servoContainerName(d *tlw.Dut) string {
	return d.GetChromeos().GetServo().GetContainerName()
}
