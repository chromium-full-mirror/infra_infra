// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"

	"go.chromium.org/luci/auth"

	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

func TestPrepareAdminParams(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	newMetadata := metadata.Pairs(ufsUtil.Namespace, ufsUtil.OSPartnerNamespace)
	partnerCtx := metadata.NewOutgoingContext(ctx, newMetadata)
	testCases := []struct {
		name                 string
		ctx                  context.Context
		unitName             string
		realBuilderName      string
		adminService         string
		expectedNamespace    string
		expectedAdminService string
	}{
		{
			"clank deploy job",
			ctx,
			"host1",
			"deploy-clank",
			"admin-service",
			"os",
			"admin-service",
		},
		{
			"clank repair job",
			ctx,
			"host2",
			"repair-clank",
			"admin-service",
			"os",
			"admin-service",
		},
		{
			"clank deploy job for partners",
			partnerCtx,
			"partner_host1",
			"deploy-clank",
			"admin-service",
			"os-partner",
			"",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			adminParams, err := PrepareAdminParams(tc.ctx, tc.unitName, tc.realBuilderName, tc.adminService, nil, auth.Options{})
			if err != nil {
				t.Errorf("unexpected error: %s", err)
			}
			if adminParams.ContextNamespace != tc.expectedNamespace {
				t.Errorf("unexpected namespace %s (expected %s)", adminParams.ContextNamespace, tc.expectedNamespace)
			}
			if adminParams.AdminService != tc.expectedAdminService {
				t.Errorf("unexpected service %s (expected %s)", adminParams.AdminService, tc.expectedAdminService)
			}
		})

	}
}
