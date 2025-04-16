// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"fmt"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/libs/skylab/inventory/autotest/labels"
	"go.chromium.org/infra/libs/skylab/inventory/swarming"
)

func ToTestFinderRequest(testPlan *api.InternalTestplan) (*api.CrosTestFinderRequest, error) {
	centralizedSuitesPrefix := "centralizedsuite:"
	// TODO... switch
	requestedSuite, ok := testPlan.GetSuiteInfo().GetSuiteRequest().GetSuiteRequest().(*api.SuiteRequest_TestSuite)
	if !ok {
		return nil, errors.New("SuiteRequest is not TestSuite")
	}
	testSuite := requestedSuite.TestSuite
	if testSuite != nil && strings.HasPrefix(testSuite.Name, centralizedSuitesPrefix) {
		return &api.CrosTestFinderRequest{
			CentralizedSuite: strings.TrimPrefix(testSuite.Name, centralizedSuitesPrefix),
			MetadataRequired: true,
		}, nil
	}
	return &api.CrosTestFinderRequest{
		TestSuites:       []*api.TestSuite{testSuite},
		MetadataRequired: true,
	}, nil
}

func FillTestCasesIntoTestPlan(testPlan *api.InternalTestplan, resp *api.CrosTestFinderResponse) error {
	if len(resp.GetTestSuites()) == 0 {
		return nil
	}

	// Only need to check the [0] index; as test-finder only populates that.
	metadataList, ok := resp.GetTestSuites()[0].Spec.(*api.TestSuite_TestCasesMetadata)
	if !ok {
		return errors.New("no test cases metadata in the response")
	}

	for _, metadata := range metadataList.TestCasesMetadata.GetValues() {
		testPlan.TestCases = append(testPlan.TestCases, tfToCTPTestCase(metadata))
	}
	return nil
}

func tfToCTPTestCase(metadata *api.TestCaseMetadata) *api.CTPTestCase {
	tc := &api.CTPTestCase{
		Name:     metadata.GetTestCase().GetId().GetValue(),
		Metadata: metadata,
	}

	deps := Converter(tc.GetMetadata().GetTestCase().GetDependencies())
	if len(deps) != 0 {
		tc.Metadata.TestCase.Dependencies = deps
	}
	return tc
}

func Converter(deps []*api.TestCase_Dependency) []*api.TestCase_Dependency {
	convertedDeps := []string{}
	for _, dep := range deps {
		f := dep.GetValue()
		converted := convertDep(f)
		// If the dep can't be converted, let it flow through naturally. Bot params should handel the case where its invalid
		if len(converted) == 0 {
			convertedDeps = append(convertedDeps, f)
		} else {
			convertedDeps = append(convertedDeps, converted...)
		}
	}
	finalDeps := []*api.TestCase_Dependency{}
	for _, dep := range convertedDeps {
		tcD := &api.TestCase_Dependency{
			Value: dep,
		}
		finalDeps = append(finalDeps, tcD)
	}
	return finalDeps
}

func convertDep(dep string) []string {
	deps := []string{dep}
	parsedDeps := labels.Revert(deps)

	depsf := []string{}
	for k, v := range swarming.Convert(parsedDeps) {
		for _, innerv := range v {
			depsf = append(depsf, fmt.Sprintf("%s:%s", k, innerv))

		}
	}
	return depsf
}
