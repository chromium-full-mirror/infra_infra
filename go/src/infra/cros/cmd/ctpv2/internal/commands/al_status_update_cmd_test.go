// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	androidapi "go.chromium.org/infra/cros/cmd/common_lib/android_api"
	mock_androidapi "go.chromium.org/infra/cros/cmd/common_lib/android_api/mocks"
	"go.chromium.org/infra/cros/cmd/common_lib/ants/androidbuildinternal/v3"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2/data"
)

func TestUpdateInvocationProperties(t *testing.T) {
	ctx := context.Background()

	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockInvService := mock_androidapi.NewMockInvocationService(mockCtl)
	s := &androidapi.Service{InvocationService: mockInvService}
	inv := &androidbuildinternal.Invocation{InvocationId: "I123"}

	testCases := []struct {
		name     string
		cbIngest bool
		sealInv  bool
	}{
		{
			name:     "cbProp",
			cbIngest: true,
		},
		{
			name: "noProp",
		},
		{
			name:     "sealedInvocation",
			cbIngest: true,
			sealInv:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := []*api.Arg{}
			if tc.cbIngest {
				args = append(args, &api.Arg{Flag: common.InvocationDataFlag, Value: common.CbIngestionValue})
			}

			wu := &androidapi.WorkUnitNode{}
			wu.SetWorkUnit(&androidbuildinternal.WorkUnit{InvocationId: "I123"})
			cmd := &AlStatusUpdateCmd{
				AlStateInfo: &data.AlStateInfo{
					WorkUnitTrees: map[string]*androidapi.WorkUnitTree{"test": {Head: wu}},
				},
				CtpRequest: &api.CTPRequest{
					SuiteRequest: &api.SuiteRequest{
						SuiteRequest: &api.SuiteRequest_TestSuite{
							TestSuite: &api.TestSuite{
								ExecutionMetadata: &api.ExecutionMetadata{Args: args},
							},
						},
					},
				},
				service: s,
			}

			if tc.cbIngest {
				if tc.sealInv {
					inv.SchedulerState = "error"
				}
				mockInvService.EXPECT().Get(ctx, "I123").Return(inv, nil)
				inv.Properties = []*androidbuildinternal.Property{
					{Name: common.CbPropName, Value: "yes"},
					{Name: common.CbMetricsPropName, Value: "yes"},
				}
				if !tc.sealInv {
					mockInvService.EXPECT().Update("I123", inv).Return(inv, nil)
				}
			}
			err := cmd.updateInvocationProperties(ctx)
			if !tc.sealInv && err != nil {
				t.Errorf("Unexpected error: %q", err)
			}
		})
	}
}

func TestALInvocationInformation(t *testing.T) {
	testCases := []struct {
		name           string
		dutInfo        *labapi.Dut
		swReqKeyValues []*api.KeyValue
	}{
		{
			name: "allIncluded",
			dutInfo: &labapi.Dut{
				DutType: &labapi.Dut_Chromeos{
					Chromeos: &labapi.Dut_ChromeOS{
						DutModel: &labapi.DutModel{
							BuildTarget: "brya",
							ModelName:   "mithrax",
						},
					},
				},
			},
			swReqKeyValues: []*api.KeyValue{
				{Key: "al_build_id", Value: "12345"},
				{Key: "al_build_target", Value: "brya-staging"},
				{Key: "al_build_branch", Value: "brya-branch"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &AlStatusUpdateCmd{
				BuildsMap: map[string]*data.BuildRequest{
					"foo": {
						SuiteInfo: &api.SuiteInfo{
							SuiteMetadata: &api.SuiteMetadata{
								SchedulingUnits: []*api.SchedulingUnit{
									{
										PrimaryTarget: &api.Target{
											SwReq: &api.LegacySW{
												KeyValues: tc.swReqKeyValues,
											},
											SwarmingDef: &api.SwarmingDefinition{
												DutInfo: tc.dutInfo,
											},
										},
									},
								},
							},
						},
					},
				},
			}

			gotBuildID, gotBuildTarget, gotRunTarget, gotBuildBranch := cmd.alInvocationInformation()

			if gotBuildID != "12345" {
				t.Errorf("Unexpected buildID: got(%s), want(%s)", gotBuildID, "12345")
			}

			if gotBuildTarget != "brya-staging" {
				t.Errorf("Unexpected buildTarget: got(%s), want(%s)", gotBuildID, "brya-staging")
			}
			if gotBuildBranch != "brya-branch" {
				t.Errorf("Unexpected buildBranch: got(%s), want(%s)", gotBuildID, "brya-branch")
			}

			wantRunTarget := fmt.Sprintf("%s_%s", tc.dutInfo.GetChromeos().DutModel.BuildTarget, tc.dutInfo.GetChromeos().DutModel.ModelName)
			if gotRunTarget != wantRunTarget {
				t.Errorf("Unexpected runTarget: got(%s), want(%s)", gotRunTarget, wantRunTarget)
			}
		})
	}
}
