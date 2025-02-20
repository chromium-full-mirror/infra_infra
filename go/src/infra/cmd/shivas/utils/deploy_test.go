// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/types/known/structpb"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cmd/shivas/site"
	schedulingapi "go.chromium.org/infra/libs/fleet/scheduling/api"
	"go.chromium.org/infra/libs/skylab/buildbucket"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// stubClient returns "safe" values and stores the params of the last labpack
// call. You should usually use `newStubClient()` since we need LastCall to be
// non-nil.
type stubClient struct {
	LastCall *buildbucket.ScheduleLabpackTaskParams
}

// newStubClient creates a properly initiated stub client for use
func newStubClient() stubClient {
	return stubClient{LastCall: &buildbucket.ScheduleLabpackTaskParams{}}
}

func (c stubClient) ScheduleLabpackTask(ctx context.Context, params *buildbucket.ScheduleLabpackTaskParams, _ string) (string, int64, error) {
	// Since this func is pass by value, we need to change the value at the
	// address of the pointer (since the address remains constant b/t calls).
	*c.LastCall = *params
	return "fake", 0, nil
}

func (c stubClient) CreateLabpackTask(ctx context.Context, params *buildbucket.ScheduleLabpackTaskParams, _ schedulingapi.TaskSchedulingAPI) (string, int64, error) {
	// TODO copy logic from ScheduleLabpackTask during migration.
	return "fake", 0, errors.Reason("Not expected to be called").Err()
}

// TestScheduleDeployTask tests params propagates appropriately
func TestScheduleDeployTask(t *testing.T) {
	tests := []struct {
		name         string
		ctx          context.Context
		deployParams *DeployTaskParams
		expectedCall *buildbucket.ScheduleLabpackTaskParams
	}{
		{
			name: "no explicit namespace",
			ctx:  context.Background(),
			deployParams: &DeployTaskParams{
				BBBuilderName: "deploy",
				BBProject:     "chromeos",
				BBBucket:      "labpack_runner",
			},
			expectedCall: &buildbucket.ScheduleLabpackTaskParams{
				UnitName: "test-unit",
				Props: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"admin_service":       structpb.NewStringValue("skylab-staging-bot-fleet.appspot.com"),
						"configuration":       structpb.NewStringValue(""),
						"disable_cft":         structpb.NewBoolValue(false),
						"enable_recovery":     structpb.NewBoolValue(true),
						"inventory_service":   structpb.NewStringValue("staging.ufs.api.cr.dev"),
						"inventory_namespace": structpb.NewStringValue("os"),
						"no_metrics":          structpb.NewBoolValue(false),
						"no_stepper":          structpb.NewBoolValue(false),
						"task_name":           structpb.NewStringValue("deploy"),
						"unit_name":           structpb.NewStringValue("test-unit"),
						"update_inventory":    structpb.NewBoolValue(true),
					},
				},
				ExtraTags:      []string{"test-session", "task:deploy", "client:shivas", "inventory_namespace:os", "version:prod", "service_name:shivas"},
				BuilderName:    "deploy",
				BuilderProject: "chromeos",
				BuilderBucket:  "labpack_runner",
			},
		},
		{
			name: "explicit namespace",
			ctx:  SetupContext(context.Background(), ufsUtil.OSPartnerNamespace),
			deployParams: &DeployTaskParams{
				BBBuilderName: "deploy",
				BBProject:     "chromeos",
				BBBucket:      "labpack_runner",
			},
			expectedCall: &buildbucket.ScheduleLabpackTaskParams{
				UnitName: "test-unit",
				Props: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"admin_service":       structpb.NewStringValue(""),
						"configuration":       structpb.NewStringValue(""),
						"disable_cft":         structpb.NewBoolValue(false),
						"enable_recovery":     structpb.NewBoolValue(true),
						"inventory_service":   structpb.NewStringValue("staging.ufs.api.cr.dev"),
						"inventory_namespace": structpb.NewStringValue("os-partner"),
						"no_metrics":          structpb.NewBoolValue(false),
						"no_stepper":          structpb.NewBoolValue(false),
						"task_name":           structpb.NewStringValue("deploy"),
						"unit_name":           structpb.NewStringValue("test-unit"),
						"update_inventory":    structpb.NewBoolValue(true),
					},
				},
				ExtraTags:      []string{"test-session", "task:deploy", "client:shivas", "inventory_namespace:os-partner", "version:prod", "service_name:shivas"},
				BuilderName:    "deploy",
				BuilderProject: "chromeos",
				BuilderBucket:  "labpack_runner",
			},
		},
		{
			name: "explicit params",
			ctx:  context.Background(),
			deployParams: &DeployTaskParams{
				BBBuilderName: "special-deploy",
				BBBucket:      "eli-bucket",
				BBProject:     "eli-project",
			},
			expectedCall: &buildbucket.ScheduleLabpackTaskParams{
				UnitName: "test-unit",
				Props: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"admin_service":       structpb.NewStringValue("skylab-staging-bot-fleet.appspot.com"),
						"configuration":       structpb.NewStringValue(""),
						"disable_cft":         structpb.NewBoolValue(false),
						"enable_recovery":     structpb.NewBoolValue(true),
						"inventory_service":   structpb.NewStringValue("staging.ufs.api.cr.dev"),
						"inventory_namespace": structpb.NewStringValue("os"),
						"no_metrics":          structpb.NewBoolValue(false),
						"no_stepper":          structpb.NewBoolValue(false),
						"task_name":           structpb.NewStringValue("deploy"),
						"unit_name":           structpb.NewStringValue("test-unit"),
						"update_inventory":    structpb.NewBoolValue(true),
					},
				},
				ExtraTags:      []string{"test-session", "task:deploy", "client:shivas", "inventory_namespace:os", "version:prod", "service_name:shivas"},
				BuilderName:    "special-deploy",
				BuilderProject: "eli-project",
				BuilderBucket:  "eli-bucket",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// t.Parallel() disabled since we have global state used to verify calls
			client := newStubClient()
			tt.deployParams.Client = client
			tt.deployParams.Env = site.Dev(false)
			tt.deployParams.Unit = "test-unit"
			tt.deployParams.SessionTag = "test-session"
			tt.deployParams.UseLatestVersion = false
			err := ScheduleDeployTask(tt.ctx, *tt.deployParams)
			if err != nil {
				t.Errorf("unexpected err: %s", err)
			}
			if diff := cmp.Diff(client.LastCall, tt.expectedCall, cmpopts.IgnoreUnexported(structpb.Struct{}, structpb.Value{})); diff != "" {
				t.Errorf("unexpected diff in calls: %s", diff)
			}
		})
	}
}
