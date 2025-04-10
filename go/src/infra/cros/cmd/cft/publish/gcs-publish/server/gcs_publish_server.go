// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// GRPC Server impl
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

	"go.chromium.org/infra/cros/cmd/cft/common/portdiscovery"
	"go.chromium.org/infra/cros/cmd/cft/publish/commonutils/metadata"
	"go.chromium.org/infra/cros/cmd/cft/publish/gcs-publish/service"
)

type GcsPublishServer struct {
	options *metadata.ServerMetadata
	manager *lro.Manager
	server  *grpc.Server
}

func NewGcsPublishServer(options *metadata.ServerMetadata) (*GcsPublishServer, func(), error) {
	var conns []*grpc.ClientConn
	closer := func() {
		for _, conn := range conns {
			conn.Close()
		}
		conns = nil
	}

	return &GcsPublishServer{
		options: options,
	}, closer, nil
}

func (ps *GcsPublishServer) Start() error {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", ps.options.Port))
	if err != nil {
		return fmt.Errorf("failed to create listener at %d", ps.options.Port)
	}

	// Write port number to ~/.cftmeta for go/cft-port-discovery
	err = portdiscovery.WriteServiceMetadata("gcs-publish", l.Addr().String(), nil)
	if err != nil {
		log.Println("Warning: error when writing to metadata file: ", err)
	}

	ps.manager = lro.New()
	defer ps.manager.Close()

	ps.server = grpc.NewServer()
	api.RegisterGenericPublishServiceServer(ps.server, ps)
	longrunning.RegisterOperationsServer(ps.server, ps.manager)
	reflection.Register(ps.server)

	log.Println("gcs-publish-service listen to request at ", l.Addr().String())
	return ps.server.Serve(l)
}

func (ps *GcsPublishServer) Publish(ctx context.Context, req *api.PublishRequest) (*longrunning.Operation, error) {
	log.Println("Received api.PublishRequest: ", req)
	op := ps.manager.NewOperation()
	out := &api.PublishResponse{
		Status: api.PublishResponse_STATUS_SUCCESS,
	}

	defer func() {
		ps.manager.SetResult(op.Name, out)
	}()

	gps, err := service.NewGcsPublishService(ctx, req)
	if err != nil {
		log.Printf("failed to create new gcs publish service: %s", err)
		out.Status = api.PublishResponse_STATUS_INVALID_REQUEST
		out.Message = fmt.Sprintf("failed to create new gcs publish service: %s", err.Error())
		return op, fmt.Errorf("failed to create new gcs publish service: %s", err)
	}

	if err := gps.UploadToGS(ctx); err != nil {
		log.Printf("upload to gs failed: %s", err)
		out.Status = api.PublishResponse_STATUS_FAILURE
		out.Message = fmt.Sprintf("failed upload to gs: %s", err.Error())
		return op, fmt.Errorf("failed upload to gs: %s", err)
	}

	if gps.EnableXTSArchiver {
		if err := gps.ArchiveXTSResults(ctx); err != nil {
			log.Printf("XTS archiver failed: %s", err)
			out.Status = api.PublishResponse_STATUS_FAILURE
			out.Message = fmt.Sprintf("failed to archive XTS results: %s", err.Error())
			return op, fmt.Errorf("failed to archive XTS results: %s", err)
		}
	}

	log.Println("Finished Successfuly!")
	return op, nil
}
