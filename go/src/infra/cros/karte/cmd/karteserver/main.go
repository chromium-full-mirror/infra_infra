// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

// This is the entrypoint for the Karte service in production and dev.
// Control is transferred here, inside the Docker container, when the
// application starts.

import (
	"net/http"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/option"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/config/server/cfgmodule"
	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/cron"
	"go.chromium.org/luci/server/gaeemulation"
	"go.chromium.org/luci/server/module"

	"go.chromium.org/infra/cros/karte/internal/externalclients"
	"go.chromium.org/infra/cros/karte/internal/frontend"
	"go.chromium.org/infra/cros/karte/internal/identifiers"
	"go.chromium.org/infra/libs/bqwrapper"
)

// Transfer control to the LUCI server
//
// NOTE: if you are running this code locally, you need to set an explicit project
// using an environment variable.
func main() {
	modules := []module.Module{
		gaeemulation.NewModuleFromFlags(),
		cfgmodule.NewModuleFromFlags(),
		cron.NewModuleFromFlags(),
	}

	server.Main(nil, modules, func(srv *server.Server) error {
		t, err := auth.GetRPCTransport(srv.Context, auth.AsSelf, auth.WithScopes(auth.CloudOAuthScopes...))
		if err != nil {
			return err
		}
		logging.Infof(srv.Context, "Installing dependencies into context")
		srv.Context = identifiers.Use(srv.Context, identifiers.NewDefault())
		bqClient, err := bigquery.NewClient(srv.Context, srv.Options.CloudProject, option.WithHTTPClient(&http.Client{Transport: t}))
		if err != nil {
			return err
		}
		client := bqwrapper.NewCloudBQ(bqClient)
		srv.Context = externalclients.UseBQ(srv.Context, client)
		logging.Infof(srv.Context, "Starting server.")
		logging.Infof(srv.Context, "Installing Services.")
		k := frontend.NewKarteFrontend(srv.Options.CloudProject)
		frontend.InstallServices(k, srv)
		logging.Infof(srv.Context, "Initialization finished.")
		return nil
	})
}
