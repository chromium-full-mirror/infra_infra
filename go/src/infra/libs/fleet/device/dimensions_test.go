// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package device

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"

	models "go.chromium.org/infra/unifiedfleet/api/v1/models"
	lab "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

// TestGetPools tests that GetPools passes an appropriately annotated name to the
func TestGetPools(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	c := &mockGetPoolsClient{}
	expectedPools := []string{"aaaa"}
	actualPools, err := GetPools(ctx, c, "a")
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if diff := cmp.Diff(expectedPools, actualPools); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
	expectedName := map[string]bool{"a": true}
	actualName := c.names
	if diff := cmp.Diff(expectedName, actualName); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
}

// FakeMachine is a fake DUT with pool "aaaa".
var fakeMachine = &models.MachineLSE{
	Lse: &models.MachineLSE_ChromeosMachineLse{
		ChromeosMachineLse: &models.ChromeOSMachineLSE{
			ChromeosLse: &models.ChromeOSMachineLSE_DeviceLse{
				DeviceLse: &models.ChromeOSDeviceLSE{
					Device: &models.ChromeOSDeviceLSE_Dut{
						Dut: &lab.DeviceUnderTest{
							Pools: []string{"aaaa"},
						},
					},
				},
			},
		},
	},
}

// mockGetPoolsClient mimics a UFS client and records the results of look up.
type mockGetPoolsClient struct {
	names map[string]bool
}

// GetMachineLSE always returns a fake machine.
func (f *mockGetPoolsClient) GetMachineLSE(ctx context.Context, in *ufsAPI.GetMachineLSERequest, opts ...grpc.CallOption) (*models.MachineLSE, error) {
	if f.names == nil {
		f.names = map[string]bool{}
	}
	f.names[in.GetName()] = true
	return fakeMachine, nil
}

// GetDeviceData always returns a fake host.
//
// This function never returns a scheduling unit, although multi-dut scheduling
// units are supported in real life.
func (f *mockGetPoolsClient) GetDeviceData(ctx context.Context, in *ufsAPI.GetDeviceDataRequest, opts ...grpc.CallOption) (*ufsAPI.GetDeviceDataResponse, error) {
	if f.names == nil {
		f.names = map[string]bool{}
	}
	f.names[in.GetHostname()] = true
	return &ufsAPI.GetDeviceDataResponse{
		ResourceType: ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE,
		Resource: &ufsAPI.GetDeviceDataResponse_ChromeOsDeviceData{
			ChromeOsDeviceData: &models.ChromeOSDeviceData{
				LabConfig: fakeMachine,
			},
		},
	}, nil
}
