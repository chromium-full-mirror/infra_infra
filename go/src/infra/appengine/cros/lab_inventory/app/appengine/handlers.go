// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"go.chromium.org/luci/config/server/cfgmodule"
	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/gaeemulation"
	"go.chromium.org/luci/server/module"

	"go.chromium.org/infra/appengine/cros/lab_inventory/app/cron"
	"go.chromium.org/infra/appengine/cros/lab_inventory/app/frontend"
)

func main() {

	modules := []module.Module{
		gaeemulation.NewModuleFromFlags(),
		cfgmodule.NewModuleFromFlags(),
	}
	server.Main(nil, modules, func(srv *server.Server) error {
		frontend.InstallServices(srv)
		cron.InstallHandlers(srv.Routes, nil)
		return nil
	})
}
