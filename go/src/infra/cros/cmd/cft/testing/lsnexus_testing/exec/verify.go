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

// verify verifies LSNexus APIs.
func verify(ctx context.Context, logger *log.Logger, a *args) error {
	var lsnexusTestServerAddr string
	var err error

	// If no address is provided, start a local test server.
	if a.lsNexusAddr == "" {
		var stopLsNexusTestServer func()
		lsnexusTestServerAddr, stopLsNexusTestServer, err = lsnexusTesting.StartServer(
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
		defer stopLsNexusTestServer() // Ensure the locally started server is stopped.
		logger.Printf("LSNexus test server started at: %s for verification", lsnexusTestServerAddr)
	} else {
		// Use the provided server address.
		lsnexusTestServerAddr = a.lsNexusAddr
		logger.Printf("Connecting to existing LSNexus server at: %s for verification", lsnexusTestServerAddr)
	}

	// Connect to the LSNexus test server we just started
	conn, err := grpc.NewClient(lsnexusTestServerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to create LSNexus client to %s: %w", lsnexusTestServerAddr, err)
	}
	defer conn.Close()
	cl := lsnexus.NewLSNexusServiceClient(conn)

	if err := verifyServodAPIs(ctx, logger, a, cl); err != nil {
		return fmt.Errorf("failed to verify servod related APIs via LSNexus at %s: %w", lsnexusTestServerAddr, err)
	}
	// Verify file-related APIs.
	if err := verifyFileAPIs(ctx, logger, a, cl); err != nil {
		return fmt.Errorf("failed to verify file related APIs via LSNexus at %s: %w", lsnexusTestServerAddr, err)
	}

	if a.lsNexusAddr == "" {
		logger.Println("Verifying DownloadServoLogs API...")
		if _, err := cl.DownloadServoLogs(ctx, &lsnexus.DownloadServoLogsRequest{}); err != nil {
			return fmt.Errorf("failed to download servo logs via LSNexus at %s: %w", lsnexusTestServerAddr, err)
		}
	}

	logger.Println("LSNexus verification completed successfully.")
	return nil
}
