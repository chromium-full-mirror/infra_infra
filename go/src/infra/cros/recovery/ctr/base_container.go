// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ctr

import (
	"context"
	"fmt"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/recovery/internal/log"
)

// BaseContainer describe API to work with container manipulation.
type BaseContainer interface {
	Name() string
	Exec(ctx context.Context, cmd string) (outstd, errstd string, err error)
	Stop(ctx context.Context, doItQuite bool) error
	IsClosed() bool
}
type baseContainerImpl struct {
	name     string
	isCLosed bool
	ci       *infoImpl
}

// Name returns the name of container.
func (c *baseContainerImpl) Name() string {
	return c.name
}

// Exec executes docker command command for the container.
func (c *baseContainerImpl) Exec(ctx context.Context, cmd string) (outstd, errstd string, _ error) {
	return "", "", errors.Reason("run: not implemented").Err()
}

// Stop stops conrtainer and mark it as closed.
func (c *baseContainerImpl) Stop(ctx context.Context, doItQuite bool) error {
	if c.isCLosed || c.ci == nil || c.ci.ctr == nil {
		return nil
	}
	if err := c.ci.ctr.StopContainer(ctx, c.name); err != nil {
		if doItQuite {
			log.Debugf(ctx, "Stop container %q: %s", c.name, err)
		}
		return errors.Annotate(err, "stop container").Err()
	}
	c.isCLosed = true
	return nil
}

// IsClosed tells if the container was closed.
func (c *baseContainerImpl) IsClosed() bool {
	return c.isCLosed
}

type ContainerInfo struct {
	ImageName string
}

func (c *infoImpl) GetContainer(ctx context.Context, req *testapi.StartTemplatedContainerRequest) (_ BaseContainer, rErr error) {
	step, ctx := build.StartStep(ctx, fmt.Sprintf("Get container %s", req.GetName()))
	defer func() { step.End(rErr) }()
	if req.GetName() == "" {
		return nil, errors.Reason("get container: invalid request").Err()
	} else if c.ctr.CtrClient == nil {
		return nil, errors.Reason("get container: ctr-client not found, probably server is not started").Err()
	}
	if existContainer, ok := c.containerCache[req.GetName()]; ok && !existContainer.IsClosed() {
		log.Infof(ctx, "Got container %q from cache!", existContainer.Name())
		return existContainer, nil
	}
	container := &baseContainerImpl{
		isCLosed: false,
		name:     req.GetName(),
		ci:       c,
	}
	req.ArtifactDir = c.artifactsDir
	res, err := c.ctr.StartTemplatedContainer(ctx, req)
	if err != nil {
		return nil, errors.Annotate(err, "get container %q", container.name).Err()
	}
	c.containerCache[container.name] = container
	log.Infof(ctx, "Container %q started: %v", container.name, res)
	return container, nil
}
