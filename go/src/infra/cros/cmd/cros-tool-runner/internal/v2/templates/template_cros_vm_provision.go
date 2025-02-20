// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package templates

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/v2/commands"
	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/v2/state"
)

const (
	authTokenFile = "/authToken.txt"
)

type crosVMProvisionProcessor struct {
	cmdExecutor           cmdExecutor
	defaultServerPort     string // Default port used in cros-vm-provision
	dockerArtifactDirName string // Path on the drone where service put the logs by default
}

func newCrosVMProvisionProcessor() *crosVMProvisionProcessor {
	return &crosVMProvisionProcessor{
		cmdExecutor:           &commands.ContextualExecutor{},
		defaultServerPort:     "80",
		dockerArtifactDirName: "/tmp/vm-provision",
	}
}

func (p *crosVMProvisionProcessor) Process(request *api.StartTemplatedContainerRequest) (*api.StartContainerRequest, error) {
	t := request.GetTemplate().GetCrosVmProvision()
	if t == nil {
		return nil, status.Error(codes.Internal, "unable to process")
	}

	port := portZero
	expose := make([]string, 0)
	if request.Network != hostNetworkName {
		port = p.defaultServerPort
		expose = append(expose, port)
	}

	err := generateAuthFile(request.ArtifactDir)
	if err != nil {
		return nil, err
	}

	volume := fmt.Sprintf("%s:%s", request.ArtifactDir, p.dockerArtifactDirName)
	additionalOptions := &api.StartContainerRequest_Options{
		Network: request.Network,
		Expose:  expose,
		Volume:  []string{volume},
	}

	startCommand := []string{
		"vm-provision",
		"-port", port,
		"-log", p.dockerArtifactDirName,
	}
	go authCopier(request.Name, request.ArtifactDir, p.dockerArtifactDirName)
	return &api.StartContainerRequest{Name: request.Name, ContainerImage: request.ContainerImage, AdditionalOptions: additionalOptions, StartCommand: startCommand}, nil
}

func (p *crosVMProvisionProcessor) discoverPort(request *api.StartTemplatedContainerRequest) (*api.Container_PortBinding, error) {
	// delegate to default impl.
	return defaultDiscoverPort(p.cmdExecutor, request)
}

// generateAuthFile generates file with auth token to be consumed by vm-provision
func generateAuthFile(dir string) error {

	//execute gcloud command to generate gcloud auth print-access-token
	gcloudcmd := &commands.GcloudAuthTokenPrint{}
	token, _, err := gcloudcmd.Execute(context.Background())
	if err != nil {
		return status.Error(codes.Internal, "unable to execute gcloud command for vm provision")
	}
	filepath := fmt.Sprintf("%s%s", dir, authTokenFile)

	// Create the file
	file, err := os.Create(filepath)
	if err != nil {
		return status.Error(codes.Internal, "unable to create token file for vm provision")
	}
	defer file.Close()

	// Write the token to the file
	_, err = file.WriteString(strings.TrimSpace(token))
	if err != nil {
		return status.Error(codes.Internal, "unable to write to token file for vm provision")
	}
	return nil
}

// authCopier generates auth and copies to vm-provision container. If it finds remove signal present, then it removes auth file and skips generating one.
func authCopier(name string, source string, destination string) {
	// The first auth token is generated and mounted at the container startup. The goroutine only generates
	// consequent tokens after 1 minute as long as remove signal doesn't exist.
	startTime := time.Now()
	authFilePath := fmt.Sprintf("%s%s", source, authTokenFile)
	authFileRemoveSignalPath := fmt.Sprintf("%s/%s", source, "removeSignal.txt")

	for {
		// Check if the auth file remove signal exists.
		if _, err := os.Stat(authFileRemoveSignalPath); err == nil {
			// Remove the auth file
			removeFile(authFilePath)
			break
		} else if time.Since(startTime) > 60*time.Second {
			containerID := state.ServerState.Containers.GetIdForOwner(name)
			if containerID == "" {
				log.Printf("vm-provision container not started yet")
			} else {
				// Generate and copy to vm-provision docker container
				if err := generateAuthFile(source); err != nil {
					log.Printf("Error generating auth for vm-provision during goroutine")
				}
				mountDockerFile(source, destination, containerID, authTokenFile)
			}
			startTime = time.Now()
		}
	}
}

// removeFile removes the file at given filepath
func removeFile(filePath string) {
	// Check if the file exists before attempting to remove it
	if _, err := os.Stat(filePath); err == nil {
		// Attempt to remove the file
		if err := os.Remove(filePath); err != nil {
			log.Printf("Failed to remove auth file: %s", err)
		} else {
			log.Printf("Successfully removed auth file.")
		}
	}
}

// mountDockerFile mounts a file to a given destination dir for a container ID
func mountDockerFile(source string, destination string, containerID string, file string) {
	cmd := &commands.DockerCp{Source: source + file, Destination: containerID + ":" + destination + file}
	_, _, err := cmd.Execute(context.Background())
	if err != nil {
		log.Printf("Failed to copy auth file for vm-provision during goroutine")
	} else {
		log.Printf("Successfully copied auth file for vm-provision during goroutine")
	}
}
