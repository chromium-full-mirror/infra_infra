// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ctr

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"go.chromium.org/luci/common/errors"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/recovery/internal/log"
)

// BaseContainer describe API to work with containers.
type BaseContainer interface {
	Name() string
	Exec(ctx context.Context, cmd string) (outstd, errstd string, err error)
	Close(ctx context.Context) error
	IsClosed() bool
	GetClient(ctx context.Context) (*grpc.ClientConn, error)
	ServiceAddress(ctx context.Context) (string, error)
}
type baseContainerImpl struct {
	name string
	ci   *serviceInfoImpl

	// With delay.
	serviceAddr string
	conn        *grpc.ClientConn
}

// Name returns the name of container.
func (c *baseContainerImpl) Name() string {
	return c.name
}

// Exec executes docker command command for the container.
func (c *baseContainerImpl) Exec(ctx context.Context, cmd string) (outstd, errstd string, _ error) {
	if c.IsClosed() {
		return "", "", errors.Reason("exec: container is closed").Err()
	}
	return "", "", errors.Reason("run: not implemented").Err()
}

// Close closes active resources.
func (c *baseContainerImpl) Close(ctx context.Context) error {
	if c.IsClosed() {
		return nil
	}
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			return errors.Annotate(err, "close container %q", c.Name()).Err()
		}
	}
	c.ci = nil
	return nil
}

// IsClosed tells if the container was closed.
func (c *baseContainerImpl) IsClosed() bool {
	return c == nil || c.ci == nil
}

// GetClient creates and return client to work with a secrvice running in the container.
func (c *baseContainerImpl) GetClient(ctx context.Context) (*grpc.ClientConn, error) {
	if c.conn != nil {
		return c.conn, nil
	}
	addr, err := c.ServiceAddress(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "get client %q", c.Name()).Err()
	}
	conn, err := common.ConnectWithService(ctx, addr)
	if err != nil {
		return nil, errors.Annotate(err, "get client %q", c.Name()).Err()
	}
	c.conn = conn
	log.Debugf(ctx, "Client for %q created!", c.Name())
	return c.conn, nil
}

// serviceAddress reads an address of the service running in the container.
func (c *baseContainerImpl) ServiceAddress(ctx context.Context) (string, error) {
	if c.IsClosed() {
		return "", errors.Reason("service address: container is closed").Err()
	}
	if c.serviceAddr == "" {
		res, err := c.ci.ctr.GetContainer(ctx, c.Name())
		if err != nil {
			return "", errors.Annotate(err, "service address").Err()
		}
		for _, p := range res.GetContainer().GetPortBindings() {
			if p.GetHostIp() == "" || p.GetHostPort() == 0 || p.GetProtocol() != "tcp" {
				continue
			}
			c.serviceAddr = fmt.Sprintf("%s:%d", p.GetHostIp(), p.GetHostPort())
			log.Debugf(ctx, "Container %q: service runs on %q address", c.Name(), c.serviceAddr)
			break
		}
		if c.serviceAddr == "" {
			return "", errors.Reason("service address of %q: not found", c.Name()).Err()
		}
	}
	return c.serviceAddr, nil
}
