// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package localtlw provides local implementation of TLW Access.
package localtlw

import (
	"context"

	"go.chromium.org/luci/common/errors"

	tlwio "go.chromium.org/infra/cros/recovery/internal/localtlw/io"
	"go.chromium.org/infra/cros/recovery/internal/localtlw/localproxy"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// validateCopyRequest validates that all data is provided as part of request.
func validateCopyRequest(req *tlw.CopyRequest) error {
	if req.Resource == "" {
		return errors.Reason("resource is not provided").Err()
	} else if req.PathSource == "" {
		return errors.Reason("src path is empty").Err()
	} else if req.PathDestination == "" {
		return errors.Reason("destination path is not provided").Err()
	}
	return nil
}

// CopyFileTo copies file to remote device from local.
func (c *tlwClient) CopyFileTo(ctx context.Context, req *tlw.CopyRequest) error {
	if err := validateCopyRequest(req); err != nil {
		return errors.Annotate(err, "copy file to").Err()
	}
	if err := tlwio.CopyFileTo(ctx, c.sshProvider, req); err != nil {
		return errors.Annotate(err, "copy file to").Err()
	}
	return nil
}

// CopyFileFrom copies file from remote device to local.
func (c *tlwClient) CopyFileFrom(ctx context.Context, req *tlw.CopyRequest) (mainErr error) {
	if vErr := validateCopyRequest(req); vErr != nil {
		return errors.Annotate(vErr, "copy file from").Err()
	}
	dut, dErr := c.getDevice(ctx, req.Resource)
	if dErr != nil {
		return errors.Annotate(dErr, "copy file from %q", req.Resource).Err()
	}
	// The containerized servo-host does not support SSH so we need use docker client.
	if c.isServoHost(req.Resource) && isServodContainer(dut) {
		d, cErr := c.dockerClient(ctx)
		if cErr != nil {
			return errors.Annotate(cErr, "copy file from %q", req.Resource).Err()
		}
		containerName := servoContainerName(dut)
		if up, iErr := d.IsUp(ctx, containerName); iErr != nil {
			return errors.Annotate(iErr, "copy file from %q", req.Resource).Err()
		} else if !up {
			log.Infof(ctx, "Copy file from: servod container %s is down!", containerName)
			return errors.Annotate(iErr, "copy file from %q", req.Resource).Err()
		}
		mainErr = d.CopyFrom(ctx, containerName, req.PathSource, req.PathDestination)
	} else {
		// Use dirrect copy if hosts support SSH.
		mainErr = tlwio.CopyFileFrom(ctx, c.sshProvider, &tlw.CopyRequest{
			Resource:        localproxy.BuildAddr(req.Resource),
			PathSource:      req.PathSource,
			PathDestination: req.PathDestination,
		})
	}
	return errors.Annotate(mainErr, "copy file from %q", req.Resource).Err()
}

// CopyDirectoryTo copies directory to remote device from local, recursively.
func (c *tlwClient) CopyDirectoryTo(ctx context.Context, req *tlw.CopyRequest) error {
	if err := tlwio.CopyDirectoryTo(ctx, c.sshProvider, req); err != nil {
		return errors.Annotate(err, "copy directory to").Err()
	}
	return nil
}

// CopyDirectoryFrom copies directory from remote device to local, recursively.
func (c *tlwClient) CopyDirectoryFrom(ctx context.Context, req *tlw.CopyRequest) error {
	// TODO (vkjoshi@): Need to add support for containerized
	// servo-hosts, analogous to that in CopyFileFrom.
	if err := tlwio.CopyDirectoryFrom(ctx, c.sshProvider, &tlw.CopyRequest{
		Resource:        localproxy.BuildAddr(req.Resource),
		PathSource:      req.PathSource,
		PathDestination: req.PathDestination,
	}); err != nil {
		return errors.Annotate(err, "copy directory from").Err()
	}
	return nil
}
