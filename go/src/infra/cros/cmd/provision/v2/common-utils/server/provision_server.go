// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// GRPC Server impl
package server

import (
	"context"
	"errors"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"go.chromium.org/chromiumos/config/go/longrunning"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/lro"
	common_utils "go.chromium.org/chromiumos/test/provision/v2/common-utils"
	"go.chromium.org/chromiumos/test/provision/v2/common-utils/metadata"
	"go.chromium.org/chromiumos/test/util/portdiscovery"
)

type ProvisionServer struct {
	options        *metadata.ServerMetadata
	dutClient      api.DutServiceClient
	manager        *lro.Manager
	executor       ProvisionExecutor
	conns          []*grpc.ClientConn
	servoNexusAddr string
}

func NewProvisionServer(options *metadata.ServerMetadata, executor ProvisionExecutor) (*ProvisionServer, func(), error) {
	ps := &ProvisionServer{
		options:  options,
		executor: executor,
		conns:    []*grpc.ClientConn{},
	}

	if options.DutAddress != "" {
		dutConn, err := grpc.Dial(options.DutAddress, grpc.WithInsecure())
		if err != nil {
			return nil, ps.closeProvisionServer, fmt.Errorf("failed to connect to dut-service, %s", err)
		}
		ps.conns = append(ps.conns, dutConn)
		ps.dutClient = api.NewDutServiceClient(dutConn)
	}

	return ps, ps.closeProvisionServer, nil
}

func (ps *ProvisionServer) closeProvisionServer() {
	for _, conn := range ps.conns {
		conn.Close()
	}
	ps.conns = nil
}

func (ps *ProvisionServer) Start() error {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", ps.options.Port))
	if err != nil {
		return fmt.Errorf("failed to create listener at %d", ps.options.Port)
	}

	// Write port number to ~/.cftmeta for go/cft-port-discovery
	err = portdiscovery.WriteServiceMetadata("provision", l.Addr().String(), ps.options.Log)
	if err != nil {
		ps.options.Log.Println("Warning: error when writing to metadata file: ", err)
	}

	ps.manager = lro.New()
	defer ps.manager.Close()
	server := grpc.NewServer()
	api.RegisterGenericProvisionServiceServer(server, ps)
	longrunning.RegisterOperationsServer(server, ps.manager)
	ps.options.Log.Println("provisionservice listen to request at ", l.Addr().String())
	return server.Serve(l)
}

// StartUp handles the initialization of the GenericProvisionService by passing in parameters through the ProvisionStartupRequest.
func (ps *ProvisionServer) StartUp(ctx context.Context, req *api.ProvisionStartupRequest) (*api.ProvisionStartupResponse, error) {
	ps.options.Log.Println("Received api.ProvisionStartupRequest: ", req)
	response := api.ProvisionStartupResponse{}

	if err := ps.executor.Validate(req); err != nil {
		response.Status = api.ProvisionStartupResponse_STATUS_INVALID_REQUEST
		return &response, err
	}
	if req.GetServoNexusAddr() != nil {
		ps.servoNexusAddr = fmt.Sprintf("%v:%v", req.GetServoNexusAddr().GetAddress(), req.GetServoNexusAddr().GetPort())
	}
	ps.options.Dut = req.Dut
	ps.options.DutAddress = fmt.Sprintf("%s:%d", req.GetDutServer().GetAddress(), req.GetDutServer().GetPort())
	dutConn, err := grpc.Dial(ps.options.DutAddress, grpc.WithInsecure())
	if err != nil {
		response.Status = api.ProvisionStartupResponse_STATUS_STARTUP_FAILED
		return &response, fmt.Errorf("failed to connect to dut-service, %s", err)
	}
	ps.conns = append(ps.conns, dutConn)
	ps.dutClient = api.NewDutServiceClient(dutConn)

	response.Status = api.ProvisionStartupResponse_STATUS_SUCCESS
	return &response, nil
}

func (ps *ProvisionServer) Install(ctx context.Context, req *api.InstallRequest) (*longrunning.Operation, error) {
	ps.options.Log.Println("Received api.InstallCrosRequest: ", req)
	op := ps.manager.NewOperation()
	response := api.InstallResponse{}

	installResp, md, err := ps.installTarget(ctx, req)
	if err != nil && installResp != api.InstallResponse_STATUS_SUCCESS {
		ps.options.Log.Printf("failed provision, %s", err)
	}

	ps.options.Log.Printf("Raw Status: %s", md)
	fv := ""
	stringValue := &wrapperspb.StringValue{}
	if err := md.UnmarshalTo(stringValue); err != nil {
		ps.options.Log.Printf("Error unmarshalling: %s", err.Error())
		fv = "No Status"
	} else {
		fv = stringValue.GetValue()
	}

	response.Status = installResp
	response.Message = fv
	ps.manager.SetResult(op.Name, &response)
	ps.options.Log.Printf("Provision set OP Response to:%s ", &response)
	// Note: Do not return the err here, as it causes the op response to not be set.
	// Since the op will carry the failure reason, just set the op.
	return op, nil
}

// installTarget installs a specified version of the software on the target, along
// with any specified DLCs.
func (ps *ProvisionServer) installTarget(ctx context.Context, req *api.InstallRequest) (api.InstallResponse_Status, *anypb.Any, error) {
	ps.options.Log.Println("Received api.InstallRequest: ", req)

	if ps.dutClient == nil {
		return api.InstallResponse_STATUS_PRE_PROVISION_SETUP_FAILED, nil, errors.New("error: no dut client found")
	}

	fs, err := ps.executor.GetFirstState(ps.options.Dut, ps.dutClient, ps.servoNexusAddr, req)
	if err != nil {
		return api.InstallResponse_STATUS_INVALID_REQUEST, nil, err
	}

	return common_utils.ExecuteStateMachine(ctx, fs, ps.options.Log)
}
