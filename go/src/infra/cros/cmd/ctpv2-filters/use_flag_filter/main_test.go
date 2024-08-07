// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"encoding/json"
	"io"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	storage_path "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

// GenerateInternalTestPlan builds internalTestPlan with two test cases for which it takes the build deps as input. The first test has
// one android and two chromos type(skyrim & octopus) scheduling units, the second test has two chromeos type(dedede & skyrim) scheduling units,
func GenerateInternalTestPlan(builDepsTest1 []*api.TestCase_BuildDeps, buildDepsTest2 []*api.TestCase_BuildDeps) *api.InternalTestplan {
	skyrim := &testapi.Dut_ChromeOS{DutModel: &testapi.DutModel{
		BuildTarget: "skyrim",
		ModelName:   "craaskov",
	}}

	octopus := &testapi.Dut_ChromeOS{DutModel: &testapi.DutModel{
		BuildTarget: "octopus",
		ModelName:   "bluebird",
	}}

	dedede := &testapi.Dut_ChromeOS{DutModel: &testapi.DutModel{
		BuildTarget: "dedede",
		ModelName:   "crota",
	}}

	android := &testapi.Dut_Android{DutModel: &testapi.DutModel{
		BuildTarget: "android",
		ModelName:   "android",
	}}
	metadata, err := anypb.New(&api.CrOSProvisionMetadata{})
	if err != nil {
		log.Fatalf("Error creating anypb.Any: %v", err)
	}

	internalTestPlan := &api.InternalTestplan{
		TestCases: []*api.CTPTestCase{
			{
				Name: "test1",
				Metadata: &api.TestCaseMetadata{
					TestCase: &api.TestCase{
						Id: &api.TestCase_Id{
							Value: "test1",
						},
						BuildDependencies: builDepsTest1,
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
									"installPath": "gs://chromeos-image-archive/skyrim-release/R128-15964.4.0",
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
					{
						SchedulingUnits: []*api.SchedulingUnit{
							{
								PrimaryTarget: &api.Target{
									SwarmingDef: &api.SwarmingDefinition{
										DutInfo: &testapi.Dut{
											DutType: &testapi.Dut_Android_{Android: android},
										},
										SwarmingLabels: []string{
											"label-peripheral_wifi_state:WORKING",
											"label-wificell:True",
										},
									},
									SwReq: &api.LegacySW{},
								},
								DynamicUpdateLookupTable: map[string]string{
									"board":       "android",
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
						BuildDependencies: buildDepsTest2,
					},
				},
				SchedulingUnitOptions: []*api.SchedulingUnitOptions{
					{
						SchedulingUnits: []*api.SchedulingUnit{
							{
								PrimaryTarget: &api.Target{
									SwarmingDef: &api.SwarmingDefinition{
										DutInfo: &testapi.Dut{
											DutType: &testapi.Dut_Chromeos{Chromeos: dedede},
										},
										SwarmingLabels: []string{
											"label-peripheral_wifi_state:WORKING",
											"label-wificell:True",
										},
									},
									SwReq: &api.LegacySW{},
								},
								DynamicUpdateLookupTable: map[string]string{
									"board":       "dedede",
									"installPath": "gs://chromeos-image-archive/skyrim-release/R128-15964.4.0",
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
				AnalyticsName:   "wifi_endtoend_perbuild__wificell_perbuild__wifi_endtoend__tauto__new_build",
				TestArgs:        "",
			},
		},
	}
	return internalTestPlan
}

func GenerateBoardUseFlagDict(boardsToAdd []string) map[string]map[string]bool {

	useFlagDict := make(map[string]map[string]bool)

	useFlagsForSkyrim := []string{
		"common", "skyrim",
	}

	useFlagsForOctopus := []string{
		"common", "octopus",
	}

	useFlagsForDedede := []string{
		"common", "dedede",
	}

	skyrimSet := make(map[string]bool)
	for _, str := range useFlagsForSkyrim {
		skyrimSet[str] = true
	}

	octopusSet := make(map[string]bool)
	for _, str := range useFlagsForOctopus {
		octopusSet[str] = true
	}

	dededeSet := make(map[string]bool)
	for _, str := range useFlagsForDedede {
		dededeSet[str] = true
	}

	for _, board := range boardsToAdd {
		if board == "skyrim" {
			useFlagDict["skyrim"] = skyrimSet
		} else if board == "octopus" {
			useFlagDict["octopus"] = octopusSet
		} else if board == "dedede" {
			useFlagDict["dedede"] = dededeSet
		}
	}

	return useFlagDict

}

func JSONSerialize(any interface{}) string {
	json, _ := json.Marshal(any)
	return string(json)
}

// All test build deps expression qualify. None removed.
func TestUpdateTestCases_buildDeps_in_use_flags_dict(t *testing.T) {
	buildDepsTest1 := []*api.TestCase_BuildDeps{
		{
			Value: "common,!notpresent",
		},
	}
	internalTestPlan := GenerateInternalTestPlan(buildDepsTest1, buildDepsTest1)
	filteredInternalTestPlan := proto.Clone(internalTestPlan).(*api.InternalTestplan)
	emptyLogger := log.New(io.Discard, "", 0)
	err := updateTestCases(filteredInternalTestPlan, GenerateBoardUseFlagDict([]string{"skyrim", "octopus", "dedede"}), emptyLogger)
	if err != nil {
		t.Errorf("updateTestCases failed: %v", err)
		return
	}

	assert.Equal(t, JSONSerialize(filteredInternalTestPlan.GetTestCases()), JSONSerialize(internalTestPlan.GetTestCases()))
}

// Few test build deps expression qualify. Partially removed.
func TestUpdateTestCases_buildDeps_in_use_flags_dict_partial_match(t *testing.T) {
	// this test should qualify for "skyrim" chromeos build target and DUTs other than chromeos type
	buildDepsTest1 := []*api.TestCase_BuildDeps{
		{
			Value: "skyrim,!notpresent",
		},
		{
			Value: "invalid-use-flag",
		},
	}
	// this test should qualify for "dedede" chromeos build target and DUTs other than chromeos type
	buildDepsTest2 := []*api.TestCase_BuildDeps{
		{
			Value: "dedede",
		},
	}
	internalTestPlan := GenerateInternalTestPlan(buildDepsTest1, buildDepsTest2)
	filteredInternalTestPlan := proto.Clone(internalTestPlan).(*api.InternalTestplan)
	emptyLogger := log.New(io.Discard, "", 0)
	err := updateTestCases(filteredInternalTestPlan, GenerateBoardUseFlagDict([]string{"octopus", "skyrim", "dedede"}), emptyLogger)
	if err != nil {
		t.Errorf("updateTestCases failed: %v", err)
		return
	}

	for _, testCase := range filteredInternalTestPlan.GetTestCases() {
		if testCase.Name == "test1" {
			assert.Equal(t, len(testCase.GetSchedulingUnitOptions()), 2)
			for _, schedulingUnitOption := range testCase.GetSchedulingUnitOptions() {
				assert.Equal(t, len(schedulingUnitOption.GetSchedulingUnits()), 1)
			}

		}
		if testCase.Name == "test2" {
			assert.Equal(t, len(testCase.GetSchedulingUnitOptions()), 1)
			for _, schedulingUnitOption := range testCase.GetSchedulingUnitOptions() {
				assert.Equal(t, len(schedulingUnitOption.GetSchedulingUnits()), 1)
			}
		}
	}

}

// No test build deps expression qualifies. All ChromeOS Dut type schduling units removed.
func TestUpdateTestCases_buildDeps_not_in_use_flags_dict(t *testing.T) {
	buildDepsTest1 := []*api.TestCase_BuildDeps{
		{
			Value: "common,notpresent",
		},
		{
			Value: "!common",
		},
	}
	internalTestPlan := GenerateInternalTestPlan(buildDepsTest1, buildDepsTest1)
	filteredInternalTestPlan := proto.Clone(internalTestPlan).(*api.InternalTestplan)
	emptyLogger := log.New(io.Discard, "", 0)
	err := updateTestCases(filteredInternalTestPlan, GenerateBoardUseFlagDict([]string{"skyrim", "octopus", "dedede"}), emptyLogger)
	if err != nil {
		t.Errorf("updateTestCases failed: %v", err)
		return
	}

	for _, testCase := range filteredInternalTestPlan.GetTestCases() {
		if testCase.Name == "test1" {
			assert.Equal(t, len(testCase.GetSchedulingUnitOptions()), 1)
			for _, schedulingUnitOption := range testCase.GetSchedulingUnitOptions() {
				assert.Equal(t, len(schedulingUnitOption.GetSchedulingUnits()), 1)
			}

		}
		if testCase.Name == "test2" {
			assert.Equal(t, len(testCase.GetSchedulingUnitOptions()), 0)
			for _, schedulingUnitOption := range testCase.GetSchedulingUnitOptions() {
				assert.Equal(t, len(schedulingUnitOption.GetSchedulingUnits()), 1)
			}
		}
	}
}

// Partial test hwDef expression qualifies. No ChromeOS Dut type hwDef removed.
func TestUpdateTestCases_buildDeps_expression_qualifies(t *testing.T) {
	buildDepsTest1 := []*api.TestCase_BuildDeps{
		{
			Value: "common,notpresent",
		},
		{
			Value: "common",
		},
	}
	internalTestPlan := GenerateInternalTestPlan(buildDepsTest1, buildDepsTest1)
	filteredInternalTestPlan := proto.Clone(internalTestPlan).(*api.InternalTestplan)
	emptyLogger := log.New(io.Discard, "", 0)
	err := updateTestCases(filteredInternalTestPlan, GenerateBoardUseFlagDict([]string{"skyrim", "octopus", "dedede"}), emptyLogger)
	if err != nil {
		t.Errorf("updateTestCases failed: %v", err)
		return
	}
	assert.Equal(t, JSONSerialize(filteredInternalTestPlan.GetTestCases()), JSONSerialize(internalTestPlan.GetTestCases()))
}

// No use flag dict. All scheduling options intact.
func TestUpdateTestCases_use_flag_dict_missing_board(t *testing.T) {
	buildDepsTest1 := []*api.TestCase_BuildDeps{
		{
			Value: "uncommon",
		},
	}
	internalTestPlan := GenerateInternalTestPlan(buildDepsTest1, buildDepsTest1)
	filteredInternalTestPlan := proto.Clone(internalTestPlan).(*api.InternalTestplan)
	emptyLogger := log.New(io.Discard, "", 0)
	err := updateTestCases(filteredInternalTestPlan, GenerateBoardUseFlagDict([]string{""}), emptyLogger)
	if err != nil {
		t.Errorf("updateTestCases failed: %v", err)
		return
	}

	assert.Equal(t, JSONSerialize(filteredInternalTestPlan.GetTestCases()), JSONSerialize(internalTestPlan.GetTestCases()))
}
