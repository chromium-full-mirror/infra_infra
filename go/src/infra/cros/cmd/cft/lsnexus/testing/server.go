// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testing provides utilities for testing LSNexus.
package testing

import (
	internalServer "go.chromium.org/infra/cros/cmd/cft/lsnexus/internal/server"
)

// StartServer initializes and starts an LSNexus gRPC server for testing purposes
// by calling an internal testing helper.
// It pre-configures the LSNexus instance with the provided parameters.
// Returns the server's address, a function to stop the server, and any error.
func StartServer(
	artifactDir string,
	board string,
	model string,
	pools []string,
	servodSerial string,
	servodContainer string,
	servodPort int32,
	bolsAddr string) (addr string, stopFunc func(), err error) {

	return internalServer.StartServerForTesting(
		artifactDir,
		board,
		model,
		pools,
		servodSerial,
		servodContainer,
		servodPort,
		bolsAddr,
	)
}
