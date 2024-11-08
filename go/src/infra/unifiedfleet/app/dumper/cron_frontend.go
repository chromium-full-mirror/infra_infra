// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dumper

import (
	"context"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/auth"

	api "infra/unifiedfleet/api/v1/cron"
)

// InstallCronServices installs ...
func InstallCronServices(apiServer *server.Server) {
	apiServer.ConfigurePRPC(func(p *prpc.Server) { p.AccessControl = prpc.AllowOriginAll })
	api.RegisterCronServer(apiServer, &api.DecoratedCron{
		Service: &CronServerImpl{},
		Prelude: checkCronAccess,
	})
}

func checkCronAccess(ctx context.Context, rpcName string, _ proto.Message) (context.Context, error) {
	logging.Debugf(ctx, "Check access for %s", rpcName)
	// Only we trigger the cron jobs.
	// Distributed fleet temporarily allowed during read realms rollout.
	group := []string{"mdb/chrome-fleet-software-team", "mdb/chromeos-distributed-fleet-platform"}
	allow, err := auth.IsMember(ctx, group...)
	if err != nil {
		logging.Errorf(ctx, "Check group %q membership failed: %s", group, err.Error())
		return ctx, status.Errorf(codes.Internal, "can't check access group membership: %s", err)
	}
	if !allow {
		return ctx, status.Errorf(codes.PermissionDenied, "%s is not a member of %s", auth.CurrentIdentity(ctx), group)
	}
	logging.Infof(ctx, "%s is a member of %s", auth.CurrentIdentity(ctx), group)
	return ctx, nil
}
