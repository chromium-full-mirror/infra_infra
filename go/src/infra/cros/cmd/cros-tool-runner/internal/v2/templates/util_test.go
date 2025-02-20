// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package templates

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	labApi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/v2/commands"
	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/v2/state"
)

func TestParsePortBindingString(t *testing.T) {
	original := "80/tcp -> 10.88.0.1:42222"
	expect := &api.Container_PortBinding{
		ContainerPort: 80,
		Protocol:      "tcp",
		HostIp:        "10.88.0.1",
		HostPort:      42222,
	}
	parsed, err := TemplateUtils.parsePortBindingString(original)
	if err != nil {
		t.Error(err.Error())
	}
	if parsed.String() != expect.String() {
		t.Errorf("Result doesn't match\nexpect: %v\nactual: %v", expect, parsed)
	}
}

func TestParseMultilinePortBindings(t *testing.T) {
	original := "80/tcp -> 10.88.0.1:42222\n81/tcp -> 0.0.0.0:42223"
	expect := []*api.Container_PortBinding{
		{ContainerPort: 80, Protocol: "tcp", HostIp: "10.88.0.1", HostPort: 42222},
		{ContainerPort: 81, Protocol: "tcp", HostIp: "0.0.0.0", HostPort: 42223},
	}
	parsed, err := TemplateUtils.parseMultilinePortBindings(original)
	if err != nil {
		t.Error(err.Error())
	}
	if len(parsed) != len(expect) || parsed[0].String() != expect[0].String() || parsed[1].String() != expect[1].String() {
		t.Errorf("Result doesn't match\nexpect: %v\nactual: %v", expect, parsed)
	}
}

func TestParseMultilinePortBindings_ipv6BindingIgnored(t *testing.T) {
	original := "80/tcp -> 0.0.0.0:42222\n80/tcp -> :::42222"
	expect := []*api.Container_PortBinding{
		{ContainerPort: 80, Protocol: "tcp", HostIp: "0.0.0.0", HostPort: 42222},
	}
	parsed, err := TemplateUtils.parseMultilinePortBindings(original)
	if err != nil {
		t.Error(err.Error())
	}
	if len(parsed) != len(expect) || parsed[0].String() != expect[0].String() {
		t.Errorf("Result doesn't match\nexpect: %v\nactual: %v", expect, parsed)
	}
}

func TestParseMultilinePortBindings_empty(t *testing.T) {
	original := "\n"
	parsed, err := TemplateUtils.parseMultilinePortBindings(original)
	if err != nil {
		t.Error(err.Error())
	}
	if len(parsed) != 0 {
		t.Errorf("Expect empty port bindings returned")
	}
}

func TestEndpointToAddress(t *testing.T) {
	endpoint := &labApi.IpEndpoint{
		Address: "xyz",
		Port:    123,
	}
	address := TemplateUtils.endpointToAddress(endpoint)
	if address != "xyz:123" {
		t.Errorf("Incorrect address conversion %s", address)
	}
}

func TestLookupContainerPortBindings_errorRetrievePort(t *testing.T) {
	util := templateUtils{
		cmdExecutor:    getMockCmdExecutorWithError("error retrieve port"),
		templateRouter: &RequestRouter{},
	}
	bindings, err := util.LookupContainerPortBindings("my-container")
	if err == nil {
		t.Errorf("Expect error")
	}
	if bindings != nil {
		t.Errorf("expect bindings to be nil")
	}
}

func TestLookupContainerPortBindings_hasDockerPortBinding(t *testing.T) {
	expect := []*api.Container_PortBinding{
		{ContainerPort: 80, Protocol: "tcp", HostIp: "0.0.0.0", HostPort: 42222},
	}
	util := templateUtils{
		cmdExecutor: &mockCmdExecutor{
			executeFunc: func(ctx context.Context, cmd commands.Command) (string, string, error) {
				return "80/tcp -> 0.0.0.0:42222\n80/tcp -> :::42222", "", nil
			},
		},
		templateRouter: &RequestRouter{},
	}
	bindings, err := util.LookupContainerPortBindings("my-container")
	check(t, err, nil)
	check(t, len(bindings), len(expect))
	check(t, bindings[0].String(), expect[0].String())
}

func TestLookupContainerPortBindings_fallbackToDiscoveryPortBinding(t *testing.T) {
	expect := []*api.Container_PortBinding{
		{ContainerPort: 4222, Protocol: "tcp", HostIp: "localhost", HostPort: 42222},
	}

	containerId := "my-container-id"
	state.ServerState.TemplateRequest.Add(containerId, &api.StartTemplatedContainerRequest{})
	util := templateUtils{
		cmdExecutor: &mockCmdExecutor{
			executeFunc: func(ctx context.Context, cmd commands.Command) (string, string, error) {
				cmdType := reflect.TypeOf(cmd).String()
				if cmdType == "*commands.ContainerPort" {
					return "\n", "", nil
				}
				if cmdType == "*commands.ContainerInspect" {
					return containerId, "", nil
				}
				return "", "", errors.New("unknown command")
			},
		},
		templateRouter: &mockTemplateProcessor{
			portDiscoverFunc: func(req *api.StartTemplatedContainerRequest) (*api.Container_PortBinding, error) {
				return expect[0], nil
			}},
	}

	bindings, err := util.LookupContainerPortBindings("my-container")
	check(t, err, nil)
	check(t, len(bindings), len(expect))
	check(t, bindings[0].String(), expect[0].String())
}
