// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package containers

import (
	"context"
	"fmt"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/common_lib/tools/crostoolrunner"
)

// NonTemplatedContainer represents the cft non-templated container.
type NonTemplatedContainer struct {
	AbstractContainer

	LogsDirToMount string

	Network        string
	Volumes        []string
	Expose         []string
	EnvVars        []string
	StartCmd       []string
	TestResultsDir string

	StartContainerReq *api.StartContainerRequest
}

func NewNonTemplatedContainer(
	contType interfaces.ContainerType,
	namePrefix string,
	network string,
	containerImage string,
	ctr *crostoolrunner.CrosToolRunner) *NonTemplatedContainer {

	cont := &NonTemplatedContainer{AbstractContainer: NewAbstractContainer(contType, namePrefix, network, containerImage, ctr)}
	cont.ConcreteContainer = cont
	return cont
}

// Initialize initializes the container.
func (cont *NonTemplatedContainer) Initialize(
	ctx context.Context,
	template *api.Template) error {
	if template != nil {
		logging.Infof(ctx, "Warning: template provided for non-templated container. Will be ignored.")
	}

	err := cont.AbstractContainer.InitializeBase(ctx)
	if err != nil {
		return errors.Annotate(err, "initialization failed for base container: ").Err()
	}

	if cont.Network == "" {
		cont.Network = common.ContainerDefaultNetwork
	}

	if cont.LogsDirToMount != "" {
		cont.Volumes = append(cont.Volumes, fmt.Sprintf("%s:%s", cont.TempDirLoc, cont.LogsDirToMount))
	}

	additionalOptions := &api.StartContainerRequest_Options{
		Network: cont.Network,
		Expose:  cont.Expose,
		Volume:  cont.Volumes,
		Env:     cont.EnvVars,
	}

	cont.StartContainerReq = &api.StartContainerRequest{
		Name:              cont.Name,
		ContainerImage:    cont.containerImage,
		StartCommand:      cont.StartCmd,
		AdditionalOptions: additionalOptions}

	cont.state = ContainerStateInitialized
	return nil
}

// StartContainer starts the container.
func (cont *NonTemplatedContainer) StartContainer(ctx context.Context) (*api.StartContainerResponse, error) {
	if cont.StartContainerReq == nil {
		return nil, fmt.Errorf("startContainerRequest is nil")
	}
	if cont.ctr == nil {
		return nil, fmt.Errorf("ctr client is nil")
	}
	var err error
	cont.StartContainerResp, err = cont.ctr.StartContainer(ctx, cont.StartContainerReq)
	if err != nil {
		return nil, errors.Annotate(err, "error starting non-templated container: ").Err()
	}

	cont.state = ContainerStateStarted
	return cont.StartContainerResp, nil
}
