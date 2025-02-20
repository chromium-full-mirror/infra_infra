// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"flag"

	"google.golang.org/grpc"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/openid"

	"go.chromium.org/infra/vm_leaser/internal/acl"
	"go.chromium.org/infra/vm_leaser/internal/frontend"
)

// InstallServices takes a VM Leaser service server and exposes it to a
// LUCI prpc.Server.
func InstallServices(s *frontend.Server, srv grpc.ServiceRegistrar) {
	api.RegisterVMLeaserServiceServer(srv, s)
}

func main() {
	srvEnv := flag.String(
		"service-env",
		"dev",
		"The service deployment environment (dev/prod).",
	)

	server.Main(nil, nil, func(srv *server.Server) error {
		logging.Infof(srv.Context, "Starting server.")

		// This allows auth to use Identity tokens.
		srv.SetRPCAuthMethods([]auth.Method{
			// The primary authentication method.
			&openid.GoogleIDTokenAuthMethod{
				AudienceCheck: openid.AudienceMatchesHost,
				SkipNonJWT:    true, // pass OAuth2 access tokens through
			},
			// Backward compatibility for RPC Explorer and old clients.
			&auth.GoogleOAuth2Method{
				Scopes: []string{"https://www.googleapis.com/auth/userinfo.email"},
			},
		})

		// Per-RPC authorization interceptor.
		srv.RegisterUnifiedServerInterceptors(acl.RPCAccessInterceptor)

		logging.Infof(srv.Context, "Installing Services.")
		InstallServices(frontend.NewServer(*srvEnv), srv)

		logging.Infof(srv.Context, "Initialization finished.")
		return nil
	})
}
