// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package containers

import (
	"context"
	"fmt"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	managers "go.chromium.org/infra/cros/cmd/cros_test_platformV2/docker_managers"
)

type TemplatedContainer struct {
	AbstractContainer

	StartTemplatedContainerReq *api.StartTemplatedContainerRequest
}

func NewGenericContainer(
	namePrefix string,
	containerImage string,
	ctr managers.ContainerManager) *TemplatedContainer {

	cont := &TemplatedContainer{AbstractContainer: NewAbstractContainer(namePrefix, containerImage, ctr)}
	cont.ConcreteContainer = cont
	return cont
}

// Initialize initializes the container.
func (cont *TemplatedContainer) Initialize(
	ctx context.Context,
	template *api.Template) error {

	err := cont.AbstractContainer.InitializeBase(ctx)
	if err != nil {
		return errors.Annotate(err, "initialization failed for base container: ").Err()
	}

	if template == nil {
		return fmt.Errorf("No template provided for templated container!")
	}
	switch t := template.Container.(type) {
	case *api.Template_Generic:
		if err = cont.initializeGenericTemplate(ctx, t.Generic); err != nil {
			return errors.Annotate(err, "initialization failed for generic template: ").Err()
		}
	default:
		return fmt.Errorf("Provided template %v not found!", t)

	}
	if cont.TempDirLoc == "" {
		return fmt.Errorf("TempDirLoc is empty but required for ArtifactDir")
	}

	cont.StartTemplatedContainerReq = &api.StartTemplatedContainerRequest{
		Name:           cont.Name,
		ContainerImage: cont.containerImage,
		Template:       template,
		Network:        common.ContainerDefaultNetwork,
		ArtifactDir:    cont.TempDirLoc}

	cont.state = ContainerStateInitialized

	return nil
}

func (cont *TemplatedContainer) initializeGenericTemplate(
	ctx context.Context,
	genericTemplate *api.GenericTemplate) error {

	if genericTemplate == nil {
		return fmt.Errorf("Provided GenericTemplate is nil!")
	}

	if genericTemplate.GetDockerArtifactDir() == "" {
		return fmt.Errorf("No docker artifact directory provided for generic template!")
	}

	if genericTemplate.GetBinaryArgs() == nil {
		return fmt.Errorf("No args provided for generic template")
	}

	return nil
}

// StartContainer starts the container.
func (cont *TemplatedContainer) StartContainer(ctx context.Context) (*api.StartContainerResponse, error) {
	if cont.StartTemplatedContainerReq == nil {
		return nil, fmt.Errorf("StartTemplatedContainerRequest is nil!")
	}
	if cont.ctr == nil {
		return nil, fmt.Errorf("Ctr is nil!")
	}
	var err error

	cont.StartContainerResp, err = cont.ctr.StartContainer(ctx, cont.StartTemplatedContainerReq)
	if err != nil {
		return nil, errors.Annotate(err, "error starting templated container: ").Err()
	}

	cont.state = ContainerStateStarted

	return cont.StartContainerResp, nil
}
