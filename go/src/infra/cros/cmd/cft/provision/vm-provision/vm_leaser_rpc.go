// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/longrunning"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/lro"
)

// VMLeaserServiceServer is implementation of vm_leaser.proto
type VMLeaserServiceServer struct {
	logger            *log.Logger
	vmleaserClient    api.VMLeaserServiceClient
	manager           *lro.Manager
	authTokenFilePath string
}

// NewServer creates an execution server
func NewServer(logger *log.Logger, authTokenFilePath string) (*grpc.Server, func()) {
	s := &VMLeaserServiceServer{
		logger:            logger,
		manager:           lro.New(),
		authTokenFilePath: authTokenFilePath,
	}
	server := grpc.NewServer()
	var conns []*grpc.ClientConn
	closer := func() {
		for _, conn := range conns {
			conn.Close()
		}
		conns = nil
	}
	api.RegisterGenericProvisionServiceServer(server, s)
	longrunning.RegisterOperationsServer(server, s.manager)
	reflection.Register(server)

	return server, closer
}

func (s *VMLeaserServiceServer) StartUp(ctx context.Context, req *api.ProvisionStartupRequest) (*api.ProvisionStartupResponse, error) {
	s.logger.Println("Received api.ProvisionStartupRequest: ", req)
	response := api.ProvisionStartupResponse{}
	response.Status = api.ProvisionStartupResponse_STATUS_SUCCESS
	return &response, nil
}

// Install calls the VMLeaser service endpoints
func (s *VMLeaserServiceServer) Install(ctx context.Context, req *api.InstallRequest) (*longrunning.Operation, error) {
	if req == nil || req.Metadata == nil || req.Metadata.TypeUrl == "" {
		return nil, fmt.Errorf("nnvalid request: empty request or missing metadata type url")
	}

	op := s.manager.NewOperation()
	metadataType := req.Metadata.TypeUrl

	switch metadataType {
	case "type.googleapis.com/chromiumos.test.api.LeaseVMRequest":
		return s.handleLeaseVMRequest(ctx, op, req)
	case "type.googleapis.com/chromiumos.test.api.ReleaseVMRequest":
		return s.handleReleaseVMRequest(ctx, op, req)
	default:
		return nil, fmt.Errorf("invalid metadata type: %s in request", metadataType)
	}
}

func (s *VMLeaserServiceServer) handleLeaseVMRequest(_ context.Context, op *longrunning.Operation, req *api.InstallRequest) (*longrunning.Operation, error) {
	s.logger.Printf("Setting up new connection to VM Leaser service")
	vmleaserClient, closeFunc, err := getVMLeaserClient(s.logger)
	if err != nil {
		return nil, err
	}
	// close grpc connection
	defer closeFunc()
	s.vmleaserClient = vmleaserClient
	var leaseVMReq api.LeaseVMRequest
	if err := req.Metadata.UnmarshalTo(&leaseVMReq); err != nil {
		s.logger.Printf("Invalid request: %s", err)
		return nil, err
	}
	s.logger.Printf("Making lease gRPC call")
	token, err := readAuthToken(s.authTokenFilePath)
	if err != nil {
		s.logger.Fatalln("Failed to read auth token: ", err)
		return nil, err
	}
	if token == "" {
		s.logger.Fatalln("Aborting initialization as token is empty")
		return nil, err
	}
	authorization := "Bearer " + token
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("Authorization", authorization))
	resp, err := s.vmleaserClient.LeaseVM(ctx, &leaseVMReq)
	if err != nil {
		s.logger.Printf("Failed to make gRPC request: %s", err)
		return nil, err
	}
	anyResp := &anypb.Any{}
	if err := anyResp.MarshalFrom(resp); err != nil {
		s.logger.Printf("Failed to marshal response: %s", err)
		return nil, err
	}
	provisionResp := &api.InstallResponse{
		Metadata: anyResp,
	}
	s.logger.Printf("gRPC request succeeded")
	s.manager.SetResult(op.Name, provisionResp)
	return op, nil
}

func (s *VMLeaserServiceServer) handleReleaseVMRequest(_ context.Context, op *longrunning.Operation, req *api.InstallRequest) (*longrunning.Operation, error) {
	s.logger.Printf("Setting up new connection to VM Leaser service")
	vmleaserClient, closeFunc, err := getVMLeaserClient(s.logger)
	if err != nil {
		return nil, err
	}
	// close grpc connection
	defer closeFunc()
	s.vmleaserClient = vmleaserClient
	var releaseVMReq api.ReleaseVMRequest
	if err := req.Metadata.UnmarshalTo(&releaseVMReq); err != nil {
		s.logger.Printf("Invalid request: %s", err)
		return nil, err
	}
	s.logger.Printf("Making release gRPC call")
	token, err := readAuthToken(s.authTokenFilePath)
	if err != nil {
		s.logger.Fatalln("Failed to read auth token: ", err)
		return nil, err
	}
	if token == "" {
		s.logger.Fatalln("Aborting initialization as token is empty")
		return nil, err
	}
	authorization := "Bearer " + token
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("Authorization", authorization))
	resp, err := s.vmleaserClient.ReleaseVM(ctx, &releaseVMReq)
	if err != nil {
		s.logger.Printf("Failed to make gRPC request: %s", err)
		return nil, err
	}
	anyResp := &anypb.Any{}
	if err := anyResp.MarshalFrom(resp); err != nil {
		s.logger.Printf("Failed to marshal response: %s", err)
		return nil, err
	}
	provisionResp := &api.InstallResponse{
		Metadata: anyResp,
	}
	s.logger.Printf("gRPC request succeeded")
	s.manager.SetResult(op.Name, provisionResp)
	return op, nil
}

// readAuthToken reads the authorization token required for calling vm leaser service
func readAuthToken(authTokenFilePath string) (string, error) {
	content, err := os.ReadFile(authTokenFilePath)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(content))

	return token, nil
}

// getVMLeaserClient returns a client to communicate with VM Leaser service
func getVMLeaserClient(logger *log.Logger) (api.VMLeaserServiceClient, func() error, error) {
	// Set up gRPC connection
	conn, err := grpc.NewClient(vmLeaserEndPoint, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, "")))
	if err != nil {
		logger.Fatalln("Failed to connect with VM leaser service: ", err)
		return nil, nil, err
	}
	closeFunc := func() error {
		return conn.Close()

	}

	return api.NewVMLeaserServiceClient(conn), closeFunc, nil
}
