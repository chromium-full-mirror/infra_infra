// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dumper

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"

	"go.chromium.org/luci/gae/impl/memory"

	dronequeenapi "go.chromium.org/infra/appengine/drone-queen/api"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	chromeosLab "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
	"go.chromium.org/infra/unifiedfleet/app/model/inventory"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

// testingContext sets up an environment in memory datastore
func testingContext() context.Context {
	c := context.Background()
	return memory.UseWithAppID(c, ("dev~infra-unified-fleet-system"))
}

// addMachineLSE registers a machine
func addMachineLSE(ctx context.Context, name string) (*ufspb.MachineLSE, error) {
	m, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
		Name: fmt.Sprintf("machinelse-%s", name),
	})
	if err != nil {
		return nil, fmt.Errorf("Error creating machineLSE: %w", err)
	}

	return m, nil
}

// addMachineLSEHive registers a machine with hive
func addMachineLSEHive(ctx context.Context, name string, hive string) (*ufspb.MachineLSE, error) {
	m, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
		Name: name,
		Lse: &ufspb.MachineLSE_ChromeosMachineLse{
			ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
				ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
					DeviceLse: &ufspb.ChromeOSDeviceLSE{
						Device: &ufspb.ChromeOSDeviceLSE_Dut{
							Dut: &chromeosLab.DeviceUnderTest{
								Hive: hive,
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("Error creating machineLSE: %w", err)
	}

	return m, nil
}

// addMachineLSELabsationHive registers a machine labstation  with hive
func addMachineLSELabstationHive(ctx context.Context, name string, hive string) (*ufspb.MachineLSE, error) {
	m, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
		Name: name,
		Lse: &ufspb.MachineLSE_ChromeosMachineLse{
			ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
				ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
					DeviceLse: &ufspb.ChromeOSDeviceLSE{
						Device: &ufspb.ChromeOSDeviceLSE_Labstation{
							Labstation: &chromeosLab.Labstation{
								Hive: hive,
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("Error creating machineLSE labstation: %w", err)
	}

	return m, nil
}

// Singleton client
var (
	client = &stubDroneQueenClientImpl{}
)

// stubDroneQueenClientImpl is a stub implementation of drone queen client used for testing
type stubDroneQueenClientImpl struct {
	lastDeclareDUTsCall *dronequeenapi.DeclareDutsRequest
}

// DeclareDuts is a noop that stores the last call in the stub
func (c *stubDroneQueenClientImpl) DeclareDuts(ctx context.Context, in *dronequeenapi.DeclareDutsRequest, opts ...grpc.CallOption) (*dronequeenapi.DeclareDutsResponse, error) {
	c.lastDeclareDUTsCall = in
	return &dronequeenapi.DeclareDutsResponse{}, nil
}

// getStubSingletonDroneQueenClient returns the stub singleton client
func getStubSingletonDroneQueenClient(ctx context.Context) (dronequeenapi.InventoryProviderClient, error) {
	return client, nil
}

// TestPushToDroneQueenNamespaces creates DUTs in three namespaces, and then
// verifies correct behavior wrt which namespaces we push DUTs for
func TestPushToDroneQueenNamespaces(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	droneQueenGenerator = getStubSingletonDroneQueenClient

	osCtx, _ := util.SetupDatastoreNamespace(ctx, util.OSNamespace)
	partnerCtx, _ := util.SetupDatastoreNamespace(ctx, util.OSPartnerNamespace)
	browserCtx, _ := util.SetupDatastoreNamespace(ctx, util.BrowserNamespace)

	osMachine, _ := addMachineLSE(osCtx, "os")
	partnerMachine, _ := addMachineLSE(partnerCtx, "partner")
	_, _ = addMachineLSE(browserCtx, "browser")

	// DUT with hive
	osMachineHive, _ := addMachineLSEHive(osCtx, "os1", "hive1")
	// Satlab DUT without hive
	osMachineSatlabNoHive, _ := addMachineLSEHive(osCtx, "satlab-abc-host1", "")
	// Satlab DUT with hive
	osMachineSatlabWithHive, _ := addMachineLSEHive(osCtx, "satlab-abc-host2", "satlab-1")
	// DUT with cloudbots hive should not be declared
	if _, err := addMachineLSEHive(osCtx, "cloudbots-host", "cloudbots"); err != nil {
		t.Errorf("err add cloudbots-host: %s", err)
	}
	// Labstation with cloudbots hive should not be declared
	if _, err := addMachineLSELabstationHive(osCtx, "cloudbots-labstation1", "cloudbots-large"); err != nil {
		t.Errorf("err add cloudbots-labstation1: %s", err)
	}
	// Labstation with non cloudbot hive should be declared
	if _, err := addMachineLSELabstationHive(osCtx, "cloudbots-labstation2", "e"); err != nil {
		t.Errorf("err add cloudbots-labstation2: %s", err)
	}

	// only want os, partner machines to be pushed
	want := &dronequeenapi.DeclareDutsRequest{
		AvailableDuts: []*dronequeenapi.DeclareDutsRequest_Dut{
			{Name: osMachine.Name, Hive: ""},
			{Name: osMachineSatlabNoHive.Name, Hive: "satlab-abc"},
			{Name: "cloudbots-labstation2", Hive: "e"},
			{Name: osMachineHive.Name, Hive: "hive1"},
			{Name: osMachineSatlabWithHive.Name, Hive: "satlab-1"},
			{Name: partnerMachine.Name, Hive: ""},
		},
	}

	err := pushToDroneQueen(ctx)
	if err != nil {
		t.Errorf("err when pushing to drone queen: %s", err)
	}

	if diff := cmp.Diff(client.lastDeclareDUTsCall.AvailableDuts, want.AvailableDuts, cmpopts.IgnoreUnexported(dronequeenapi.DeclareDutsRequest_Dut{}, dronequeenapi.DeclareDutsRequest{}), cmpopts.SortSlices(func(x, y *dronequeenapi.DeclareDutsRequest_Dut) bool { return x.Name > y.Name })); diff != "" {
		t.Errorf("Call to drone queen had unexpected diff:\n%s", diff)
	}
}
