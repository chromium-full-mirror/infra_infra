// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// GRPC Server impl
package cli

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"runtime/debug"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/longrunning"
	"go.chromium.org/chromiumos/config/go/test/api"
	api1 "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/lro"

	"go.chromium.org/infra/cros/cmd/cft/common/portdiscovery"
	ashchromeservice "go.chromium.org/infra/cros/cmd/provision/ash-chrome-provision/service"
	state_machine "go.chromium.org/infra/cros/cmd/provision/ash-chrome-provision/state-machine"
)

// AshChromeProvisionServer is the top level class for the ash-chrome provisioning server.
type AshChromeProvisionServer struct {
	// dutServer provides an interface to manipulate DUT via cros-dut
	// service. Its address may be specified either when server is created,
	// or later in user's AshChromeProvisionRequest.
	dutServer api.DutServiceClient
	dut       *api1.Dut

	log        *log.Logger
	listenPort int

	manager *lro.Manager

	cacheServer url.URL
}

func ipEndpointToHostPort(i *api1.IpEndpoint) (string, error) {
	if len(i.GetAddress()) == 0 {
		return "", errors.New("IpEndpoint missing address")
	}
	if i.GetPort() == 0 {
		return "", errors.New("IpEndpoint missing port")
	}
	return fmt.Sprintf("%v:%v", i.GetAddress(), i.GetPort()), nil
}

// NewAshChromeProvisionServer returns a new AshChromeProvisionServer, a closer function, and an error.
func NewAshChromeProvisionServer(listenPort int, log *log.Logger) (*AshChromeProvisionServer, func(), error) {
	manager := lro.New()
	return &AshChromeProvisionServer{
		listenPort: listenPort,
		log:        log,
		manager:    manager,
	}, manager.Close, nil
}

// Start starts the grpc server.
func (ps *AshChromeProvisionServer) Start() error {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", ps.listenPort))
	if err != nil {
		return fmt.Errorf("failed to create listener at %d", ps.listenPort)
	}
	server := grpc.NewServer()
	api.RegisterGenericProvisionServiceServer(server, ps)
	longrunning.RegisterOperationsServer(server, ps.manager)
	ps.log.Println("provisionservice listen to request at ", l.Addr().String())

	// Write port number to ~/.cftmeta for go/cft-port-discovery
	err = portdiscovery.WriteServiceMetadata("provision", l.Addr().String(), ps.log)
	if err != nil {
		ps.log.Println("Warning: error when writing to metadata file: ", err)
	}

	return server.Serve(l)
}

// StartUp handles the initialization of the GenericProvisionService by passing in parameters through the ProvisionStartupRequest.
func (ps *AshChromeProvisionServer) StartUp(ctx context.Context, req *api.ProvisionStartupRequest) (*api.ProvisionStartupResponse, error) {
	ps.log.Println("Received api.ProvisionStartupRequest: ", req)
	response := api.ProvisionStartupResponse{}

	if err := ps.validateStartupRequest(req); err != nil {
		response.Status = api.ProvisionStartupResponse_STATUS_INVALID_REQUEST
		return &response, err
	}

	dutServAddr, err := ipEndpointToHostPort(req.DutServer)
	if err != nil {
		response.Status = api.ProvisionStartupResponse_STATUS_INVALID_REQUEST
		return &response, errors.Wrap(err, "failed to parse IpEndpoint of Dut Server")
	}
	dutServer, err := connectToDutServer(dutServAddr)
	if err != nil {
		response.Status = api.ProvisionStartupResponse_STATUS_STARTUP_FAILED
		return &response, errors.Wrap(err, "connect to dut server")
	}
	ps.dutServer = dutServer

	cacheServerAddr, err := ipEndpointToHostPort(req.Dut.GetCacheServer().GetAddress())
	if err != nil {
		response.Status = api.ProvisionStartupResponse_STATUS_INVALID_REQUEST
		return &response, errors.Wrap(err, "failed to parse IpEndpoint of cache server")
	}
	ps.cacheServer.Scheme = "http"
	ps.cacheServer.Host = cacheServerAddr
	if req.Dut.GetCacheServer().GetAddress().Address == "localhost" {
		response.Status = api.ProvisionStartupResponse_STATUS_INVALID_REQUEST
		return &response, errors.New("ProvisionStartupRequest: cache_server_address must be visible from DUT, i.e. no localhost")
	}
	ps.dut = req.GetDut()

	response.Status = api.ProvisionStartupResponse_STATUS_SUCCESS
	return &response, nil
}

func (ps *AshChromeProvisionServer) validateStartupRequest(req *api.ProvisionStartupRequest) error {
	if req == nil {
		return errors.New("ProvisionStartupRequest is required")
	}
	if req.Dut == nil {
		return errors.New("ProvisionStartupRequest: dut is required")
	}
	if req.DutServer == nil {
		return errors.New("ProvisionStartupRequest: dut_server is required")
	}
	if req.Dut.GetCacheServer() == nil {
		return errors.New("ProvisionStartupRequest: dut.cache_server is required")
	}
	if req.Dut.GetCacheServer().GetAddress() == nil {
		return errors.New("ProvisionStartupRequest: dut.cache_server.address is required")
	}
	if req.Dut.GetCacheServer().GetAddress().Address == "" {
		return errors.New("ProvisionStartupRequest: dut.cache_server.address.address is required")
	}
	if req.Dut.GetCacheServer().GetAddress().Port == 0 {
		return errors.New("ProvisionStartupRequest: dut.cache_server.address.port is required")
	}
	return nil
}

// Install starts the ash chrome provisioning in the background, and returns a long running operation or an error.
func (ps *AshChromeProvisionServer) Install(ctx context.Context, req *api.InstallRequest) (*longrunning.Operation, error) {
	ps.log.Println("Received api.InstallCrosRequest: ", req)
	op := ps.manager.NewOperation()

	go ps.doProvision(context.Background(), req, op.Name)

	return op, nil
}

func (ps *AshChromeProvisionServer) doProvision(ctx context.Context, req *api.InstallRequest, lroName string) {
	response := api.InstallResponse{}
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			ps.log.Printf("Panic detected: %+v stack: %s", r, stack)
			response.Status = api.InstallResponse_STATUS_PROVISIONING_FAILED
			err, ok := r.(error)
			if ok {
				response.Message = err.Error()
			} else {
				response.Message = fmt.Sprintf("panic: %+v", r)
			}
		}
		ps.manager.SetResult(lroName, &response)
		ps.log.Printf("Provision set OP Response to:%s ", response.String())
	}()

	ashService, status, err := ashchromeservice.NewAshChromeService(ctx, ps.dutServer, ps.cacheServer, req, ps.dut)
	if err != nil {
		ps.log.Printf("Failed to initialize AshChrome Service: %v", err)
		response.Status = status
		response.Message = err.Error()
		return
	}
	// Clean up the temporary directories on the DUT.
	defer ashService.DeleteArchiveDirectories()

	response.Status = api.InstallResponse_STATUS_SUCCESS
	var ashChromeResponse *anypb.Any
	// Execute state machine
	cs := state_machine.NewAshChromePrepareState(ashService)
	for cs != nil {
		var metadata *anypb.Any
		metadata, response.Status, err = cs.Execute(ctx, ps.log)
		if metadata != nil {
			ashChromeResponse = metadata
		}
		if err != nil {
			ps.log.Printf("State machine failed: %v", err)
			break
		}
		cs = cs.Next()
	}
	if err != nil {
		response.Message = err.Error()
	}
	response.Metadata = ashChromeResponse
}
