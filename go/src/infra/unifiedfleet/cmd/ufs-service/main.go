// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"regexp"
	"strings"

	"cloud.google.com/go/profiler"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/openid"
	"go.chromium.org/luci/server/gaeemulation"
	"go.chromium.org/luci/server/module"
	"go.chromium.org/luci/server/router"

	version_compare "infra/libs/version"
	"infra/unifiedfleet/app/acl"
	"infra/unifiedfleet/app/config"
	"infra/unifiedfleet/app/external"
	"infra/unifiedfleet/app/frontend"
	"infra/unifiedfleet/app/untrusted"
	"infra/unifiedfleet/app/util"
)

// flag to control erroring out if namespace is not provided
const rejectCallforNamespace = false

func main() {
	modules := []module.Module{
		gaeemulation.NewModuleFromFlags(),
	}

	cfgLoader := config.Loader{}
	cfgLoader.RegisterFlags(flag.CommandLine)

	server.Main(nil, modules, func(srv *server.Server) error {
		// We closely follow the profiler-enabling documentation available at the following URL:
		// https://cloud.google.com/profiler/docs/profiling-go#enabling-profiler-api
		cfg := profiler.Config{
			Service: "ufs-service",
			// TODO(gregorynisbet): replace with commit hash or some other smarter way of getting
			//                      the UFS version.
			ServiceVersion:    "1.0.0",
			EnableOCTelemetry: false,
			ProjectID:         srv.Options.CloudProject,
		}

		profilerStartErr := profiler.Start(cfg)
		// TODO(gregorynisbet): Upgrade this to a panic once enabling the profiler is reliable enough in prod.
		if profilerStartErr == nil {
			logging.Infof(srv.Context, "profiler started successfully: 848356fa-b429-4e0e-bc63-a8cfbe3b5b75")
		} else {
			logging.Errorf(srv.Context, "%s\n", errors.Annotate(profilerStartErr, "error encountered when setting up profiler during startup").Err())
		}

		// Load service config form a local file (deployed via GKE),
		// periodically reread it to pick up changes without full restart.
		if _, err := cfgLoader.Load(srv.Context); err != nil {
			return err
		}
		srv.RunInBackground("ufs.config", cfgLoader.ReloadLoop)

		acl.Register(cfgLoader.Config())
		srv.Context = config.Use(srv.Context, cfgLoader.Config())
		srv.Context = external.WithServerInterface(srv.Context)

		var err error
		srv.Context, err = external.UsePubSub(srv.Context, srv.Options.CloudProject)
		if err != nil {
			// If we fail to set up PubSub then UFS will not work properly anyway.
			// The exact error message is very important. If we panic here, that will guarantee that the user sees the whole thing.
			// See b:267829708 for details.
			panic(err)
		}

		srv.RegisterUnaryServerInterceptors(versionInterceptor, namespaceInterceptor, frontend.PartnerInterceptor)
		frontend.InstallServices(srv)

		// Add authenticator for handling JWT tokens. This is required to
		// authenticate PubSub push responses sent as HTTP POST requests. See
		// https://cloud.google.com/pubsub/docs/push?hl=en#authentication_and_authorization
		openIDCheck := auth.Authenticator{
			Methods: []auth.Method{
				&openid.GoogleIDTokenAuthMethod{
					AudienceCheck: openid.AudienceMatchesHost,
				},
			},
		}
		frontend.InstallHandlers(srv.Routes, router.NewMiddlewareChain(openIDCheck.GetMiddleware()))
		untrusted.EnsureVerifierSubscription(srv.Context)
		return nil
	})
}

// namespaceInterceptor interceptor to set namespace for the datastore
func namespaceInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "Retrieving metadata failed.")
	}

	// TODO(eshwarn): this is to check http request from device ticketfiler
	// remove this once verified. Used only for logging messages
	var v, n string
	version, ok := md["user-agent"]
	if ok {
		v = version[0]
	}

	namespace, ok := md[util.Namespace]
	if ok {
		// TODO(eshwarn): this is to check http request from device ticketfiler
		// remove this once verified. Used only for logging messages
		n = namespace[0]

		ns := strings.ToLower(namespace[0])
		datastoreNamespace, ok := util.ClientToDatastoreNamespace[ns]
		if ok {
			ctx, err = util.SetupDatastoreNamespace(ctx, datastoreNamespace)
			if err != nil {
				return nil, err
			}
		} else if rejectCallforNamespace {
			return nil, status.Errorf(codes.InvalidArgument, "namespace %s in the context metadata is invalid. Valid namespaces: [%s]", namespace[0], strings.Join(util.ValidClientNamespaceStr(), ", "))
		}
	} else if rejectCallforNamespace {
		return nil, status.Errorf(codes.InvalidArgument, "namespace is not set in the context metadata. Valid namespaces: [%s]", strings.Join(util.ValidClientNamespaceStr(), ", "))
	}

	// TODO(eshwarn): this is to check http request from device ticketfiler
	// remove this once verified.
	logging.Debugf(ctx, "user-agent = %s and namespace = %s", v, n)

	resp, err = handler(ctx, req)
	return
}

// versionInterceptor interceptor to handle client version check per RPC call
func versionInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "Retrieving metadata failed.")
	}
	user, userAgentExists, userAgentErr := validateUserAgent(ctx, md)
	if userAgentExists && userAgentErr != nil {
		return nil, userAgentErr
	}
	if !userAgentExists {
		return nil, status.Errorf(codes.InvalidArgument, "user-agent is not specified in the incoming request")
	}
	defer func() {
		code := codes.OK
		if err != nil {
			code = grpc.Code(err)
		}
		ufsGRPCServerCount.Add(ctx, 1, info.FullMethod, int(code), user)
	}()
	if blockSkylabWritesToMachineLSE(info, user) {
		logging.Infof(ctx, "Blocking useragent: %s RPC: %s", user, info.FullMethod)
		return nil, status.Errorf(codes.PermissionDenied, "blocking skylab writes to UFS MachineLSE")
	}
	logging.Debugf(ctx, "Successfully pass user-agent version check for user %s", user)
	resp, err = handler(ctx, req)
	return
}

// Assuming the version number for major, minor and patch are less than 1000.
var versionRegex = regexp.MustCompile(`[0-9]{1,3}`)

// validateUserAgent returns a tuple
//
//	(if user-agent exists, if user-agent is valid)
func validateUserAgent(ctx context.Context, md metadata.MD) (string, bool, error) {
	cfg := config.Get(ctx)
	if cfg == nil {
		return "", false, status.Errorf(codes.Unavailable,
			"Config not found, Try again in a few minutes")
	}
	version, ok := md["user-agent"]
	// Only check version for skylab commands which already set user-agent
	if ok {
		if len(version) == 0 || version[0] == "" {
			return version[0], ok, status.Errorf(codes.FailedPrecondition, "User agent doesn't advertise itself")
		}
		// TODO(xixuan): remove this check
		// Traffic from trawler has a default userAgent "Googlebot/2.1" if no special userAgent is approved yet.
		// So before b/179652204 is approved, temporarily allow all traffic from trawler.
		if strings.Contains(version[0], "Googlebot") {
			return version[0], ok, nil
		}
		for _, client := range cfg.Clients {
			if strings.Contains(
				strings.ToLower(version[0]),
				strings.ToLower(client.GetName())) {
				if version_compare.GEQ(version[0], client.GetVersion()) {
					return version[0], ok, nil
				} else {
					return "", ok, status.Errorf(codes.FailedPrecondition,
						fmt.Sprintf("Unsupported client version %s, Please update %s to %s or above", version[0], client.GetName(), client.GetVersion()))
				}
			}
		}
		// Default action
		if cfg.AllowUnrecognizedClients {
			logging.Warningf(ctx, "Allow client %s", version)
			return "", ok, nil
		} else {
			return "", ok, status.Errorf(codes.FailedPrecondition, fmt.Sprintf("Unsupported client %s", version[0]))
		}
	}
	return version[0], ok, nil
}

// This is to block older version of skylab tool(deprecated) only from updating MachineLSE in UFS.
// Does not block skylab_swarming_worker, skylab_local_state, shivas and other clients.
func blockSkylabWritesToMachineLSE(info *grpc.UnaryServerInfo, userAgent string) bool {
	if strings.Contains(userAgent, "skylab/") &&
		(strings.Contains(info.FullMethod, "CreateMachineLSE") ||
			strings.Contains(info.FullMethod, "UpdateMachineLSE") ||
			strings.Contains(info.FullMethod, "DeleteMachineLSE")) {
		return true
	}
	return false
}
