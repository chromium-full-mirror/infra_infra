// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package templates

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/config/go/test/api"

	"infra/cros/cmd/cros-tool-runner/internal/v2/commands"
)

const (
	hostNetworkName = "host"      // docker `host` network
	localhostIp     = "localhost" // localhost or 127.0.0.1
	protocolTcp     = "tcp"       // tcp protocol value in docker port binding
	// 0 is a special port. When `-port=0` is used to start a service , a random
	// available port will be allocated. go/cft-port-discovery
	portZero = "0"
)

var aCrosDutProcessor = newCrosDutProcessor()
var aCrosProvisionProcessor = newCrosProvisionProcessor()
var aCrosTestProcessor = newCrosTestProcessor()
var aCrosGcsPublishProcessor = newCrosGcsPublishProcessor()
var aCrosTkoPublishProcessor = newCrosTkoPublishProcessor()
var aCrosRdbPublishProcessor = newCrosRdbPublishProcessor()
var aCacheServerProcessor = newCacheServerProcessor()
var aCrosFwProvisionProcessor = newCrosFwProvisionProcessor()
var aCrosVMProvisionProcessor = newCrosVMProvisionProcessor()
var aGenericProcessor = newGenericProcessor()
var aPostProcessProcessor = newPostProcessProcessor()

// TemplateProcessor converts a container-specific template into a valid generic
// StartContainerRequest. Besides request conversions, a TemplateProcessor is
// also aware of a container's dependencies of other containers, whose addresses
// are determined at runtime. The addresses are provided as IpEndpoint
// placeholders in a template, and TemplateProcessor use placeholderPopulators
// to populate actual values.
type TemplateProcessor interface {
	Process(*api.StartTemplatedContainerRequest) (*api.StartContainerRequest, error)
}

// portDiscoverer provides a mechanism to discover the port that the service
// listens to in a templated container, especially for the use case when the
// docker `host` network is used. It provides equivalent API as the docker port
// bindings when docker bridge networks are used. go/cft-port-discovery
type portDiscoverer interface {
	discoverPort(*api.StartTemplatedContainerRequest) (*api.Container_PortBinding, error)
}

// crosTemplate aggregates interfaces that must be implemented by most templates
type crosTemplate interface {
	TemplateProcessor
	portDiscoverer
}

// RequestRouter is the entry point to template processing.
type RequestRouter struct{}

func (r *RequestRouter) Process(request *api.StartTemplatedContainerRequest) (*api.StartContainerRequest, error) {
	actualProcessor, err := r.getActualProcessor(request)
	if err != nil {
		return nil, err
	}
	return actualProcessor.Process(request)
}

func (r *RequestRouter) discoverPort(request *api.StartTemplatedContainerRequest) (*api.Container_PortBinding, error) {
	actualProcessor, err := r.getActualProcessor(request)
	if err != nil {
		return nil, err
	}
	return actualProcessor.discoverPort(request)
}

func (r *RequestRouter) getActualProcessor(request *api.StartTemplatedContainerRequest) (crosTemplate, error) {
	if request.GetTemplate().Container == nil {
		return nil, status.Error(codes.InvalidArgument, "No template set in the request")
	}
	switch t := request.GetTemplate().Container.(type) {
	case *api.Template_Generic:
		return aGenericProcessor, nil
	case *api.Template_CrosDut:
		return aCrosDutProcessor, nil
	case *api.Template_CrosProvision:
		return aCrosProvisionProcessor, nil
	case *api.Template_CrosTest:
		return aCrosTestProcessor, nil
	case *api.Template_CacheServer:
		return aCacheServerProcessor, nil
	case *api.Template_CrosPublish:
		return r.getActualPublishProcessor(t.CrosPublish.PublishType)
	case *api.Template_CrosFwProvision:
		return aCrosFwProvisionProcessor, nil
	case *api.Template_CrosVmProvision:
		return aCrosVMProvisionProcessor, nil
	case *api.Template_PostProcess:
		return aPostProcessProcessor, nil
	default:
		return nil, status.Error(codes.Unimplemented, fmt.Sprintf("%v to be implemented", t))
	}
}

func (*RequestRouter) getActualPublishProcessor(publishType api.CrosPublishTemplate_PublishType) (crosTemplate, error) {
	switch publishType {
	case api.CrosPublishTemplate_PUBLISH_GCS:
		return aCrosGcsPublishProcessor, nil
	case api.CrosPublishTemplate_PUBLISH_TKO:
		return aCrosTkoPublishProcessor, nil
	case api.CrosPublishTemplate_PUBLISH_RDB:
		return aCrosRdbPublishProcessor, nil
	default:
		return nil, status.Error(codes.Unimplemented, fmt.Sprintf("%v to be implemented", publishType))
	}
}

// defaultDiscoverPort is the standard impl for go/cft-port-discovery across
// all templated containers. Each template processor is expected to have
// customized behavior specifically for its container, e.g. retry, polling...
// The returned Container_PortBinding will have ContainerPort and Protocol(tcp)
// populated for bridge network, and HostIp and HostPort fields are also
// populated for the `host` network.
func defaultDiscoverPort(cmdExecutor cmdExecutor, request *api.StartTemplatedContainerRequest) (*api.Container_PortBinding, error) {
	cmd := &commands.DockerExec{
		Name:        request.Name,
		ExecCommand: []string{"/bin/bash", "-c", "source ~/.cftmeta && echo $SERVICE_PORT"},
	}
	stdout, stderr, err := cmdExecutor.Execute(context.Background(), cmd)
	if err != nil {
		return nil, fmt.Errorf("%w with stderr: %s", err, stderr)
	}
	servicePort, err := strconv.Atoi(strings.TrimSpace(stdout))
	if err != nil {
		return nil, err
	}
	portBinding := &api.Container_PortBinding{
		ContainerPort: int32(servicePort),
		Protocol:      protocolTcp,
	}
	if request.Network == hostNetworkName {
		portBinding.HostPort = portBinding.ContainerPort
		portBinding.HostIp = localhostIp
	}
	return portBinding, nil
}
