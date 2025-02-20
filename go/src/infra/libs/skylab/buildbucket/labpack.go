// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package buildbucket

import (
	"context"
	"fmt"
	"log"
	"strings"

	structbuilder "google.golang.org/protobuf/types/known/structpb"

	"go.chromium.org/luci/common/errors"

	schedulingapi "go.chromium.org/infra/libs/fleet/scheduling/api"
)

// Params are the parameters to the labpack job.
type Params struct {
	// BuilderProject -- treated as "chromeos" by default.
	BuilderProject string
	// BuilderBucket -- treated as "labpack_runner" by default.
	BuilderBucket string
	// BuilderName -- treated as "labpack_builder" by default
	BuilderName string
	// UnitName is the DUT or similar that we are scheduling against.
	// For example, a DUT hostname is a valid UnitName.
	UnitName string
	// TaskName is used to drive the recovery process, e.g. "labstation_deploy".
	TaskName string
	// Whether recovery actions are enabled or not.
	EnableRecovery bool
	// Hostname of the admin service.
	AdminService string
	// Hostname of the inventory service.
	InventoryService string
	// Namespace to use in inventory service.
	InventoryNamespace string
	// Whether to update the inventory or not when the task is finished.
	UpdateInventory bool
	// NoStepper determines whether the log stepper things.
	NoStepper bool
	// NoMetrics determines whether metrics recording (Karte) is in effect.
	NoMetrics bool
	// Disable CFT as part of task preparation.
	DisableCft bool
	// ExpectedState is the state that the DUT must be in in order for the task to trigger.
	// For example, a repair task MUST NOT be eligible to run on a "ready" DUT since that would
	// be a waste of resources.
	ExpectedState string
	// Configuration is a base64-encoded string of the job config.
	Configuration string
	// Extra tags setting to the swarming task.
	ExtraTags []string
}

// AsMap takes the parameters and flattens it into a map with string keys.
//
// Note that some fields, for example "builder_name" and "expected_state" intentionally do NOT
// end up as properties here.
func (p *Params) AsMap() map[string]interface{} {
	return map[string]interface{}{
		"unit_name":           p.UnitName,
		"task_name":           p.TaskName,
		"enable_recovery":     p.EnableRecovery,
		"admin_service":       p.AdminService,
		"inventory_service":   p.InventoryService,
		"update_inventory":    p.UpdateInventory,
		"no_stepper":          p.NoStepper,
		"no_metrics":          p.NoMetrics,
		"configuration":       p.Configuration,
		"inventory_namespace": p.InventoryNamespace,
		"disable_cft":         p.DisableCft,
	}
}

// CIPD version used for scheduling PARIS.
type CIPDVersion string

const (
	// Use prod version of CIPD package.
	CIPDProd CIPDVersion = "prod"
	// Use latest version of CIPD package.
	CIPDLatest CIPDVersion = "latest"
)

// Validate takes a cipd version and checks whether it's valid.
func (v CIPDVersion) Validate() error {
	switch v {
	case CIPDProd:
		return nil
	case CIPDLatest:
		return nil
	default:
		return errors.Reason("validate cipd version: unrecognized version %q", string(v)).Err()
	}
}

// ScheduleTask schedules a buildbucket task.
func ScheduleTask(ctx context.Context, client Client, v CIPDVersion, params *Params, serviceName string) (string, int64, error) {
	return CreateTask(ctx, client, nil, v, params, serviceName)
}

// CreateTask creates a task in scheduling service.
func CreateTask(ctx context.Context, client Client, sc schedulingapi.TaskSchedulingAPI, v CIPDVersion, params *Params, serviceName string) (url string, taskID int64, err error) {
	if client == nil {
		return "", 0, errors.Reason("schedule task: client cannot be nil").Err()
	}
	if params == nil {
		return "", 0, errors.Reason("schedule task: params cannot be nil").Err()
	}

	// Apply defaults.
	if params.BuilderName == "" {
		params.BuilderName = "labpack_builder"
	}
	if params.BuilderProject == "" {
		params.BuilderProject = "chromeos"
	}
	if params.BuilderBucket == "" {
		params.BuilderBucket = "labpack_runner"
	}

	props, err := structbuilder.NewStruct(params.AsMap())
	if err != nil {
		return "", 0, err
	}

	extraTags := params.ExtraTags
	extraTags = append(extraTags, fmt.Sprintf("service_name:%s", serviceName))

	p := &ScheduleLabpackTaskParams{
		BuilderName:      params.BuilderName,
		BuilderBucket:    params.BuilderBucket,
		BuilderProject:   params.BuilderProject,
		UnitName:         params.UnitName,
		ExpectedDUTState: params.ExpectedState,
		Props:            props,
		ExtraTags:        extraTags,
	}
	switch v {
	case CIPDProd:
		log.Println("Request to use prod CIPD version")
	case CIPDLatest:
		log.Println("Request to use latest CIPD version")
		if !strings.HasSuffix(params.BuilderName, "-latest") {
			p.BuilderName = fmt.Sprintf("%s-latest", params.BuilderName)
		}
	default:
		return "", 0, errors.Reason("scheduling task: unsupported CIPD version %s", v).Err()
	}
	if sc == nil {
		// TODO(b:347729967): remove when rollout finished.
		url, taskID, err = client.ScheduleLabpackTask(ctx, p, serviceName)
	} else {
		url, taskID, err = client.CreateLabpackTask(ctx, p, sc)
	}
	if err != nil {
		return "", 0, errors.Annotate(err, "scheduling task").Err()
	}
	return url, taskID, nil
}
