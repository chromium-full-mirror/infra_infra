// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"
	"testing"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// TestAddDutNamespace tests the output of getNamespace to ensure the
// namespace is correctly transformed and/or an error is thrown
func TestAddDutNamespace(t *testing.T) {
	tests := []struct {
		name              string
		passedNamespace   string
		expectedNamespace string
		expectedErr       bool
	}{
		{
			"if no namespace passed, default to OS",
			"",
			ufsUtil.OSNamespace,
			false,
		},
		{
			"if namespace passed, use that",
			ufsUtil.OSPartnerNamespace,
			ufsUtil.OSPartnerNamespace,
			false,
		},
		{
			"if invalid namespace passed, throw error regardless",
			"fake",
			"fake",
			true,
		},
		{
			"ensure browser namespace is invalid",
			ufsUtil.BrowserNamespace,
			ufsUtil.BrowserNamespace,
			true,
		},
	}
	for _, tt := range tests {
		//t.Parallel() -- sets environment variables, cannot be parallelized.
		t.Setenv("SHIVAS_NAMESPACE", "")
		t.Run(tt.name, func(t *testing.T) {
			c := addDUT{}
			c.envFlags.Register(&c.Flags)
			c.GetFlags().Set("namespace", tt.passedNamespace)

			ns, err := c.getNamespace()
			if ns != tt.expectedNamespace {
				t.Errorf("Expected namespace: %s, got namespace: %s", tt.expectedNamespace, ns)
			}
			if (err != nil) != tt.expectedErr {
				t.Errorf("Expected error: %t, got error: %t", tt.expectedErr, (err != nil))
			}
		})
	}
}

func TestValidateDutAndAssetLocation(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context
		ic          ufsAPI.FleetClient
		dutParam    *dutDeployUFSParams
		expectedErr bool
	}{
		{
			name: "DUT name without zone prefix returns no error",
			ctx:  context.Background(),
			ic:   nil,
			dutParam: &dutDeployUFSParams{
				DUT:   &ufspb.MachineLSE{Name: "non-standard-dut-name"},
				Asset: &ufspb.Asset{},
			},
			expectedErr: false,
		},
		{
			name: "DUT name with chromeos1 prefix and asset matching zone returns no error",
			ctx:  context.Background(),
			ic:   nil,
			dutParam: &dutDeployUFSParams{
				DUT:   &ufspb.MachineLSE{Name: "chromeos1-row1-rack1-host1"},
				Asset: &ufspb.Asset{Location: &ufspb.Location{Zone: ufspb.Zone_ZONE_CHROMEOS1}},
			},
			expectedErr: false,
		},
		{
			name: "DUT name with chromium-chromeos8 prefix and asset matching zone returns no error",
			ctx:  context.Background(),
			ic:   nil,
			dutParam: &dutDeployUFSParams{
				DUT:   &ufspb.MachineLSE{Name: "chromium-chromeos8-row1-rack1-host1"},
				Asset: &ufspb.Asset{Location: &ufspb.Location{Zone: ufspb.Zone_ZONE_SFO36_OS_CHROMIUM}},
			},
			expectedErr: false,
		},
		{
			name: "DUT name with chrome-chromeos8 prefix and asset matching zone returns no error",
			ctx:  context.Background(),
			ic:   nil,
			dutParam: &dutDeployUFSParams{
				DUT:   &ufspb.MachineLSE{Name: "chrome-chromeos8-row1-rack1-host1"},
				Asset: &ufspb.Asset{Location: &ufspb.Location{Zone: ufspb.Zone_ZONE_SFO36_OS}},
			},
			expectedErr: false,
		},
		{
			name: "DUT name with chrome-perf-waterfall-chromeos8 prefix and asset matching zone returns no error",
			ctx:  context.Background(),
			ic:   nil,
			dutParam: &dutDeployUFSParams{
				DUT:   &ufspb.MachineLSE{Name: "chrome-perf-waterfall-chromeos8-row1-rack1-host1"},
				Asset: &ufspb.Asset{Location: &ufspb.Location{Zone: ufspb.Zone_ZONE_SFO36_OS}},
			},
			expectedErr: false,
		},
		{
			name: "DUT name with chrome-perf-pinpoint-chromeos8 prefix and asset matching zone returns no error",
			ctx:  context.Background(),
			ic:   nil,
			dutParam: &dutDeployUFSParams{
				DUT:   &ufspb.MachineLSE{Name: "chrome-perf-pinpoint-chromeos8-row1-rack1-host1"},
				Asset: &ufspb.Asset{Location: &ufspb.Location{Zone: ufspb.Zone_ZONE_SFO36_OS}},
			},
			expectedErr: false,
		},
		{
			name: "DUT name with zone prefix and asset not matching zone returns error",
			ctx:  context.Background(),
			ic:   nil,
			dutParam: &dutDeployUFSParams{
				DUT:   &ufspb.MachineLSE{Name: "chromeos1-row1-rack1-host1"},
				Asset: &ufspb.Asset{Location: &ufspb.Location{Zone: ufspb.Zone_ZONE_SFO36_OS}},
			},
			expectedErr: true,
		},
		{
			name: "DUT name with Satlab Zone",
			ctx:  context.Background(),
			ic:   nil,
			dutParam: &dutDeployUFSParams{
				DUT:   &ufspb.MachineLSE{Name: "satlab-abc123-host1"},
				Asset: &ufspb.Asset{Location: &ufspb.Location{Zone: ufspb.Zone_ZONE_SATLAB}},
			},
			expectedErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateDutAndAssetLocation(tt.ctx, tt.ic, tt.dutParam)
			if (err != nil) != tt.expectedErr {
				t.Errorf("Expected error: %t, got error: %t", tt.expectedErr, (err != nil))
			}
		})
	}
}
