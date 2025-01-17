// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"log"
	"os"
	"testing"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	storage_path "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

func getInternalTestPlan() *api.InternalTestplan {
	skyrim := &testapi.Dut_ChromeOS{DutModel: &testapi.DutModel{
		BuildTarget: "skyrim",
		ModelName:   "frostflow",
	}}
	octopus := &testapi.Dut_ChromeOS{DutModel: &testapi.DutModel{
		BuildTarget: "octopus",
		ModelName:   "bluebird",
	}}
	metadata, err := anypb.New(&api.CrOSProvisionMetadata{})
	if err != nil {
		log.Fatalf("Error creating anypb.Any: %v", err)
	}

	req := &api.InternalTestplan{
		TestCases: []*api.CTPTestCase{
			{
				Name: "test1",
				Metadata: &api.TestCaseMetadata{
					TestCase: &api.TestCase{
						Id: &api.TestCase_Id{
							Value: "test1",
						},
						BuildDependencies: []*api.TestCase_BuildDeps{
							{
								Value: "b1",
							},
							{
								Value: "b2",
							},
						},
					},
				},
				SchedulingUnitOptions: []*api.SchedulingUnitOptions{
					{
						SchedulingUnits: []*api.SchedulingUnit{
							{
								PrimaryTarget: &api.Target{
									SwarmingDef: &api.SwarmingDefinition{
										DutInfo: &testapi.Dut{
											DutType: &testapi.Dut_Chromeos{Chromeos: octopus},
										},
										SwarmingLabels: []string{
											"label-peripheral_wifi_state:WORKING",
											"label-wificell:True",
										},
									},
									SwReq: &api.LegacySW{},
								},
								DynamicUpdateLookupTable: map[string]string{
									"board":       "octopus",
									"installPath": "gs://chromeos-image-archive/octopus-release/R128-15964.4.0",
								},
							},
						},
					},
					{
						SchedulingUnits: []*api.SchedulingUnit{
							{
								PrimaryTarget: &api.Target{
									SwarmingDef: &api.SwarmingDefinition{
										DutInfo: &testapi.Dut{
											DutType: &testapi.Dut_Chromeos{Chromeos: skyrim},
										},
										SwarmingLabels: []string{
											"label-peripheral_wifi_state:WORKING",
											"label-wificell:True",
										},
									},
									SwReq: &api.LegacySW{},
								},
								DynamicUpdateLookupTable: map[string]string{
									"board":       "skyrim",
									"installPath": "gs://chromeos-image-archive/skyrim-release/R128-15964.4.0",
								},
							},
						},
					},
				},
			},
			{
				Name: "test2",
				Metadata: &api.TestCaseMetadata{
					TestCase: &api.TestCase{
						Id: &api.TestCase_Id{
							Value: "test2",
						},
						BuildDependencies: []*api.TestCase_BuildDeps{
							{
								Value: "b1",
							},
							{
								Value: "b2",
							},
						},
					},
				},
				SchedulingUnitOptions: []*api.SchedulingUnitOptions{
					{
						SchedulingUnits: []*api.SchedulingUnit{
							{
								PrimaryTarget: &api.Target{
									SwarmingDef: &api.SwarmingDefinition{
										DutInfo: &testapi.Dut{
											DutType: &testapi.Dut_Chromeos{Chromeos: octopus},
										},
										SwarmingLabels: []string{
											"label-peripheral_wifi_state:WORKING",
											"label-wificell:True",
										},
									},
									SwReq: &api.LegacySW{},
								},
								DynamicUpdateLookupTable: map[string]string{
									"board":       "octopus",
									"installPath": "gs://chromeos-image-archive/octopus-release/R128-15964.4.0",
								},
							},
						},
					},
				},
			},
		},
		SuiteInfo: &api.SuiteInfo{
			SuiteMetadata: &api.SuiteMetadata{
				Pool: "wificell_perbuild",
				ExecutionMetadata: &api.ExecutionMetadata{
					Args: []*api.Arg{},
				},
				SchedulerInfo: &api.SchedulerInfo{
					Scheduler: 2,
					QsAccount: "unmanaged_p4",
				},
				DynamicUpdates: []*api.UserDefinedDynamicUpdate{
					{
						FocalTaskFinder: &api.FocalTaskFinder{
							Finder: &api.FocalTaskFinder_First_{
								First: &api.FocalTaskFinder_First{
									TaskType: 2,
								},
							},
						},
						UpdateAction: &api.UpdateAction{
							Action: &api.UpdateAction_Insert_{
								Insert: &api.UpdateAction_Insert{
									InsertType: 1,
									Task: &api.CrosTestRunnerDynamicRequest_Task{
										OrderedContainerRequests: []*api.ContainerRequest{
											{
												DynamicIdentifier: "crosDutServer_primary",
												Container: &api.Template{
													Container: &api.Template_CrosDut{},
												},
												DynamicDeps: []*api.DynamicDep{
													{
														Key:   "crosDut.cacheServer",
														Value: "device_primary.dut.cacheServer.address",
													},
													{
														Key:   "crosDut.dutAddress",
														Value: "device_primary.dutserver",
													},
												},
												ContainerImageKey: "cros-dut",
											},
											{
												DynamicIdentifier: "cros-provision_primary",
												Container: &api.Template{
													Container: &api.Template_Generic{
														Generic: &api.GenericTemplate{
															BinaryName: "cros-provision",
															BinaryArgs: []string{
																"server",
																"-port",
																"0",
															},
															DockerArtifactDir: "/tmp/provisionservice",
															AdditionalVolumes: []string{
																"/creds:/creds",
															},
														},
													},
												},
												ContainerImageKey: "cros-dut",
											},
										},
										Task: &api.CrosTestRunnerDynamicRequest_Task_Provision{
											Provision: &api.ProvisionTask{
												ServiceAddress: &testapi.IpEndpoint{},
												StartupRequest: &api.ProvisionStartupRequest{},
												InstallRequest: &api.InstallRequest{
													ImagePath: &storage_path.StoragePath{
														HostType: 2,
														Path:     "${installPath}",
													},
													Metadata: metadata,
												},
												DynamicDeps: []*api.DynamicDep{
													{
														Key:   "serviceAddress",
														Value: "cros-provision_primary",
													},
													{
														Key:   "startupRequest.dut",
														Value: "device_primary.dut",
													},
													{
														Key:   "startupRequest.dutServer",
														Value: "crosDutServer_primary",
													},
												},
												Target:            "primary",
												DynamicIdentifier: "cros-provision_primary",
											},
										},
									},
								},
							},
						},
					},
				},
				SchedulingUnits: []*api.SchedulingUnit{
					{
						PrimaryTarget: &api.Target{
							SwarmingDef: &api.SwarmingDefinition{
								DutInfo: &testapi.Dut{
									DutType: &testapi.Dut_Chromeos{Chromeos: skyrim},
								},
								ProvisionInfo: []*api.ProvisionInfo{
									{
										InstallRequest: &api.InstallRequest{
											ImagePath: &storage_path.StoragePath{
												HostType: 2,
												Path:     "gs://chromeos-image-archive/skyrim-release/R128-15964.4.0",
											},
										},
									},
								},
							},
							SwReq: &api.LegacySW{
								Build:   "release",
								GcsPath: "gs://chromeos-image-archive/skyrim-release/R128-15964.4.0",
								KeyValues: []*api.KeyValue{
									{
										Key:   "chromeos_build_gcs_bucket",
										Value: "chromeos-image-archive",
									},
									{
										Key:   "chromeos_build",
										Value: "skyrim-release/R128-15964.4.0",
									},
								},
							},
						},
						DynamicUpdateLookupTable: map[string]string{
							"board":       "skyrim",
							"installPath": "gs://chromeos-image-archive/skyrim-release/R128-15964.4.0",
						},
					},
				},
			},
			SuiteRequest: &api.SuiteRequest{
				SuiteRequest: &api.SuiteRequest_TestSuite{
					TestSuite: &api.TestSuite{
						Name: "wifi_endtoend",
						Spec: &api.TestSuite_TestCaseTagCriteria_{
							TestCaseTagCriteria: &api.TestSuite_TestCaseTagCriteria{
								Tags: []string{"suite:wifi_endtoend"},
							},
						},
					},
				},
				MaximumDuration: durationpb.New(142200),
				TestArgs:        "None",
				AnalyticsName:   "wifi_endtoend_perbuild__wificell_perbuild__wifi_endtoend__tauto__new_build",
			},
		},
	}
	return req
}
func TestUpdateTestCases_OneTestFlaky_OneSchedulingUnitRemoved(t *testing.T) {
	log := log.New(os.Stdout, "test", 1)
	internalTestPlanReq := getInternalTestPlan()

	// stabilityData result
	removeBoardTestMap := make(map[string][]string)
	// test1 is scheduled on Octopus, it's expected to be removed
	removeBoardTestMap["octopus"] = []string{"test1"}

	err := updateTestCases(internalTestPlanReq, removeBoardTestMap, log)
	if err != nil {
		t.Errorf("Error not expected. Recieved %s", err)
	}

	for _, tc := range internalTestPlanReq.GetTestCases() {
		if tc.Name == "test1" {
			if len(tc.GetSchedulingUnitOptions()) != 1 {
				t.Errorf("Expected number of scheduling unit options : 1, found %d", len(tc.GetSchedulingUnitOptions()))
			}
		}
		if tc.Name == "test2" {
			if len(tc.GetSchedulingUnitOptions()) != 1 {
				t.Errorf("Expected number of scheduling unit options : 1, found %d", len(tc.GetSchedulingUnitOptions()))
			}
		}
	}
}

func TestUpdateTestCases_NoTestFlaky_NoSchedulingUnitRemoved(t *testing.T) {
	log := log.New(os.Stdout, "test", 1)
	internalTestPlanReq := getInternalTestPlan()

	// stabilityData result
	removeBoardTestMap := make(map[string][]string)
	// test3 is not in request, so no scheduling unit should be removed
	removeBoardTestMap["octopus"] = []string{"test3"}

	err := updateTestCases(internalTestPlanReq, removeBoardTestMap, log)
	if err != nil {
		t.Errorf("Error not expected. Recieved %s", err)
	}

	for _, tc := range internalTestPlanReq.GetTestCases() {
		if tc.Name == "test1" {
			if len(tc.GetSchedulingUnitOptions()) != 2 {
				t.Errorf("Expected number of scheduling unit options : 2, found %d", len(tc.GetSchedulingUnitOptions()))
			}
		}
		if tc.Name == "test2" {
			if len(tc.GetSchedulingUnitOptions()) != 1 {
				t.Errorf("Expected number of scheduling unit options : 1, found %d", len(tc.GetSchedulingUnitOptions()))
			}
		}
	}
}

func TestUpdateTestCases_OneTestFlaky_BothSchedulingUnitsRemoved(t *testing.T) {
	log := log.New(os.Stdout, "test", 1)
	internalTestPlanReq := getInternalTestPlan()

	// stabilityData result
	removeBoardTestMap := make(map[string][]string)
	// test1 is scheduled on Octopus & Skyrim only. Since both of these are expected to be removed, there should be no Scheduling unit options and hence the test itself should be removed.
	removeBoardTestMap["octopus"] = []string{"test1"}
	removeBoardTestMap["skyrim"] = []string{"test1"}

	err := updateTestCases(internalTestPlanReq, removeBoardTestMap, log)
	if err != nil {
		t.Errorf("Error not expected. Recieved %s", err)
	}

	for _, tc := range internalTestPlanReq.GetTestCases() {
		if tc.Name == "test1" {
			t.Errorf("test1 is not expected to be in updated internal test plan request")
		}
		if tc.Name == "test2" {
			if len(tc.GetSchedulingUnitOptions()) != 1 {
				t.Errorf("Expected number of scheduling unit options : 1, found %d", len(tc.GetSchedulingUnitOptions()))
			}
		}
	}
}

func TestUpdateTestCases_TwoTestsFlaky_TwoSchedulingUnitsRemoved(t *testing.T) {
	log := log.New(os.Stdout, "test", 1)
	internalTestPlanReq := getInternalTestPlan()

	// stabilityData result
	removeBoardTestMap := make(map[string][]string)
	// test1 is scheduled on Octopus & Skyrim only & test 2 is scheduled on Octopus only. Updated internal test plan request should only have test1 scheduled for Skyrim
	removeBoardTestMap["octopus"] = []string{"test1", "test2"}

	err := updateTestCases(internalTestPlanReq, removeBoardTestMap, log)
	if err != nil {
		t.Errorf("Error not expected. Recieved %s", err)
	}

	for _, tc := range internalTestPlanReq.GetTestCases() {
		if tc.Name == "test1" {
			if len(tc.GetSchedulingUnitOptions()) != 1 {
				t.Errorf("Expected number of scheduling unit options : 1, found %d", len(tc.GetSchedulingUnitOptions()))
			}
		}
		if tc.Name == "test2" {
			t.Errorf("test2 is not expected to be in updated internal test plan request")
		}
	}
}

func TestUpdateTestCases_TwoTestsFlaky_TwoDistinctSchedulingUnitsRemoved(t *testing.T) {
	log := log.New(os.Stdout, "test", 1)

	internalTestPlanReq := getInternalTestPlan()

	// stabilityData result
	removeBoardTestMap := make(map[string][]string)
	// test1 is scheduled on Octopus & Skyrim only & test 2 is scheduled on Octopus only. Updated internal test plan request should only have test1 scheduled for Skyrim
	removeBoardTestMap["skyrim"] = []string{"test1"}
	removeBoardTestMap["octopus"] = []string{"test2"}

	err := updateTestCases(internalTestPlanReq, removeBoardTestMap, log)
	if err != nil {
		t.Errorf("Error not expected. Recieved %s", err)
	}

	for _, tc := range internalTestPlanReq.GetTestCases() {
		if tc.Name == "test1" {
			if len(tc.GetSchedulingUnitOptions()) != 1 {
				t.Errorf("Expected number of scheduling unit options : 1, found %d", len(tc.GetSchedulingUnitOptions()))
			}
		}
		if tc.Name == "test2" {
			t.Errorf("test2 is not expected to be in updated internal test plan request")
		}
	}
}

func TestUpdateTestCases_TwoTestsFlaky_AllSchedulingUnitsRemoved(t *testing.T) {
	log := log.New(os.Stdout, "test", 1)
	internalTestPlanReq := getInternalTestPlan()

	// stabilityData result
	removeBoardTestMap := make(map[string][]string)
	// test1 is scheduled on Octopus & Skyrim only & test 2 is scheduled on Octopus only. Updated internal test plan request should only have test1 scheduled for Skyrim
	removeBoardTestMap["skyrim"] = []string{"test1"}
	removeBoardTestMap["octopus"] = []string{"test1", "test2"}

	err := updateTestCases(internalTestPlanReq, removeBoardTestMap, log)
	if err != nil {
		t.Errorf("Error not expected. Recieved %s", err)
	}

	if len(internalTestPlanReq.GetTestCases()) > 0 {
		t.Errorf("No test cases expected, found : %d", len(internalTestPlanReq.GetTestCases()))
	}
}
