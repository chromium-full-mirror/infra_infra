// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build integration || e2e

package frontend

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	"go.chromium.org/infra/unifiedfleet/app/config"
	"go.chromium.org/infra/unifiedfleet/app/model/configuration"
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
	"go.chromium.org/infra/unifiedfleet/app/util"
	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/auth/identity"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"
)

func TestRequestMachineLSEStateChange(t *testing.T) {
	t.Parallel()

	ctx := gaetesting.TestingContextWithAppID("dev~infra-unified-fleet-system")
	ctx = config.Use(ctx, &config.Config{})

	setupDatastore(ctx, t)

	mlsePrototype := &ufspb.MachineLSEPrototype{
		Name: "browser:no-vm",
	}
	_, err := configuration.CreateMachineLSEPrototype(ctx, mlsePrototype)
	assert.Loosely(t, err, should.BeNil)

	fleet := &FleetServerImpl{}

	t.Run("check errors", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			user    string
			state   ufspb.State
			force   bool
			wantErr string
		}{
			{
				"non-human cannot force",
				"my@service-account.com",
				ufspb.State_STATE_DISABLED,
				true,
				"cannot force",
			},
			{
				"non-human can only set ready/disabled",
				"my@service-account.com",
				ufspb.State_STATE_NEEDS_REPAIR,
				false,
				"allowed",
			},
			{
				"bad user email",
				"xyz",
				ufspb.State_STATE_READY,
				false,
				"invalid email",
			},
		}
		for i, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				lse := createMachineLSE(t, ctx, fleet, 100+i)
				lse.ResourceState = tc.state
				ctx = auth.WithState(ctx, &authtest.FakeState{Identity: identity.Identity(fmt.Sprintf("user:%s", tc.user))})
				ureq := &ufsAPI.UpdateMachineLSERequest{
					MachineLSE:       lse,
					ForceStateUpdate: tc.force,
				}
				_, err = fleet.UpdateMachineLSE(ctx, ureq)
				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, err.Error(), should.ContainSubstring(tc.wantErr))
			})
		}
	})

	t.Run("test state changes", func(t *testing.T) {
		type updateOp struct {
			user    string
			state   ufspb.State
			byForce bool
		}
		cases := []struct {
			name      string
			preSteps  []updateOp
			wantState ufspb.State
		}{
			{
				"human change state",
				[]updateOp{
					{"user1@example.com", ufspb.State_STATE_NEEDS_REPAIR, false},
				},
				ufspb.State_STATE_NEEDS_REPAIR,
			},
			{
				"non-human change state",
				[]updateOp{
					{"my@service-account.com", ufspb.State_STATE_DISABLED, false},
				},
				ufspb.State_STATE_DISABLED,
			},
			{
				"human overwrite non-human state change",
				[]updateOp{
					{"my@service-account.com", ufspb.State_STATE_DISABLED, false},
					{"user1@example.com", ufspb.State_STATE_NEEDS_REPAIR, false},
				},
				ufspb.State_STATE_NEEDS_REPAIR,
			},
			{
				"non-human cannot overwrite human state change",
				[]updateOp{
					{"user1@example.com", ufspb.State_STATE_NEEDS_REPAIR, false},
					{"my@service-account.com", ufspb.State_STATE_DISABLED, false},
				},
				ufspb.State_STATE_NEEDS_REPAIR,
			},
			{
				"go to disabled when human work done",
				[]updateOp{
					{"user1@example.com", ufspb.State_STATE_NEEDS_REPAIR, false},
					{"my@service-account.com", ufspb.State_STATE_DISABLED, false},
					{"user2@example.com", ufspb.State_STATE_READY, false},
				},
				ufspb.State_STATE_DISABLED,
			},
			{
				"keep the state when non-human is done",
				[]updateOp{
					{"my@service-account.com", ufspb.State_STATE_DISABLED, false},
					{"user1@example.com", ufspb.State_STATE_NEEDS_REPAIR, false},
					{"my@service-account.com", ufspb.State_STATE_READY, false},
				},
				ufspb.State_STATE_NEEDS_REPAIR,
			},
			{
				"huamn user force state change",
				[]updateOp{
					{"my@service-account.com", ufspb.State_STATE_DISABLED, false},
					{"user1@example.com", ufspb.State_STATE_NEEDS_REPAIR, true},
				},
				ufspb.State_STATE_NEEDS_REPAIR,
			},
			{
				"forcing state back to ready resets the state record",
				[]updateOp{
					{"my@service-account.com", ufspb.State_STATE_DISABLED, false},
					{"user1@example.com", ufspb.State_STATE_READY, true},
					{"my@service-account.com", ufspb.State_STATE_DISABLED, false},
				},
				ufspb.State_STATE_DISABLED,
			},
		}
		for i, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				lse := createMachineLSE(t, ctx, fleet, 200+i)
				lseName := lse.GetName()
				var resp *ufspb.MachineLSE
				for _, s := range tc.preSteps {
					ureq := &ufsAPI.UpdateMachineLSERequest{MachineLSE: &ufspb.MachineLSE{
						Name:                lseName,
						Hostname:            lse.Hostname,
						ResourceState:       s.state,
						Machines:            lse.Machines,
						MachineLsePrototype: lse.MachineLsePrototype,
					}}
					ctx = auth.WithState(ctx, &authtest.FakeState{Identity: identity.Identity(fmt.Sprintf("user:%s", s.user))})
					resp, err = fleet.UpdateMachineLSE(ctx, ureq)
					assert.Loosely(t, err, should.BeNil)
				}
				assert.Loosely(t, resp.GetResourceState().String(), should.Equal(tc.wantState.String()))
			})
		}
	})
}
func createMachineLSE(t *testing.T, ctx context.Context, Fleet *FleetServerImpl, suffix int) *ufspb.MachineLSE {
	t.Helper()
	machineName := fmt.Sprintf("machine-%d", suffix)
	machine := &ufspb.Machine{Name: machineName}
	_, err := registration.CreateMachine(ctx, machine)
	assert.Loosely(t, err, should.BeNil)

	name := fmt.Sprintf("machineLSE-%d", suffix)
	lse := &ufspb.MachineLSE{
		Name:     util.AddPrefix(util.MachineLSECollection, name),
		Hostname: name,
	}
	lse.MachineLsePrototype = "browser:no-vm"
	lse.Lse = &ufspb.MachineLSE_ChromeBrowserMachineLse{
		ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
	}
	lse.Machines = []string{machineName}

	req := &ufsAPI.CreateMachineLSERequest{
		MachineLSE:   lse,
		MachineLSEId: name,
	}
	resp, err := Fleet.CreateMachineLSE(ctx, req)
	assert.Loosely(t, err, should.BeNil)
	return resp
}

func setupDatastore(ctx context.Context, t *testing.T) {
	t.Helper()
	// 1. Define the Datastore emulator container
	req := testcontainers.ContainerRequest{
		Image:        "google/cloud-sdk:emulators",
		ExposedPorts: []string{"8081/tcp"},
		// Command to start the Datastore emulator
		Cmd:        []string{"gcloud", "emulators", "firestore", "start", "--database-mode=datastore-mode", "--host-port=0.0.0.0:8081", "--project=test-project"},
		WaitingFor: wait.ForLog("Dev App Server is now running"),
	}

	// 2. Start the container
	emulator, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start container: %v", err)
	}
	t.Cleanup(func() {
		emulator.Terminate(ctx)
	})

	// 3. Get the mapped port
	host, err := emulator.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}
	port, err := emulator.MappedPort(ctx, "8081")
	if err != nil {
		t.Fatalf("failed to get mapped port: %v", err)
	}
	emulatorAddress := fmt.Sprintf("%s:%s", host, port.Port())

	// 4. Set the DATASTORE_EMULATOR_HOST environment variable
	os.Setenv("DATASTORE_EMULATOR_HOST", emulatorAddress)
	t.Cleanup(func() {
		os.Unsetenv("DATASTORE_EMULATOR_HOST")
	})
}
