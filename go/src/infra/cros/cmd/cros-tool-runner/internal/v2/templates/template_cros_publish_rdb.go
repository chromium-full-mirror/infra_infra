// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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

const DockerRdbPublishLogsDir = "/tmp/rdb-publish/"
const DockerRdbPublishLuciContextDir = "/tmp/rdb-luci-context/"
const DockerRdbPublishServiceAcctsCredsDir = "/tmp/rdb-publish-service-creds/"
const DockerRdbLuciContextDir = "/tmp/rdb-luci-context/"
const DockerRdbPublishTestResultsDir = "/tmp/test/results" // Follows how cros_testexec stores test artifacts
const DockerRdbPublishPort = "43149"
const crosTestDirPrefix = "cros-test-"

const LuciContext = "LUCI_CONTEXT"

type crosRdbPublishProcessor struct {
	TemplateProcessor
	cmdExecutor                   cmdExecutor
	defaultServerPort             string // Default port used
	dockerArtifactDirName         string // Path on the docker where service put the logs by default
	dockerPublishLuciDirName      string // Path on the docker where publish src dir will be mounted to
	dockerServiceAcctCredsDirName string // Path on the docker where service accts dir will be mounted to
	dockerPublishSrcResultDirName string // Path on the docker where publish src result dir will be mounted to
}

func newCrosRdbPublishProcessor() *crosRdbPublishProcessor {
	return &crosRdbPublishProcessor{
		cmdExecutor:                   &commands.ContextualExecutor{},
		defaultServerPort:             DockerRdbPublishPort,
		dockerArtifactDirName:         DockerRdbPublishLogsDir,
		dockerPublishLuciDirName:      DockerRdbLuciContextDir,
		dockerServiceAcctCredsDirName: DockerRdbPublishServiceAcctsCredsDir,
		dockerPublishSrcResultDirName: DockerRdbPublishTestResultsDir,
	}
}

func (p *crosRdbPublishProcessor) Process(request *api.StartTemplatedContainerRequest) (*api.StartContainerRequest, error) {
	t := request.GetTemplate().GetCrosPublish()
	if t == nil {
		return nil, status.Error(codes.Internal, "unable to process")
	}

	volumes := []string{}
	envVars := []string{}

	volumes = append(volumes, fmt.Sprintf("%s:%s", request.GetArtifactDir(), p.dockerArtifactDirName))
	// Set LUCI_CONTEXT inside container
	if luciContextLoc, present := os.LookupEnv(LuciContext); present == true {
		luciContextParentDir := filepath.Dir(luciContextLoc)
		luciContextBase := filepath.Base(luciContextLoc)
		envVars = append(envVars, fmt.Sprintf("%s=%s", LuciContext, filepath.Join(p.dockerPublishLuciDirName, luciContextBase)))
		volumes = append(volumes, fmt.Sprintf("%s:%s", luciContextParentDir, p.dockerPublishLuciDirName))
	}
	if _, err := os.Stat(HostServiceAcctCredsDir); err == nil {
		volumes = append(volumes, fmt.Sprintf("%s:%s", HostServiceAcctCredsDir, p.dockerServiceAcctCredsDirName))
	}

	crosTestDir, err := common.FindDirWithPrefix(t.GetPublishSrcDir(), crosTestDirPrefix)
	if err != nil {
		fmt.Printf("Error finding cros-test dir: %v", err)
	} else {
		// All test result artifacts will be in <src_artifact_dir>/cros-test/results.
		resultDir := filepath.Join(crosTestDir, "cros-test", "results")
		volumes = append(volumes, fmt.Sprintf("%s:%s", resultDir, p.dockerPublishSrcResultDirName))
	}

	// Add GCE Metadata Server env vars.
	envVars = append(envVars, gceMetadataEnvVars()...)

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
		"rdb-publish",
		"server",
		"-port", port,
	}
	return &api.StartContainerRequest{Name: request.Name, ContainerImage: request.ContainerImage, AdditionalOptions: additionalOptions, StartCommand: startCommand}, nil
}

func (p *crosRdbPublishProcessor) discoverPort(request *api.StartTemplatedContainerRequest) (*api.Container_PortBinding, error) {
	// delegate to default impl, any template-specific logic should be implemented here.
	return defaultDiscoverPort(p.cmdExecutor, request)
}
