// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"encoding/json"
	"log"
	"os"
	"testing"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	storage_path "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

func GenerateSchedulingUnitOptions(boards []string) []*api.SchedulingUnitOptions {
	SchedulingUnitSkyrim := []*api.SchedulingUnit{
		{
			PrimaryTarget: &api.Target{
				SwarmingDef: &api.SwarmingDefinition{
					DutInfo: &testapi.Dut{
						DutType: &testapi.Dut_Chromeos{
							Chromeos: &testapi.Dut_ChromeOS{
								DutModel: &testapi.DutModel{
									BuildTarget: "skyrim",
									ModelName:   "bluebird",
								},
							},
						},
					},
					SwarmingLabels: []string{
						"label-peripheral_wifi_state:WORKING",
						"label-wificell:True",
					},
				},
			},
			DynamicUpdateLookupTable: map[string]string{
				"board":       "skyrim",
				"installPath": "gs://chromeos-image-archive/skyrim-release/R128-15964.4.0",
			},
		},
	}

	SchedulingUnitBetty := []*api.SchedulingUnit{
		{
			PrimaryTarget: &api.Target{
				SwarmingDef: &api.SwarmingDefinition{
					DutInfo: &testapi.Dut{
						DutType: &testapi.Dut_Chromeos{
							Chromeos: &testapi.Dut_ChromeOS{
								DutModel: &testapi.DutModel{
									BuildTarget: "betty",
								},
							},
						},
					},
					SwarmingLabels: []string{
						"label-peripheral_wifi_state:WORKING",
						"label-wificell:True",
					},
				},
			},
			DynamicUpdateLookupTable: map[string]string{
				"board":       "betty",
				"installPath": "gs://chromeos-image-archive/betty-release/R128-15964.4.0",
			},
		},
	}
	SchedulingUnitDedede := []*api.SchedulingUnit{
		{
			PrimaryTarget: &api.Target{
				SwarmingDef: &api.SwarmingDefinition{
					DutInfo: &testapi.Dut{
						DutType: &testapi.Dut_Chromeos{
							Chromeos: &testapi.Dut_ChromeOS{
								DutModel: &testapi.DutModel{
									BuildTarget: "dedede",
								},
							},
						},
					},
					SwarmingLabels: []string{
						"label-peripheral_wifi_state:WORKING",
						"label-wificell:True",
					},
				},
			},
			DynamicUpdateLookupTable: map[string]string{
				"board":       "betty",
				"installPath": "gs://chromeos-image-archive/dedede-release/R128-15964.4.0",
			},
		},
	}

	var SchedulingUnitOptions []*api.SchedulingUnitOptions
	for _, board := range boards {
		switch board {
		case "skyrim":
			SchedulingUnitOptions = append(SchedulingUnitOptions, &api.SchedulingUnitOptions{SchedulingUnits: SchedulingUnitSkyrim})
		case "betty":
			SchedulingUnitOptions = append(SchedulingUnitOptions, &api.SchedulingUnitOptions{SchedulingUnits: SchedulingUnitBetty})
		case "dedede":
			SchedulingUnitOptions = append(SchedulingUnitOptions, &api.SchedulingUnitOptions{SchedulingUnits: SchedulingUnitDedede})
		}

	}

	return SchedulingUnitOptions
}

func GenerateCTPTestCase(name string, vmCompatible bool, SchedulingUnitOptions []*api.SchedulingUnitOptions) *api.CTPTestCase {

	ctpTestCase := &api.CTPTestCase{
		Name: name,
		Metadata: &api.TestCaseMetadata{
			TestCase: &api.TestCase{
				Id: &api.TestCase_Id{
					Value: "test1",
				},
				Tags: []*api.TestCase_Tag{
					{
						Value: "value",
					},
				},
			},
			TestCaseInfo: &api.TestCaseInfo{
				HwAgnostic: &api.HwAgnostic{
					Value: vmCompatible,
				},
			},
		},
		SchedulingUnitOptions: SchedulingUnitOptions,
	}
	return ctpTestCase
}

func GenerateSuiteInfo() *api.SuiteInfo {
	metadata, err := anypb.New(&api.CrOSProvisionMetadata{})
	if err != nil {
		log.Fatalf("Error creating anypb.Any: %v", err)
	}
	SuiteInfo := &api.SuiteInfo{
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
	}
	return SuiteInfo
}

func JSONSerialize(any any) string {
	json, _ := json.Marshal(any)
	return string(json)
}

func Test_GetImageVersionFromInstallPath(t *testing.T) {

	releaseImage := "gs://chromeos-image-archive/skyrim-release/R128-15964.4.0"
	expectedImageVersion := "R128-15964.4.0"
	nonReleaseImage := "betty-arc-t-cq/R130-16020.0.0-104081-8737801324706956561"

	version, err := getImageVersionFromInstallPath(releaseImage)
	if err != nil {
		t.Errorf("Error expected to be nil, got %s", err)
	}
	if version != expectedImageVersion {
		t.Errorf("Expected %s, got %s", expectedImageVersion, version)
	}
	version, err = getImageVersionFromInstallPath(nonReleaseImage)
	if err == nil {
		t.Errorf("Error not expected to be nil, got %s", err)
	}
	if version != "" {
		t.Errorf("Expected %s, got %s", "", version)
	}

}

func Test_GetTargetedImageVersion(t *testing.T) {
	schedulingUnitOptions := GenerateSchedulingUnitOptions([]string{"skyrim", "betty"})
	testCase := GenerateCTPTestCase("test1", true, schedulingUnitOptions)
	internalTestPlan := &api.InternalTestplan{
		TestCases: []*api.CTPTestCase{testCase},
	}
	expectedTargetedVersion := "R128-15964.4.0"
	targetedImageVersion := getTargetedImageVersion(internalTestPlan, log.New(os.Stdout, "DEBUG", log.LstdFlags))

	if targetedImageVersion != expectedTargetedVersion {
		t.Errorf("Expected R128-15964.4.0, got %s", targetedImageVersion)
	}
}

func Test_IsTestVMCompatible(t *testing.T) {
	compatibleTest := &api.CTPTestCase{
		Metadata: &api.TestCaseMetadata{
			TestCase: &api.TestCase{
				Id: &api.TestCase_Id{
					Value: "test1",
				},
				Tags: []*api.TestCase_Tag{
					{
						Value: "hw_agnostic",
					},
				},
			},
			TestCaseInfo: &api.TestCaseInfo{
				HwAgnostic: &api.HwAgnostic{
					Value: true,
				},
			},
		},
	}
	nonCompatibleTest := &api.CTPTestCase{
		Metadata: &api.TestCaseMetadata{
			TestCase: &api.TestCase{
				Id: &api.TestCase_Id{
					Value: "test1",
				},
				Tags: []*api.TestCase_Tag{
					{
						Value: "randomTag",
					},
				},
			},
		},
	}
	if isTestVMCompatible(compatibleTest) != true {
		t.Errorf("Expected true, got %t", isTestVMCompatible(compatibleTest))
	}

	if isTestVMCompatible(nonCompatibleTest) == true {
		t.Errorf("Expected true, got %t", isTestVMCompatible(compatibleTest))
	}
}

func Test_UpdateSchedulingUnitOptions_NonVMCompatibleTest(t *testing.T) {
	boards := []string{"skyrim", "betty"}
	expectedBoards := make(map[string]bool)
	expectedBoards["skyrim"] = true
	expectedBoards["betty"] = true
	schedulingUnitOptions := GenerateSchedulingUnitOptions(boards)
	testCase := GenerateCTPTestCase("test1", false, schedulingUnitOptions)
	internalTestPlan := &api.InternalTestplan{
		TestCases: []*api.CTPTestCase{testCase},
	}

	err := updateSchedulingUnitOptions(internalTestPlan, "reven-vmtest", "R128-15964.4.0", log.New(os.Stdout, "DEBUG", log.LstdFlags))
	if err != nil {
		t.Errorf("Expected nil, got %s", err)
	}
	if len(internalTestPlan.TestCases[0].SchedulingUnitOptions) != 2 {
		t.Errorf("Expected 2, got %d", len(internalTestPlan.TestCases[0].SchedulingUnitOptions))
	}

	for _, tc := range internalTestPlan.GetTestCases() {
		for _, schedulingUnitOptions := range tc.GetSchedulingUnitOptions() {
			for _, schedulingUnit := range schedulingUnitOptions.GetSchedulingUnits() {
				board := schedulingUnit.GetDynamicUpdateLookupTable()["board"]
				if !expectedBoards[board] {
					t.Errorf("Board %s not expected to be in Scheduling unit", board)
				}
			}
		}
	}

}

func Test_UpdateSchedulingUnitOptions_VMCompatibleTest_WithVMSchedulingUnit(t *testing.T) {
	schedulingUnitOptions := GenerateSchedulingUnitOptions([]string{"skyrim", "betty"})
	testCase := GenerateCTPTestCase("test1", true, schedulingUnitOptions)
	internalTestPlan := &api.InternalTestplan{
		TestCases: []*api.CTPTestCase{testCase},
	}

	err := updateSchedulingUnitOptions(internalTestPlan, "reven-vmtest", "R128-15964.4.0", log.New(os.Stdout, "DEBUG", log.LstdFlags))
	if err != nil {
		t.Errorf("Expected nil, got %s", err)
	}

	if len(internalTestPlan.TestCases[0].GetSchedulingUnitOptions()) != 1 {
		t.Errorf("Expected 2, got %d", len(internalTestPlan.TestCases[0].SchedulingUnitOptions))
	}
	expectedBoard := "betty"
	updatedBoard := internalTestPlan.TestCases[0].GetSchedulingUnitOptions()[0].GetSchedulingUnits()[0].GetDynamicUpdateLookupTable()["board"]
	if expectedBoard != updatedBoard {
		t.Errorf("Expected %s, got %s", expectedBoard, updatedBoard)
	}
	if len(internalTestPlan.TestCases[0].GetSchedulingUnitOptions()[0].GetSchedulingUnits()) != 1 {
		t.Errorf("Expected 1, got %d", len(internalTestPlan.TestCases[0].SchedulingUnitOptions))
	}
}

func Test_UpdateSchedulingUnitOptions_VMCompatibleTest_WithNoVMSchedulingUnit(t *testing.T) {
	schedulingUnitOptions := GenerateSchedulingUnitOptions([]string{"skyrim", "dedede"})
	testCase := GenerateCTPTestCase("test1", true, schedulingUnitOptions)
	internalTestPlan := &api.InternalTestplan{
		TestCases: []*api.CTPTestCase{testCase},
	}

	err := updateSchedulingUnitOptions(internalTestPlan, "reven-vmtest", "R128-15964.4.0", log.New(os.Stdout, "DEBUG", log.LstdFlags))
	if err != nil {
		t.Errorf("Expected nil, got %s", err)
	}

	if len(internalTestPlan.TestCases[0].GetSchedulingUnitOptions()) != 1 {
		t.Errorf("Expected 2, got %d", len(internalTestPlan.TestCases[0].SchedulingUnitOptions))
	}
	expectedBoard := "reven-vmtest"
	updatedBoard := internalTestPlan.TestCases[0].GetSchedulingUnitOptions()[0].GetSchedulingUnits()[0].GetDynamicUpdateLookupTable()["board"]
	if expectedBoard != updatedBoard {
		t.Errorf("Expected %s, got %s", expectedBoard, updatedBoard)
	}
	if len(internalTestPlan.TestCases[0].GetSchedulingUnitOptions()[0].GetSchedulingUnits()) != 1 {
		t.Errorf("Expected 1, got %d", len(internalTestPlan.TestCases[0].SchedulingUnitOptions))
	}
}
