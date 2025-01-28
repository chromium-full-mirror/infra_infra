// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"regexp"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/router"

	api "infra/unifiedfleet/api/v1/rpc"
	"infra/unifiedfleet/app/acl"
	"infra/unifiedfleet/app/config"
	"infra/unifiedfleet/app/untrusted"
	"infra/unifiedfleet/app/util"
)

// AllowOriginAllAndCustomHeaders is similar to LUCI's prpc.AllowOriginAll
// function but includes custom Namespace header used by UFS. Allows
// web clients to set Namespace for UFS.
func AllowOriginAllAndCustomHeaders(ctx context.Context, origin string) prpc.AccessControlDecision {
	return prpc.AccessControlDecision{
		AllowCrossOriginRequests: true,
		AllowCredentials:         true,
		AllowHeaders:             []string{util.Namespace},
	}
}

// InstallServices installs ...
func InstallServices(apiServer *server.Server) {
	apiServer.ConfigurePRPC(func(p *prpc.Server) {
		p.AccessControl = AllowOriginAllAndCustomHeaders
		// Some clients send field masks as JSON dict, not as strings. This requires
		// enabling non-standard field masks semantics.
		p.EnableNonStandardFieldMasks = true
	})
	api.RegisterFleetServer(apiServer, &api.DecoratedFleet{
		Service: &FleetServerImpl{},
		Prelude: checkAccess,
	})
}

// InstallHandlers installs non PRPC handlers
func InstallHandlers(r *router.Router, mc router.MiddlewareChain) {
	mc = mc.Extend(func(ctx *router.Context, next router.Handler) {
		context, err := checkAccess(ctx.Request.Context(), ctx.HandlerPath, nil)
		ctx.Request = ctx.Request.WithContext(context)
		if err != nil {
			logging.Errorf(ctx.Request.Context(), "Failed authorization %v", err)
			return
		}
		next(ctx)
	})
	r.POST("/pubsub/hart", mc, HaRTPushHandler)
	r.POST(untrusted.VerifierEndpoint, mc, untrusted.DeploymentVerifier)
}

// checkAccess verifies that the request is from an authorized user.
func checkAccess(ctx context.Context, rpcName string, _ proto.Message) (context.Context, error) {
	logging.Debugf(ctx, "Check access for %s", rpcName)
	// Everyone can call the RPC to check fleet test policy
	if rpcName == "CheckFleetTestsPolicy" {
		return ctx, nil
	}
	group := acl.Resolve(rpcName)
	allow, err := auth.IsMember(ctx, group...)
	if err != nil {
		logging.Errorf(ctx, "Check group '%s' membership failed: %s", group, err.Error())
		return ctx, status.Errorf(codes.Internal, "can't check access group membership: %s", err)
	}
	if !allow {
		return ctx, status.Errorf(codes.PermissionDenied, "%s is not a member of %s", auth.CurrentIdentity(ctx), group)
	}
	logging.Infof(ctx, "%s is a member of %s", auth.CurrentIdentity(ctx), group)
	return ctx, nil
}

// Determines if a given email is an @google.com email
var googlerRegex = regexp.MustCompile(`^.*@google.com$`)

// isGoogler determines if the context belongs to a google authed account.
func isGoogler(ctx context.Context) bool {
	id := string(auth.CurrentIdentity(ctx))
	return googlerRegex.MatchString(id)
}

// isAllowlistedRPC determines if a RPC call is allowlisted for arbitrary
// namespace access in the configuration.
func isAllowlistedRPC(ctx context.Context, info *grpc.UnaryServerInfo) bool {
	method := info.FullMethod
	cfg := config.Get(ctx)

	for _, v := range cfg.PartnerRPCAllowlist {
		if method == v {
			return true
		}
	}

	return false
}

// PartnerInterceptor rejects any calls from partner accounts that don't use
// the os-partner namespace. Relies on having a namespace set and should only
// be called after another interceptor that calls `SetupDatastoreNamespace` or
// an equivalent.
func PartnerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	// ignore this check for google emails, needed as some @google.com accounts
	// could be in the CRIA groups used to determine partners.
	if isGoogler(ctx) {
		return handler(ctx, req)
	}

	if isAllowlistedRPC(ctx, info) {
		return handler(ctx, req)
	}

	cfg := config.Get(ctx)
	sfpGroups := cfg.PartnerACLGroups
	sfpMember, err := auth.IsMember(ctx, sfpGroups...)
	if err != nil {
		logging.Errorf(ctx, "Error fetching auth info: %s", err)
		return nil, status.Error(codes.Internal, "error fetching auth perms")
	}

	ns := util.GetDatastoreNamespace(ctx)

	if sfpMember && ns != util.OSPartnerNamespace {
		logging.Errorf(ctx, "Blocking caller: %s making RPC: %s in namespace: %s", auth.CurrentUser(ctx), info.FullMethod, ns)
		return nil, status.Error(codes.PermissionDenied, "partners only have access to `os-partner` namespace")
	}

	return handler(ctx, req)
}
