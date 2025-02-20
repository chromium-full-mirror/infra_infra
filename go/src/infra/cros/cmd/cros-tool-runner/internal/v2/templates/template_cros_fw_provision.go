// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package templates

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/v2/commands"
)

type crosFwProvisionProcessor struct {
	cmdExecutor           cmdExecutor
	defaultServerPort     string // Default port used in cros-fw-provision
	dockerArtifactDirName string // Path on the drone where service put the logs by default
}

func newCrosFwProvisionProcessor() *crosFwProvisionProcessor {
	return &crosFwProvisionProcessor{
		cmdExecutor:           &commands.ContextualExecutor{},
		defaultServerPort:     "8080",
		dockerArtifactDirName: "/tmp/cros-fw-provision",
	}
}

func (p *crosFwProvisionProcessor) Process(request *api.StartTemplatedContainerRequest) (*api.StartContainerRequest, error) {
	t := request.GetTemplate().GetCrosFwProvision()
	if t == nil {
		return nil, status.Error(codes.Internal, "unable to process")
	}

	port := portZero
	expose := make([]string, 0)
	if request.Network != hostNetworkName {
		port = p.defaultServerPort
		expose = append(expose, port)
	}

	volume := fmt.Sprintf("%s:%s", request.ArtifactDir, p.dockerArtifactDirName)
	additionalOptions := &api.StartContainerRequest_Options{
		Network: request.Network,
		Expose:  expose,
		Volume:  []string{volume},
	}

	startCommand := []string{
		"cros-fw-provision",
		"server",
		"-port", port,
		"-log-path", p.dockerArtifactDirName,
	}
	return &api.StartContainerRequest{Name: request.Name, ContainerImage: request.ContainerImage, AdditionalOptions: additionalOptions, StartCommand: startCommand}, nil
}

func (p *crosFwProvisionProcessor) discoverPort(request *api.StartTemplatedContainerRequest) (*api.Container_PortBinding, error) {
	// delegate to default impl.
	return defaultDiscoverPort(p.cmdExecutor, request)
}
