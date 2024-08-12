// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows
// +build !windows

package templates

import (
	"testing"

	_go "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	labApi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

func TestCrosProvisionPopulate(t *testing.T) {
	processor := newCrosProvisionProcessor()
	expect := labApi.IpEndpoint{Address: "localhost", Port: 80}
	processor.placeholderPopulator = newMockWithEndpoint(&expect)
	request := getFullCrosProvisionTemplateRequest("mynet")

	convertedRequest, err := processor.Process(request)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	check(t, convertedRequest.Name, request.Name)
	check(t, convertedRequest.ContainerImage, request.ContainerImage)
	check(t, convertedRequest.AdditionalOptions.Network, "mynet")
	check(t, convertedRequest.AdditionalOptions.Volume[0], "/tmp:/tmp/provisionservice")
	check(t, convertedRequest.StartCommand[0], "cros-provision")
}

func TestCrosProvisionPopulate_hostNetwork(t *testing.T) {
	processor := newCrosProvisionProcessor()
	expect := labApi.IpEndpoint{Address: "localhost", Port: 80}
	processor.placeholderPopulator = newMockWithEndpoint(&expect)
	request := getFullCrosProvisionTemplateRequest("host")

	convertedRequest, err := processor.Process(request)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	check(t, convertedRequest.Name, request.Name)
	check(t, convertedRequest.ContainerImage, request.ContainerImage)
	check(t, convertedRequest.AdditionalOptions.Network, "host")
	check(t, len(convertedRequest.AdditionalOptions.Expose), 0)
	check(t, convertedRequest.AdditionalOptions.Volume[0], "/tmp:/tmp/provisionservice")
	check(t, convertedRequest.StartCommand[0], "cros-provision")
	check(t, convertedRequest.StartCommand[len(convertedRequest.StartCommand)-1],
		"0")
}

func getFullCrosProvisionTemplateRequest(network string) *api.StartTemplatedContainerRequest {
	return &api.StartTemplatedContainerRequest{
		Name:           "my-container",
		ContainerImage: "gcr.io/image:123",
		Network:        network,
		ArtifactDir:    "/tmp",
		Template: &api.Template{
			Container: &api.Template_CrosProvision{
				CrosProvision: &api.CrosProvisionTemplate{
					InputRequest: &api.CrosProvisionRequest{
						DutServer: &labApi.IpEndpoint{Address: "ctr-host-port://dut-name", Port: 0},
						Dut:       &labApi.Dut{Id: &labApi.Dut_Id{Value: "chromeos6-row4-rack5-host14"}},
						ProvisionState: &api.ProvisionState{SystemImage: &api.ProvisionState_SystemImage{
							SystemImagePath: &_go.StoragePath{
								Path:     "gs://chromeos-image-archive/kevin-cq/R104-14895.0.0-66173-8812350496939596961",
								HostType: _go.StoragePath_GS,
							}}},
					}}}}}
}
