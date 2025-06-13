// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package templates

import (
	"errors"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	labApi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

type mockPlaceholderPopulator struct {
	populateFunc func(*labApi.IpEndpoint) (*labApi.IpEndpoint, error)
}

func (m *mockPlaceholderPopulator) populate(endpoint *labApi.IpEndpoint) (*labApi.IpEndpoint, error) {
	return m.populateFunc(endpoint)
}

func newMockWithError() *mockPlaceholderPopulator {
	return &mockPlaceholderPopulator{
		populateFunc: func(endpoint *labApi.IpEndpoint) (*labApi.IpEndpoint, error) {
			return endpoint, errors.New("some error")
		}}
}

func newMockWithEndpoint(expect *labApi.IpEndpoint) *mockPlaceholderPopulator {
	return &mockPlaceholderPopulator{
		populateFunc: func(endpoint *labApi.IpEndpoint) (*labApi.IpEndpoint, error) {
			return expect, nil
		}}
}

func TestProcessPlaceholders(t *testing.T) {
	processor := newCrosProvisionProcessor()
	expect := &labApi.IpEndpoint{Address: "localhost", Port: 12345}
	processor.placeholderPopulator = newMockWithEndpoint(expect)
	request := &api.StartTemplatedContainerRequest{
		Template: &api.Template{
			Container: &api.Template_CrosProvision{
				CrosProvision: &api.CrosProvisionTemplate{
					InputRequest: &api.CrosProvisionRequest{
						DutServer: &labApi.IpEndpoint{Address: "ctr-host-port://dut-name", Port: 0},
					}}}}}

	processor.processPlaceholders(request)

	actual := request.Template.GetCrosProvision().InputRequest.DutServer
	if actual.Address != expect.Address || actual.Port != expect.Port {
		t.Fatalf("IpEndpoint wasn't populated  %s.", actual)
	}
}

func TestProcessPlaceholders_errorIgnored(t *testing.T) {
	processor := newCrosProvisionProcessor()
	expect := labApi.IpEndpoint{Address: "dut-name", Port: 0}
	processor.placeholderPopulator = newMockWithError()
	request := &api.StartTemplatedContainerRequest{
		Template: &api.Template{
			Container: &api.Template_CrosProvision{
				CrosProvision: &api.CrosProvisionTemplate{
					InputRequest: &api.CrosProvisionRequest{
						DutServer: &labApi.IpEndpoint{Address: "dut-name", Port: 0},
					}}}}}
	processor.processPlaceholders(request)

	actual := request.Template.GetCrosProvision().InputRequest.DutServer
	if actual.Address != expect.Address || actual.Port != expect.Port {
		t.Fatalf("IpEndpoint wasn't populated  %s.", actual)
	}
}

func getCrosProvisionTemplateRequest(network string) *api.StartTemplatedContainerRequest {
	return &api.StartTemplatedContainerRequest{
		Name:           "my-container",
		ContainerImage: "gcr.io/image:123",
		Network:        network,
		Template: &api.Template{
			Container: &api.Template_CrosProvision{
				CrosProvision: &api.CrosProvisionTemplate{}}}}
}
