// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"go.chromium.org/luci/server"

	"go.chromium.org/infra/unifiedfleet/cmd/ufs-service/serverlib"
)

func main() {
	modules := serverlib.Modules()
	cfgLoader := serverlib.ConfigLoader()
	server.Main(nil, modules, serverlib.ServerMain(cfgLoader))
}
