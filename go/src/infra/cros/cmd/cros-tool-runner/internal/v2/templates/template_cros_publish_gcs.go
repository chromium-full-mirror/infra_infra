// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package templates

import (
	"fmt"
	"os"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/v2/commands"
)

const DockerGcsPublishLogsDir = "/tmp/gcs-publish/"
const DockerGcsPublishServiceAcctsCredsDir = "/tmp/gcs-publish-service-creds/"
const DockerGcsPublishTestArtifactsDir = "/tmp/gcs-publish-test-artifacts/"
const DockerGcsPublishPort = "43147"

type crosGcsPublishProcessor struct {
	TemplateProcessor
	cmdExecutor                   cmdExecutor
	defaultServerPort             string // Default port used
	dockerArtifactDirName         string // Path on the docker where service put the logs by default
	dockerPublishSrcDirName       string // Path on the docker where publish src dir will be mounted to
	dockerServiceAcctCredsDirName string // Path on the docker where service accts dir will be mounted to
}

func newCrosGcsPublishProcessor() *crosGcsPublishProcessor {
	return &crosGcsPublishProcessor{
		cmdExecutor:                   &commands.ContextualExecutor{},
		defaultServerPort:             DockerGcsPublishPort,
		dockerArtifactDirName:         DockerGcsPublishLogsDir,
		dockerPublishSrcDirName:       DockerGcsPublishTestArtifactsDir,
		dockerServiceAcctCredsDirName: DockerGcsPublishServiceAcctsCredsDir,
	}
}

func (p *crosGcsPublishProcessor) Process(request *api.StartTemplatedContainerRequest) (*api.StartContainerRequest, error) {
	t := request.GetTemplate().GetCrosPublish()
	if t == nil {
		return nil, status.Error(codes.Internal, "unable to process")
	}
	volumes := []string{}
	volumes = append(volumes, fmt.Sprintf("%s:%s", request.GetArtifactDir(), p.dockerArtifactDirName))
	volumes = append(volumes, fmt.Sprintf("%s:%s", t.GetPublishSrcDir(), p.dockerPublishSrcDirName))
	if _, err := os.Stat(HostServiceAcctCredsDir); err == nil {
		volumes = append(volumes, fmt.Sprintf("%s:%s", HostServiceAcctCredsDir, p.dockerServiceAcctCredsDirName))
	}

	// Get GCE Metadata Server env vars
	envVars := gceMetadataEnvVars()

	port := portZero
	expose := make([]string, 0)
	if request.GetNetwork() != hostNetworkName {
		port = p.defaultServerPort
		expose = append(expose, port)
	}
	additionalOptions := &api.StartContainerRequest_Options{
		Network: request.Network,
		Expose:  expose,
		Volume:  volumes,
		Env:     envVars,
	}
	startCommand := []string{
		"gcs-publish",
		"server",
		"-port", port,
	}
	return &api.StartContainerRequest{Name: request.Name, ContainerImage: request.ContainerImage, AdditionalOptions: additionalOptions, StartCommand: startCommand}, nil
}

func (p *crosGcsPublishProcessor) discoverPort(request *api.StartTemplatedContainerRequest) (*api.Container_PortBinding, error) {
	// delegate to default impl, any template-specific logic should be implemented here.
	return defaultDiscoverPort(p.cmdExecutor, request)
}
