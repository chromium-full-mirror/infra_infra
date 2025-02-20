// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"os"

	"infra/cros/cmd/cft/execution/cros-test/cli"
)

func main() {
	os.Exit(cli.MainInternal(context.Background()))
}
