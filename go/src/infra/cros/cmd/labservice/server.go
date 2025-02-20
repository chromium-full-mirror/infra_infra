// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	lsapi "go.chromium.org/infra/cros/cmd/labservice/api"
	"go.chromium.org/infra/cros/cmd/labservice/internal/ufs"
	"go.chromium.org/infra/cros/cmd/labservice/internal/ufs/cache"
)

// A server implements the lab service RPCs.
type server struct {
	labapi.UnimplementedInventoryServiceServer

	// The client needs a context which is request specific, so the client
	// needs to be created per incoming request.
	ufsClientFactory ufs.ClientFactory
	// cacheLocator is used to cache available caching servers across requests.
	cacheLocator *cache.Locator

	// Path to TKO database socket file.
	tkoSocket string
}

func newServer(c *serverConfig) *server {
	l := cache.NewLocator()
	l.SetPreferredServices(c.preferredCachingServices)
	return &server{
		ufsClientFactory: ufs.ClientFactory{
			Service:            c.ufsService,
			ServiceAccountPath: c.serviceAccountPath,
		},
		cacheLocator: l,
		tkoSocket:    c.tkoSocket,
	}
}

// A serverConfig configures newServer.
type serverConfig struct {
	preferredCachingServices []string
	serviceAccountPath       string
	tkoSocket                string
	ufsService               string
}

func (s *server) GetDutTopology(req *labapi.GetDutTopologyRequest, stream labapi.InventoryService_GetDutTopologyServer) error {
	ctx := stream.Context()
	id := req.GetId().GetValue()
	if id == "" {
		return status.Errorf(codes.InvalidArgument, "no id provided")
	}
	c, err := s.ufsClientFactory.NewClient(ctx)
	if err != nil {
		return status.Errorf(codes.Unknown, "%s", err)
	}
	// Cache locator is global and shared concurrently,
	// while ufs client is per request for call context
	inv := ufs.NewInventory(c, s.cacheLocator)
	dt, err := inv.GetDutTopology(ctx, id)
	if err != nil {
		// GetDutTopology adds the gRPC status.
		return err
	}
	return stream.Send(&labapi.GetDutTopologyResponse{
		Result: &labapi.GetDutTopologyResponse_Success_{
			Success: &labapi.GetDutTopologyResponse_Success{
				DutTopology: dt,
			},
		},
	})
}

func (s *server) GetTkoAddress(ctx context.Context, in *lsapi.GetTkoAddressRequest) (*lsapi.GetTkoAddressResponse, error) {
	return &lsapi.GetTkoAddressResponse{
		SocketPath: s.tkoSocket,
	}, nil
}
