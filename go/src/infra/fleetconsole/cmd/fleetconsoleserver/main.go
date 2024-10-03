// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main is the entrypoint to the fleet console server.
package main

import (
	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/gaeemulation"
	"go.chromium.org/luci/server/module"
)

func main() {
	mods := []module.Module{
		gaeemulation.NewModuleFromFlags(),
	}

	server.Main(nil, mods, func(*server.Server) error {
		return nil
	})
}
