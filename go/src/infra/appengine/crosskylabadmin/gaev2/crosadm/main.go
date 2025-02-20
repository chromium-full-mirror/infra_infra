// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This is the main entrypoint for the GAEv2 version of CrOSSkylabAdmin.
// As of right now, it is not functional.
package main

import (
	"google.golang.org/grpc"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/config/server/cfgmodule"
	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/gaeemulation"
	"go.chromium.org/luci/server/module"

	"go.chromium.org/infra/appengine/crosskylabadmin/api/fleet/v1"
	"go.chromium.org/infra/appengine/crosskylabadmin/internal/app/frontend"
)

// Main is the entrypoint for the GAEv2 version of CrOSSkylabAdmin.
func main() {
	modules := []module.Module{
		gaeemulation.NewModuleFromFlags(),
		cfgmodule.NewModuleFromFlags(),
	}

	server.Main(nil, modules, func(srv *server.Server) error {
		logging.Infof(srv.Context, "Installing services.")
		installServices(srv)
		logging.Infof(srv.Context, "Finished installing services.")
		return nil
	})
}

// Install the CrOSSkylabAdminServices into a prpc registrar.
func installServices(r grpc.ServiceRegistrar) {
	fleet.RegisterTrackerServer(r, &fleet.DecoratedTracker{
		Service: &frontend.TrackerServerImpl{
			SwarmingFactory: nil,
		},
		Prelude: frontend.CheckAccess,
	})
	// The primary use case for this API is the stable version API.
	fleet.RegisterInventoryServer(r, &fleet.DecoratedInventory{
		Service: &frontend.ServerImpl{},
		Prelude: frontend.CheckAccess,
	})
}
