// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package run

import (
	"context"
	"fmt"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"

	"go.chromium.org/infra/cmd/crosfleet/internal/buildbucket"
	crosbb "go.chromium.org/infra/cros/lib/buildbucket"
	models "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsapi "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

type fakeUFSClient struct{}

// CheckFleetTestsPolicy checks the fleet test policy for the given test parameters.
func (c fakeUFSClient) CheckFleetTestsPolicy(ctx context.Context, req *ufsapi.CheckFleetTestsPolicyRequest, opt ...grpc.CallOption) (*ufsapi.CheckFleetTestsPolicyResponse, error) {
	return &ufsapi.CheckFleetTestsPolicyResponse{
		IsTestValid: true,
		TestStatus: &ufsapi.TestStatus{
			Code: ufsapi.TestStatus_OK,
		},
	}, nil
}

// GetMachineLSE gets information about a DUT.
func (c fakeUFSClient) GetMachineLSE(ctx context.Context, req *ufsapi.GetMachineLSERequest, opt ...grpc.CallOption) (*models.MachineLSE, error) {
	return nil, nil
}

// GetMachine retrieves the details of the machine.
func (c *fakeUFSClient) GetMachine(ctx context.Context, req *ufsapi.GetMachineRequest, opt ...grpc.CallOption) (*models.Machine, error) {
	return nil, nil
}

func TestSuiteNoModels(t *testing.T) {
	t.Parallel()
	r := suiteRun{
		testCommonFlags: testCommonFlags{
			exitEarly:          true,
			repeats:            1,
			priority:           DefaultSwarmingPriority,
			releaseRetryUrgent: true,
			release:            "R112-15357.0.0",
			board:              "drallion",
			pool:               "DUT_POOL_QUOTA",
		},
		allowDupes: true,
	}
	ctx := context.Background()

	ufs := &fakeUFSClient{}
	bb := &buildbucket.FakeClient{
		Client: buildbucket.FakeBuildClient{
			ExpectedSchedule: []buildbucket.ScheduleParams{
				{
					BuilderName: "cros_test_platform",
					Props: map[string]interface{}{
						"requests.default.params.scheduling.qsAccount": "release_p0",
					},
					Tags: map[string]string{
						"crosfleet-tool": "suite",
						"label-board":    "drallion",
						"label-image":    "drallion-release/R112-15357.0.0",
						"label-pool":     "DUT_POOL_QUOTA",
						"user_agent":     "crosfleet",
					},
				},
			},
		},
	}
	if err := r.innerRun(nil, []string{"bvt-installer"}, ctx, bb, ufs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSuiteModels(t *testing.T) {
	t.Parallel()
	r := suiteRun{
		testCommonFlags: testCommonFlags{
			exitEarly: true,
			repeats:   1,
			priority:  DefaultSwarmingPriority,
			qsAccount: "release_direct_sched",
			release:   "R112-15357.0.0",
			board:     "drallion",
			models:    []string{"drallion", "drallion360"},
			pool:      "DUT_POOL_QUOTA",
		},
		allowDupes: true,
	}
	ctx := context.Background()

	ufs := &fakeUFSClient{}
	bb := &buildbucket.FakeClient{
		Client: buildbucket.FakeBuildClient{
			ExpectedSchedule: []buildbucket.ScheduleParams{
				{
					BuilderName: "cros_test_platform",
					Props: map[string]interface{}{
						"requests.default.params.scheduling.qsAccount": "release_direct_sched",
					},
					Tags: map[string]string{
						"crosfleet-tool": "suite",
						"label-board":    "drallion",
						"label-model":    "drallion",
						"label-image":    "drallion-release/R112-15357.0.0",
						"label-pool":     "DUT_POOL_QUOTA",
						"user_agent":     "crosfleet",
					},
				}, {
					BuilderName: "cros_test_platform",
					Props: map[string]interface{}{
						"requests.default.params.scheduling.qsAccount": "release_direct_sched",
					},
					Tags: map[string]string{
						"crosfleet-tool": "suite",
						"label-board":    "drallion",
						"label-model":    "drallion360",
						"label-image":    "drallion-release/R112-15357.0.0",
						"label-pool":     "DUT_POOL_QUOTA",
						"user_agent":     "crosfleet",
					},
				},
			},
		},
	}
	err := r.innerRun(nil, []string{"bvt-installer"}, ctx, bb, ufs)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSuiteDedupeNoModels_Run(t *testing.T) {
	t.Parallel()
	r := suiteRun{
		testCommonFlags: testCommonFlags{
			exitEarly: true,
			repeats:   1,
			priority:  DefaultSwarmingPriority,
			release:   "R112-15357.0.0",
			board:     "drallion",
			pool:      "DUT_POOL_QUOTA",
		},
	}
	ctx := context.Background()
	suite := "bvt-installer"

	ufs := &fakeUFSClient{}
	bb := &buildbucket.FakeClient{
		ExpectedGetIncompleteBuildsWithTags: []*buildbucket.ExpectedGetWithTagsCall{
			{
				Tags: map[string]string{
					"crosfleet-tool": "suite",
					"user_agent":     "crosfleet",
					"label-image":    "drallion-release/R112-15357.0.0",
					"label-suite":    suite,
				},
				Response: []*buildbucketpb.Build{},
			},
		},
		Client: buildbucket.FakeBuildClient{
			ExpectedSchedule: []buildbucket.ScheduleParams{
				{
					BuilderName: "cros_test_platform",
					Props: map[string]interface{}{
						"requests.default.params.scheduling.priority": "140",
					},
					Tags: map[string]string{
						"crosfleet-tool": "suite",
						"label-board":    "drallion",
						"label-image":    "drallion-release/R112-15357.0.0",
						"label-pool":     "DUT_POOL_QUOTA",
						"user_agent":     "crosfleet",
					},
				},
			},
		},
	}
	err := r.innerRun(nil, []string{suite}, ctx, bb, ufs)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSuiteDedupeNoModels_NoRun(t *testing.T) {
	t.Parallel()
	r := suiteRun{
		testCommonFlags: testCommonFlags{
			exitEarly:   true,
			repeats:     1,
			priority:    DefaultSwarmingPriority,
			bucket:      defaultImageBucket,
			timeoutMins: 360,
			release:     "R112-15357.0.0",
			board:       "drallion",
			pool:        "DUT_POOL_QUOTA",
			cft:         true,
			testArgs:    "testArgsExample",
		},
		tagIncludes: []string{"tag1"},
	}
	ctx := context.Background()
	suite := "bvt-installer"

	ufs := &fakeUFSClient{}
	bb := &buildbucket.FakeClient{
		ExpectedGetIncompleteBuildsWithTags: []*buildbucket.ExpectedGetWithTagsCall{
			{
				Tags: map[string]string{
					"crosfleet-tool": "suite",
					"user_agent":     "crosfleet",
					"label-image":    "drallion-release/R112-15357.0.0",
					"label-suite":    suite,
				},
				Response: []*buildbucketpb.Build{
					{
						Builder: &buildbucketpb.BuilderID{
							Builder: "cros_test_platform",
						},
						Status: buildbucketpb.Status_SCHEDULED,
						Tags: []*buildbucketpb.StringPair{
							{
								Key:   "label-image",
								Value: "drallion-release/R112-15357.0.0",
							},
							{
								Key:   "label-suite",
								Value: suite,
							},
						},
						Input: &buildbucketpb.Build_Input{
							Properties: getInputProps(t, ""),
						},
					},
				},
			},
		},
		Client: buildbucket.FakeBuildClient{},
	}
	err := r.innerRun(nil, []string{suite}, ctx, bb, ufs)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSuiteDedupeModels_Run(t *testing.T) {
	t.Parallel()
	r := suiteRun{
		testCommonFlags: testCommonFlags{
			exitEarly: true,
			repeats:   1,
			priority:  DefaultSwarmingPriority,
			release:   "R112-15357.0.0",
			board:     "drallion",
			models:    []string{"drallion", "drallion360"},
			pool:      "DUT_POOL_QUOTA",
		},
	}
	ctx := context.Background()
	suite := "bvt-installer"

	ufs := &fakeUFSClient{}
	bb := &buildbucket.FakeClient{
		ExpectedGetIncompleteBuildsWithTags: []*buildbucket.ExpectedGetWithTagsCall{
			{
				Tags: map[string]string{
					"crosfleet-tool": "suite",
					"user_agent":     "crosfleet",
					"label-image":    "drallion-release/R112-15357.0.0",
					"label-suite":    suite,
					"label-model":    "drallion",
				},
				Response: []*buildbucketpb.Build{
					{
						Builder: &buildbucketpb.BuilderID{
							Builder: "cros_test_platform",
						},
						Status: buildbucketpb.Status_SCHEDULED,
						Tags: []*buildbucketpb.StringPair{
							{
								Key:   "label-image",
								Value: "drallion-release/R112-15357.0.0",
							},
							{
								Key:   "label-suite",
								Value: suite,
							},
							{
								Key:   "label-model",
								Value: "drallion",
							},
						},
					},
				},
			},
			{
				Tags: map[string]string{
					"crosfleet-tool": "suite",
					"user_agent":     "crosfleet",
					"label-image":    "drallion-release/R112-15357.0.0",
					"label-suite":    suite,
					"label-model":    "drallion360",
				},
				Response: []*buildbucketpb.Build{},
			},
		},
		Client: buildbucket.FakeBuildClient{
			ExpectedSchedule: []buildbucket.ScheduleParams{
				{
					BuilderName: "cros_test_platform",
					Tags: map[string]string{
						"crosfleet-tool": "suite",
						"label-board":    "drallion",
						"label-image":    "drallion-release/R112-15357.0.0",
						// drallion had an existing run, only expect drallion360
						"label-model": "drallion360",
						"label-pool":  "DUT_POOL_QUOTA",
						"user_agent":  "crosfleet",
					},
				},
			},
		},
	}
	err := r.innerRun(nil, []string{suite}, ctx, bb, ufs)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func getInputProps(t *testing.T, model string) *structpb.Struct {
	t.Helper()
	inputProps, err := structpb.NewStruct(map[string]interface{}{
		"requests": map[string]interface{}{
			"my-custom-request-name": map[string]interface{}{
				"params": map[string]interface{}{
					"decorations":        map[string]interface{}{},
					"freeformAttributes": map[string]interface{}{},
					"hardwareAttributes": map[string]interface{}{},
					"metadata": map[string]interface{}{
						"containerMetadataUrl":   "gs://chromeos-image-archive/drallion-release/R112-15357.0.0/metadata/containers.jsonpb",
						"debugSymbolsArchiveUrl": "gs://chromeos-image-archive/drallion-release/R112-15357.0.0",
						"testMetadataUrl":        "gs://chromeos-image-archive/drallion-release/R112-15357.0.0",
					},
					"retry": map[string]interface{}{},
					"scheduling": map[string]interface{}{
						"managedPool": "MANAGED_POOL_QUOTA",
						"priority":    "140",
					},
					"softwareAttributes": map[string]interface{}{
						"buildTarget": map[string]interface{}{
							"name": "drallion",
						},
					},
					"runViaCft": true,
					"softwareDependencies": []interface{}{
						map[string]interface{}{
							"chromeosBuildGcsBucket": "chromeos-image-archive",
						},
						map[string]interface{}{
							"chromeosBuild": "drallion-release/R112-15357.0.0",
						},
					},
					"time": map[string]interface{}{
						"maximumDuration": "21600s",
					},
					"trv2StepsConfig": map[string]any{
						"hwTestConfig": map[string]any{},
					},
				},
				"testPlan": map[string]interface{}{
					"suite": []interface{}{
						map[string]interface{}{
							"name":     "bvt-installer",
							"testArgs": "testArgsExample",
						},
					},
					"tagCriteria": map[string]interface{}{
						"tags": []interface{}{"tag1"},
					},
				},
			},
			"default": map[string]interface{}{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tags := []interface{}{
		"crosfleet-tool:suite",
		"label-board:drallion",
		"label-image:drallion-release/R112-15357.0.0",
		"label-pool:DUT_POOL_QUOTA",
		"label-priority:140",
		"label-suite:bvt-installer",
	}
	if model != "" {
		tags = append(tags, fmt.Sprintf("label-model:%s", model))
		if err := crosbb.SetProperty(inputProps, "requests.my-custom-request-name.params.hardwareAttributes.model", model); err != nil {
			t.Fatal(err)
		}
	}
	if err := crosbb.SetProperty(inputProps, "requests.my-custom-request-name.params.decorations.tags", tags); err != nil {
		t.Fatal(err)
	}
	return inputProps
}

func TestSuiteDedupeModels_NoRun(t *testing.T) {
	t.Parallel()
	r := suiteRun{
		testCommonFlags: testCommonFlags{
			exitEarly:   true,
			repeats:     1,
			priority:    DefaultSwarmingPriority,
			bucket:      defaultImageBucket,
			timeoutMins: 360,
			release:     "R112-15357.0.0",
			board:       "drallion",
			models:      []string{"drallion", "drallion360"},
			pool:        "DUT_POOL_QUOTA",
			cft:         true,
			testArgs:    "testArgsExample",
		},
		tagIncludes: []string{"tag1"},
	}
	ctx := context.Background()
	suite := "bvt-installer"

	ufs := &fakeUFSClient{}
	bb := &buildbucket.FakeClient{
		ExpectedGetIncompleteBuildsWithTags: []*buildbucket.ExpectedGetWithTagsCall{
			{
				Tags: map[string]string{
					"crosfleet-tool": "suite",
					"user_agent":     "crosfleet",
					"label-image":    "drallion-release/R112-15357.0.0",
					"label-suite":    suite,
					"label-model":    "drallion",
				},
				Response: []*buildbucketpb.Build{
					{
						Builder: &buildbucketpb.BuilderID{
							Builder: "cros_test_platform",
						},
						Status: buildbucketpb.Status_SCHEDULED,
						Tags: []*buildbucketpb.StringPair{
							{
								Key:   "label-image",
								Value: "drallion-release/R112-15357.0.0",
							},
							{
								Key:   "label-suite",
								Value: suite,
							},
							{
								Key:   "label-model",
								Value: "drallion",
							},
						},
						Input: &buildbucketpb.Build_Input{
							Properties: getInputProps(t, "drallion"),
						},
					},
				},
			},
			{
				Tags: map[string]string{
					"crosfleet-tool": "suite",
					"user_agent":     "crosfleet",
					"label-image":    "drallion-release/R112-15357.0.0",
					"label-suite":    suite,
					"label-model":    "drallion360",
				},
				Response: []*buildbucketpb.Build{
					{
						Builder: &buildbucketpb.BuilderID{
							Builder: "cros_test_platform",
						},
						Status: buildbucketpb.Status_SCHEDULED,
						Tags: []*buildbucketpb.StringPair{
							{
								Key:   "label-image",
								Value: "drallion-release/R112-15357.0.0",
							},
							{
								Key:   "label-suite",
								Value: suite,
							},
							{
								Key:   "label-model",
								Value: "drallion360",
							},
						},
						Input: &buildbucketpb.Build_Input{
							Properties: getInputProps(t, "drallion360"),
						},
					},
				},
			},
			{
				Tags: map[string]string{
					"crosfleet-tool": "suite",
					"user_agent":     "crosfleet",
					"label-image":    "drallion-release/R112-15357.0.0",
					"label-suite":    suite,
					"label-model":    "drallion360",
				},
				Response: []*buildbucketpb.Build{},
			},
		},
		Client: buildbucket.FakeBuildClient{},
	}
	if err := r.innerRun(nil, []string{suite}, ctx, bb, ufs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
