// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"fmt"
	"io"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/errors"

	schedulingapi "go.chromium.org/infra/libs/fleet/scheduling/api"
	"go.chromium.org/infra/libs/skylab/buildbucket"
	"go.chromium.org/infra/libs/skylab/swarming"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

const (
	DeploymentBuilderName = "deploy"
	RecoveryBuilderName   = "repair"
	VerifyBuilderName     = "verify"
	ReserveBuilderName    = "reserve"
	ShivasClientTag       = "client:shivas"
)

// AdminParams contains params for buildbucket to trigger admin tasks
type AdminParams struct {
	SchedukeClient   schedulingapi.TaskSchedulingAPI
	ContextNamespace string
	AdminService     string
}

// PrepareAdminParams prepars params for buildbucket to trigger admin tasks
func PrepareAdminParams(ctx context.Context, unitName, realBuilderName, adminService string, ufsC ufsAPI.FleetClient, authOpts auth.Options) (*AdminParams, error) {
	contextNamespace := ReadContextNamespace(ctx, ufsUtil.OSNamespace)
	if contextNamespace == ufsUtil.OSPartnerNamespace {
		// Partner do not have options with stable version.
		adminService = ""
	}
	if !buildbucket.IfUseScheduke(realBuilderName) {
		return &AdminParams{
			SchedukeClient:   nil,
			ContextNamespace: contextNamespace,
			AdminService:     adminService,
		}, nil
	}
	sc, err := SchedukeClient(ctx, ufsC, authOpts, unitName)
	if err != nil {
		return nil, errors.Annotate(err, "creating Scheduke client").Err()
	}
	return &AdminParams{
		SchedukeClient:   sc,
		ContextNamespace: contextNamespace,
		AdminService:     adminService,
	}, nil
}

// GetHive returns hive for a host in UFS
func GetHive(ctx context.Context, ic ufsAPI.FleetClient, hostname string) string {
	dut, err := ic.GetMachineLSE(ctx, &ufsAPI.GetMachineLSERequest{
		Name: ufsUtil.AddPrefix(ufsUtil.MachineLSECollection, hostname),
	})
	// If DUT doesn't exist, just return empty hive as the job won't be triggered anyway
	if err != nil {
		return ""
	}
	return ufsUtil.GetHiveForDut(hostname, dut.GetChromeosMachineLse().GetDeviceLse().GetDut().GetHive())
}

// PrintTasksBatchLink prints batch link for scheduled tasks.
func PrintTasksBatchLink(wr io.Writer, swarmingService, commonTag string) {
	fmt.Fprintf(wr, "### Batch tasks URL ###\n")
	fmt.Fprintf(wr, "Created tasks: %s\n", TasksBatchLink(swarmingService, commonTag))
}

// TasksBatchLink created batch link to swarming for scheduled tasks.
func TasksBatchLink(swarmingService, commonTag string) string {
	return swarming.TaskListURLForTags(swarmingService, []string{commonTag})
}
