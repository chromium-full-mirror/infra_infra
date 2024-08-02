// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servod

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/docker"
	ufspb "infra/unifiedfleet/api/v1/models/chromeos/lab"
)

// TestStartServodContainerStartsContainer tests for the execution of `docker.Start()` in various conditions and ensures we use the correct args to do so
func TestStartServodContainerStartsContainer(t *testing.T) {
	t.Parallel()

	type test struct {
		fc                    FakeDockerClient
		containerName         string
		expectContainerLaunch bool
	}

	tests := []test{
		{NewFakeDockerClient(false, false), "testContainer", true},                       // no errs, should launch
		{NewFakeDockerClient(true, false), "testContainer", false},                       // error when calling docker.Up()
		{NewFakeDockerClient(false, true), "testContainer", false},                       // error when calling docker.Start()
		{NewFakeDockerClient(false, false, "testContainer"), "testContainer", false},     // testContainer is already running
		{NewFakeDockerClient(false, false, "testContainer"), "differentContainer", true}, // a container, but not the container, is running
	}

	for _, tc := range tests {
		dockerArgs := buildServodContainerArgs(ServodContainerOptions{tc.containerName, "board", "model", "serial", true, ufspb.ServoSetupType_SERVO_SETUP_REGULAR, "", false, ""})

		startServodContainer(context.Background(), &tc.fc, tc.containerName, dockerArgs)
		if tc.fc.containerLaunched != tc.expectContainerLaunch {
			t.Errorf("Expected container launch: %t\nActual container launch: %t\n", tc.expectContainerLaunch, tc.fc.containerLaunched)
			t.Errorf("Testcase: %v", tc)
		}

		if tc.fc.containerLaunched {
			if diff := cmp.Diff(tc.fc.runningContainers[tc.containerName], *dockerArgs); diff != "" {
				t.Errorf("Container launched with unexpected args: %s", diff)
			}
		}
	}
}

// TestBuildServodDockerArgs tests that when we build the correct container args
func TestBuildServodDockerArgs(t *testing.T) {
	t.Setenv("SERVOD_CONTAINER_LABEL", "latest") // functionality under test relies on env

	type test struct {
		opts               ServodContainerOptions
		expectedDockerArgs *docker.ContainerArgs
	}

	tests := []test{
		{ServodContainerOptions{"test_container", "board", "model", "serial", false, ufspb.ServoSetupType_SERVO_SETUP_REGULAR, "", false, ""},
			&docker.ContainerArgs{
				Detached:     true,
				ImageName:    "us-docker.pkg.dev/chromeos-partner-moblab/common-core/servod:latest",
				PublishPorts: nil,
				ExposePorts:  nil,
				EnvVar:       []string{"BOARD=board", "MODEL=model", "SERIAL=serial", "PORT=9999"},
				Volumes:      []string{"/dev:/dev", "serial_log:/var/log/servod_9999/"},
				Network:      "default_satlab",
				Privileged:   true,
				Exec:         []string{"tail", "-f", "/dev/null"},
			},
		},
		{ServodContainerOptions{"test_container", "board2", "model2", "serial2", true, ufspb.ServoSetupType_SERVO_SETUP_REGULAR, "", false, ""},
			&docker.ContainerArgs{
				Detached:     true,
				ImageName:    "us-docker.pkg.dev/chromeos-partner-moblab/common-core/servod:latest",
				PublishPorts: nil,
				ExposePorts:  nil,
				EnvVar:       []string{"BOARD=board2", "MODEL=model2", "SERIAL=serial2", "PORT=9999"},
				Volumes:      []string{"/dev:/dev", "serial2_log:/var/log/servod_9999/"},
				Network:      "default_satlab",
				Privileged:   true,
				Exec:         []string{"bash", "/start_servod.sh"},
			},
		},
		{ServodContainerOptions{"test_container", "board", "model", "serial", false, ufspb.ServoSetupType_SERVO_SETUP_DUAL_V4, "", false, ""},
			&docker.ContainerArgs{
				Detached:     true,
				ImageName:    "us-docker.pkg.dev/chromeos-partner-moblab/common-core/servod:latest",
				PublishPorts: nil,
				ExposePorts:  nil,
				EnvVar:       []string{"BOARD=board", "MODEL=model", "SERIAL=serial", "PORT=9999", "DUAL_V4=1"},
				Volumes:      []string{"/dev:/dev", "serial_log:/var/log/servod_9999/"},
				Network:      "default_satlab",
				Privileged:   true,
				Exec:         []string{"tail", "-f", "/dev/null"},
			},
		},
		{ServodContainerOptions{"test_container", "board", "model", "serial", false, ufspb.ServoSetupType_SERVO_SETUP_DUAL_V4, "", true, ""},
			&docker.ContainerArgs{
				Detached:     true,
				ImageName:    "us-docker.pkg.dev/chromeos-partner-moblab/common-core/servod:latest",
				PublishPorts: nil,
				ExposePorts:  nil,
				EnvVar:       []string{"BOARD=board", "MODEL=model", "SERIAL=serial", "PORT=9999", "DUAL_V4=1", "REC_MODE=1"},
				Volumes:      []string{"/dev:/dev", "serial_log:/var/log/servod_9999/"},
				Network:      "default_satlab",
				Privileged:   true,
				Exec:         []string{"tail", "-f", "/dev/null"},
			},
		},
		{ServodContainerOptions{"test_container", "board", "model", "serial", false, ufspb.ServoSetupType_SERVO_SETUP_DUAL_V4, "", true, "hello"},
			&docker.ContainerArgs{
				Detached:     true,
				ImageName:    "us-docker.pkg.dev/chromeos-partner-moblab/common-core/servod:hello",
				PublishPorts: nil,
				ExposePorts:  nil,
				EnvVar:       []string{"BOARD=board", "MODEL=model", "SERIAL=serial", "PORT=9999", "DUAL_V4=1", "REC_MODE=1"},
				Volumes:      []string{"/dev:/dev", "serial_log:/var/log/servod_9999/"},
				Network:      "default_satlab",
				Privileged:   true,
				Exec:         []string{"tail", "-f", "/dev/null"},
			},
		},
		{ServodContainerOptions{"test_container", "board", "model", "serial", false, ufspb.ServoSetupType_SERVO_SETUP_DUAL_V4, "rex.xml", true, "hello"},
			&docker.ContainerArgs{
				Detached:     true,
				ImageName:    "us-docker.pkg.dev/chromeos-partner-moblab/common-core/servod:hello",
				PublishPorts: nil,
				ExposePorts:  nil,
				EnvVar:       []string{"BOARD=board", "MODEL=model", "SERIAL=serial", "PORT=9999", "CONFIG=rex.xml", "DUAL_V4=1", "REC_MODE=1"},
				Volumes:      []string{"/dev:/dev", "serial_log:/var/log/servod_9999/"},
				Network:      "default_satlab",
				Privileged:   true,
				Exec:         []string{"tail", "-f", "/dev/null"},
			},
		},
	}

	for _, tc := range tests {
		dockerArgs := buildServodContainerArgs(tc.opts)

		if diff := cmp.Diff(dockerArgs, tc.expectedDockerArgs); diff != "" {
			t.Errorf("Unexpected docker args created: %s", diff)
		}
	}
}

// Implementation of docker client. Allows specifying whether docker.Up or docker.Start calls will error
// Also keeps a map of "running" containers, and notes if a container was launched after initialization
type FakeDockerClient struct {
	runningContainers map[string]docker.ContainerArgs
	isUpErr           bool // whether the d.Up call errs
	startContainerErr bool // whether the d.Start call errs
	containerLaunched bool // whether a call to Start launched the container
}

// NewFakeDockerClient is the recommended way of creating a FakeDockerClient because it handles the map initialization in runningContainers
func NewFakeDockerClient(isUpErr bool, startContainerErr bool, containerNames ...string) FakeDockerClient {
	var fc FakeDockerClient

	fc.isUpErr = isUpErr
	fc.startContainerErr = startContainerErr
	fc.runningContainers = make(map[string]docker.ContainerArgs)

	for _, cn := range containerNames {
		fc.runningContainers[cn] = docker.ContainerArgs{}
	}

	return fc
}

// Fake implementation of `Start` which adds a container to `runningContainers` and sets `containerLaunched` to be true
// Will error if FakeClient.startContainerErr is true
func (fc *FakeDockerClient) Start(ctx context.Context, containerName string, req *docker.ContainerArgs, timeout time.Duration) (*docker.StartResponse, error) {
	if fc.startContainerErr {
		return nil, errors.Reason("Error during Start execution").Err()
	}

	fc.runningContainers[containerName] = *req
	fc.containerLaunched = true
	return &docker.StartResponse{}, nil
}

// Fake implementation of `IsUp` which uses the `runningContainers` map's keys to decide if a given container "isUp"
// Will error if FakeClient.isUpErr is true
func (fc *FakeDockerClient) IsUp(ctx context.Context, containerName string) (bool, error) {
	if fc.isUpErr {
		return false, errors.Reason("Error during IsUp execution").Err()
	}

	_, keyFoundOk := fc.runningContainers[containerName]
	return keyFoundOk, nil
}

// Fake implementation of `Remove` which simply attempts to remove `containerName` from `runningContainers`
func (fc *FakeDockerClient) Remove(ctx context.Context, containerName string, force bool) error {
	// docker rm with force will return a 0 exit code even if container doesnt exist so we do that
	// https://github.com/docker/cli/issues/2677
	delete(fc.runningContainers, containerName)
	return nil
}

// getContainerNames returns a slice of all currently "running" containers' names
func (fc *FakeDockerClient) getContainerNames() []string {
	keys := make([]string, 0, len(fc.runningContainers))
	for k := range fc.runningContainers {
		keys = append(keys, k)
	}
	return keys
}
