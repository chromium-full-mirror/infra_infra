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

	"go.chromium.org/chromiumos/config/go/longrunning"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/lro"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

type PublishServiceServer struct {
	api.UnimplementedGenericPublishServiceServer

	concreteService PublishService

	serverLogger *log.Logger
}

type PublishService interface {
	Publish(ctx context.Context, log *log.Logger, req *api.PublishRequest) (*api.PublishResponse, error)
}

func NewServer(logger *log.Logger, concreteService PublishService) *grpc.Server {
	s := &PublishServiceServer{
		concreteService: concreteService,
		serverLogger:    logger,
	}

	server := grpc.NewServer(grpc.MaxRecvMsgSize(common.MaxPublishMsgSize), grpc.MaxSendMsgSize(common.MaxPublishMsgSize))
	api.RegisterGenericPublishServiceServer(server, s)
	reflection.Register(server)

	return server
}

func (s *PublishServiceServer) Publish(ctx context.Context, req *api.PublishRequest) (resp *longrunning.Operation, err error) {
	defer CapturePanic(s.serverLogger, &err)
	manager := lro.New()
	defer manager.Close()
	resp = manager.NewOperation()

	s.serverLogger.Printf("Received Start Request: %s", req)

	publishResp, err := s.concreteService.Publish(ctx, s.serverLogger, req)
	if err != nil {
		s.serverLogger.Printf("Failed to execute `Start`: %s", err)
		return
	}

	if err := manager.SetResult(resp.Name, publishResp); err != nil {
		log.Printf("Unable to set output result: %s", err)
	}

	return
}

func CapturePanic(logger *log.Logger, err *error) {
	if r := recover(); r != nil {
		richError := fmt.Errorf("%s\n%s", r, string(debug.Stack()))
		*err = richError
	}
}
