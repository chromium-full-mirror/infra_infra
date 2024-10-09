// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package serverlib contains the main server loop and the modules used.
package serverlib

import (
	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/gaeemulation"
	"go.chromium.org/luci/server/module"
)

func Modules() []module.Module {
	return []module.Module{
		gaeemulation.NewModuleFromFlags(),
	}
}

func ServerMain(src *server.Server) error {
	return nil
}
