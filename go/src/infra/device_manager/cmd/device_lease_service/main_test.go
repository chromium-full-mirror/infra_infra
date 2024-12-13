// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main_test

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/server"

	"infra/device_manager/internal/database"
	"infra/device_manager/internal/external"
	"infra/device_manager/internal/frontend"
	"infra/device_manager/internal/jobs"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
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

	opts, err := server.OptionsFromEnv(&server.Options{GRPCAddr: ":0"})
	if err != nil {
		t.Fatal(err)
	}
	luciServer, err := server.New(ctx, *opts, nil)
	if err != nil {
		t.Fatal(err)
	}
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
	s.ServiceClients.UFSClient = newUFS(luciServer.Context, t)
	t.Run("import UFS data to DB", func(t *testing.T) {
		testcases := []struct {
			name              string
			prepareSQL        string
			wantInactiveCount int
		}{
			{
				"Import all to empty DB",
				"",
				0,
			},
			{
				// Mark some devices as inactive, the import should reset them back to
				// active.
				"Activate the inactive devices if they are valid in UFS",
				`update "Devices" set is_active=false where id in (select id from "Devices" where is_active=true limit 500);`,
				0,
			},
			{
				// Change the device ID so we won't find them from UFS, resulting in
				// inactive status.
				"Deactivate the active devices if they are not valid in UFS",
				`update "Devices" set id=id || '_xxx'  where id in (select id from "Devices" where is_active=true limit 500);`,
				500,
			},
		}
		clients := s.ServiceClients
		// We actually don't know the exact number of devices in UFS, so we use the
		// first import result as the baseline for the following tests.
		// When comparing the result, the number may not exactly same due to ongoing
		// deployment etc. We suppose it's ok as long as they are almost the same.
		wantActive := -1
		allowedRange := 50

		for _, tc := range testcases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				// Don't parallel the subtests. They depend on each other.
				if tc.prepareSQL != "" {
					_, _ = clients.DBClient.Conn.Exec(tc.prepareSQL)
				}
				if err := jobs.ImportUFSDevices(ctx, clients, ""); err != nil {
					t.Errorf("ImportUFSDevices() errors %s, want nil", err)
				}

				var active, inactive int
				r := clients.DBClient.Conn.QueryRow(`select count(is_active=true OR NULL) as active_count, count(is_active=false OR NULL) as inactive_count from "Devices";`)
				_ = r.Scan(&active, &inactive)
				t.Logf("active: %d, inactive: %d", active, inactive)
				// We don't know the exact number of devices will be imported, so we
				// just check if it looks right. By 2024, there are ~ 17K devices.
				if active < 15_000 || active > 20_000 {
					t.Errorf("ImportUFSDevices() = %d active devices, want >15K && <20K", active)
				}
				if wantActive < 0 {
					wantActive = active
				} else {
					if math.Abs(float64(active-wantActive)) > float64(allowedRange) {
						t.Errorf("ImportUFSDevices() = %d active devices, want ~ %d", active, wantActive)
					}
				}
				if math.Abs(float64(inactive-tc.wantInactiveCount)) > float64(allowedRange) {
					t.Errorf("ImportUFSDevices() = %d inactive devices, want ~ %d", inactive, tc.wantInactiveCount)
				}
			})
		}
	})
	t.Run("Lease a device", func(t *testing.T) {
		testcases := []struct {
			name      string
			isActive  bool
			state     string
			wantError bool
		}{
			{
				name:      "lease an inactive device",
				isActive:  false,
				state:     "AVAILABLE",
				wantError: true,
			},
		}
		for _, tc := range testcases {
			tc := tc
			query := fmt.Sprintf(`select dut_id from "Devices" where is_active=%t and dut_state='DEVICE_STATE_%s' limit 1;`, tc.isActive, tc.state)
			var dutID string
			_ = s.ServiceClients.DBClient.Conn.QueryRow(query).Scan(&dutID)
			rsp, err := s.LeaseDevice(ctx, &api.LeaseDeviceRequest{
				IdempotencyKey: "7161090b-0e91-4e6b-9665-8d45a55b83e3",
				HardwareDeviceReqs: &api.HardwareRequirements{
					SchedulableLabels: map[string]*api.HardwareRequirements_LabelValues{
						"dut_id": {Values: []string{dutID}},
					},
				},
			})
			t.Logf("rsp %v", rsp)
			if tc.wantError && err == nil {
				t.Errorf("LeaseDevice(is_active:%t,state:%s) error nil, want error", tc.isActive, tc.state)
			}
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

func newUFS(ctx context.Context, t *testing.T) ufsAPI.FleetClient {
	t.Helper()
	ufs, err := external.NewUFSClient(ctx, external.UFSServiceURI)
	if err != nil {
		t.Fatalf("new UFS client: %s", err)
	}
	return ufs
}
