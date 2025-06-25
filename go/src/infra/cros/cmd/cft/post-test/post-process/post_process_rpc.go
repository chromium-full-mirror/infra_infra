// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
)

// PostTestServiceServer implementation of dut_service.proto
type PostTestServiceServer struct {
	api.UnimplementedPostTestServiceServer
	logger    *log.Logger
	dutClient api.DutServiceClient
	conns     []*grpc.ClientConn
}

// NewServer creates an execution server.
func NewServer(logger *log.Logger, dutAddress string) (*grpc.Server, func(), error) {
	s := &PostTestServiceServer{
		logger: logger,
		conns:  []*grpc.ClientConn{},
	}

	if dutAddress != "" {
		dutConn, err := grpc.Dial(dutAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("DutConn Failed!")
			return nil, s.closePostTestServer, fmt.Errorf("failed to connect to dut-service, %s", err)
		}
		s.conns = append(s.conns, dutConn)
		s.dutClient = api.NewDutServiceClient(dutConn)
		log.Printf("Dut Conn Established")
	}

	server := grpc.NewServer()
	api.RegisterPostTestServiceServer(server, s)
	reflection.Register(server)
	// longrunning.RegisterOperationsServer(server, s.manager)
	logger.Println("crostestservice listen to request at ")
	return server, s.closePostTestServer, nil
}

func (s *PostTestServiceServer) closePostTestServer() {
	for _, conn := range s.conns {
		conn.Close()
	}
	s.conns = nil
}

// StartUp handles the initialization of the PostTestService.
func (s *PostTestServiceServer) StartUp(ctx context.Context, req *api.PostTestStartUpRequest) (*api.PostTestStartUpResponse, error) {
	s.logger.Println("Received StartUp: ", req)
	response := api.PostTestStartUpResponse{}

	if req.DutServer == nil || req.GetDutServer().GetAddress() == "" {
		response.Status = api.PostTestStartUpResponse_STATUS_INVALID_REQUEST
		return &response, fmt.Errorf("PostService: startup request missing DutServer")
	}

	dutAddr := fmt.Sprintf("%s:%d", req.GetDutServer().GetAddress(), req.GetDutServer().GetPort())
	dutConn, err := grpc.Dial(dutAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		response.Status = api.PostTestStartUpResponse_STATUS_STARTUP_FAILED
		return &response, fmt.Errorf("PostService: failed to connect to dut-service, %s", err)
	}
	s.conns = append(s.conns, dutConn)
	s.dutClient = api.NewDutServiceClient(dutConn)
	s.logger.Printf("PostService: Dut Conn Established")

	response.Status = api.PostTestStartUpResponse_STATUS_SUCCESS
	return &response, nil
}

// RunActivity calls the parseAndRunCmds (post-process flow) in main.
func (s *PostTestServiceServer) RunActivity(ctx context.Context, req *api.RunActivityRequest) (*api.RunActivityResponse, error) {
	s.logger.Printf("Received RunActivity: %q", req)

	if s.dutClient == nil {
		return nil, fmt.Errorf("PostService: missing DUT client")
	}

	s.logger.Printf("Start to run post process activity: %s", req)
	resp, err := parseAndRunCmds(req.Request, s.logger, s.dutClient, nil)
	if err != nil {
		return nil, errors.Annotate(err, "PostService: failed to run RunActivity RPC Command").Err()
	}
	s.logger.Printf("PostService RunActivity RPC Command: %s was successful", req)
	s.logger.Printf("Returning %q", resp)

	return resp, nil
}

// RunActivities calls the parseAndRunCmds (post-process flow) in main.
func (s *PostTestServiceServer) RunActivities(ctx context.Context, req *api.RunActivitiesRequest) (*api.RunActivitiesResponse, error) {
	s.logger.Printf("Received RunActivities: %q", req)

	if s.dutClient == nil {
		return nil, fmt.Errorf("PostService: missing DUT client")
	}

	s.logger.Printf("Start to run post process activities: %s", req)
	resp, err := parseCommandsAndRun(req, s.logger, s.dutClient)
	if err != nil {
		return nil, errors.Annotate(err, "PostService: failed to run RunActivities RPC Command").Err()
	}
	s.logger.Printf("PostService RunActivities RPC Command: %s was successful", req)
	s.logger.Printf("Returning: %q", resp)

	return resp, nil
}
