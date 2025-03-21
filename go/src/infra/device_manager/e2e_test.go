// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// The compose module of testcontainers-go doesn't work on Mac
//go:build linux
// +build linux

// Package e2e_test runs the service using docker-compose and executes end to
// end tests.
package e2e_test

import (
	"context"
	"flag"
	"fmt"
	"os/user"
	"strconv"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.chromium.org/chromiumos/config/go/test/api"
)

var e2e = flag.Bool("e2e", false, "Run the end to end tests, which may take much longer than unit tests")

func TestMain(t *testing.T) {
	t.Parallel()
	if !*e2e {
		t.Skip("Skipping because this is an end to end test (use -e2e to run this test)")
	}
	ctx := context.Background()
	c := newLeaseClient(t)
	req := &api.ReleaseDeviceRequest{LeaseId: "2660025e-724b-4666-bc72-e121a70da94a"}
	_, _ = c.ReleaseDevice(ctx, req)
}

// Helper function to create gRPC client for the service.
func newLeaseClient(t *testing.T) api.DeviceLeaseServiceClient {
	t.Helper()

	_, address := setupCompose(t)
	t.Logf("device_lease_service listensing on %s", address)

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect to gRPC server: %v", err)
	}
	defer conn.Close()

	t.Logf("gRPC client connected successfully!")
	return api.NewDeviceLeaseServiceClient(conn)
}

// Helper function to start the Docker Compose stack.
func setupCompose(t *testing.T) (compose.ComposeStack, string) {
	t.Helper()

	ctx := context.Background()
	composeFilePath := "./docker-compose.dev.yml"

	composeStack, err := compose.NewDockerCompose(composeFilePath)
	if err != nil {
		t.Fatalf("failed to create compose stack: %v", err)
	}
	t.Cleanup(func() {
		if err := composeStack.Down(ctx, compose.RemoveOrphans(true)); err != nil {
			t.Fatalf("failed to tear down compose stack: %v", err)
		}
	})

	uid, gid := getCurrentUserIDs(t)
	// Wait for port 50051 on device_lease_service.
	err = composeStack.
		WaitForService("device_lease_service", wait.ForListeningPort("50051/tcp")).
		WithEnv(map[string]string{"CURRENT_UID": fmt.Sprintf("%d:%d", uid, gid)}).
		Up(ctx)
	if err != nil {
		t.Fatalf("failed to start compose stack: %v", err)
	}

	srv, err := composeStack.ServiceContainer(ctx, "device_lease_service")
	if err != nil {
		t.Fatalf("failed to get device_lease_service container: %v", err)
	}
	address, err := srv.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("failed to setupCompose: %v", err)
	}

	return composeStack, address
}

// getCurrentUserIDs returns the current user's UID and GID as integers.
// It fails the test if any errors occur.
func getCurrentUserIDs(t *testing.T) (int, int) {
	t.Helper()

	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("Error getting current user: %v", err)
	}

	userID, err := strconv.Atoi(currentUser.Uid)
	if err != nil {
		t.Fatalf("Error converting UID to integer: %v", err)
	}

	groupID, err := strconv.Atoi(currentUser.Gid)
	if err != nil {
		t.Fatalf("Error converting GID to integer: %v", err)
	}

	return userID, groupID
}
