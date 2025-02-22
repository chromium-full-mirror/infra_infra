// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package finders contains the implementations of the abstract finder interface.
package finders

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	"go.chromium.org/chromiumos/config/go/test/api"
)

// func TestTFSuiteFromTestplan(t *testing.T) {
// 	ctx := context.Background()
// 	log := log.New(io.Discard, "", 0)

// 	req := &api.InternalTestplan{
// 		SuiteInfo: common.GenerateSuiteInfoTF(),
// 	}
// 	fmt.Println(req)

// 	fmt.Println("!! ABOVE")
// 	internalTFFinder := NewTradefedFinder(ctx, req, log)
// 	internalTFFinder.FindTestsAB()
// 	fmt.Println("!! BELOW")
// 	fmt.Println(req)
// 	t.Fatalf("foo")
// }

// func TestFoo(t *testing.T) {
// 	te, err := getTFSourceData(context.Background(), "cros-xts-metadata")
// 	if err != nil {
// 		t.Fatalf("unable to get source data: %s", err)
// 	}
// 	metadata := translateTFSrcToMetadata(te)
// 	internalTestPlan := &api.InternalTestplan{
// 		SuiteInfo: common.GenerateSuiteInfoTF(),
// 	}
// 	foo, err := TPtoSuite(internalTestPlan)
// 	if err != nil {
// 		t.Fatalf("unable to translate data: %s", err)
// 	}
// 	res, err := finder.MatchedTestsForSuites(metadata, foo)
// 	if err != nil {
// 		t.Fatalf("err during test: %s", err)
// 	}
// 	if len(res) < 1 {
// 		t.Fatalf("did not find any tests")
// 	}
// }

func TestCreateTestCaseFromTFTestInfo(t *testing.T) {
	testCases := []struct {
		name            string
		testInfo        Module
		suiteName       string
		branch          string
		targets         []string
		buildID         string
		targetBuilds    []TargetBuild
		version         string
		expected        *api.TestCaseMetadata
		expectedFailure bool
	}{
		{
			name: "basic",
			testInfo: Module{
				Name:         "testModule1",
				Parameters:   []string{"param1", "param2"},
				Abis:         []string{"abi1", "abi2"},
				BugComponent: []int32{1234, 5678},
			},
			suiteName:    "suite1",
			branch:       "main",
			targets:      []string{"target1", "target2"},
			buildID:      "build123",
			targetBuilds: []TargetBuild{},
			version:      "1.0",
			expected: &api.TestCaseMetadata{
				TestCase: &api.TestCase{
					Name: "suite1.testModule1",
					Tags: []*api.TestCase_Tag{
						{Value: "param:param1"},
						{Value: "param:param2"},
						{Value: "suite:suite1"},
						{Value: "branch:main"},
						{Value: "build_id:build123"},
						{Value: "target:target1"},
						{Value: "target:target2"},
						{Value: "build_id:abi1"},
						{Value: "build_id:abi2"},
					},
					Id: &api.TestCase_Id{
						Value: "tradefed.suite1.testModule1",
					},
				},
				TestCaseInfo: &api.TestCaseInfo{
					Owners:       []*api.Contact{},
					BugComponent: &api.BugComponent{Value: "1234"},
				},
				TestCaseExec: &api.TestCaseExec{
					TestHarness: &api.TestHarness{
						TestHarnessType: &api.TestHarness_Tradefed_{
							Tradefed: &api.TestHarness_Tradefed{},
						},
					},
				},
			},
		},
		{
			name: "No Parameters",
			testInfo: Module{
				Name:         "testModule2",
				Parameters:   []string{},
				Abis:         []string{"abi3"},
				BugComponent: []int32{},
			},
			suiteName:    "suite2",
			branch:       "",
			targets:      []string{},
			buildID:      "",
			targetBuilds: []TargetBuild{},
			version:      "1.0",
			expected: &api.TestCaseMetadata{
				TestCase: &api.TestCase{
					Name: "suite2.testModule2",
					Tags: []*api.TestCase_Tag{
						{Value: "suite:suite2"},
						{Value: "build_id:abi3"},
					},
					Id: &api.TestCase_Id{
						Value: "tradefed.suite2.testModule2",
					},
				},
				TestCaseInfo: &api.TestCaseInfo{
					Owners:       []*api.Contact{},
					BugComponent: nil,
				},
				TestCaseExec: &api.TestCaseExec{
					TestHarness: &api.TestHarness{
						TestHarnessType: &api.TestHarness_Tradefed_{
							Tradefed: &api.TestHarness_Tradefed{},
						},
					},
				},
			},
		},
		{
			name: "Target Builds",
			testInfo: Module{
				Name:         "testModule3",
				Parameters:   []string{"param3"},
				Abis:         []string{"abi3"},
				BugComponent: []int32{},
			},
			suiteName: "suite3",
			branch:    "newBranch",
			targets:   []string{"target3"},
			buildID:   "build456",
			targetBuilds: []TargetBuild{
				{
					Target:  "targetX",
					BuildId: "buildX",
					Abi:     "abiX",
				},
				{
					Target:  "targetY",
					BuildId: "buildY",
					Abi:     "abiY",
				},
			},
			version: "1.5",
			expected: &api.TestCaseMetadata{
				TestCase: &api.TestCase{
					Name: "suite3.testModule3",
					Tags: []*api.TestCase_Tag{
						{Value: "param:param3"},
						{Value: "suite:suite3"},
						{Value: "branch:newBranch"},
						{Value: "build_id:build456"},
						{Value: "target:target3"},
						{Value: "target_build=target:targetX,build_id:buildX,abi:abiX"},
						{Value: "target_build=target:targetY,build_id:buildY,abi:abiY"},
					},
					Id: &api.TestCase_Id{
						Value: "tradefed.suite3.testModule3",
					},
				},
				TestCaseInfo: &api.TestCaseInfo{
					Owners:       []*api.Contact{},
					BugComponent: nil,
				},
				TestCaseExec: &api.TestCaseExec{
					TestHarness: &api.TestHarness{
						TestHarnessType: &api.TestHarness_Tradefed_{
							Tradefed: &api.TestHarness_Tradefed{},
						},
					},
				},
			},
		},
		{
			name: "Multiple Bug Components",
			testInfo: Module{
				Name:         "testModule4",
				Parameters:   []string{"param4"},
				Abis:         []string{"abi4"},
				BugComponent: []int32{1111, 2222, 3333},
			},
			suiteName:    "suite4",
			branch:       "testBranch",
			targets:      []string{},
			buildID:      "build789",
			targetBuilds: []TargetBuild{},
			version:      "1.0",
			expected: &api.TestCaseMetadata{
				TestCase: &api.TestCase{
					Name: "suite4.testModule4",
					Tags: []*api.TestCase_Tag{
						{Value: "param:param4"},
						{Value: "suite:suite4"},
						{Value: "branch:testBranch"},
						{Value: "build_id:build789"},
						{Value: "build_id:abi4"},
					},
					Id: &api.TestCase_Id{
						Value: "tradefed.suite4.testModule4",
					},
				},
				TestCaseInfo: &api.TestCaseInfo{
					Owners:       []*api.Contact{},
					BugComponent: &api.BugComponent{Value: "1111"},
				},
				TestCaseExec: &api.TestCaseExec{
					TestHarness: &api.TestHarness{
						TestHarnessType: &api.TestHarness_Tradefed_{
							Tradefed: &api.TestHarness_Tradefed{},
						},
					},
				},
			},
		},
		{
			name: "Empty target builds, version 1.5",
			testInfo: Module{
				Name:         "testModule5",
				Parameters:   []string{"param5"},
				Abis:         []string{"abi5"},
				BugComponent: []int32{},
			},
			suiteName:    "suite5",
			branch:       "newBranch",
			targets:      []string{"target5"},
			buildID:      "build987",
			targetBuilds: []TargetBuild{},
			version:      "1.5",
			expected: &api.TestCaseMetadata{
				TestCase: &api.TestCase{
					Name: "suite5.testModule5",
					Tags: []*api.TestCase_Tag{
						{Value: "param:param5"},
						{Value: "suite:suite5"},
						{Value: "branch:newBranch"},
						{Value: "build_id:build987"},
						{Value: "target:target5"},
						{Value: "build_id:abi5"},
					},
					Id: &api.TestCase_Id{
						Value: "tradefed.suite5.testModule5",
					},
				},
				TestCaseInfo: &api.TestCaseInfo{
					Owners:       []*api.Contact{},
					BugComponent: nil,
				},
				TestCaseExec: &api.TestCaseExec{
					TestHarness: &api.TestHarness{
						TestHarnessType: &api.TestHarness_Tradefed_{
							Tradefed: &api.TestHarness_Tradefed{},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := createTestCaseFromTFTestInfo(tc.testInfo, tc.suiteName, tc.branch, tc.targets, tc.buildID, tc.targetBuilds, tc.version)
			if diff := cmp.Diff(tc.expected, actual, protocmp.Transform()); diff != "" {
				t.Errorf("createTestCaseFromTFTestInfo mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFormatName(t *testing.T) {
	testCases := []struct {
		name      string
		modName   string
		suiteName string
		expected  string
	}{
		{
			name:      "basic",
			modName:   "testMod",
			suiteName: "suite",
			expected:  "suite.testMod",
		},
		{
			name:      "empty suite",
			modName:   "testMod2",
			suiteName: "",
			expected:  ".testMod2",
		},
		{
			name:      "empty mod",
			modName:   "",
			suiteName: "suite2",
			expected:  "suite2.",
		},
		{
			name:      "empty both",
			modName:   "",
			suiteName: "",
			expected:  ".",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := formatName(tc.modName, tc.suiteName)
			if actual != tc.expected {
				t.Errorf("formatName(%q, %q) = %q; want %q", tc.modName, tc.suiteName, actual, tc.expected)
			}
		})
	}
}

func TestBugComponent(t *testing.T) {
	testCases := []struct {
		name     string
		input    []int32
		expected *api.BugComponent
	}{
		{
			name:     "basic",
			input:    []int32{1234},
			expected: &api.BugComponent{Value: "1234"},
		},
		{
			name:     "multiple",
			input:    []int32{1111, 2222},
			expected: &api.BugComponent{Value: "1111"},
		},
		{
			name:     "empty",
			input:    []int32{},
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := bugComponent(tc.input)
			if diff := cmp.Diff(tc.expected, actual, protocmp.Transform()); diff != "" {
				t.Errorf("bugComponent mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
