// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servertemplate

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"go.chromium.org/chromiumos/config/go/test/api"
)

type GenericServiceServer struct {
	api.UnimplementedGenericServiceServer

	concreteService GenericService

	serverLogger *log.Logger
}

type GenericService interface {
	Start(ctx context.Context, log *log.Logger, start *api.GenericStartRequest) (*api.GenericStartResponse, error)
	Run(ctx context.Context, log *log.Logger, start *api.GenericRunRequest) (*api.GenericRunResponse, error)
	Stop(ctx context.Context, log *log.Logger, start *api.GenericStopRequest) (*api.GenericStopResponse, error)
}

func NewServer(logger *log.Logger, concreteService GenericService) *grpc.Server {
	s := &GenericServiceServer{
		concreteService: concreteService,
		serverLogger:    logger,
	}

	server := grpc.NewServer()
	api.RegisterGenericServiceServer(server, s)
	reflection.Register(server)

	return server
}

func (s *GenericServiceServer) Start(ctx context.Context, req *api.GenericStartRequest) (resp *api.GenericStartResponse, err error) {
	defer CapturePanic(s.serverLogger, &err)

	s.serverLogger.Printf("Received Start Request: %s", req)

	resp, err = s.concreteService.Start(ctx, s.serverLogger, req)
	if err != nil {
		s.serverLogger.Printf("Failed to execute `Start`: %s", err)
	}

	return
}

func (s *GenericServiceServer) Run(ctx context.Context, req *api.GenericRunRequest) (resp *api.GenericRunResponse, err error) {
	defer CapturePanic(s.serverLogger, &err)

	s.serverLogger.Printf("Received Run Request: %s", req)

	resp, err = s.concreteService.Run(ctx, s.serverLogger, req)
	if err != nil {
		s.serverLogger.Printf("Failed to execute `Run`: %s", err)
	}

	return
}

func (s *GenericServiceServer) Stop(ctx context.Context, req *api.GenericStopRequest) (resp *api.GenericStopResponse, err error) {
	defer CapturePanic(s.serverLogger, &err)

	s.serverLogger.Printf("Received Stop Request: %s", req)

	resp, err = s.concreteService.Stop(ctx, s.serverLogger, req)
	if err != nil {
		s.serverLogger.Printf("Failed to execute `Stop`: %s", err)
	}

	return
}

func CapturePanic(logger *log.Logger, err *error) {
	if r := recover(); r != nil {
		richError := fmt.Errorf("%s\n%s", r, string(debug.Stack()))
		*err = richError
	}
}
