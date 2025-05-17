// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements an executable to test BOLS.

package main

import (
	"context"
	"os"

	"go.chromium.org/infra/cros/cmd/cft/testing/bols_testing/exec"
)

func main() {
	os.Exit(exec.BOLSTestingInternal(context.Background()))
}
