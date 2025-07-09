// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package admintask provides functions for scheduling admin tasks.
package admintask

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/libs/fleet/device"
	"go.chromium.org/infra/libs/skylab/buildbucket"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

const (
	ClientTag = "client:fleet-admin-lib"
)

func (c *Client) ScheduleRepairTask(
	ctx context.Context,
	t *RepairTaskRequest,
) (result *ScheduleTaskResult, err error) {

	builderName, taskName := t.getBuilderAndTaskName()

	hive := ufsUtil.GetHiveForDut(t.UnitName, getHive(ctx, c.UFSClient, t.UnitName))
	realBuilderName := buildbucket.BuilderNamePerHive(builderName, hive)

	c.InventoryNamespace = readContextNamespace(ctx, ufsUtil.OSNamespace)
	if c.InventoryNamespace == ufsUtil.OSPartnerNamespace {
		// Partner do not have options with stable version.
		c.AdminServiceAddress = ""
	}

	if !buildbucket.IfUseScheduke(realBuilderName) {
		c.SchedukeClient = nil
	}

	di, err := device.GetDeviceInfo(ctx, c.UFSClient, t.UnitName)
	if err != nil {
		return nil, errors.Annotate(err, "failed to get device info").Err()
	}

	url, _, err := buildbucket.CreateTask(
		ctx,
		c.BBClient,
		c.SchedukeClient,
		c.Version,
		&buildbucket.Params{
			UnitName:           t.UnitName,
			UnitID:             di.ID,
			TaskName:           taskName,
			BuilderName:        realBuilderName,
			BuilderBucket:      t.BuilderBucket,
			EnableRecovery:     !t.OnlyVerify,
			AdminService:       c.AdminServiceAddress,
			InventoryService:   c.InventoryServiceAddress,
			InventoryNamespace: c.InventoryNamespace,
			UpdateInventory:    t.UpdateInventory,
			ExtraTags:          t.ExtraTags,
		},
		ClientTag,
	)

	if err != nil {
		return nil, errors.Annotate(err, "failed to create task").Err()
	}
	result = &ScheduleTaskResult{
		TaskURL: url,
	}
	return result, nil
}
