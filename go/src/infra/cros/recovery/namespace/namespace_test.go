// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package namespace

import (
	"context"
	"testing"

	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

func TestIsPartner(t *testing.T) {
	// Test cases for TestDUTPlans
	var cases = []struct {
		name      string
		isPartner bool
	}{
		{"", false},
		{"os", false},
		{"partner", false},
		{"os-partner", true},
		{ufsUtil.OSNamespace, false},
		{ufsUtil.BrowserNamespace, false},
		{ufsUtil.OSPartnerNamespace, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			c := Set(ctx, tc.name)
			got := IsPartner(c)
			if got != tc.isPartner {
				t.Errorf("%q -> = received %v, want %v", tc.name, got, tc.isPartner)
			}
		})
	}
}
