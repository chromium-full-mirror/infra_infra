// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements an executable to test LSNexus.

package main

import (
	"context"
	"os"

	"go.chromium.org/infra/cros/cmd/cft/testing/lsnexus_testing/exec"
)

func main() {
	os.Exit(exec.LSNexusTestingInternal(context.Background()))
}
