// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package server

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"

	"infra/cros/cmd/cros-tool-runner/internal/v2/commands"
)

func getService(executor CommandExecutor) *ContainerServerImpl {
	return &ContainerServerImpl{
		executor: executor,
	}
}

type mockExecutor struct {
	commandsExecuted     []string          // stores command type names. e.g. *commands.DockerRun
	commandsToThrowError map[string]string // tells executor to throw error. key is command type name, value is error message in stderr
}

func (m *mockExecutor) Execute(ctx context.Context, cmd commands.Command) (string, string, error) {
	cmdType := reflect.TypeOf(cmd).String()
	if errMsg, ok := m.commandsToThrowError[cmdType]; ok {
		return "", errMsg, errors.New(errMsg)
	}

	m.commandsExecuted = append(m.commandsExecuted, cmdType)
	return fmt.Sprintf("mockExecutor: %s executed", cmdType), "", nil
}

func TestCreateNetwork_invalidArgument_missingName(t *testing.T) {
	service := getService(&mockExecutor{})
	_, err := service.CreateNetwork(context.Background(), &api.CreateNetworkRequest{})
	if err == nil {
		t.Fatalf("Expect invalidArgument error")
	}
}

func TestCreateNetwork_cannotRetrieveId(t *testing.T) {
	errorMapping := make(map[string]string)
	errorMapping["*commands.NetworkList"] = "Some unknown error"
	executor := mockExecutor{commandsToThrowError: errorMapping}
	service := getService(&executor)
	_, err := service.CreateNetwork(context.Background(), &api.CreateNetworkRequest{Name: "mynet"})
	if err == nil {
		t.Fatalf("Expect notFound error")
	}
}

func TestCreateNetwork_success(t *testing.T) {
	executor := mockExecutor{}
	service := getService(&executor)
	_, err := service.CreateNetwork(context.Background(), &api.CreateNetworkRequest{Name: "mynet"})
	if err != nil {
		t.Fatalf("Expect success")
	}
	if len(executor.commandsExecuted) != 2 {
		t.Fatalf("Expect 2 commands have been executed")
	}
	if executor.commandsExecuted[0] != "*commands.NetworkCreate" {
		t.Fatalf("Expect network create have been executed")
	}
	if executor.commandsExecuted[1] != "*commands.NetworkList" {
		t.Fatalf("Expect network list have been executed")
	}
}

func TestStartContainer_invalidArgument_missingName(t *testing.T) {
	service := getService(&mockExecutor{})
	_, err := service.StartContainer(context.Background(), &api.StartContainerRequest{})
	if err == nil {
		t.Fatalf("Expect invalidArgument error")
	}
}

func TestStartContainer_invalidArgument_missingContainerImage(t *testing.T) {
	service := getService(&mockExecutor{})
	_, err := service.StartContainer(context.Background(), &api.StartContainerRequest{Name: "my-container"})
	if err == nil {
		t.Fatalf("Expect invalidArgument error")
	}
}

func TestStartContainer_invalidArgument_missingStartCommand(t *testing.T) {
	service := getService(&mockExecutor{})
	_, err := service.StartContainer(context.Background(), &api.StartContainerRequest{
		Name:           "my-container",
		ContainerImage: "us-docker.pkg.dev/cros-registry/test-services/cros-dut:8811903382633993457",
	})
	if err == nil {
		t.Fatalf("Expect invalidArgument error")
	}
}

func TestStartContainer_invalidPort_multiple(t *testing.T) {
	service := getService(&mockExecutor{})
	_, err := service.StartContainer(context.Background(), &api.StartContainerRequest{
		Name:              "my-container",
		ContainerImage:    "us-docker.pkg.dev/cros-registry/test-services/cros-dut:8811903382633993457",
		StartCommand:      []string{"cros-dut"},
		AdditionalOptions: &api.StartContainerRequest_Options{Expose: []string{"80", "90"}},
	})
	if err == nil {
		t.Fatalf("Expect unimpelemented error")
	}
}

func TestStartContainer_invalidPort_range(t *testing.T) {
	service := getService(&mockExecutor{})
	_, err := service.StartContainer(context.Background(), &api.StartContainerRequest{
		Name:              "my-container",
		ContainerImage:    "us-docker.pkg.dev/cros-registry/test-services/cros-dut:8811903382633993457",
		StartCommand:      []string{"cros-dut"},
		AdditionalOptions: &api.StartContainerRequest_Options{Expose: []string{"80-90"}},
	})
	if err == nil {
		t.Fatalf("Expect unimpelemented error")
	}
}

func TestStartContainer_emptyExpose_passValidation(t *testing.T) {
	service := getService(&mockExecutor{})
	_, err := service.StartContainer(context.Background(), &api.StartContainerRequest{
		Name:              "my-container",
		ContainerImage:    "us-docker.pkg.dev/cros-registry/test-services/cros-dut:8811903382633993457",
		StartCommand:      []string{"cros-dut"},
		AdditionalOptions: &api.StartContainerRequest_Options{Expose: []string{}},
	})
	if err != nil {
		t.Fatalf("Unexpected error")
	}
}

func TestStartContainer_nilExpose_passValidation(t *testing.T) {
	service := getService(&mockExecutor{})
	_, err := service.StartContainer(context.Background(), &api.StartContainerRequest{
		Name:              "my-container",
		ContainerImage:    "us-docker.pkg.dev/cros-registry/test-services/cros-dut:8811903382633993457",
		StartCommand:      []string{"cros-dut"},
		AdditionalOptions: &api.StartContainerRequest_Options{Expose: nil},
	})
	if err != nil {
		t.Fatalf("Unexpected error")
	}
}

func checkExecutedCommands(commands []string, executor *mockExecutor, t *testing.T) {
	if c := len(executor.commandsExecuted); c != len(commands) {
		t.Fatalf("Expect %d commands has been executed, but got %d", len(commands), c)
	}
	for i, command := range commands {
		if got := executor.commandsExecuted[i]; got != command {
			t.Fatalf("Expect execution of %q but executed %q", command, got)
		}
	}
}

func TestStartContainer_pullError_ignored(t *testing.T) {
	errorMapping := make(map[string]string)
	errorMapping["*commands.DockerPull"] = "Permission \"artifactregistry.repositories.downloadArtifacts\" denied on resource"
	executor := mockExecutor{commandsToThrowError: errorMapping}
	service := getService(&executor)
	_, err := service.StartContainer(context.Background(), &api.StartContainerRequest{
		Name:           "my-container",
		ContainerImage: "us-docker.pkg.dev/cros-registry/test-services/cros-dut:8811903382633993457",
		StartCommand:   []string{"cros-dut"},
	})
	if err != nil {
		t.Fatalf("Expect pull error to be ignored")
	}
	commands := []string{
		"*commands.DockerStop",
		"*commands.DockerRemove",
		"*commands.DockerRun",
	}
	checkExecutedCommands(commands, &executor, t)
}
func TestStartContainer_stopError_ignored(t *testing.T) {
	errorMapping := make(map[string]string)
	errorMapping["*commands.DockerStop"] = "some error"
	executor := mockExecutor{commandsToThrowError: errorMapping}
	service := getService(&executor)
	_, err := service.StartContainer(context.Background(), &api.StartContainerRequest{
		Name:           "my-container",
		ContainerImage: "us-docker.pkg.dev/cros-registry/test-services/cros-dut:8811903382633993457",
		StartCommand:   []string{"cros-dut"},
	})
	if err != nil {
		t.Fatalf("Expect stop error to be ignored")
	}
	commands := []string{
		"*commands.DockerPull",
		"*commands.DockerRemove",
		"*commands.DockerRun",
	}
	checkExecutedCommands(commands, &executor, t)
}

func TestStartContainer_removeError_ignored(t *testing.T) {
	errorMapping := make(map[string]string)
	errorMapping["*commands.DockerRemove"] = "some error"
	executor := mockExecutor{commandsToThrowError: errorMapping}
	service := getService(&executor)
	_, err := service.StartContainer(context.Background(), &api.StartContainerRequest{
		Name:           "my-container",
		ContainerImage: "us-docker.pkg.dev/cros-registry/test-services/cros-dut:8811903382633993457",
		StartCommand:   []string{"cros-dut"},
	})
	if err != nil {
		t.Fatalf("Expect stop error to be ignored")
	}
	commands := []string{
		"*commands.DockerPull",
		"*commands.DockerStop",
		"*commands.DockerRun",
	}
	checkExecutedCommands(commands, &executor, t)
}

func TestStartContainer_runError(t *testing.T) {
	errorMapping := make(map[string]string)
	errorMapping["*commands.DockerRun"] = "Some unknown error"
	executor := mockExecutor{commandsToThrowError: errorMapping}
	service := getService(&executor)
	_, err := service.StartContainer(context.Background(), &api.StartContainerRequest{
		Name:           "my-container",
		ContainerImage: "us-docker.pkg.dev/cros-registry/test-services/cros-dut:8811903382633993457",
		StartCommand:   []string{"cros-dut"},
	})
	if err == nil {
		t.Fatalf("Expect unknown error")
	}
	commands := []string{
		"*commands.DockerPull",
		"*commands.DockerStop",
		"*commands.DockerRemove",
	}
	checkExecutedCommands(commands, &executor, t)
}

func TestStartContainer_success(t *testing.T) {
	executor := mockExecutor{}
	service := getService(&executor)
	_, err := service.StartContainer(context.Background(), &api.StartContainerRequest{
		Name:           "my-container",
		ContainerImage: "us-docker.pkg.dev/cros-registry/test-services/cros-dut:8811903382633993457",
		StartCommand:   []string{"cros-dut"},
	})
	if err != nil {
		t.Fatalf("Expect success")
	}
	commands := []string{
		"*commands.DockerPull",
		"*commands.DockerStop",
		"*commands.DockerRemove",
		"*commands.DockerRun",
	}
	checkExecutedCommands(commands, &executor, t)
}

func TestStackCommands(t *testing.T) {
	executor := mockExecutor{}
	service := getService(&executor)
	_, err := service.StackCommands(context.Background(), &api.StackCommandsRequest{
		Requests: []*api.StackCommandsRequest_Stackable{
			{
				Command: &api.StackCommandsRequest_Stackable_CreateNetwork{
					CreateNetwork: &api.CreateNetworkRequest{Name: "bridge2"},
				},
			},
			{
				Command: &api.StackCommandsRequest_Stackable_StartContainer{
					StartContainer: &api.StartContainerRequest{
						Name:           "my-container",
						ContainerImage: "us-docker.pkg.dev/cros-registry/test-services/cros-dut:8811903382633993457",
						StartCommand:   []string{"cros-dut"},
					},
				},
			},
		}})
	if err != nil {
		t.Fatalf("Expect success")
	}
	commands := []string{
		"*commands.NetworkCreate",
		"*commands.NetworkList",
		"*commands.DockerPull",
		"*commands.DockerStop",
		"*commands.DockerRemove",
		"*commands.DockerRun",
	}
	checkExecutedCommands(commands, &executor, t)
}

func TestLoginRegistry_withActualTokenValue(t *testing.T) {
	executor := mockExecutor{}
	service := getService(&executor)
	_, err := service.LoginRegistry(context.Background(), &api.LoginRegistryRequest{
		Username: "oauth2accesstoken",
		Password: "someGibberishTOkEnVaLUe",
		Registry: "gcr.io",
	})
	if err != nil {
		t.Fatalf("Expect success")
	}
	commands := []string{
		"*commands.DockerLogin",
	}
	checkExecutedCommands(commands, &executor, t)
}

func TestLoginRegistry_withCommandSubstitution(t *testing.T) {
	executor := mockExecutor{}
	service := getService(&executor)
	_, err := service.LoginRegistry(context.Background(), &api.LoginRegistryRequest{
		Username: "oauth2accesstoken",
		Password: "$(gcloud auth print-access-token)",
		Registry: "gcr.io",
	})
	if err != nil {
		t.Fatalf("Expect success")
	}
	commands := []string{
		"*commands.GcloudAuthTokenPrint",
		"*commands.DockerLogin",
	}
	checkExecutedCommands(commands, &executor, t)
}

func TestLoginRegistry_withExtension(t *testing.T) {
	executor := mockExecutor{}
	service := getService(&executor)
	_, err := service.LoginRegistry(context.Background(), &api.LoginRegistryRequest{
		Username: "oauth2accesstoken",
		Password: "$(gcloud auth print-access-token)",
		Registry: "gcr.io",
		Extensions: &api.LoginRegistryExtensions{
			GcloudAuthServiceAccountArgs: []string{"--key-file=path/to/key.json"},
		},
	})
	if err != nil {
		t.Fatalf("Expect success")
	}
	commands := []string{
		"*commands.GcloudAuthServiceAccount",
		"*commands.GcloudAuthTokenPrint",
		"*commands.DockerLogin",
	}
	checkExecutedCommands(commands, &executor, t)
}

func TestLoginRegistry_withExtensionError(t *testing.T) {
	errorMapping := make(map[string]string)
	errorMapping["*commands.GcloudAuthServiceAccount"] = "Some unknown error"
	executor := mockExecutor{commandsToThrowError: errorMapping}
	service := getService(&executor)
	response, err := service.LoginRegistry(context.Background(), &api.LoginRegistryRequest{
		Username: "oauth2accesstoken",
		Password: "$(gcloud auth print-access-token)",
		Registry: "gcr.io",
		Extensions: &api.LoginRegistryExtensions{
			GcloudAuthServiceAccountArgs: []string{"--key-file=path/to/key.json"},
		},
	})
	if err != nil {
		t.Fatalf("Expect success")
	}
	if response.ExtensionsOutput[0] != "Some unknown error" {
		t.Fatalf("Expect extension error output to be in the response %v %v", response.ExtensionsOutput, executor.commandsExecuted)
	}
	if response.Message == "" {
		t.Fatalf("Expect message in the response")
	}
}
