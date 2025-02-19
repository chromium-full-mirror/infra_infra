// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package serverlib

import (
	"context"
	"testing"
)

func TestControlPRPCAccessAllowed(t *testing.T) {
	t.Parallel()
	testCases := []string{
		"http://localhost",
		"http://localhost:8080",
		"https://ci.chromium.org",
		"https://luci.app",
		"https://staging.luci.app",
		"https://luci-milo.appspot.com",
		"https://luci-milo-dev.appspot.com",
		"https://abc-dot-luci-milo-dev.appspot.com",
		"https://abc-123-dot-luci-milo.appspot.com",
	}

	ctx := context.Background()
	for _, origin := range testCases {
		decision := controlPRPCAccess(ctx, origin)
		if !decision.AllowCrossOriginRequests || !decision.AllowCredentials {
			t.Errorf("Expected %s to be allowed, but it was blocked", origin)
		}
	}
}

func TestControlPRPCAccessDisallowed(t *testing.T) {
	t.Parallel()
	testCases := []string{
		"http://localhost.test",
		"http://localhost:8000",
		"http://ci.chromium.org",
		"https://chromium.org",
		"https://ci.chromium.test.com",
		"https://test-luci-milo.appspot.com",
		"https://luci-milo.appspot.com.test.com",
		"https://luci-milo-dev.appspot.com.test.com",
		"https://staging-luci.appspot.com",
	}

	ctx := context.Background()
	for _, origin := range testCases {
		decision := controlPRPCAccess(ctx, origin)
		if decision.AllowCrossOriginRequests || decision.AllowCredentials {
			t.Errorf("Expected %s to be blocked, but it was allowed", origin)
		}
	}
}
