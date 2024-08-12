// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tests

import (
	"context"
	"testing"

	"go.chromium.org/luci/server/servertest"
)

// TestEmptyServer verifies an empty LUCI server can start.
//
// It is mostly testing go.mod dependencies are in consistent state. This was
// added because OpenTelemetry packages often need to be updated in tandem to
// specific compatible versions. Updating them one-by-one (as `go mod tidy`
// sometimes does) may result in runtime errors when starting servers.
//
// If this test breaks, make sure all OpenTelemetry modules in go.mod are
// updated together to compatible versions. This may require updating other
// dependencies too if they themselves depend on OpenTelemetry.
//
// See https://github.com/open-telemetry/opentelemetry-go/issues/2341
func TestEmptyServer(t *testing.T) {
	srv, err := servertest.RunServer(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv.Shutdown()
}
