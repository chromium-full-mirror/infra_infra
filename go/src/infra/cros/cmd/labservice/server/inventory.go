// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package server

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/labservice/internal/ufs"
	"go.chromium.org/infra/cros/cmd/labservice/internal/ufs/cache"
)

// inventoryServer implements the lab service RPCs.
type inventoryServer struct {
	labapi.UnimplementedInventoryServiceServer

	// The client needs a context which is request specific, so the client
	// needs to be created per incoming request.
	ufsClientFactory ufs.ClientFactory
	// cacheLocator is used to cache available caching servers across requests.
	cacheLocator *cache.Locator
}

// newInventoryServer creates a new inventory server.
func newInventoryServer(c *Config) *inventoryServer {
	l := cache.NewLocator()
	l.SetPreferredServices(c.PreferredCachingServices)
	return &inventoryServer{
		ufsClientFactory: ufs.ClientFactory{
			Service:            c.UFSService,
			ServiceAccountPath: c.ServiceAccountPath,
		},
		cacheLocator: l,
	}
}

// GetDutTopology gets the DUT topology for a given DUT.
func (s *inventoryServer) GetDutTopology(req *labapi.GetDutTopologyRequest, stream labapi.InventoryService_GetDutTopologyServer) error {
	ctx := stream.Context()
	id := req.GetId().GetValue()
	if id == "" {
		return status.Errorf(codes.InvalidArgument, "no id provided")
	}
	ufsClient, err := s.ufsClientFactory.NewClient(ctx)
	if err != nil {
		return status.Errorf(codes.Unknown, "%s", err)
	}
	// Cache locator is global and shared concurrently,
	// while ufs client is per request for call context
	inv := ufs.NewInventory(ufsClient, s.cacheLocator)
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
