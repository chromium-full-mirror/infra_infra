// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package templates

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/v2/commands"
)

type genericProcessor struct {
	cmdExecutor cmdExecutor
}

func newGenericProcessor() *genericProcessor {
	return &genericProcessor{
		cmdExecutor: &commands.ContextualExecutor{},
	}
}

func (p *genericProcessor) Process(request *api.StartTemplatedContainerRequest) (*api.StartContainerRequest, error) {
	t := request.GetTemplate().GetGeneric()
	if t == nil {
		return nil, status.Error(codes.Internal, "unable to process")
	}

	artifactVolume := fmt.Sprintf("%s:%s", request.ArtifactDir, t.DockerArtifactDir)
	volumes := []string{}
	volumes = append(volumes, artifactVolume)
	volumes = append(volumes, t.AdditionalVolumes...)
	envs := []string{}
	for _, env := range t.Env {
		// Check if environment variable is already set within the request
		// via the <NAME>=<VALUE> format.
		if len(strings.Split(env, "=")) == 2 {
			envs = append(envs, env)
		} else {
			// Check if environment variable exists.
			envValue := os.Getenv(env)
			if envValue != "" {
				envs = append(envs, env)
			}
		}
	}
	// Required for Satlab's Docker TLS daemon.
	// DOCKER_CERT_PATH is available only on DRONE thus in cannot be included
	// in StartTemplatedContainerRequest.
	if slices.Contains(envs, "DOCKER_CERT_PATH") {
		path := os.Getenv("DOCKER_CERT_PATH")
		volumes = append(volumes, fmt.Sprintf("%s:%s", path, path))
	}
	additionalOptions := &api.StartContainerRequest_Options{
		Network: request.Network,
		Expose:  t.Expose,
		Volume:  volumes,
		Env:     envs,
	}
	// Add cloudbots related options
	// TODO (cdelagarza): fix this before uncommenting.
	// if env.IsCloudBot() {
	//	cloudbotsOptions := cloudbotsAdditionalOptions()
	// additionalOptions.Volume = append(additionalOptions.Volume, cloudbotsOptions.Volume...)
	//	additionalOptions.Env = append(additionalOptions.Env, cloudbotsOptions.Env...)
	// }
	startCommand := []string{
		t.BinaryName,
	}
	startCommand = append(startCommand, t.BinaryArgs...)
	return &api.StartContainerRequest{Name: request.Name, ContainerImage: request.ContainerImage, AdditionalOptions: additionalOptions, StartCommand: startCommand}, nil
}

func (p *genericProcessor) discoverPort(request *api.StartTemplatedContainerRequest) (*api.Container_PortBinding, error) {
	// delegate to default impl, any template-specific logic should be implemented here.
	return defaultDiscoverPort(p.cmdExecutor, request)
}
