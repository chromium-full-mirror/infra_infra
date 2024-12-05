// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"log"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/test/pre_process/cmd/pre-process/structs"
)

// Filter struct tracks the data that was removed, items not found, and other relevant details.
type Filter struct {
	data     map[string]structs.SignalFormat
	removed  []string
	notFound []string
	req      *api.FilterFlakyRequest
}

// filterCases filters the provided test cases based on the prepopulated f.data.
// NOTE: f.data must be populated before calling this method.
func (f *Filter) filterCases(testcases []*api.TestCase, testSuiteName string, log *log.Logger) *api.TestSuite {
	keepCases := []*api.TestCase{}
	log.Printf("Filtering Cases...\n")

	// Iterate through the test cases, keeping only those that are eligible.
	for _, tc := range testcases {
		value := tc.GetId().GetValue()
		if f.testEligible(value) {
			keepCases = append(keepCases, tc)
		}
	}

	return &api.TestSuite{
		Name: testSuiteName,
		Spec: &api.TestSuite_TestCases{
			TestCases: &api.TestCaseList{TestCases: keepCases},
		},
	}
}

// filterMetadata filters metadata based on the prepopulated f.data.
// NOTE: f.data must be populated before calling this method.
func (f *Filter) filterMetadata(testCaseMetadataList []*api.TestCaseMetadata, testSuiteName string, log *log.Logger) *api.TestSuite {
	filteredMetadata := []*api.TestCaseMetadata{}
	log.Printf("Filtering metadata list...\n")

	// Iterate through the test metadatalist, keeping only those that are eligible.
	for _, metadata := range testCaseMetadataList {
		value := metadata.GetTestCase().GetId().GetValue()
		if f.testEligible(value) {
			filteredMetadata = append(filteredMetadata, metadata)
		}
	}

	return &api.TestSuite{
		Name: testSuiteName,
		Spec: &api.TestSuite_TestCasesMetadata{
			TestCasesMetadata: &api.TestCaseMetadataList{Values: filteredMetadata},
		},
	}
}

// testEligible will return false if the stabiliyData shows the test is unstable, else true.
// This indicates that if there is no stabilityData, we will return true.
func (f *Filter) testEligible(value string) bool {
	// This loop doesn't "determine" eligibity, as that is left to the policy.
	// It is simply looping through the given tests, and checking for their "signal" (ie, eligibity)
	// and applying returning that; in conjunction with applying the defaultEnabled rule.
	_, ok := f.data[value]
	isAllowed := false

	// If the test is found in the stabilityData, check for its signal and use that
	if ok {
		isAllowed = f.data[value].Signal
		if isAllowed {
			return true
		}
		f.removed = append(f.removed, value)
	} else {
		// If the test is not found, and `DefaultEnabled` is set, keep the test
		f.notFound = append(f.notFound, value)
		if f.req.DefaultEnabled {
			return true
		}
	}
	log.Printf("Test %s marked as not eligible.\n", value)
	return false
}
