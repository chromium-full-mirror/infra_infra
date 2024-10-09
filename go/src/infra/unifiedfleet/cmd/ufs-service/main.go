// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"flag"

	"go.chromium.org/luci/server"

	"infra/unifiedfleet/cmd/ufs-service/serverlib"
)

func main() {
	modules := serverlib.Modules()
	cfgLoader := serverlib.ConfigLoader(flag.CommandLine)
	server.Main(nil, modules, serverlib.ServerMainFactory(cfgLoader))
}
