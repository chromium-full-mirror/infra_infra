// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main is the entrypoint to BotsRegulator.
package main

import (
	"context"
	"flag"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/config/server/cfgmodule"
	"go.chromium.org/luci/server"
	scron "go.chromium.org/luci/server/cron"
	"go.chromium.org/luci/server/module"

	"go.chromium.org/infra/cros/botsregulator/internal/cron"
	"go.chromium.org/infra/cros/botsregulator/internal/regulator"
)

// migrateSeenInfo caches the last successful migrate-bots run.
var migrateSeenInfo cron.LastSeenConfig

func main() {
	mods := []module.Module{
		scron.NewModuleFromFlags(),
		cfgmodule.NewModule(&cfgmodule.ModuleOptions{ServiceHost: "config.luci.app"}),
	}

	r := regulator.RegulatorOptions{}
	r.RegisterFlags(flag.CommandLine)

	server.Main(nil, mods, func(srv *server.Server) error {
		logging.Infof(srv.Context, "starting server")

		scron.RegisterHandler("regulate-bots", func(ctx context.Context) error {
			ctx = logging.SetField(ctx, "activity", "regulate-bots")
			return cron.Regulate(ctx, &r)
		})
		scron.RegisterHandler("migrate-bots", func(ctx context.Context) error {
			ctx = logging.SetField(ctx, "activity", "migrate-bots")
			return cron.Migrate(ctx, &r, &migrateSeenInfo)
		})
		scron.RegisterHandler("cleanup-bots", func(ctx context.Context) error {
			ctx = logging.SetField(ctx, "activity", "cleanup-bots")
			return cron.Cleanup(ctx, &r)
		})
		return nil
	})
}
