// Copyright 2022 The Chromium Authors
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
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/lro"

	"go.chromium.org/infra/cros/cmd/cft/common/portdiscovery"
	"go.chromium.org/infra/cros/cmd/provision/common-utils/cache"
	firmwareservice "go.chromium.org/infra/cros/cmd/provision/cros-fw-provision/service"
	state_machine "go.chromium.org/infra/cros/cmd/provision/cros-fw-provision/state-machine"
)

// FWProvisionServer is the top level class for the firmware provisioning server.
type FWProvisionServer struct {
	// dutServer provides an interface to manipulate DUT via cros-dut
	// service. Its address may be specified either when server is created,
	// or later in user's ProvisionFirmwareRequest
	dutServer api.DutServiceClient

	// servoClient provides an interface to manipulate DUT via cros-servod
	// service. Its address may be specified either when server is created,
	// or later in user's ProvisionFirmwareRequest
	servoClient api.ServodServiceClient
	servoConfig *labapi.Servo

	log        *log.Logger
	listenPort int

	manager *lro.Manager

	cacheServer url.URL

	board string
	model string
}

// NewFWProvisionServer returns a new FWProvisionServer, a closer function, and an error.
func NewFWProvisionServer(listenPort int, log *log.Logger) (*FWProvisionServer, func(), error) {
	manager := lro.New()
	return &FWProvisionServer{
		listenPort: listenPort,
		log:        log,
		manager:    manager,
	}, manager.Close, nil
}

// Start starts the grpc server.
func (ps *FWProvisionServer) Start() error {
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
func (ps *FWProvisionServer) StartUp(ctx context.Context, req *api.ProvisionStartupRequest) (*api.ProvisionStartupResponse, error) {
	ps.log.Println("Received api.ProvisionStartupRequest: ", req)
	response := api.ProvisionStartupResponse{}

	if err := ps.validateStartupRequest(req); err != nil {
		response.Status = api.ProvisionStartupResponse_STATUS_INVALID_REQUEST
		return &response, err
	}

	ps.board = req.Dut.GetChromeos().DutModel.BuildTarget
	ps.model = req.Dut.GetChromeos().DutModel.ModelName

	dutServAddr, err := cache.IPEndpointToHostPort(req.DutServer)
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

	cacheServerAddr, err := cache.IPEndpointToHostPort(req.Dut.GetCacheServer().GetAddress())
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

	if req.ServoNexusAddr != nil && req.GetDut().GetChromeos().GetServo().GetPresent() {
		servoServerAddr, err := cache.IPEndpointToHostPort(req.ServoNexusAddr)
		if err != nil {
			response.Status = api.ProvisionStartupResponse_STATUS_INVALID_REQUEST
			return &response, errors.Wrap(err, "failed to parse IpEndpoint of Servo Nexus")
		}
		servoClient, err := connectToCrosServod(servoServerAddr)
		if err != nil {
			response.Status = api.ProvisionStartupResponse_STATUS_STARTUP_FAILED
			return &response, errors.Wrap(err, "connect to Servo Nexus")
		}
		ps.servoClient = servoClient
		ps.servoConfig = req.GetDut().GetChromeos().GetServo()
	}

	response.Status = api.ProvisionStartupResponse_STATUS_SUCCESS
	return &response, nil
}

func (ps *FWProvisionServer) validateStartupRequest(req *api.ProvisionStartupRequest) error {
	if req == nil {
		return errors.New("ProvisionStartupRequest is required")
	}
	if req.Dut == nil {
		return errors.New("ProvisionStartupRequest: dut is required")
	}
	if req.Dut.GetChromeos() == nil {
		return errors.New("ProvisionStartupRequest: dut.chromeos is required")
	}
	if req.Dut.GetChromeos().DutModel == nil {
		return errors.New("ProvisionStartupRequest: dut.chromeos.dut_model is required")
	}
	if req.Dut.GetChromeos().DutModel.BuildTarget == "" {
		return errors.New("ProvisionStartupRequest: dut.chromeos.dut_model.build_target is required")
	}
	if req.Dut.GetChromeos().DutModel.ModelName == "" {
		return errors.New("ProvisionStartupRequest: dut.chromeos.dut_model.model_name is required")
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

// Install starts the firmware provisioning in the background, and returns a long running operation or an error.
func (ps *FWProvisionServer) Install(ctx context.Context, req *api.InstallRequest) (*longrunning.Operation, error) {
	ps.log.Println("Received api.InstallCrosRequest: ", req)
	op := ps.manager.NewOperation()

	go ps.doProvision(context.Background(), req, op.Name)

	return op, nil
}

func (ps *FWProvisionServer) doProvision(ctx context.Context, req *api.InstallRequest, lroName string) {
	response := api.InstallResponse{}
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			ps.log.Printf("Panic detected: %+v stack: %s", r, stack)
			response.Status = api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED
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

	fwService, status, err := firmwareservice.NewFirmwareService(ctx, ps.dutServer, ps.servoClient, ps.cacheServer,
		ps.board, ps.model, false, req, ps.servoConfig)
	if err != nil {
		ps.log.Printf("Failed to initialize Firmware Service: %v", err)
		response.Status = status
		response.Message = err.Error()
		return
	}
	// Clean up the temporary directories on the DUT.
	defer fwService.DeleteArchiveDirectories()

	response.Status = api.InstallResponse_STATUS_SUCCESS
	var firmwareResponse *api.FirmwareProvisionResponse
	// Execute state machine
	cs := state_machine.NewFirmwarePrepareState(fwService)
	for cs != nil {
		var metadata *api.FirmwareProvisionResponse
		metadata, response.Status, err = cs.Execute(ctx, ps.log)
		if metadata != nil {
			firmwareResponse = metadata
		}
		if err != nil {
			ps.log.Printf("State machine failed: %v", err)
			break
		}
		cs = cs.Next()
	}
	// If the state machine didn't return metadata, get one from the fwService.
	if firmwareResponse == nil {
		firmwareResponse = fwService.GetVersions()
	}
	if err != nil {
		response.Message = err.Error()
	}
	response.Metadata, err = anypb.New(firmwareResponse)
	if err != nil {
		ps.log.Printf("Failed to create AnyPb: %v", err)
	}
}
