// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package frontend implements the drone queen service.
package frontend

import (
	"context"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/server/auth"

	"go.chromium.org/infra/appengine/drone-queen/api"
	"go.chromium.org/infra/appengine/drone-queen/internal/config"
)

// RegisterServers registers RPC servers.
func RegisterServers(srv grpc.ServiceRegistrar) {
	var q DroneQueenImpl
	api.RegisterDroneServer(srv, &api.DecoratedDrone{
		Service: &q,
		Prelude: checkDroneAccess,
	})
	api.RegisterInventoryProviderServer(srv, &api.DecoratedInventoryProvider{
		Service: &q,
		Prelude: checkInventoryProviderAccess,
	})
	api.RegisterInspectServer(srv, &api.DecoratedInspect{
		Service: &q,
		Prelude: checkInspectAccess,
	})
}

func checkDroneAccess(ctx context.Context, _ string, _ proto.Message) (context.Context, error) {
	g := config.Get(ctx).GetAccessGroups()
	allow, err := auth.IsMember(ctx, g.GetDrones())
	if err != nil {
		return ctx, status.Errorf(codes.Internal, "can't check access group membership: %s", err)
	}
	if !allow {
		return ctx, status.Errorf(codes.PermissionDenied, "permission denied")
	}
	return ctx, nil
}

func checkInventoryProviderAccess(ctx context.Context, _ string, _ proto.Message) (context.Context, error) {
	g := config.Get(ctx).GetAccessGroups()
	allow, err := auth.IsMember(ctx, g.GetInventoryProviders())
	if err != nil {
		return ctx, status.Errorf(codes.Internal, "can't check access group membership: %s", err)
	}
	if !allow {
		return ctx, status.Errorf(codes.PermissionDenied, "permission denied")
	}
	return ctx, nil
}

func checkInspectAccess(ctx context.Context, _ string, _ proto.Message) (context.Context, error) {
	g := config.Get(ctx).GetAccessGroups()
	allow, err := auth.IsMember(ctx, g.GetInspectors())
	if err != nil {
		return ctx, status.Errorf(codes.Internal, "can't check access group membership: %s", err)
	}
	if !allow {
		return ctx, status.Errorf(codes.PermissionDenied, "permission denied")
	}
	return ctx, nil
}
