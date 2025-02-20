// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ufs

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/luci/auth/client/authcli"

	ufsModels "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsApi "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

// TestValidUFSHostname ensures we return err when hostname given to UFS client is nil
func TestValidUFSHostname(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	_, err := NewUFSClient(ctx, "", &authcli.Flags{})
	if err == nil {
		t.Errorf("Expected an error when invalid host passed")
	}
}

// TestGetDut ensures we call the appropriate UFS method (getmachinelse) and pass along the req
func TestGetDut(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dutName := "host1"
	fc := fakeUFSClient{}

	expectedDut := &ufsModels.MachineLSE{Name: dutName}

	actualDut, err := fc.GetDut(ctx, &ufsApi.GetMachineLSERequest{Name: dutName})
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}

	// only compares Name since that is the only field our fake client changes
	if diff := cmp.Diff(expectedDut.GetName(), actualDut.GetName()); diff != "" {
		t.Errorf("Unexpected diff: %s", diff)
	}
}

// TestGetAsset ensures we call the appropriate UFS method (getasset) and pass along the req
func TestGetAsset(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dutName := "host1"
	fc := fakeUFSClient{}

	expectedAsset := &ufsModels.Asset{Name: dutName}

	actualAsset, err := fc.GetAsset(ctx, &ufsApi.GetAssetRequest{Name: dutName})
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}

	if diff := cmp.Diff(expectedAsset.GetName(), actualAsset.GetName()); diff != "" {
		t.Errorf("Unexpected diff: %s", diff)
	}
}

// fakeUFSClient is a fake implementation of UFS interface used for testing
type fakeUFSClient struct{}

// GetAsset is used in fake UFSClient implementation
// returns an Asset with all default fields except for the Name, which is fetched from the request used in the function
func (f *fakeUFSClient) GetAsset(ctx context.Context, req *ufsApi.GetAssetRequest) (*ufsModels.Asset, error) {
	return &ufsModels.Asset{
		Name: req.GetName(),
	}, nil
}

// GetDut is used in fake UFSClient implementation
// returns an MachineLSE with all default fields except for the Name, which is fetched from the request used in the function
func (f *fakeUFSClient) GetDut(ctx context.Context, req *ufsApi.GetMachineLSERequest) (*ufsModels.MachineLSE, error) {
	return &ufsModels.MachineLSE{
		Name: req.GetName(),
	}, nil
}
