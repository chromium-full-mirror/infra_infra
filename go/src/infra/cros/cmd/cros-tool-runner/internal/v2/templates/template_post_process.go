// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package templates defines the container templates
package templates

import (
	"fmt"
	"os"
	"path/filepath"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/config/go/test/api"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/cros-tool-runner/internal/v2/commands"
)

const DockerPostProcessLogsDir = "/tmp/post-process/"
const DockerPostProcessLuciContextDir = "/tmp/post-process-luci-context/"
const DockerPostProcessServiceAcctsCredsDir = "/tmp/post-process-service-creds/"
const DockerPostProcessPort = "43151"
const DockerPostProcessTestResultsDir = "/tmp/test/results" // Follows how cros_testexec stores test artifacts

// postProcessProcessor is the processor for post-process template.
type postProcessProcessor struct {
	TemplateProcessor
	cmdExecutor                       cmdExecutor
	defaultServerPort                 string // Default port used
	dockerArtifactDirName             string // Path on the docker where service put the logs by default
	dockerPostProcessLuciDirName      string // Path on the docker where post process src dir will be mounted to
	dockerServiceAcctCredsDirName     string // Path on the docker where service accts dir will be mounted to
	dockerPostProcessSrcResultDirName string // Path on the docker where post process src result dir will be mounted to
}

// newPostProcessProcessor creates a new postProcessProcessor.
func newPostProcessProcessor() *postProcessProcessor {
	return &postProcessProcessor{
		cmdExecutor:                       &commands.ContextualExecutor{},
		defaultServerPort:                 DockerPostProcessPort,
		dockerArtifactDirName:             DockerPostProcessLogsDir,
		dockerPostProcessLuciDirName:      DockerPostProcessLuciContextDir,
		dockerServiceAcctCredsDirName:     DockerPostProcessServiceAcctsCredsDir,
		dockerPostProcessSrcResultDirName: DockerPostProcessTestResultsDir,
	}
}

// Process processes the request to start up the templated container.
func (p *postProcessProcessor) Process(request *api.StartTemplatedContainerRequest) (*api.StartContainerRequest, error) {
	t := request.GetTemplate().GetPostProcess()
	if t == nil {
		return nil, status.Error(codes.Internal, "unable to process")
	}

	volumes := []string{}
	envVars := []string{}

	volumes = append(volumes, fmt.Sprintf("%s:%s", request.GetArtifactDir(), p.dockerArtifactDirName))
	// Set LUCI_CONTEXT inside container
	if luciContextLoc, present := os.LookupEnv(LuciContext); present {
		luciContextParentDir := filepath.Dir(luciContextLoc)
		luciContextBase := filepath.Base(luciContextLoc)
		envVars = append(envVars, fmt.Sprintf("%s=%s", LuciContext, filepath.Join(p.dockerPostProcessLuciDirName, luciContextBase)))
		volumes = append(volumes, fmt.Sprintf("%s:%s", luciContextParentDir, p.dockerPostProcessLuciDirName))
	}
	if _, err := os.Stat(HostServiceAcctCredsDir); err == nil {
		volumes = append(volumes, fmt.Sprintf("%s:%s", HostServiceAcctCredsDir, p.dockerServiceAcctCredsDirName))
	}

	crosTestDir, err := common.FindDirWithPrefix(t.GetPostProcessSrcDir(), CrosTestDirPrefix)
	if err != nil {
		fmt.Printf("Error finding cros-test dir: %v", err)
	} else {
		// All test result artifacts will be in <src_artifact_dir>/cros-test/results.
		resultDir := filepath.Join(crosTestDir, "cros-test", "results")
		volumes = append(volumes, fmt.Sprintf("%s:%s", resultDir, p.dockerPostProcessSrcResultDirName))
	}

	// Add GCE Metadata Server env vars.
	envVars = append(envVars, gceMetadataEnvVars()...)

	port := portZero
	expose := make([]string, 0)
	additionalOptions := &api.StartContainerRequest_Options{
		Network: request.Network,
		Expose:  expose,
		Volume:  volumes,
		Env:     envVars,
	}
	startCommand := []string{
		"post-process",
		"server",
		"-port", port,
	}
	return &api.StartContainerRequest{Name: request.Name, ContainerImage: request.ContainerImage, AdditionalOptions: additionalOptions, StartCommand: startCommand}, nil
}

// discoverPort discovers the port to the templated container.
func (p *postProcessProcessor) discoverPort(request *api.StartTemplatedContainerRequest) (*api.Container_PortBinding, error) {
	// delegate to default impl, any template-specific logic should be implemented here.
	return defaultDiscoverPort(p.cmdExecutor, request)
}
