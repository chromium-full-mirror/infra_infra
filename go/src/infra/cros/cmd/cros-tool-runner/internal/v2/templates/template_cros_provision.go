// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package templates

import (
	"fmt"
	"log"
	"path"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/v2/commands"
)

type crosProvisionProcessor struct {
	placeholderPopulator  placeholderPopulator
	cmdExecutor           cmdExecutor
	defaultServerPort     string // Default port used in cros-provision
	dockerArtifactDirName string // Path on the drone where service put the logs by default
	inputFileName         string // File in artifact dir to be passed to cros-provision
}

func newCrosProvisionProcessor() *crosProvisionProcessor {
	return &crosProvisionProcessor{
		placeholderPopulator:  newPopulatorRouter(),
		cmdExecutor:           &commands.ContextualExecutor{},
		defaultServerPort:     "80",
		dockerArtifactDirName: "/tmp/provisionservice",
		inputFileName:         "in.json",
	}
}

func (p *crosProvisionProcessor) Process(request *api.StartTemplatedContainerRequest) (*api.StartContainerRequest, error) {
	t := request.GetTemplate().GetCrosProvision()
	if t == nil {
		return nil, status.Error(codes.Internal, "unable to process")
	}

	volume := fmt.Sprintf("%s:%s", request.ArtifactDir, p.dockerArtifactDirName)
	port := portZero
	expose := make([]string, 0)
	additionalOptions := &api.StartContainerRequest_Options{
		Network: request.Network,
		Expose:  expose,
		Volume:  []string{volume},
	}
	startCommand := []string{
		"cros-provision",
		"server",
		"-metadata", path.Join(p.dockerArtifactDirName, p.inputFileName), // input file flag for cros-provision v2 is metadata
		"-port", port,
	}
	p.processPlaceholders(request)
	err := p.writeInputFile(request)
	if err != nil {
		return nil, err
	}
	return &api.StartContainerRequest{Name: request.Name, ContainerImage: request.ContainerImage, AdditionalOptions: additionalOptions, StartCommand: startCommand}, nil
}

func (p *crosProvisionProcessor) discoverPort(request *api.StartTemplatedContainerRequest) (*api.Container_PortBinding, error) {
	// delegate to default impl, any template-specific logic should be implemented here.
	return defaultDiscoverPort(p.cmdExecutor, request)
}

func (p *crosProvisionProcessor) processPlaceholders(request *api.StartTemplatedContainerRequest) {
	t := request.GetTemplate().GetCrosProvision()
	if t.InputRequest.DutServer == nil {
		return
	}
	populatedDutServer, err := p.placeholderPopulator.populate(t.InputRequest.DutServer)
	if err != nil {
		log.Printf("warning: error %v when processing dut server placeholder %v"+
			" in cros-provision input request, skipping to process template as is",
			err, t.InputRequest.DutServer)
		return
	}
	t.InputRequest.DutServer = populatedDutServer
}

func (p *crosProvisionProcessor) writeInputFile(request *api.StartTemplatedContainerRequest) error {
	t := request.GetTemplate().GetCrosProvision()
	filePath := path.Join(request.ArtifactDir, p.inputFileName)
	return TemplateUtils.writeToFile(filePath, t.InputRequest)
}
