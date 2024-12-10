// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"go.chromium.org/chromiumos/config/go/test/api"

	"infra/device_manager/internal/database"
	"infra/device_manager/internal/frontend"
)

var e2e = flag.Bool("e2e", false, "Run the end to end tests, which may take much longer than unit tests")

func TestLeaseDevice(t *testing.T) {
	t.Parallel()
	if !*e2e {
		t.Skip("Skipping because this is an end to end test (use -e2e to run this test)")
	}
	ctx := context.Background()
	cfg := setupDBContainer(ctx, t)
	InitDBSchema(t, cfg.DBPort)
	client := newDBClient(ctx, t, cfg)

	s := frontend.NewServer()
	s.ServiceClients.DBClient = client

	t.Run("request to empty DB", func(t *testing.T) {
		badRequets := []struct {
			name string
			req  *api.HardwareRequirements
		}{
			{
				"empty HW req",
				nil,
			},
		}

		for _, tc := range badRequets {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				rsp, err := s.LeaseDevice(ctx, &api.LeaseDeviceRequest{
					IdempotencyKey:     "c80b7379-3594-4299-abf1-6efef198c900",
					HardwareDeviceReqs: tc.req,
				})
				t.Logf("rsp is %v", rsp)
				t.Logf("error is %v", err)
				if err == nil {
					t.Errorf("LeaseDevice(%s) error nil, want error: rsp=%v", tc.name, rsp)
				}
			})
		}
	})
}

func setupDBContainer(ctx context.Context, t *testing.T) *database.DatabaseConfig {
	t.Helper()
	password := "password"
	db := "device_manager_db"
	user := "postgres"
	port := "5432"

	req := testcontainers.ContainerRequest{
		Image: "postgres:15",
		Env: map[string]string{
			"POSTGRES_PASSWORD": password,
			"POSTGRES_USER":     user,
			"POSTGRES_DB":       db,
		},
		ExposedPorts: []string{port + "/tcp"},
		WaitingFor:   wait.ForLog("database system is ready to accept connections"),
	}
	postgres, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("setup db: %s", err)
	}
	t.Cleanup(func() { testcontainers.CleanupContainer(t, postgres) })

	mappedPort, err := postgres.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("get database mapped port: %s", err)
	}

	return &database.DatabaseConfig{
		DBHost:           "localhost",
		DBPort:           mappedPort.Port(),
		DBUser:           user,
		DBPasswordSecret: password,
		DBName:           db,
		// Importtant! otherwise there will be some DB connection issues.
		ConnMaxLifetime: time.Minute,
		MaxIdleConns:    50,
		MaxOpenConns:    50,
	}
}

func InitDBSchema(t *testing.T, dbPort string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "device-manager-e2e-*")
	if err != nil {
		t.Fatalf("set up python venv: %s", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	if out, err := exec.Command("python3", "-m", "venv", dir).CombinedOutput(); err != nil {
		t.Fatalf("create venv: %s: %s", err, out)
	}
	// Set environment so we run everything in the virtual environment.
	os.Setenv("PATH", fmt.Sprintf("%s:%s", filepath.Join(dir, "bin"), os.Getenv("PATH")))

	if out, err := exec.Command("pip", "install", "-r", "../../requirements.txt").CombinedOutput(); err != nil {
		t.Fatalf("install requirements: %s: %s", err, out)
	}
	os.Setenv("ALEMBIC_ENV", "alloydb.dev")
	cmd := exec.Command("alembic", "-x", fmt.Sprintf("port=%s", dbPort), "upgrade", "head")
	cmd.Dir = "../.."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("alembic the schema: %s:%s", err, out)
	}
}

func newDBClient(ctx context.Context, t *testing.T, cfg *database.DatabaseConfig) *database.Client {
	t.Helper()
	client, err := frontend.NewDBClient(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to connect to db: %s", err)
	}
	t.Cleanup(func() {
		client.Conn.Close()
	})
	return client
}
