// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dutinfo

import (
	"testing"

	"infra/cros/recovery/tlw"
	ufslab "infra/unifiedfleet/api/v1/models/chromeos/lab"
)

// TestTrancateString tests trancateString function.
func TestConvertSIMProviders(t *testing.T) {
	t.Parallel()

	// Test cases for TestDUTPlans
	var testCases = []struct {
		name string
		ufs  ufslab.NetworkProvider
		tlw  tlw.Cellular_NetworkProvider
	}{
		{
			name: "default",
			tlw:  tlw.Cellular_NETWORK_UNSPECIFIED,
			ufs:  ufslab.NetworkProvider_NETWORK_OTHER,
		},
		{
			name: "unsupported",
			tlw:  tlw.Cellular_NETWORK_UNSUPPORTED,
			ufs:  ufslab.NetworkProvider_NETWORK_UNSUPPORTED,
		},
		{
			name: "amarisoft",
			tlw:  tlw.Cellular_NETWORK_AMARISOFT,
			ufs:  ufslab.NetworkProvider_NETWORK_AMARISOFT,
		},
		{
			name: "att",
			tlw:  tlw.Cellular_NETWORK_ATT,
			ufs:  ufslab.NetworkProvider_NETWORK_ATT,
		},
		{
			name: "povo",
			tlw:  tlw.Cellular_NETWORK_POVO,
			ufs:  ufslab.NetworkProvider_NETWORK_POVO,
		},
	}

	for _, c := range testCases {
		t.Run(c.name, func(t *testing.T) {
			tlw := convertSIMProviders(c.ufs)
			if tlw != c.tlw {
				t.Errorf("failed to convert %s to tlw, got: %s want: %s", c.ufs, tlw, c.tlw)
			}

			ufs := convertSIMProviderToUFS(c.tlw)
			if ufs != c.ufs {
				t.Errorf("failed to convert %s to ufs, got: %s want: %s", c.tlw, ufs, c.ufs)
			}
		})
	}
}
