// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cron implements handlers for appengine cron targets in this app.
//
// All actual logic related to fleet management should be implemented in the
// main fleet API. These handlers should only encapsulate the following bits of
// logic:
//
//   - Calling other API as the appengine service account user.
//   - Translating luci-config driven admin task parameters.
package cron

import (
	"context"
	"time"

	"go.chromium.org/luci/server/cron"

	"go.chromium.org/infra/appengine/drone-queen/internal/config"
	"go.chromium.org/infra/appengine/drone-queen/internal/middleware"
	"go.chromium.org/infra/appengine/drone-queen/internal/queries"
)

// InstallHandlers installs global handlers for cron jobs that are part of this app.
func InstallHandlers() {
	install := func(id string, h cron.Handler) {
		cron.RegisterHandler(id, chain(
			h,
			middleware.CronTrace(id),
			config.CronConfig,
		))
	}
	install("import-service-config", importServiceConfig)
	install("free-invalid-duts", freeInvalidDUTs)
	install("prune-expired-drones", pruneExpiredDrones)
	install("prune-drained-duts", pruneDrainedDUTs)
}

// chain is a helper for chaining cron handler wrappers.
// First wrapper is outermost.
func chain(h cron.Handler, w ...middleware.CronWrapper) cron.Handler {
	for i := len(w) - 1; i >= 0; i-- {
		h = w[i](h)
	}
	return h
}

func importServiceConfig(ctx context.Context) error {
	return config.Import(ctx)
}

func freeInvalidDUTs(ctx context.Context) (err error) {
	defer func() {
		freeInvalidDUTsTick.Add(ctx, 1, config.Instance(ctx), err == nil)
	}()
	return queries.FreeInvalidDUTs(ctx, time.Now())
}

func pruneExpiredDrones(ctx context.Context) (err error) {
	defer func() {
		pruneExpiredDronesTick.Add(ctx, 1, config.Instance(ctx), err == nil)
	}()
	return queries.PruneExpiredDrones(ctx, time.Now())
}

func pruneDrainedDUTs(ctx context.Context) (err error) {
	defer func() {
		pruneDrainedDUTsTick.Add(ctx, 1, config.Instance(ctx), err == nil)
	}()
	return queries.PruneDrainedDUTs(ctx)
}
