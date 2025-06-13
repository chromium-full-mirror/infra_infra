// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package templates

import (
	"log"
	"regexp"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/config/go/test/lab/api"
)

// placeholderPopulator is the interface to populate a placeholder IpEndpoint
// with actual values. The input uses URI schemes in address to indicate how the
// value should be populated.
// A placeholderPopulator then populates the value using proper commands.
type placeholderPopulator interface {
	// populate takes IpEndpoint template and returns a new IpEndpoint with actual
	// values populated
	// InvalidArgument error should be returned if unable to populate.
	populate(*api.IpEndpoint) (*api.IpEndpoint, error)
}

// Scheme definitions
const (
	// ContainerPortScheme is designed for docker bridge networks where the
	// communication within a network may use container name as the address.
	// The scheme indicates the port number used in the container (and
	// exposed and published when docker run) need to be populated into the
	// template. For example, if the service running in the container listens to
	// port 80 and the port has been exposed and published during docker run.
	// An IpEndpoint address of `ctr-container-port://container-name` indicates
	// that the address should be replaced with `container-name` and the port
	// should be replaced with actual port `80`. (Note that within a network, a
	// container can be referenced by name.)
	// To use this scheme, the port number in the input IpEndpoint must be 0.
	ContainerPortScheme = "ctr-container-port"
	// LocalhostPortScheme is designed for the docker `host` network (where
	// networking is shared with the host) for containers that have
	// go/cft-port-discovery implemented.
	// The schema indicates the host address and port number need to be populated
	// into the template. As the host address will always be `localhost`. The
	// current templated container must be in the `host` network.
	LocalhostPortScheme = "ctr-localhost-port"
)

// populatorRouter is the entry point that implements placeholderPopulator
type populatorRouter struct {
	containerLookuper ContainerLookuper
}

func newPopulatorRouter() placeholderPopulator {
	return &populatorRouter{containerLookuper: &TemplateUtils}
}

// extract returns scheme, a copy of IpEndpoint with address replaced by
// container name
func (pr *populatorRouter) extract(endpoint *api.IpEndpoint) (string, *api.IpEndpoint, error) {
	r := regexp.MustCompile(`(?P<Scheme>ctr-[\w-]+)://(?P<ContainerName>.+)`)
	match := r.FindStringSubmatch(endpoint.Address)
	if len(match) != 3 {
		return "", endpoint, status.Error(codes.InvalidArgument, "Not a valid template placeholder")
	}
	scheme := match[1]
	containerName := match[2]
	return scheme, &api.IpEndpoint{Address: containerName, Port: endpoint.Port}, nil
}

func (pr *populatorRouter) populate(input *api.IpEndpoint) (*api.IpEndpoint, error) {
	scheme, updatedEndpoint, err := pr.extract(input)
	if err != nil {
		log.Printf("skip unpopulatable input %v", input)
		return input, err
	}
	switch scheme {
	case ContainerPortScheme:
		actualPopulator := containerPortPopulator{pr.containerLookuper}
		return actualPopulator.populate(updatedEndpoint)
	case LocalhostPortScheme:
		actualPopulator := localhostPortPopulator{pr.containerLookuper}
		return actualPopulator.populate(updatedEndpoint)
	default:
		return input, status.Error(codes.InvalidArgument, "Scheme is unrecognized")
	}
}

// containerPortPopulator populates container port using the updated IpEndpoint
// template supplied by the router
type containerPortPopulator struct {
	containerLookup ContainerLookuper
}

func (p *containerPortPopulator) populate(input *api.IpEndpoint) (*api.IpEndpoint, error) {
	ports, err := p.containerLookup.LookupContainerPortBindings(input.Address)
	if err != nil {
		return input, err
	}
	if len(ports) != 1 {
		return input, status.Error(codes.FailedPrecondition, getPortBindingErrorMessage(len(ports)))
	}
	if input.Port != 0 {
		return input, status.Error(codes.InvalidArgument, "The port number must be 0 to be used with ctr-container-port scheme")
	}
	return &api.IpEndpoint{Address: input.Address, Port: int32(ports[0].ContainerPort)}, nil
}

func getPortBindingErrorMessage(numberOfPortBindings int) string {
	if numberOfPortBindings == 0 {
		return "The container doesn't have any port bindings. Make sure to expose the service port when start container."
	}
	if numberOfPortBindings > 1 {
		return "The container has more than one port bindings. Make sure to expose only one service port when start container."
	}
	return ""
}

// localhostPortPopulator populates host address and port using the updated
// IpEndpoint template supplied by the router
type localhostPortPopulator struct {
	containerLookup ContainerLookuper
}

func (p *localhostPortPopulator) populate(input *api.IpEndpoint) (*api.IpEndpoint, error) {
	ports, err := p.containerLookup.LookupContainerPortBindings(input.Address)
	if err != nil {
		return input, err
	}
	if len(ports) == 0 {
		return input, status.Error(codes.FailedPrecondition,
			"ctr-localhost-port scheme cannot find any port bindings. Make sure port discovery has been implemented in your containerized service")
	}
	if len(ports) > 1 {
		return input, status.Error(codes.FailedPrecondition,
			"ctr-localhost-port scheme only supports one service port. Make sure you are using host network and port discovery")
	}
	if ports[0].ContainerPort != ports[0].HostPort {
		return input, status.Error(codes.FailedPrecondition,
			"ctr-localhost-port scheme only supports host network where container port and host port are the same")
	}
	if input.Port != 0 {
		return input, status.Error(codes.InvalidArgument, "The port number must be 0 to be used with ctr-localhost-port scheme")
	}
	return &api.IpEndpoint{Address: localhostIp, Port: ports[0].HostPort}, nil
}
