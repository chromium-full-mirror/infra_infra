// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package templates

import (
	"fmt"
	"os"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/config/go/test/api"

	"infra/cros/cmd/cros-tool-runner/internal/v2/commands"
	"infra/cros/internal/env"
)

type crosDutProcessor struct {
	cmdExecutor           cmdExecutor
	defaultServerPort     string // Default port used in cros-provision
	dockerArtifactDirName string // Path on the drone where service put the logs by default
}

func newCrosDutProcessor() *crosDutProcessor {
	return &crosDutProcessor{
		cmdExecutor:           &commands.ContextualExecutor{},
		defaultServerPort:     "80",
		dockerArtifactDirName: "/tmp/cros-dut",
	}
}

func (p *crosDutProcessor) Process(request *api.StartTemplatedContainerRequest) (*api.StartContainerRequest, error) {
	t := request.GetTemplate().GetCrosDut()
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
		Env:     additionalEnvs(),
	}
	// Add cloudbots related options
	if env.IsCloudBot() {
		cloudbotsOptions := cloudbotsAdditionalOptions()
		additionalOptions.Volume = append(additionalOptions.Volume, cloudbotsOptions.Volume...)
		additionalOptions.Env = append(additionalOptions.Env, cloudbotsOptions.Env...)
	}
	startCommand := []string{
		"cros-dut",
		"-dut_address", TemplateUtils.endpointToAddress(t.DutAddress),
		"-cache_address", TemplateUtils.endpointToAddress(t.CacheServer),
		"-port", port,
	}
	return &api.StartContainerRequest{Name: request.Name, ContainerImage: request.ContainerImage, AdditionalOptions: additionalOptions, StartCommand: startCommand}, nil
}

func (p *crosDutProcessor) discoverPort(request *api.StartTemplatedContainerRequest) (*api.Container_PortBinding, error) {
	// delegate to default impl, any template-specific logic should be implemented here.
	return defaultDiscoverPort(p.cmdExecutor, request)
}

func additionalEnvs() []string {
	var env []string
	bbidEnv := os.Getenv("LOGDOG_STREAM_PREFIX")
	if bbidEnv != "" {
		s := strings.Split(bbidEnv, "/")
		bbid := s[len(s)-1]
		env = append(env, fmt.Sprintf("BUILD_BUCKET_ID=%s", bbid))
	}

	swarmingTaskID := os.Getenv("SWARMING_TASK_ID")
	if swarmingTaskID != "" {
		env = append(env, fmt.Sprintf("SWARMING_TASK_ID=%s", swarmingTaskID))
	}
	return env
}

func cloudbotsAdditionalOptions() *api.StartContainerRequest_Options {
	o := &api.StartContainerRequest_Options{
		Volume: []string{},
		Env: []string{
			fmt.Sprintf("SWARMING_BOT_ID=%s", os.Getenv("SWARMING_BOT_ID")),
		},
	}
	// cloudbots environment variables
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "CLOUDBOTS_") {
			o.Env = append(o.Env, env)
		}
	}
	// cloudbots host files
	if v, found := os.LookupEnv("CLOUDBOTS_CA_CERTIFICATE"); found {
		o.Volume = append(o.Volume, fmt.Sprintf("%s:%s", v, v))
	}
	return o
}
