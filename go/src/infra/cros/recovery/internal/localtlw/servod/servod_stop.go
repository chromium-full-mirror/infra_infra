// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package servod provides functions to manage connection and communication with servod daemon on servo-host.
package servod

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/localtlw/localproxy"
	"go.chromium.org/infra/cros/recovery/internal/localtlw/ssh"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// StopServodRequest holds data to stop servod container.
type StopServodRequest struct {
	Host        string
	Options     *tlw.ServodOptions
	SSHProvider ssh.SSHProvider
	// Containers info.
	ContainerName    string
	ContainerNetwork string
}

// StopServod stops servod daemon on servo-host.
// Method detect and working with all type of hosts.
func StopServod(ctx context.Context, req *StopServodRequest) error {
	switch {
	case req.Host == "":
		return errors.Reason("stop servod: host is ot specified").Err()
	case req.SSHProvider == nil:
		return errors.Reason("stop servod: SSH provider is not specified").Err()
	case req.Options == nil:
		return errors.Reason("stop servod: options is not specified").Err()
	case req.Options.GetServodPort() <= 0 && req.ContainerName == "":
		return errors.Reason("stop servod: servod port is not specified").Err()
	case req.ContainerName == "":
		// regular labstation
		return stopServodLabstation(ctx, req)
	case req.ContainerName == req.Host:
		return stopServodOnLocalContainer(ctx, req)
	case req.ContainerName != req.Host:
		return stopServodOnRemoteContainer(ctx, req)
	default:
		return errors.Reason("stop servod: unsupported case").Err()
	}
}

func stopServodOnLocalContainer(ctx context.Context, req *StopServodRequest) error {
	log.Debugf(ctx, "Stop servod on local container with %#v", req.Options)
	d, err := newDockerClient(ctx)
	if err != nil {
		return errors.Annotate(err, "stop servod %q", req.Host).Err()
	}
	if err := d.Remove(ctx, req.ContainerName, true); err != nil {
		return errors.Annotate(err, "stop servod %q", req.Host).Err()
	}
	return nil
}

func stopServodOnRemoteContainer(ctx context.Context, req *StopServodRequest) error {
	return errors.Reason("stop servod on remote container: not implemented").Err()
}

func stopServodLabstation(ctx context.Context, req *StopServodRequest) error {
	// Convert hostname to the proxy name used for local when called.
	host := localproxy.BuildAddr(req.Host)
	if stat, err := getServodStatus(ctx, host, req.Options.GetServodPort(), req.SSHProvider); err != nil {
		return errors.Annotate(err, "stop servod on labstation").Err()
	} else if stat == servodNotRunning {
		// Servod is not running.
		return nil
	}
	if err := stopServod(ctx, host, req.Options.GetServodPort(), req.SSHProvider); err != nil {
		return errors.Annotate(err, "stop servod on labstation").Err()
	}
	return nil
}
