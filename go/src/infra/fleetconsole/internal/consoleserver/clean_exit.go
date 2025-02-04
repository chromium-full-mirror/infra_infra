// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"os"

	"infra/fleetconsole/api/fleetconsolerpc"
)

// CleanExit exits the current process.
func (frontend *FleetConsoleFrontend) CleanExit(ctx context.Context, req *fleetconsolerpc.CleanExitRequest) (*fleetconsolerpc.CleanExitResponse, error) {
	os.Exit(0)
	return &fleetconsolerpc.CleanExitResponse{}, nil
}
