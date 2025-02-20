// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package templates

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"

	"go.chromium.org/chromiumos/config/go/test/api"
	labApi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/v2/commands"
	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/v2/state"
)

// ContainerLookuper provides interface to lookup information for a container
type ContainerLookuper interface {
	LookupContainerPortBindings(name string) ([]*api.Container_PortBinding, error)
}

// cmdExecutor interfaces commands.ContextualExecutor
type cmdExecutor interface {
	Execute(ctx context.Context, cmd commands.Command) (string, string, error)
}

// templateUtils implements ContainerLookuper
type templateUtils struct {
	cmdExecutor    cmdExecutor
	templateRouter crosTemplate
}

var TemplateUtils = templateUtils{
	cmdExecutor:    &commands.ContextualExecutor{},
	templateRouter: &RequestRouter{},
}

// parsePortBindingString parses the output from `docker container port` command
// The input string example: `81/tcp -> 0.0.0.0:42223`
// Unsupported binding format (e.g. IPv6) is ignored, and both binding and error
// are returned as nil. Other unexpected errors will return error.
func (*templateUtils) parsePortBindingString(input string) (*api.Container_PortBinding, error) {
	r := regexp.MustCompile(`(?P<ContainerPort>\d+)/(?P<Protocol>\w+) -> (?P<HostIp>[\d\\.]+):(?P<HostPort>\d+)`)
	match := r.FindStringSubmatch(input)
	if match == nil {
		log.Printf("warning: ignore unrecognized port binding input %s", input)
		return nil, nil
	}
	containerPort, err := strconv.Atoi(match[1])
	if err != nil {
		return nil, err
	}
	hostPort, err := strconv.Atoi(match[4])
	if err != nil {
		return nil, err
	}
	return &api.Container_PortBinding{
		ContainerPort: int32(containerPort),
		Protocol:      match[2],
		HostIp:        match[3],
		HostPort:      int32(hostPort),
	}, nil
}

// parseMultilinePortBindings parses multiline output from `docker container
// port` command since Docker allows multiple ports to be published in one
// container. However, the CTRv2 server only allows one port to be published.
// Unsupported binding format (e.g. IPv6) is ignored.
func (u *templateUtils) parseMultilinePortBindings(multiline string) ([]*api.Container_PortBinding, error) {
	result := make([]*api.Container_PortBinding, 0)
	for _, line := range strings.Split(multiline, "\n") {
		if line == "" {
			continue
		}
		binding, err := u.parsePortBindingString(line)
		if err != nil {
			return result, err
		}
		if binding != nil {
			result = append(result, binding)
		}
	}
	return result, nil
}

func (u *templateUtils) retrieveContainerPortOutputFromCommand(name string) (string, error) {
	cmd := &commands.ContainerPort{Name: name}
	stdout, _, err := u.cmdExecutor.Execute(context.Background(), cmd)
	return strings.TrimSpace(stdout), err
}

// LookupContainerPortBindings is the API to get port bindings for a container
func (u *templateUtils) LookupContainerPortBindings(name string) ([]*api.Container_PortBinding, error) {
	output, err := u.retrieveContainerPortOutputFromCommand(name)
	if err != nil {
		return nil, err
	}
	bindings, err := u.parseMultilinePortBindings(output)
	if err != nil || len(bindings) > 0 {
		return bindings, err
	}
	// If bindings are empty, check port discovery.
	return u.getPortDiscoveryBindings(u.getTemplateRequest(name)), nil
}

// endpointToAddress converts an endpoint to an address string
func (*templateUtils) endpointToAddress(endpoint *labApi.IpEndpoint) string {
	return fmt.Sprintf("%s:%d", endpoint.Address, endpoint.Port)
}

// writeToFile writes proto message to a file
func (*templateUtils) writeToFile(file string, content proto.Message) error {
	f, err := os.Create(file)
	if err != nil {
		return errors.Annotate(err, "fail to create file %v", file).Err()
	}
	m := jsonpb.Marshaler{}
	if err := m.Marshal(f, content); err != nil {
		return errors.Annotate(err, "fail to marshal request to file %v", file).Err()
	}
	return nil
}

// getTemplateRequest retrieves the StartTemplatedContainerRequest from the
// global state.
func (u *templateUtils) getTemplateRequest(name string) *api.StartTemplatedContainerRequest {
	cmd := &commands.ContainerInspect{Names: []string{name}, Format: "{{.Id}}"}
	stdout, _, err := u.cmdExecutor.Execute(context.Background(), cmd)
	if err != nil {
		log.Printf("warning: unable to retrieve container id with name %s: %s", name, err)
		return nil
	}
	id := strings.TrimSpace(stdout)
	return state.ServerState.TemplateRequest.Get(id)
}

// getPortDiscoveryBindings retrieves port bindings from templates' port
// discovery. Note that depending on whether `host` or bridge network is used
// to start the container, the host port isn't always populated by port
// discovery.
// As currently port discovery is used as a fallback for all use cases, this
// method will swallow any errors and return empty bindings. Clients that rely
// on port discovery should handle empty port bindings as an error case.
func (u *templateUtils) getPortDiscoveryBindings(request *api.StartTemplatedContainerRequest) []*api.Container_PortBinding {
	bindings := make([]*api.Container_PortBinding, 0)
	if request == nil {
		return bindings
	}
	portBinding, err := u.templateRouter.discoverPort(request)
	if err != nil {
		log.Printf("warning: unable to discover port: %s", err)
		return bindings
	}

	bindings = append(bindings, portBinding)
	return bindings
}
