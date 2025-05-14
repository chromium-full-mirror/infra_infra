// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmds

import (
	"testing"

	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// TestGetNamespace tests the output of getNamespace to ensure the
// namespace is correctly transformed
func TestGetNamespace(t *testing.T) {
	tests := []struct {
		name              string
		passedNamespace   string
		expectedNamespace string
	}{
		{
			name:              "if no namespace passed, default to OS",
			passedNamespace:   "",
			expectedNamespace: ufsUtil.OSNamespace,
		},
		{
			name:              "if namespace passed, use that",
			passedNamespace:   ufsUtil.OSPartnerNamespace,
			expectedNamespace: ufsUtil.OSPartnerNamespace,
		},
		{
			name:              "if invalid namespace passed, swallow error and assign OS",
			passedNamespace:   "fake",
			expectedNamespace: ufsUtil.OSNamespace,
		},
		{
			name:              "ensure browser namespace is valid",
			passedNamespace:   ufsUtil.BrowserNamespace,
			expectedNamespace: ufsUtil.BrowserNamespace,
		},
	}
	for _, tt := range tests {
		//t.Parallel() -- sets environment variables, cannot be parallelized.
		t.Setenv("SHIVAS_NAMESPACE", "")
		t.Run(tt.name, func(t *testing.T) {
			c := syncDUTInfoRun{}
			c.envFlags.Register(&c.Flags)
			err := c.GetFlags().Set("namespace", tt.passedNamespace)
			if err != nil {
				t.Errorf("err setting namespace: %s", err)
			}

			ns := c.getNamespace()
			if ns != tt.expectedNamespace {
				t.Errorf("Expected namespace: %s, got namespace: %s", tt.expectedNamespace, ns)
			}
		})
	}
}
