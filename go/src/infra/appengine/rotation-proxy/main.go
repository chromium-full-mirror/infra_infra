// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// package main implements the App Engine based HTTP server to handle request
// to Rotation Proxy
package main

import (
	"context"
	"net/http"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/gaeemulation"
	"go.chromium.org/luci/server/module"
	"go.chromium.org/luci/server/router"

	rpb "go.chromium.org/infra/appengine/rotation-proxy/proto"
)

var (
	rotationExporterServiceAccount = "user:chrome-ops-rotation-exporter@system.gserviceaccount.com"
)

func checkAPIAccess(ctx context.Context, methodName string, req proto.Message) (context.Context, error) {
	identity := string(auth.CurrentIdentity(ctx))
	permissionErr := status.Errorf(codes.PermissionDenied, "%s does not have access to method %s of Rotation Proxy", identity, methodName)
	if methodName == "BatchUpdateRotations" {
		if identity != rotationExporterServiceAccount {
			return nil, permissionErr
		}
		return ctx, nil
	}
	hasAccess, err := auth.IsMember(ctx, "rotation-proxy-access")
	if err != nil {
		return nil, err
	}
	if !hasAccess {
		return nil, permissionErr
	}
	return ctx, nil
}

// checkAccess is middleware that checks if the user is authorized to
// call HTTP endpoint of rotation proxy.
func checkHttpAccess(ctx *router.Context, next router.Handler) {
	c := ctx.Request.Context()
	hasAccess, err := auth.IsMember(c, "rotation-proxy-access")
	if err != nil {
		logging.Errorf(ctx.Request.Context(), "error in checking membership %s", err.Error())
		http.Error(ctx.Writer, "Error in authorizing the user.", http.StatusInternalServerError)
		return
	}
	if !hasAccess {
		// TODO(nqmtuan): Remove logging and return permission error.
		// For now, just log and and continue.
		c := ctx.Request.Context()
		identity := string(auth.CurrentIdentity(c))
		logging.Errorf(c, "%s does not have access to rotation proxy", identity)
		next(ctx)
	} else {
		next(ctx)
	}
}

func main() {
	modules := []module.Module{
		gaeemulation.NewModuleFromFlags(),
	}

	server.Main(nil, modules, func(srv *server.Server) error {
		// Install regular http service
		srv.Routes.GET("/current/:name", router.MiddlewareChain{checkHttpAccess}, func(c *router.Context) {
			GetCurrentShiftHandler(c)
		})

		// Install PRPC service
		rpb.RegisterRotationProxyServiceServer(srv, &rpb.DecoratedRotationProxyService{
			Service: &RotationProxyServer{},
			Prelude: checkAPIAccess,
		})
		return nil
	})
}
