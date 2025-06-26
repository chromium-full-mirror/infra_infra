// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package server implement lsnexus-service API.
package server

import (
	"fmt" // Make sure fmt is imported if used in error messages

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.chromium.org/chromiumos/config/go/test/api/bols"
)

// connectToBOLS establishes a gRPC connection to the BOLS service.
// It returns the client, the connection, and an error.
func connectToBOLS(bolsAddr string) (bols.BolsServiceClient, *grpc.ClientConn, error) {
	if bolsAddr == "" {
		// This check can also be done by the caller if preferred,
		// but having it here makes this function more robust.
		return nil, nil, fmt.Errorf("BOLS address cannot be empty")
	}
	conn, err := grpc.NewClient(bolsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to BOLS at %s: %w", bolsAddr, err)
	}
	return bols.NewBolsServiceClient(conn), conn, nil
}
