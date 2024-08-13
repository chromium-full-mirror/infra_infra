// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ctr

import (
	"context"

	"go.chromium.org/luci/common/errors"
)

// BaseContainer describe API to work with containers.
type BaseContainer interface {
	Name() string
	Exec(ctx context.Context, cmd string) (outstd, errstd string, err error)
	Stop(ctx context.Context) error
	IsClosed() bool
}
type baseContainerImpl struct {
	name string
	ci   *serviceInfoImpl
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

// Close stops container and mark it as closed.
func (c *baseContainerImpl) Stop(ctx context.Context) error {
	if c.ci == nil || c.ci.ctr == nil {
		return nil
	}
	if err := c.ci.ctr.StopContainer(ctx, c.name); err != nil {
		return errors.Annotate(err, "stop container").Err()
	}
	c.ci = nil
	return nil
}

// IsClosed tells if the container was closed.
func (c *baseContainerImpl) IsClosed() bool {
	return c.ci == nil
}
