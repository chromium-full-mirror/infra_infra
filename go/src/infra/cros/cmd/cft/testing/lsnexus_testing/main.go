// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements an executable to test LSNexus.

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.chromium.org/infra/cros/cmd/cft/testing/lsnexus_testing/exec"
)

func main() {
	// Create a context that is canceled on receiving an interrupt (Ctrl-C)
	// or termination signal. This is the standard Go pattern for
	// graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The `startServer` function will block until this context is canceled.
	// When a signal is caught, `ctx.Done()` will be closed, `startServer`
	// will proceed to shut down the gRPC server, and then return.
	// `LSNexusTestingInternal` will then return, and the program will exit.
	os.Exit(exec.LSNexusTestingInternal(ctx))
}
