// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"strings"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
)

func TestDockerRun_noImage(t *testing.T) {
	dockerRun := DockerRun{
		&api.StartContainerRequest{},
	}
	_, err := dockerRun.compose()
	if err == nil {
		t.Fatalf("Exepected an error of missing image")
	}
}

func TestDockerRun_basic(t *testing.T) {
	image := "hello-world:latest"
	dockerRun := DockerRun{
		&api.StartContainerRequest{ContainerImage: image},
	}
	args, _ := dockerRun.compose()
	expect := "run -d --rm --cap-add=NET_RAW hello-world:latest timeout 30h"
	check(t, args, expect)
}

func TestDockerRun_withName(t *testing.T) {
	image := "hello-world:latest"
	name := "my-hello-world"
	dockerRun := DockerRun{
		&api.StartContainerRequest{ContainerImage: image, Name: name},
	}
	args, _ := dockerRun.compose()
	expect := "run -d --rm --cap-add=NET_RAW --name my-hello-world hello-world:latest timeout 30h"
	check(t, args, expect)
}

func TestDockerRun_withStartCommand(t *testing.T) {
	image := "cros-dut:latest"
	command := []string{"cros-dut", "-port 80"}
	dockerRun := DockerRun{
		&api.StartContainerRequest{ContainerImage: image, StartCommand: command},
	}
	args, _ := dockerRun.compose()
	expect := "run -d --rm --cap-add=NET_RAW cros-dut:latest timeout 30h cros-dut -port 80"
	check(t, args, expect)
}

func TestDockerRun_withNetwork(t *testing.T) {
	image := "cros-dut:latest"
	command := []string{"cros-dut", "-port 80"}
	network := "mynet"
	options := &api.StartContainerRequest_Options{Network: network}
	dockerRun := DockerRun{
		&api.StartContainerRequest{ContainerImage: image, StartCommand: command,
			AdditionalOptions: options},
	}
	args, _ := dockerRun.compose()
	expect := "run -d --rm --cap-add=NET_RAW --network mynet cros-dut:latest timeout 30h cros-dut -port 80"
	check(t, args, expect)
}

func TestDockerRun_withVolume(t *testing.T) {
	image := "cros-test:latest"
	command := []string{"cros-test"}
	options := &api.StartContainerRequest_Options{
		Volume: []string{
			"/tmp/build123/test:/cros-test",
		}}
	dockerRun := DockerRun{
		&api.StartContainerRequest{ContainerImage: image, StartCommand: command,
			AdditionalOptions: options},
	}
	args, _ := dockerRun.compose()
	expect := "run -d --rm --cap-add=NET_RAW --volume /tmp/build123/test:/cros-test cros-test:latest timeout 30h cros-test"
	check(t, args, expect)
}

func TestDockerRun_withEnv(t *testing.T) {
	image := "cros-dut:latest"
	command := []string{"cros-dut", "-port 80"}
	options := &api.StartContainerRequest_Options{Env: []string{"VAR1=1", "VAR2=2"}}
	dockerRun := DockerRun{
		&api.StartContainerRequest{ContainerImage: image, StartCommand: command,
			AdditionalOptions: options},
	}
	args, _ := dockerRun.compose()
	expect := "run -d --rm --cap-add=NET_RAW --env VAR1=1 --env VAR2=2 cros-dut:latest timeout 30h cros-dut -port 80"
	check(t, args, expect)
}

func check(t *testing.T, actual []string, expectStr string) {
	actualStr := strings.Join(actual, " ")
	if actualStr != expectStr {
		t.Fatalf("Arguments do not match.\nExpect: %s\nActual: %s\n", expectStr, actualStr)
	}
}
