// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"testing"

	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// TestGetDutNamespace tests the output of getNamespace to ensure the
// namespace is correctly transformed and/or an error is thrown
func TestGetDutNamespace(t *testing.T) {
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
			c := getDut{}
			c.envFlags.Register(&c.Flags)
			c.GetFlags().Set("namespace", tt.passedNamespace)

			ns, err := c.getNamespace()
			if ns != tt.expectedNamespace {
				t.Errorf("Expected namespace: %s, got namespace: %s", tt.expectedNamespace, ns)
			}
			if (err != nil) != tt.expectedErr {
				t.Errorf("Expected error: %t, got error: %t", (err != nil), tt.expectedErr)
			}
		})
	}
}
