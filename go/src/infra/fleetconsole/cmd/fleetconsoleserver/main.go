// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main is the entrypoint to the fleet console server.
package main

import (
	"go.chromium.org/luci/server"

	"infra/fleetconsole/cmd/fleetconsoleserver/serverlib"
)

func main() {
	modules := serverlib.Modules()
	options := serverlib.Options()
	server.Main(options, modules, serverlib.ServerMain)
}
