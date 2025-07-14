// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package serverlib

import (
	"context"
	"flag"
	"testing"

	"go.chromium.org/infra/fleetconsole/cmd/fleetconsoleserver/flags"
	"go.chromium.org/infra/fleetconsole/internal/ufsclient"
	"go.chromium.org/infra/libs/skylab/buildbucket"
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

func TestGetAdminServiceAddress(t *testing.T) {
	//t.Parallel() -- manipulates global flag state
	ctx := context.Background()

	originalFlag := *flags.AdminServiceAddr
	defer func() { *flags.AdminServiceAddr = originalFlag }()

	expectedAddr := "fake-admin-service-host:1234"
	flag.Set("admin-service-addr", expectedAddr)
	addr, err := GetAdminServiceAddress(ctx)
	if err != nil {
		t.Errorf("GetAdminServiceAddress returned an unexpected error: %v", err)
	}
	if addr != expectedAddr {
		t.Errorf("GetAdminServiceAddress returned %q, expected %q", addr, expectedAddr)
	}
}

func TestGetInventoryServiceAddress(t *testing.T) {
	//t.Parallel() -- manipulates global flag state
	ctx := context.Background()

	originalFlag := *flags.UfsAddr
	defer func() { *flags.UfsAddr = originalFlag }()

	testCases := []struct {
		name         string
		isProd       bool
		ufsAddrFlag  string
		expectedAddr string
		expectError  bool
	}{
		{
			name:         "Prod environment, no flag",
			isProd:       true,
			ufsAddrFlag:  "",
			expectedAddr: ufsclient.UfsProdURL,
			expectError:  false,
		},
		{
			name:         "Dev environment, no flag",
			isProd:       false,
			ufsAddrFlag:  "",
			expectedAddr: ufsclient.UfsDevURL,
			expectError:  false,
		},
		{
			name:         "Prod environment, flag set with host and port",
			isProd:       true,
			ufsAddrFlag:  "fake-ufs-prod:8800",
			expectedAddr: "fake-ufs-prod",
			expectError:  false,
		},
		{
			name:         "Dev environment, flag set with host and port",
			isProd:       false,
			ufsAddrFlag:  "fake-ufs-dev:8800",
			expectedAddr: "fake-ufs-dev",
			expectError:  false,
		},
		{
			name:         "Flag set with host only",
			isProd:       true,
			ufsAddrFlag:  "fake-ufs-host-only",
			expectedAddr: "fake-ufs-host-only",
			expectError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			flag.Set("ufs-addr", tc.ufsAddrFlag)
			addr, err := GetInventoryServiceAddress(ctx, tc.isProd)

			if tc.expectError {
				if err == nil {
					t.Error("Expected an error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if addr != tc.expectedAddr {
					t.Errorf("Returned address %q, expected %q", addr, tc.expectedAddr)
				}
			}
		})
	}
}

func TestGetInventoryNamespace(t *testing.T) {
	//t.Parallel() -- manipulates global flag state
	ctx := context.Background()

	originalFlag := *flags.InventoryNamespace
	defer func() { *flags.InventoryNamespace = originalFlag }()

	expectedNamespace := "test-namespace"
	flag.Set("inventory-namespace", expectedNamespace)
	namespace, err := GetInventoryNamespace(ctx)
	if err != nil {
		t.Errorf("GetInventoryNamespace returned an unexpected error: %v", err)
	}
	if namespace != expectedNamespace {
		t.Errorf("GetInventoryNamespace returned %q, expected %q", namespace, expectedNamespace)
	}
}

func TestGetCIPDVersion(t *testing.T) {
	//t.Parallel() -- manipulates global flag state
	ctx := context.Background()

	originalFlag := *flags.CIPDVersion
	defer func() { *flags.CIPDVersion = originalFlag }()

	// Test case 1: Flag is set.
	expectedVersion := buildbucket.CIPDVersion("latest")
	flag.Set("cipd-version", string(expectedVersion))
	version, err := GetCIPDVersion(ctx)
	if err != nil {
		t.Errorf("GetCIPDVersion returned an unexpected error: %v", err)
	}
	if version != expectedVersion {
		t.Errorf("GetCIPDVersion returned %q, expected %q", version, expectedVersion)
	}

	// Test case 2: Flag is not set (empty string).
	flag.Set("cipd-version", "")
	version, err = GetCIPDVersion(ctx)
	if err != nil {
		t.Errorf("GetCIPDVersion returned an unexpected error when flag was empty: %v", err)
	}
	if version != "" {
		t.Errorf("GetCIPDVersion returned %q, expected empty string when flag was empty", version)
	}
}
