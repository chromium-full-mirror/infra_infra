// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package exec implements the lsnexus_testing for testing functionality of BOLS.
package exec

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.chromium.org/chromiumos/config/go/test/api/lsnexus"

	lsnexusTesting "go.chromium.org/infra/cros/cmd/cft/lsnexus/testing"
)

// startServer starts the LSNexus test server and waits for a shutdown signal.
// This function is similar to verify, but it does not run any tests.
func startServer(ctx context.Context, logger *log.Logger, a *args) error {
	lsnexusTestServerAddr, stopLsNexusTestServer, err := lsnexusTesting.StartServer(
		a.workingDir,
		a.board,
		a.model,
		nil,
		a.servoSerial,
		a.servodContainer,
		int32(a.servodPort),
		a.bolsAddr,
	)
	if err != nil {
		return fmt.Errorf("failed to start LSNexus test server: %w", err)
	}

	logger.Printf("LSNexus test server started at: %s", lsnexusTestServerAddr)
	logger.Println("Server is running. Press Ctrl-C to stop.")

	// Wait for the context to be cancelled (e.g., by an interrupt signal).
	<-ctx.Done()

	logger.Println("Shutdown signal received, downloading servo logs before stopping...")

	// Create a client to connect to the server we are about to shut down.
	// We do this to perform a final action (downloading logs).
	conn, err := grpc.NewClient(lsnexusTestServerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		// Log the error but continue with shutdown.
		logger.Printf("Failed to create LSNexus client to %s for log download: %v", lsnexusTestServerAddr, err)
	} else {
		defer conn.Close()
		cl := lsnexus.NewLSNexusServiceClient(conn)

		// Use a background context because the parent context `ctx` is already canceled.
		if _, err := cl.DownloadServoLogs(context.Background(), &lsnexus.DownloadServoLogsRequest{}); err != nil {
			logger.Printf("Failed to download servo logs via LSNexus: %v", err)
		} else {
			logger.Println("Successfully downloaded servo logs.")
		}
	}

	logger.Println("Stopping the server...")
	stopLsNexusTestServer()
	logger.Println("Server stopped.")
	return nil
}
