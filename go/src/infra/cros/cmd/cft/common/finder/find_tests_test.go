// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package finder

import (
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
)

var testMetadata = []*api.TestCaseMetadata{
	{
		TestCase: &api.TestCase{
			Id: &api.TestCase_Id{
				Value: "tast.test.001",
			},
			Name: "test001",
			Tags: []*api.TestCase_Tag{
				{Value: "attr1"},
				{Value: "attr2"},
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
				{Email: "someone2@chromium.org"},
			},
		},
	},
	{
		TestCase: &api.TestCase{
			Id: &api.TestCase_Id{
				Value: "tast.test.002",
			},
			Name: "test002",
			Tags: []*api.TestCase_Tag{
				{Value: "attr1"},
				{Value: "attr2"},
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
				{Email: "someone1@chromium.org"},
				{Email: "someone2@chromium.org"},
			},
		},
	},
	{
		TestCase: &api.TestCase{
			Id: &api.TestCase_Id{
				Value: "tauto.test.003",
			},
			Name: "test003",
			Tags: []*api.TestCase_Tag{
				{Value: "attr3"},
				{Value: "attr4"},
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
				{Email: "someone1@chromium.org"},
				{Email: "someone2@chromium.org"},
			},
		},
	},
}

// TestMatchedTestsForSuites makes sure MatchedTestsForSuites return all expected metadata.
func TestMatchedTestsForSuites(t *testing.T) {
	suites := []*api.TestSuite{
		{
			Name: "suite1",
			Spec: &api.TestSuite_TestCaseIds{
				TestCaseIds: &api.TestCaseIdList{
					TestCaseIds: []*api.TestCase_Id{
						{Value: "tast.test.001"},
					},
				},
			},
		},
		{
			Name: "suite2",
			Spec: &api.TestSuite_TestCaseTagCriteria_{
				TestCaseTagCriteria: &api.TestSuite_TestCaseTagCriteria{
					Tags:        []string{"attr3"},
					TagExcludes: []string{"attr4"},
				},
			},
		},
	}
	expectedTests := map[string]bool{
		"test001": false,
		"test002": false,
	}
	matchedMdList, err := MatchedTestsForSuites(testMetadata, suites)
	if err != nil {
		t.Fatal("Failed to call MatchedTestsTestsForSuites: ", err)
	}

	for _, md := range matchedMdList {
		found, ok := expectedTests[md.TestCase.Name]
		if !ok {
			t.Errorf("Unexpected metadata: %+v", md)
		}
		if found {
			t.Errorf("Duplicate metadata: %+v", md)
		}
		expectedTests[md.TestCase.Name] = true
	}

	for test, found := range expectedTests {
		if !found {
			t.Errorf("Failed to find test: %v", test)
		}
	}
}

func TestPartialMatchDeQP(t *testing.T) {
	testMetadataLocal := []*api.TestCaseMetadata{
		{
			TestCase: &api.TestCase{
				Id: &api.TestCase_Id{
					Value: "tradefed.cts.CtsDeqpTestCases",
				},
				Name: "test001",
				Tags: []*api.TestCase_Tag{
					{Value: "attr1"},
					{Value: "attr2"},
				},
			},
		},
		{
			TestCase: &api.TestCase{
				Id: &api.TestCase_Id{
					Value: "tradefed.cts.Other",
				},
				Name: "test001",
				Tags: []*api.TestCase_Tag{
					{Value: "attr1"},
					{Value: "attr2"},
				},
			},
		},
	}

	suites := []*api.TestSuite{
		{
			Name: "suite1",
			Spec: &api.TestSuite_TestCaseIds{
				TestCaseIds: &api.TestCaseIdList{
					TestCaseIds: []*api.TestCase_Id{
						{Value: "tradefed.cts.CtsDeqpTestCases dEQP-GLES31.info#*"},
						{Value: "tradefed.cts.CtsDeqpTestCases dEQP-GLES2.info#*"},
					},
				},
			},
		},
	}
	expectedTests := map[string]bool{
		"tradefed.cts.CtsDeqpTestCases dEQP-GLES31.info#*": false,
		"tradefed.cts.CtsDeqpTestCases dEQP-GLES2.info#*":  false,
	}
	matchedMdList, err := MatchedTestsForSuites(testMetadataLocal, suites)
	if err != nil {
		t.Fatal("Failed to call MatchedTestsTestsForSuites: ", err)
	}
	for _, md := range matchedMdList {
		found, ok := expectedTests[md.TestCase.Id.Value]
		if !ok {
			t.Errorf("Unexpected metadata: %+v", md)
		}
		if found {
			t.Errorf("Duplicate metadata: %+v", md)
		}
		expectedTests[md.TestCase.Id.Value] = true
	}

	for test, found := range expectedTests {
		if !found {
			t.Errorf("Failed to find test: %v", test)
		}
	}
}

// TestMatchedTestsForSuites makes sure MatchedTestsForSuites return all expected metadata.
func TestPartialMatch(t *testing.T) {
	testMetadataLocal := []*api.TestCaseMetadata{
		{
			TestCase: &api.TestCase{
				Id: &api.TestCase_Id{
					Value: "tast.test.001",
				},
				Name: "test001",
				Tags: []*api.TestCase_Tag{
					{Value: "attr1"},
					{Value: "attr2"},
				},
			},
		},
		{
			TestCase: &api.TestCase{
				Id: &api.TestCase_Id{
					Value: "tast.test.002",
				},
				Name: "test001",
				Tags: []*api.TestCase_Tag{
					{Value: "attr1"},
					{Value: "attr2"},
				},
			},
		},
	}

	suites := []*api.TestSuite{
		{
			Name: "suite1",
			Spec: &api.TestSuite_TestCaseTagCriteria_{
				TestCaseTagCriteria: &api.TestSuite_TestCaseTagCriteria{
					TestNames: []string{"tast.test.001 foo", "tast.test.001 bar", "tast.test.002 bar", "tast.test.003 bar"},
				},
			},
		},
	}
	expectedTests := map[string]bool{
		"tast.test.001 foo": false,
		"tast.test.001 bar": false,
		"tast.test.002 bar": false,
	}
	matchedMdList, err := MatchedTestsForSuites(testMetadataLocal, suites)
	if err != nil {
		t.Fatal("Failed to call MatchedTestsTestsForSuites: ", err)
	}
	for _, md := range matchedMdList {
		found, ok := expectedTests[md.TestCase.Id.Value]
		if !ok {
			t.Errorf("Unexpected metadata: %+v", md)
		}
		if found {
			t.Errorf("Duplicate metadata: %+v", md)
		}
		expectedTests[md.TestCase.Id.Value] = true
	}

	for test, found := range expectedTests {
		if !found {
			t.Errorf("Failed to find test: %v", test)
		}
	}
}

// TestMatchedTestsForSuitesMissing make sure MatchedTestsForSuite return
// error when there is missing metadata..
func TestMatchedTestsForSuitesMissing(t *testing.T) {
	suites := []*api.TestSuite{
		{
			Name: "suite1",
			Spec: &api.TestSuite_TestCaseIds{
				TestCaseIds: &api.TestCaseIdList{
					TestCaseIds: []*api.TestCase_Id{
						{Value: "NotExist"},
					},
				},
			},
		},
	}
	if _, err := MatchedTestsForSuites(testMetadata, suites); err == nil {
		t.Fatal("Failed to get error while calling  MatchedTestsTestsForSuites with non-existing test case")
	}
}

// TestMatchedTestsForTestNameInSuites make sure MatchedTestsForSuite return
// all expected metadata.
func TestMatchedTestsForTestNameInSuites(t *testing.T) {
	suites := []*api.TestSuite{
		{
			Name: "suite1",
			Spec: &api.TestSuite_TestCaseTagCriteria_{
				TestCaseTagCriteria: &api.TestSuite_TestCaseTagCriteria{
					Tags:        []string{"attr1"},
					TagExcludes: []string{"attr4"},
					TestNames:   []string{"tast.test*"},
				},
			},
		},
	}
	expectedTests := map[string]bool{
		"tast.test.001": false,
		"tast.test.002": false,
	}
	matchedMdList, err := MatchedTestsForSuites(testMetadata, suites)
	if err != nil {
		t.Fatal("Failed to call MatchedTestsTestsForSuites: ", err)
	}
	for _, md := range matchedMdList {
		found, ok := expectedTests[md.TestCase.Id.Value]
		if !ok {
			t.Errorf("Unexpected metadata: %+v", md)
		}
		if found {
			t.Errorf("Duplicate metadata: %+v", md)
		}
		expectedTests[md.TestCase.Id.Value] = true
	}

	for test, found := range expectedTests {
		if !found {
			t.Errorf("Failed to find test: %v", test)
		}
	}

}

// TestMatchedTestsForSuitesWithNameSpaces make sure MatchedTestsForSuite return
// all expected metadata even if the suite name itself contains spaces.
func TestMatchedTestsForSuitesWithNameSpaces(t *testing.T) {
	testMetadataLocal := []*api.TestCaseMetadata{
		{
			TestCase: &api.TestCase{
				Id: &api.TestCase_Id{
					Value: "test with spaces",
				},
				Name: "test with spaces",
				Tags: []*api.TestCase_Tag{
					{Value: "attr1"},
					{Value: "attr2"},
				},
			},
		},
		{
			TestCase: &api.TestCase{
				Id: &api.TestCase_Id{
					Value: "test with spaces 2",
				},
				Name: "test with spaces 2",
				Tags: []*api.TestCase_Tag{
					{Value: "attr1"},
					{Value: "attr2"},
				},
			},
		},
	}

	suites := []*api.TestSuite{
		{
			Name: "suite.with.spaces",
			Spec: &api.TestSuite_TestCaseIds{
				TestCaseIds: &api.TestCaseIdList{
					TestCaseIds: []*api.TestCase_Id{
						{Value: "test with spaces"},
						{Value: "test with spaces 2"},
					},
				},
			},
		},
	}
	expectedTests := map[string]bool{
		"test with spaces":   false,
		"test with spaces 2": false,
	}

	matchedMdList, err := MatchedTestsForSuites(testMetadataLocal, suites)
	if err != nil {
		t.Fatal("Failed to call MatchedTestsTestsForSuites: ", err)
	}

	for _, md := range matchedMdList {
		found, ok := expectedTests[md.TestCase.Name]
		if !ok {
			t.Errorf("Unexpected metadata: %+v", md)
		}
		if found {
			t.Errorf("Duplicate metadata: %+v", md)
		}
		expectedTests[md.TestCase.Name] = true
	}

	for test, found := range expectedTests {
		if !found {
			t.Errorf("Failed to find test: %v", test)
		}
	}
}

// TestMatchedTestsForTestNameExcludesInSuites make sure MatchedTestsForSuite return
// all expected metadata excluding tests from TestNameExcludes.
func TestMatchedTestsForTestNameExcludesInSuites(t *testing.T) {
	suites := []*api.TestSuite{
		{
			Name: "suite1",
			Spec: &api.TestSuite_TestCaseTagCriteria_{
				TestCaseTagCriteria: &api.TestSuite_TestCaseTagCriteria{
					Tags:             []string{"attr1"},
					TagExcludes:      []string{"attr4"},
					TestNames:        []string{"tast*"},
					TestNameExcludes: []string{"tast.test.002"},
				},
			},
		},
	}
	expectedTests := map[string]bool{
		"tast.test.001": false,
	}
	matchedMdList, err := MatchedTestsForSuites(testMetadata, suites)
	if err != nil {
		t.Fatal("Failed to call MatchedTestsTestsForSuites: ", err)
	}

	for _, md := range matchedMdList {
		found, ok := expectedTests[md.TestCase.Id.Value]
		if !ok {
			t.Errorf("Unexpected metadata: %+v", md)
		}
		if found {
			t.Errorf("Duplicate metadata: %+v", md)
		}
		expectedTests[md.TestCase.Id.Value] = true
	}

	for test, found := range expectedTests {
		if !found {
			t.Errorf("Failed to find test: %v", test)
		}
	}

}

// TestMatch makes sure match return all metadata that match test case tag criteria.
func TestMatch(t *testing.T) {
	md := &api.TestCaseMetadata{
		TestCase: &api.TestCase{
			Id: &api.TestCase_Id{
				Value: "id1",
			},
			Name: "test001",
			Tags: []*api.TestCase_Tag{
				{Value: "attr1"},
				{Value: "attr2"},
			},
		},
	}
	tm := newTagMatcher(&api.TestSuite_TestCaseTagCriteria{
		Tags:        []string{"attr1"},
		TagExcludes: []string{"attr4"},
	})
	if !tm.match(md) {
		t.Fatal("tagMatcher failed to match a matching test metadata")
	}
}

// TestMatchExclude makes sure match exclude metadata that match test case exclude criteria.
func TestMatchExclude(t *testing.T) {
	md := &api.TestCaseMetadata{
		TestCase: &api.TestCase{
			Id: &api.TestCase_Id{
				Value: "id1",
			},
			Name: "test001",
			Tags: []*api.TestCase_Tag{
				{Value: "attr1"},
				{Value: "attr4"},
			},
		},
	}
	tm := newTagMatcher(&api.TestSuite_TestCaseTagCriteria{
		Tags:        []string{"attr1"},
		TagExcludes: []string{"attr4"},
	})
	if tm.match(md) {
		t.Fatal("tagMatcher failed to exclude a test metadata")
	}
}

// TestNameMatch makes sure match return all metadata that match test case tag criteria and atleast one test name with or without wildcard.
func TestNameMatch(t *testing.T) {
	md := &api.TestCaseMetadata{
		TestCase: &api.TestCase{
			Id: &api.TestCase_Id{
				Value: "tast.example",
			},
			Name: "test001",
			Tags: []*api.TestCase_Tag{
				{Value: "attr1"},
				{Value: "attr4"},
			},
		},
	}
	tm := newTagMatcher(&api.TestSuite_TestCaseTagCriteria{
		TestNames: []string{"tast.example foo#bar", "tast.example foo#bar2"},
	})
	if !tm.match(md) {
		t.Fatal("tagMatcher failed to match a matching test metadata")
	}
}

// TestMatchExclude makes sure match exclude metadata that match test names exclude criteria.
func TestNameMatchExclude(t *testing.T) {
	md := &api.TestCaseMetadata{
		TestCase: &api.TestCase{
			Id: &api.TestCase_Id{
				Value: "tast.example.Pass",
			},
			Name: "test001",
			Tags: []*api.TestCase_Tag{
				{Value: "attr1"},
				{Value: "attr4"},
			},
		},
	}
	tm := newTagMatcher(&api.TestSuite_TestCaseTagCriteria{
		Tags:             []string{"attr1"},
		TagExcludes:      []string{"attr3"},
		TestNames:        []string{"tast.example.Pass"},
		TestNameExcludes: []string{"tast.example.Pas*"},
	})
	if tm.match(md) {
		t.Fatal("tagMatcher failed to exclude a test metadata")
	}
}
