// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"context"
	"os"

	"go.chromium.org/luci/logdog/client/cli"

	"go.chromium.org/infra/libs/infraenv"
)

func main() {
	os.Exit(cli.Main(context.Background(), cli.Parameters{
		Args:               os.Args[1:],
		Host:               infraenv.ProdLogDogHost,
		DefaultAuthOptions: infraenv.DefaultAuthOptions(),
	}))
}
