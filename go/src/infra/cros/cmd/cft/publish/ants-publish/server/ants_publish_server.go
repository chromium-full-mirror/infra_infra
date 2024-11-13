// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package server implements a GRPC Server.
package server

import (
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"go.chromium.org/chromiumos/config/go/longrunning"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/lro"
	"go.chromium.org/chromiumos/test/publish/cmd/common-utils/metadata"
	"go.chromium.org/chromiumos/test/util/portdiscovery"

	"infra/cros/cmd/cft/publish/ants-publish/service"
)

const (
	maxGRPCSize = 1024 * 1024 * 400
)

type AntsPublishServer struct {
	options *metadata.ServerMetadata
	manager *lro.Manager
	server  *grpc.Server
}

func NewAntsPublishServer(options *metadata.ServerMetadata) (*AntsPublishServer, func(), error) {
	var conns []*grpc.ClientConn
	closer := func() {
		for _, conn := range conns {
			_ = conn.Close()
		}
		conns = nil
	}

	return &AntsPublishServer{
		options: options,
	}, closer, nil
}

func (ps *AntsPublishServer) Start() error {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", ps.options.Port))
	if err != nil {
		return fmt.Errorf("failed to create listener at %d", ps.options.Port)
	}

	// Write port number to ~/.cftmeta for go/cft-port-discovery
	err = portdiscovery.WriteServiceMetadata("ants-publish", l.Addr().String(), nil)
	if err != nil {
		log.Println("Warning: error when writing to metadata file: ", err)
	}

	ps.manager = lro.New()
	defer ps.manager.Close()

	ps.server = grpc.NewServer(grpc.MaxRecvMsgSize(maxGRPCSize), grpc.MaxSendMsgSize(maxGRPCSize))
	api.RegisterGenericPublishServiceServer(ps.server, ps)
	reflection.Register(ps.server)

	log.Println("ants-publish-service listen to request at ", l.Addr().String())
	return ps.server.Serve(l)
}

func (ps *AntsPublishServer) Publish(ctx context.Context, req *api.PublishRequest) (*longrunning.Operation, error) {
	log.Println("Received api.PublishRequest: ", req)
	op := ps.manager.NewOperation()
	out := &api.PublishResponse{
		Status: api.PublishResponse_STATUS_SUCCESS,
	}

	defer func() {
		if err := ps.manager.SetResult(op.Name, out); err != nil {
			log.Println("Unable to set output result: ", err)
		}
	}()

	aps, err := service.NewAntsPublishService(ctx, req)
	if err != nil {
		log.Printf("failed to create new ants publish service: %s", err)
		out.Status = api.PublishResponse_STATUS_INVALID_REQUEST
		out.Message = fmt.Sprintf("failed to create new ants publish service: %s", err.Error())
		return op, fmt.Errorf("failed to create new ants publish service: %w", err)
	}

	if err := aps.UploadToAnts(context.Background()); err != nil {
		log.Printf("upload to ants failed: %s", err)
		out.Status = api.PublishResponse_STATUS_FAILURE
		out.Message = fmt.Sprintf("failed upload to ants: %s", err.Error())
		return op, fmt.Errorf("failed upload to ants: %w", err)
	}

	if err := aps.UploadArtifacts(context.Background()); err != nil {
		log.Printf("upload artifacts to ants failed: %s", err)
		out.Status = api.PublishResponse_STATUS_FAILURE
		out.Message = fmt.Sprintf("failed upload artifacts to ants: %s", err.Error())
		return op, fmt.Errorf("failed artifacts upload to ants: %w", err)
	}

	log.Println("Finished Successfuly!")
	return op, nil
}
