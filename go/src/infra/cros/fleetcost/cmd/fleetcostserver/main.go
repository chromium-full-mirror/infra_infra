// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main is the entrypoint to the fleet cost server.
package main

import (
	"time"

	"go.chromium.org/luci/server"

	"infra/cros/fleetcost/cmd/fleetcostserver/serverlib"
)

// main starts the fleet cost server.
func main() {
	mods := serverlib.GetModules()

	server.Main(&server.Options{
		DefaultRequestTimeout: 60 * time.Minute,
		// The Repopulate-Cache RPC takes a while, it is a cron job, which is an internal route and uses a different deadline.
		InternalRequestTimeout: 60 * time.Minute,
	}, mods, serverlib.ServerMain)
}
