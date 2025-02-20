// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package templates

import (
	"fmt"
	"path"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/v2/commands"
)

const (
	DockerCacheServerPort    = "43150"
	DockerCacheServerLogsDir = "/tmp/cacheserver"
	googleAppCredsEnvVar     = "GOOGLE_APPLICATION_CREDENTIALS"
)

type cacheServerProcessor struct {
	TemplateProcessor
	cmdExecutor            cmdExecutor
	defaultServerPort      string // Default port used
	dockerArtifactDirName  string // Path on the docker where service put the logs by default
	serviceAccountMountDir string // Path on the docker where service account creds to be mounted
}

func newCacheServerProcessor() *cacheServerProcessor {
	return &cacheServerProcessor{
		cmdExecutor:            &commands.ContextualExecutor{},
		defaultServerPort:      DockerCacheServerPort,
		dockerArtifactDirName:  DockerCacheServerLogsDir,
		serviceAccountMountDir: "/usr/mount/creds",
	}
}

func (p *cacheServerProcessor) Process(request *api.StartTemplatedContainerRequest) (*api.StartContainerRequest, error) {
	t := request.GetTemplate().GetCacheServer()
	if t == nil {
		return nil, status.Error(codes.Internal, "unable to process")
	}
	volumeArtifact := fmt.Sprintf("%s:%s", request.ArtifactDir, p.dockerArtifactDirName)
	volumes := []string{volumeArtifact}

	exportEnvVarCommand := ""
	if t.GetServiceAccountKeyfile() != "" {
		dir, key := path.Split(t.GetServiceAccountKeyfile())
		if dir == "" || key == "" {
			return nil, status.Error(codes.InvalidArgument, "service account key file must be full path to the file")
		}
		volumeCreds := fmt.Sprintf("%s:%s", dir, p.serviceAccountMountDir)
		volumes = append(volumes, volumeCreds)
		exportEnvVarCommand = fmt.Sprintf("export %s=%s",
			googleAppCredsEnvVar, path.Join(p.serviceAccountMountDir, key))
	}

	port := portZero
	expose := make([]string, 0)
	if request.Network != hostNetworkName {
		port = p.defaultServerPort
		expose = append(expose, port)
	}
	additionalOptions := &api.StartContainerRequest_Options{
		Network: request.Network,
		Expose:  expose,
		Volume:  volumes,
	}
	startCommand := []string{
		"cacheserver",
		"-location", DockerCacheServerLogsDir,
		"-port", port,
	}
	if exportEnvVarCommand != "" {
		startCommand = []string{
			"bash", "-c",
			fmt.Sprintf("%s && %s", exportEnvVarCommand, strings.Join(startCommand, " ")),
		}
	}
	return &api.StartContainerRequest{Name: request.Name, ContainerImage: request.ContainerImage, AdditionalOptions: additionalOptions, StartCommand: startCommand}, nil
}

func (p *cacheServerProcessor) discoverPort(request *api.StartTemplatedContainerRequest) (*api.Container_PortBinding, error) {
	// delegate to default impl, any template-specific logic should be implemented here.
	return defaultDiscoverPort(p.cmdExecutor, request)
}
