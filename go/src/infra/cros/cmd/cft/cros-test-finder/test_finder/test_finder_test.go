// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package test_finder

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/protobuf/jsonpb"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cft/cros-test-finder/centralizedsuite"
)

func TestReadInput(t *testing.T) {
	expReq := &api.CrosTestFinderRequest{
		TestSuites: []*api.TestSuite{
			{
				Name: "suite1",
				Spec: &api.TestSuite_TestCaseIds{
					TestCaseIds: &api.TestCaseIdList{
						TestCaseIds: []*api.TestCase_Id{
							{
								Value: "example.Pass",
							},
							{
								Value: "example.Fail",
							},
						},
					},
				},
			},
			{
				Name: "suite2",
				Spec: &api.TestSuite_TestCaseTagCriteria_{
					TestCaseTagCriteria: &api.TestSuite_TestCaseTagCriteria{
						Tags: []string{"group:meta"},
					},
				},
			},
		},
	}

	m := jsonpb.Marshaler{}
	encodedData, err := m.MarshalToString(expReq)
	if err != nil {
		t.Fatal("Failed to marshall request")
	}
	td, err := ioutil.TempDir("", "cros-test-finder_TestReadInput_*")
	if err != nil {
		t.Fatal("Failed to create temporary dictectory: ", err)
	}

	defer os.RemoveAll(td)
	fn := filepath.Join(td, "t.json")
	if err := ioutil.WriteFile(fn, []byte(encodedData), 0644); err != nil {
		t.Fatalf("Failed to write file %v: %v", fn, err)
	}
	req, err := readInput(fn)
	if err != nil {
		t.Fatalf("Failed to read input file %v: %v", fn, err)
	}

	if !proto.Equal(req, expReq) {
		t.Errorf("Got unexpected request from readInput (-got +want):\n%v\n--\n%v\n", req, expReq)
	}
}

func TestWriteOutput(t *testing.T) {
	testInfos := []*api.TestCase{}
	for _, md := range []string{"example.Pass", "example.Fail"} {
		testInfos = append(testInfos, &api.TestCase{
			Id: &api.TestCase_Id{Value: md},
		})
	}
	expectedRspn := api.CrosTestFinderResponse{
		TestSuites: []*api.TestSuite{
			{
				Name: "suite1",
				Spec: &api.TestSuite_TestCases{
					TestCases: &api.TestCaseList{TestCases: testInfos},
				},
			},
		},
	}
	td, err := ioutil.TempDir("", "faketestrunner_TestWriteOutput_*")
	if err != nil {
		t.Fatal("Failed to create temporary dictectory: ", err)
	}
	defer os.RemoveAll(td)
	fn := filepath.Join(td, "t.json")
	if err := writeOutput(fn, &expectedRspn); err != nil {
		t.Fatalf("Failed to write file %v: %v", fn, err)
	}
	f, err := os.Open(fn)
	if err != nil {
		t.Fatalf("Failed to read response from file %v: %v", fn, err)
	}
	rspn := api.CrosTestFinderResponse{}
	if err := jsonpb.Unmarshal(f, &rspn); err != nil {
		t.Fatalf("Failed to unmarshall data from file %v: %v", fn, err)
	}

	if !proto.Equal(&rspn, &expectedRspn) {
		t.Errorf("Got unexpected reports(-got +want):\n%v\n--\n%v\n", &rspn, &expectedRspn)
	}
}

func TestCombineSuiteNames(t *testing.T) {
	suites := []*api.TestSuite{
		{
			Name: "suite1",
			Spec: &api.TestSuite_TestCaseIds{
				TestCaseIds: &api.TestCaseIdList{
					TestCaseIds: []*api.TestCase_Id{
						{
							Value: "example.Pass",
						},
						{
							Value: "example.Fail",
						},
					},
				},
			},
		},
		{
			Name: "suite2",
			Spec: &api.TestSuite_TestCaseTagCriteria_{
				TestCaseTagCriteria: &api.TestSuite_TestCaseTagCriteria{
					Tags: []string{"group:meta"},
				},
			},
		},
	}
	name := combineTestSuiteNames(suites)
	if name != "suite1,suite2" {
		t.Errorf(`Got %s from combineTestSuiteNames; wanted "suite1,suite2"`, name)
	}
}

func TestMetadataToTestSuite(t *testing.T) {
	mdList := []*api.TestCaseMetadata{
		{
			TestCase: &api.TestCase{
				Id: &api.TestCase_Id{
					Value: "tast/test001",
				},
				Name: "tastTest",
				Tags: []*api.TestCase_Tag{
					{Value: "attr1"},
					{Value: "attr2"},
				},
				Dependencies: []*api.TestCase_Dependency{
					{Value: "dep1"},
					{Value: "dep2"},
				},
			},
			TestCaseExec: &api.TestCaseExec{
				TestHarness: &api.TestHarness{
					TestHarnessType: &api.TestHarness_Tast_{
						Tast: &api.TestHarness_Tast{},
					},
				},
			},
			TestCaseInfo: &api.TestCaseInfo{
				Owners: []*api.Contact{
					{Email: "someone1@chromium.org"},
				},
			},
		},
		{
			TestCase: &api.TestCase{
				Id: &api.TestCase_Id{
					Value: "tauto/test002",
				},
				Name: "tautoTest",
				Tags: []*api.TestCase_Tag{
					{Value: "attr1"},
					{Value: "attr2"},
				},
			},
			TestCaseExec: &api.TestCaseExec{
				TestHarness: &api.TestHarness{
					TestHarnessType: &api.TestHarness_Tauto_{
						Tauto: &api.TestHarness_Tauto{},
					},
				},
			},
			TestCaseInfo: &api.TestCaseInfo{
				Owners: []*api.Contact{
					{Email: "someone2@chromium.org"},
				},
			},
		},
		{
			TestCase: &api.TestCase{
				Id: &api.TestCase_Id{
					Value: "tauto/test003",
				},
				Name: "tautoTest",
				Tags: []*api.TestCase_Tag{
					{Value: "attr3"},
				},
			},
			TestCaseExec: &api.TestCaseExec{
				TestHarness: &api.TestHarness{
					TestHarnessType: &api.TestHarness_Tauto_{
						Tauto: &api.TestHarness_Tauto{},
					},
				},
			},
			TestCaseInfo: &api.TestCaseInfo{
				Owners: []*api.Contact{
					{Email: "someone3@chromium.org"},
				},
			},
		},
	}
	testInfos := []*api.TestCase{}
	testNames := []string{"tast/test001", "tauto/test002", "tauto/test003"}
	testDeps := [][]string{{"dep1", "dep2"}, {}, {}}
	for i := range testNames {
		deps := []*api.TestCase_Dependency{}
		for _, dep := range testDeps[i] {

			deps = append(deps, &api.TestCase_Dependency{Value: dep})
		}
		if len(deps) == 0 {
			deps = nil
		}
		testInfos = append(testInfos, &api.TestCase{
			Id:           &api.TestCase_Id{Value: testNames[i]},
			Dependencies: deps,
		})

	}
	expected := api.TestSuite{
		Name: "test_suite",
		Spec: &api.TestSuite_TestCases{
			TestCases: &api.TestCaseList{TestCases: testInfos},
		},
	}
	suites := metadataToTestSuite("test_suite", mdList, false)

	if !proto.Equal(suites, &expected) {
		t.Errorf("Got unexpected test suite from metadataToTestSuite (-got +want):\n%v\n--\n%v\n", suites, &expected)
	}
}

func TestMetadataToTestSuiteWithMetaData(t *testing.T) {
	mdList := []*api.TestCaseMetadata{
		{
			TestCase: &api.TestCase{
				Id: &api.TestCase_Id{
					Value: "tast/test001",
				},
				Name: "tastTest",
				Tags: []*api.TestCase_Tag{
					{Value: "attr1"},
					{Value: "attr2"},
				},
				Dependencies: []*api.TestCase_Dependency{
					{Value: "dep1"},
					{Value: "dep2"},
				},
			},
			TestCaseExec: &api.TestCaseExec{
				TestHarness: &api.TestHarness{
					TestHarnessType: &api.TestHarness_Tast_{
						Tast: &api.TestHarness_Tast{},
					},
				},
			},
			TestCaseInfo: &api.TestCaseInfo{
				Owners: []*api.Contact{
					{Email: "someone1@chromium.org"},
				},
			},
		},
		{
			TestCase: &api.TestCase{
				Id: &api.TestCase_Id{
					Value: "tauto/test002",
				},
				Name: "tautoTest",
				Tags: []*api.TestCase_Tag{
					{Value: "attr1"},
					{Value: "attr2"},
				},
			},
			TestCaseExec: &api.TestCaseExec{
				TestHarness: &api.TestHarness{
					TestHarnessType: &api.TestHarness_Tauto_{
						Tauto: &api.TestHarness_Tauto{},
					},
				},
			},
			TestCaseInfo: &api.TestCaseInfo{
				Owners: []*api.Contact{
					{Email: "someone2@chromium.org"},
				},
			},
		},
		{
			TestCase: &api.TestCase{
				Id: &api.TestCase_Id{
					Value: "tauto/test003",
				},
				Name: "tautoTest",
				Tags: []*api.TestCase_Tag{
					{Value: "attr3"},
				},
			},
			TestCaseExec: &api.TestCaseExec{
				TestHarness: &api.TestHarness{
					TestHarnessType: &api.TestHarness_Tauto_{
						Tauto: &api.TestHarness_Tauto{},
					},
				},
			},
			TestCaseInfo: &api.TestCaseInfo{
				Owners: []*api.Contact{
					{Email: "someone3@chromium.org"},
				},
			},
		},
	}
	expected := api.TestSuite{
		Name: "test_suite",
		Spec: &api.TestSuite_TestCasesMetadata{
			TestCasesMetadata: &api.TestCaseMetadataList{Values: mdList},
		},
	}
	suites := metadataToTestSuite("test_suite", mdList, true)

	if !proto.Equal(suites, &expected) {
		t.Errorf("Got unexpected test suite from metadataToTestSuite (-got +want):\n%v\n--\n%v\n", suites, &expected)
	}
}

func convertToMetadataMap(metadataList []*api.TestCaseMetadata) map[string]*api.TestCaseMetadata {
	metadataMap := make(map[string]*api.TestCaseMetadata, len(metadataList))
	for _, metadata := range metadataList {
		metadataMap[metadata.GetTestCase().GetId().GetValue()] = metadata
	}
	return metadataMap
}

func metadataMapsAreEqual(mapA, mapB map[string]*api.TestCaseMetadata) bool {
	if len(mapA) != len(mapB) {
		return false
	}
	for testID, metadataA := range mapA {
		metadataB, ok := mapB[testID]
		if !ok {
			return false
		}
		if !proto.Equal(metadataA, metadataB) {
			return false
		}
	}
	return true
}

func TestPartialName(t *testing.T) {
	metadataList := []*api.TestCaseMetadata{
		{
			TestCase: &api.TestCase{
				Id:   &api.TestCase_Id{Value: "tradefed.cts.demo"},
				Name: "test001",
			},
		},
	}

	suites := []*api.TestSuite{
		{
			Name: "suite1",
			Spec: &api.TestSuite_TestCaseTagCriteria_{
				TestCaseTagCriteria: &api.TestSuite_TestCaseTagCriteria{
					TestNames: []string{"tradefed.cts.demo foo#bar"},
				},
			},
		},
	}

	expReq := &api.CrosTestFinderRequest{
		TestSuites: suites}
	_, b, err := getSelectedTestMetadata(nil, metadataList, expReq)
	if err != nil {
		t.Errorf("failed on err: %v", err)
	}

	g := metadataToTestSuite("foo", b, false)
	if g.GetTestCases().GetTestCases()[0].GetId().GetValue() != "tradefed.cts.demo foo#bar" {
		t.Errorf("incorrect name found")
	}
}
func TestGetSelectedTestMetadata(t *testing.T) {
	suiteSetList := &api.SuiteSetList{
		SuiteSets: []*api.SuiteSet{
			{
				Id: &api.SuiteSet_Id{Value: "power"},
				Suites: []*api.Suite_Id{
					{Value: "battery"},
					{Value: "boot"},
				},
			},
			{
				Id: &api.SuiteSet_Id{Value: "evt"},
				SuiteSets: []*api.SuiteSet_Id{
					{Value: "power"},
				},
				Suites: []*api.Suite_Id{
					{Value: "display"},
				},
			},
		},
	}
	suiteList := &api.SuiteList{
		Suites: []*api.Suite{
			{
				Id: &api.Suite_Id{Value: "battery"},
				Tests: []*api.TestCase_Id{
					{Value: "bat-life"},
				},
			},
			{
				Id: &api.Suite_Id{Value: "boot"},
				Tests: []*api.TestCase_Id{
					{Value: "boot-perf"},
					{Value: "power-load"},
				},
			},
			{
				Id: &api.Suite_Id{Value: "display"},
				Tests: []*api.TestCase_Id{
					{Value: "video-out"},
					{Value: "external-disp"},
					{Value: "power-load"},
				},
			},
		},
	}
	metadataList := []*api.TestCaseMetadata{
		{
			TestCase: &api.TestCase{
				Id:   &api.TestCase_Id{Value: "power-load"},
				Tags: []*api.TestCase_Tag{{Value: "suite:power-per-build"}},
			},
			TestCaseInfo: &api.TestCaseInfo{
				Criteria: &api.Criteria{Value: "validates device under high compute load"},
			},
		},
		{
			TestCase: &api.TestCase{
				Id:   &api.TestCase_Id{Value: "bat-life"},
				Tags: []*api.TestCase_Tag{{Value: "suite:power-per-build"}},
			},
			TestCaseInfo: &api.TestCaseInfo{
				Criteria: &api.Criteria{Value: "validates device battery life"},
			},
		},
		{
			TestCase: &api.TestCase{
				Id: &api.TestCase_Id{Value: "boot-perf"},
			},
			TestCaseInfo: &api.TestCaseInfo{
				Criteria: &api.Criteria{Value: "validate boot speed"},
			},
		},
	}
	testCases := []struct {
		name          string
		loader        centralizedsuite.MappingsLoader
		req           *api.CrosTestFinderRequest
		wantSuiteName string
		wantMetadata  []*api.TestCaseMetadata
		wantErr       error
	}{
		{
			name: "valid suites request",
			req: &api.CrosTestFinderRequest{
				TestSuites: []*api.TestSuite{
					{
						Name: "power-per-build",
						Spec: &api.TestSuite_TestCaseTagCriteria_{
							TestCaseTagCriteria: &api.TestSuite_TestCaseTagCriteria{
								Tags: []string{"suite:power-per-build"},
							},
						},
					},
				},
			},
			wantSuiteName: "power-per-build",
			wantMetadata:  metadataList[:2],
		},
		{
			name:   "valid suiteSet request",
			loader: centralizedsuite.NewInMemoryLoader(suiteSetList, suiteList),
			req: &api.CrosTestFinderRequest{
				CentralizedSuite: "power",
			},
			wantSuiteName: "power",
			wantMetadata:  metadataList,
		},
		{
			name:          "empty request",
			req:           &api.CrosTestFinderRequest{},
			wantSuiteName: "CombinedSuite",
			wantMetadata:  []*api.TestCaseMetadata{},
		},
		{
			name: "both suites and SuiteSet provided",
			req: &api.CrosTestFinderRequest{
				TestSuites: []*api.TestSuite{
					{
						Name: "power-per-build",
						Spec: &api.TestSuite_TestCaseTagCriteria_{
							TestCaseTagCriteria: &api.TestSuite_TestCaseTagCriteria{
								Tags: []string{"suite:power-per-build"},
							},
						},
					},
				},
				CentralizedSuite: "power",
			},
			wantErr: errInvalidRequest,
		},
		{
			name:   "test missing metadata",
			loader: centralizedsuite.NewInMemoryLoader(suiteSetList, suiteList),
			req: &api.CrosTestFinderRequest{
				CentralizedSuite: "evt",
			},
			wantErr: errMissingTestMetadata,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotSuiteName, gotMetadata, gotErr := getSelectedTestMetadata(testCase.loader, metadataList, testCase.req)
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("returned error does not match expected: want %v, got %v", testCase.wantErr, gotErr)
			}
			if testCase.wantErr != nil {
				return
			}
			if gotSuiteName != testCase.wantSuiteName {
				t.Fatalf("computed suite name does not match expected, want: %v, got %v", testCase.wantSuiteName, gotSuiteName)
			}
			wantMetadataMap := convertToMetadataMap(testCase.wantMetadata)
			gotMetadataMap := convertToMetadataMap(gotMetadata)
			if len(gotMetadata) != len(testCase.wantMetadata) || !metadataMapsAreEqual(wantMetadataMap, gotMetadataMap) {
				t.Fatalf("computed test list not match expected, want: %v, got %v", testCase.wantMetadata, gotMetadata)
			}
		})
	}
}

func TestTestsToMetadata(t *testing.T) {

	testCases := []struct {
		name         string
		tests        map[string]struct{}
		metadataList []*api.TestCaseMetadata
		wantMetadata []*api.TestCaseMetadata
		wantErr      error
	}{
		{
			name: "happy path",
			tests: map[string]struct{}{
				"test_1": {},
				"test_2": {},
			},
			metadataList: []*api.TestCaseMetadata{
				{TestCase: &api.TestCase{Id: &api.TestCase_Id{Value: "test_2"}}},
				{TestCase: &api.TestCase{Id: &api.TestCase_Id{Value: "test_4"}}},
				{TestCase: &api.TestCase{Id: &api.TestCase_Id{Value: "test_1"}}},
			},
			wantMetadata: []*api.TestCaseMetadata{
				{TestCase: &api.TestCase{Id: &api.TestCase_Id{Value: "test_2"}}},
				{TestCase: &api.TestCase{Id: &api.TestCase_Id{Value: "test_1"}}},
			},
		},
		{
			name: "missing metadata",
			tests: map[string]struct{}{
				"test_1": {},
				"test_3": {},
			},
			metadataList: []*api.TestCaseMetadata{
				{TestCase: &api.TestCase{Id: &api.TestCase_Id{Value: "test_2"}}},
				{TestCase: &api.TestCase{Id: &api.TestCase_Id{Value: "test_4"}}},
				{TestCase: &api.TestCase{Id: &api.TestCase_Id{Value: "test_1"}}},
			},
			wantErr: errMissingTestMetadata,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotMetadata, gotErr := testsToMetadata(testCase.tests, testCase.metadataList)
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("returned error does not match expected: want %v, got %v", testCase.wantErr, gotErr)
			}
			if testCase.wantErr != nil {
				return
			}
			wantMetadataMap := convertToMetadataMap(testCase.wantMetadata)
			gotMetadataMap := convertToMetadataMap(gotMetadata)
			if len(gotMetadata) != len(testCase.wantMetadata) || !metadataMapsAreEqual(wantMetadataMap, gotMetadataMap) {
				t.Fatalf("computed metadata list not match expected, want: %v, got %v", testCase.wantMetadata, gotMetadata)
			}
		})
	}
}
