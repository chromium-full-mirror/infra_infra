// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package templates

import (
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	labApi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

func TestCrosDutPopulate(t *testing.T) {
	processor := newCrosDutProcessor()
	request := getCrosDutTemplateRequest("mynet")

	convertedRequest, err := processor.Process(request)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	check(t, convertedRequest.Name, request.Name)
	check(t, convertedRequest.ContainerImage, request.ContainerImage)
	check(t, convertedRequest.AdditionalOptions.Network, "mynet")
	check(t, convertedRequest.AdditionalOptions.Volume[0], "/tmp:/tmp/cros-dut")
	check(t, convertedRequest.StartCommand[0], "cros-dut")
}

func TestCrosDutDiscoverPort_errorPropagated(t *testing.T) {
	processor := newCrosDutProcessor()
	request := getCrosDutTemplateRequest("mynet")
	_, err := processor.discoverPort(request)

	if err == nil {
		t.Fatalf("Expected error")
	}
}

func TestCrosDutPopulate_hostNetwork(t *testing.T) {
	processor := newCrosDutProcessor()
	request := getCrosDutTemplateRequest("host")

	convertedRequest, err := processor.Process(request)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	check(t, convertedRequest.Name, request.Name)
	check(t, convertedRequest.ContainerImage, request.ContainerImage)
	check(t, convertedRequest.AdditionalOptions.Network, "host")
	check(t, len(convertedRequest.AdditionalOptions.Expose), 0)
	check(t, convertedRequest.AdditionalOptions.Volume[0], "/tmp:/tmp/cros-dut")
	check(t, convertedRequest.StartCommand[0], "cros-dut")
	check(t, convertedRequest.StartCommand[len(convertedRequest.StartCommand)-1],
		"0")
}

func getCrosDutTemplateRequest(network string) *api.StartTemplatedContainerRequest {
	return &api.StartTemplatedContainerRequest{
		Name:           "my-container",
		ContainerImage: "gcr.io/image:123",
		Network:        network,
		ArtifactDir:    "/tmp",
		Template: &api.Template{
			Container: &api.Template_CrosDut{
				CrosDut: &api.CrosDutTemplate{
					CacheServer: &labApi.IpEndpoint{Address: "192.168.1.5", Port: 33},
					DutAddress:  &labApi.IpEndpoint{Address: "chromeos6-row4-rack5-host14", Port: 22},
				}}}}
}
