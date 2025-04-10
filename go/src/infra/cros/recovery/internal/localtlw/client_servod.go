// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package localtlw provides local implementation of TLW Access.
package localtlw

import (
	"context"
	"fmt"

	"go.chromium.org/chromiumos/config/go/api/test/xmlrpc"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/docker"
	"go.chromium.org/infra/cros/recovery/internal/localtlw/servod"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
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
