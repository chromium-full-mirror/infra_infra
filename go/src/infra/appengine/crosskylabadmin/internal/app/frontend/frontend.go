// Copyright 2018 The LUCI Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// package frontend exposes the primary pRPC API of crosskylabadmin app.

package frontend

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/appengine/gaeauth/server"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	configpb "go.chromium.org/luci/common/proto/config"
	"go.chromium.org/luci/config/appengine/gaeconfig"
	"go.chromium.org/luci/config/server/cfgmodule"
	"go.chromium.org/luci/config/validation"
	"go.chromium.org/luci/grpc/discovery"
	"go.chromium.org/luci/grpc/grpcmon"
	"go.chromium.org/luci/grpc/grpcutil"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/signing"
	"go.chromium.org/luci/server/router"

	fleet "go.chromium.org/infra/appengine/crosskylabadmin/api/fleet/v1"
	"go.chromium.org/infra/appengine/crosskylabadmin/internal/app/config"
)

// SupportedClientMajorVersionNumber indicates the minimum major client version
const SupportedClientMajorVersionNumber = 2

// InstallHandlers installs the handlers implemented by the frontend package.
func InstallHandlers(r *router.Router, mwBase router.MiddlewareChain) {
	api := prpc.Server{
		UnaryServerInterceptor: grpcutil.ChainUnaryServerInterceptors(
			grpcmon.UnaryServerInterceptor,
			grpcutil.UnaryServerPanicCatcherInterceptor,
			auth.AuthenticatingInterceptor([]auth.Method{
				&server.OAuth2Method{Scopes: []string{server.EmailScope}},
			}).Unary(),
			versionInterceptor,
		),
	}
	fleet.RegisterTrackerServer(&api, &fleet.DecoratedTracker{
		Service: &TrackerServerImpl{},
		Prelude: CheckAccess,
	})
	fleet.RegisterInventoryServer(&api, &fleet.DecoratedInventory{
		Service: &ServerImpl{},
		Prelude: CheckAccess,
	})
	configpb.RegisterConsumerServer(&api, &cfgmodule.ConsumerServer{
		Rules: &validation.Rules,
		GetConfigServiceAccountFn: func(ctx context.Context) (string, error) {
			settings, err := gaeconfig.FetchCachedSettings(ctx)
			switch {
			case err != nil:
				return "", err
			case settings.ConfigServiceHost == "":
				return "", errors.New("can not find config service host from settings")
			}
			info, err := signing.FetchServiceInfoFromLUCIService(ctx, "https://"+settings.ConfigServiceHost)
			if err != nil {
				return "", err
			}
			return info.ServiceAccountName, nil
		},
	})

	discovery.Enable(&api)
	api.InstallHandlers(r, mwBase)
}

// CheckAccess verifies that the request is from an authorized user.
//
// Servers should use checkAccess as a Prelude while handling requests to
// uniformly check access across the API.
func CheckAccess(c context.Context, _ string, _ proto.Message) (context.Context, error) {
	switch allow, err := auth.IsMember(c, config.Get(c).AccessGroup); {
	case err != nil:
		return c, status.Errorf(codes.Internal, "can't check ACL - %s", err)
	case !allow:
		return c, status.Errorf(codes.PermissionDenied, "permission denied")
	}
	return c, nil
}

// Assuming the version number for major, minor and patch are less than 1000.
var versionRegex = regexp.MustCompile(`[0-9]{1,3}`)

// versionInterceptor interceptor to handle client version check per RPC call
func versionInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	logging.Debugf(ctx, "version check based on metadata %#v", md)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "Retrieving metadata failed.")
	}
	version, ok := md["user-agent"]
	// Only check version for skylab commands which already set user-agent
	if ok && strings.HasPrefix(version[0], "skylab/") {
		majors := versionRegex.FindAllString(version[0], 1)
		if len(majors) != 1 {
			return nil, status.Errorf(codes.InvalidArgument, "user-agent %s doesn't contain major version", version[0])
		}
		major, err := strconv.ParseInt(majors[0], 10, 32)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "user-agent %s has wrong major version format", version[0])
		}
		if major < SupportedClientMajorVersionNumber {
			return nil, status.Errorf(codes.FailedPrecondition,
				"Unsupported client version. Please update your client version to v%d.X.X or above.", SupportedClientMajorVersionNumber)
		}
	}

	// Calls the handler
	return handler(ctx, req)
}
