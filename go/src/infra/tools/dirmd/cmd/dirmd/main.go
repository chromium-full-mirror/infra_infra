// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"os"

	"go.chromium.org/luci/hardcoded/chromeinfra"

	"go.chromium.org/infra/tools/dirmd/cli"
)

func main() {
	p := cli.Params{Auth: chromeinfra.DefaultAuthOptions()}
	os.Exit(cli.Main(p, os.Args[1:]))
}
