// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package serverlib contains the main server loop and the modules used.
package serverlib

import (
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/gaeemulation"
	"go.chromium.org/luci/server/module"

	"infra/fleetconsole/internal/consoleserver"
)

// Modules is the slice of luci server modules used by the fleet console.
func Modules() []module.Module {
	return []module.Module{
		gaeemulation.NewModuleFromFlags(),
	}
}

// ServerMain is the server setup.
func ServerMain(srv *server.Server) error {
	logging.Infof(srv.Context, "Begin initialization of console server.")
	consoleFrontend := consoleserver.NewFleetConsoleFrontend()
	consoleserver.InstallServices(consoleFrontend, srv)
	logging.Infof(srv.Context, "End initialization of console server.")
	return nil
}
