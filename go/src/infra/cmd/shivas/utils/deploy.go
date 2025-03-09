// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"fmt"
	"io"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cmd/shivas/site"
	schedulingapi "go.chromium.org/infra/libs/fleet/scheduling/api"
	"go.chromium.org/infra/libs/skylab/buildbucket"
	"go.chromium.org/infra/libs/skylab/swarming"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// DeployTaskParams contain fields used when scheduling deploy task
//
// Although the buildbucket bucket/LUCI project are configurable, the deploy
// task will always be scheduled in a bb BUILDER named deploy(-latest).
type DeployTaskParams struct {
	// Client interfaces with Buildbucket.
	Client buildbucket.Client
	// SchedulingClient interfaces with a scheduling API wrapper.
	SchedulingClient schedulingapi.TaskSchedulingAPI
	// Env contains env specific configs.
	Env site.Environment
	// Unit is the name of the DUT within Inventory database.
	// ex: "chromeos-rack6-host3"
	Unit string
	// SessionTag is some tag that can be used to track the build.
	SessionTag string
	// UseLatestVersion indicates whether the deploy should use the CIPD latest
	// version of the labpack binary.
	UseLatestVersion bool
	// BBBucket is the name of the bucket the deploy build runs in.
	BBBucket string
	// BBProject is the name of the LUCI project the deploy build runs in.
	BBProject string
	// BBBuilderName is the name of the builder that deploy build runs.
	BBBuilderName string
}

// SelectSchedulingClient select a proper scheduling client based on buildername
func SelectSchedulingClient(ctx context.Context, buildername string, ufsC ufsAPI.FleetClient, authOpts auth.Options, hostname string) (schedulingapi.TaskSchedulingAPI, error) {
	if !buildbucket.IfUseScheduke(buildername) {
		return nil, nil
	}
	sc, err := SchedukeClient(ctx, ufsC, authOpts, hostname)
	if err != nil {
		return nil, errors.Annotate(err, "creating Scheduke client").Err()
	}
	return sc, nil
}

// ScheduleDeployTask schedules a deploy task by Buildbucket for PARIS.
func ScheduleDeployTask(ctx context.Context, params DeployTaskParams) error {
	if params.Unit == "" {
		return errors.Reason("schedule deploy task: unit name is empty").Err()
	}
	v := buildbucket.CIPDProd
	if params.UseLatestVersion {
		v = buildbucket.CIPDLatest
	}
	adminServicePath := params.Env.AdminService
	contextNamespace := ReadContextNamespace(ctx, ufsUtil.OSNamespace)
	if contextNamespace == ufsUtil.OSPartnerNamespace {
		// Partner do not have options with stable version.
		adminServicePath = ""
	}
	p := &buildbucket.Params{
		BuilderName:    params.BBBuilderName,
		BuilderProject: params.BBProject,
		BuilderBucket:  params.BBBucket,
		UnitName:       params.Unit,
		TaskName:       string(buildbucket.Deploy),
		EnableRecovery: true,
		AdminService:   adminServicePath,
		// NOTE: We use the UFS service, not the Inventory service here.
		InventoryService:   params.Env.UnifiedFleetService,
		InventoryNamespace: contextNamespace,
		UpdateInventory:    true,
		ExtraTags: []string{
			params.SessionTag,
			"task:deploy",
			"client:shivas",
			fmt.Sprintf("inventory_namespace:%s", contextNamespace),
			fmt.Sprintf("version:%s", v),
		},
	}
	url, _, err := buildbucket.CreateTask(ctx, params.Client, params.SchedulingClient, v, p, "shivas")
	if err != nil {
		return errors.Annotate(err, "schedule deploy task").Err()
	}
	fmt.Printf("Triggered Deploy task %s. Follow the deploy job at %s\n", p.UnitName, url)
	return nil
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
