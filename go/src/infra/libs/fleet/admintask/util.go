// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package admintask provides functions for scheduling admin tasks.
package admintask

import (
	"context"

	"google.golang.org/grpc/metadata"

	schedulingapi "go.chromium.org/infra/libs/fleet/scheduling/api"
	"go.chromium.org/infra/libs/skylab/buildbucket"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// Client holds the necessary clients and configuration for scheduling autorepair tasks.
type Client struct {
	UFSClient               ufsAPI.FleetClient
	BBClient                buildbucket.Client
	SchedukeClient          schedulingapi.TaskSchedulingAPI
	AdminServiceAddress     string                  // Network address of the admin service
	InventoryServiceAddress string                  // Network address of the inventory service. i.e. UFS
	InventoryNamespace      string                  // Namespace to use for inventory operations
	Version                 buildbucket.CIPDVersion // CIPD version to use for the task
}

// internal, generic params for admin tasks.
type Task struct {
	UpdateInventory bool
	ExtraTags       []string
	BuilderName     string
	BuilderBucket   string
}

// RepairTaskRequest holds the params for scheduling a repair task.
type RepairTaskRequest struct {
	Task
	UnitName   string
	DeepRepair bool
	OnlyVerify bool
}

type ScheduleTaskResult struct {
	TaskURL string
	// TODO: b/394429368 - add relevant fields needed for fleet console backend, e.g., TaskID, BuildID, etc
}

// getBuilderAndTaskName returns the builder and task name for a given repair task options
func (r *RepairTaskRequest) getBuilderAndTaskName() (string, string) {
	builderName := r.BuilderName
	if r.OnlyVerify {
		builderName = "verify"
	}
	taskName := string(buildbucket.Recovery)
	if r.DeepRepair {
		taskName = string(buildbucket.DeepRecovery)
	}
	return builderName, taskName
}

// getHive returns hive for a host in UFS
func getHive(ctx context.Context, ic ufsAPI.FleetClient, hostname string) string {
	dut, err := ic.GetMachineLSE(ctx, &ufsAPI.GetMachineLSERequest{
		Name: ufsUtil.AddPrefix(ufsUtil.MachineLSECollection, hostname),
	})
	// If DUT doesn't exist, just return empty hive as the job won't be triggered anyway
	if err != nil {
		return ""
	}

	// TODO: will only support DUTs but not labstation - look into supporting labstation in the future
	return ufsUtil.GetHiveForDut(hostname, dut.GetChromeosMachineLse().GetDeviceLse().GetDut().GetHive())
}

// readContextNamespace read namespace value from the context
func readContextNamespace(ctx context.Context, defaultValue string) string {
	md, ok := metadata.FromOutgoingContext(ctx)
	if ok {
		for _, v := range md.Get(ufsUtil.Namespace) {
			if v != "" {
				return v
			}
		}
	}
	return defaultValue
}
