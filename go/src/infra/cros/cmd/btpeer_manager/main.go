// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main includes the main function for running
// btpeer_manager as an executable.
package main

import (
	"context"
	"os"
	"os/signal"

	"go.chromium.org/infra/cros/cmd/btpeer_manager/cmd"
	"go.chromium.org/infra/cros/cmd/btpeer_manager/log"
)

func main() {
	// Create context that will cancel when a SIGINT signal is received.
	ctx, cancel := context.WithCancel(context.Background())
	interruptSignalChannel := make(chan os.Signal, 1)
	signal.Notify(interruptSignalChannel, os.Interrupt)
	defer func() {
		signal.Stop(interruptSignalChannel)
		cancel()
	}()
	go func() {
		select {
		case <-interruptSignalChannel:
			log.Logger.Println("received SIGINT, cancelling operations")
			cancel()
		case <-ctx.Done():
		}
	}()

	// Run the command.
	rootCmd, err := cmd.RootCmd()
	if err != nil {
		log.Logger.Fatalf("Failed to build root command: %v", err)
	}
	if err = rootCmd.ExecuteContext(ctx); err != nil {
		log.Logger.Fatalf("Execution failed: %v", err)
	}
}
