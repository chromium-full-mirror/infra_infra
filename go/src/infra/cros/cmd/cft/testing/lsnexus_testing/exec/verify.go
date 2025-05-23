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
)

// verify verifies BOLS APIs.
func verify(ctx context.Context, logger *log.Logger, a *args) error {
	conn, err := grpc.NewClient(a.lsNexusAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to create BOLS client at %s: %w", a.lsNexusAddr, err)
	}
	defer conn.Close()
	cl := lsnexus.NewLSNexusServiceClient(conn)
	if a.testServod {
		if err := verifyServodAPIs(ctx, logger, a, cl); err != nil {
			return fmt.Errorf("failed to verify servod related APIs in BOLS at %s: %w", a.lsNexusAddr, err)
		}
	}
	return nil
}
