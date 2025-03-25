// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cft/cros-test-finder/test_finder"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/servertemplate"
)

const (
	address                 = "localhost"
	binName                 = "test_finder_filter"
	mdDir                   = "/tmp/test/metadata"
	centralizedSuitesPrefix = "centralizedsuite:"
)

func toCTPTestCase(metadata *api.TestCaseMetadata) *api.CTPTestCase {

	return &api.CTPTestCase{
		Name:     metadata.GetTestCase().GetName(),
		Metadata: metadata,
	}
}
func toTestFinderRequest(testPlan *api.InternalTestplan) (*api.CrosTestFinderRequest, error) {
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

// fillTestCases will make cros-test-finder request from SuiteInfo.SuiteRequest,
// and fill in the TestCases field with test cases information from the response.
func fillTestCases(ctx context.Context, testPlan *api.InternalTestplan, resp *api.CrosTestFinderResponse, log *log.Logger) error {

	// TODO loop through testSuites.
	if len(resp.GetTestSuites()) == 0 {
		return nil
	}
	metadataList, ok := resp.GetTestSuites()[0].Spec.(*api.TestSuite_TestCasesMetadata)
	if !ok {
		return errors.New("no test cases metadata in the response")
	}
	log.Println("Going to translate...")
	for _, metadata := range metadataList.TestCasesMetadata.GetValues() {
		log.Println(metadata)
		testPlan.TestCases = append(testPlan.TestCases, toCTPTestCase(metadata))
	}
	return nil
}

func executor(req *api.InternalTestplan, log *log.Logger, commonParams *servertemplate.CommonFilterParams) (*api.InternalTestplan, error) {
	ctx := context.Background()

	testFinderResponse, err := startAndRunTestFinder(req, log)
	if err != nil {
		log.Println("Error with TestFinder: ", err)
		return req, err
	}
	log.Println("Called TestFinder successfully:", testFinderResponse)

	err = fillTestCases(ctx, req, testFinderResponse, log)
	if err != nil {
		log.Println("Error with setting internalPlan: ", err)
		return req, err
	}
	return req, nil
}

func startAndRunTestFinder(testPlan *api.InternalTestplan, log *log.Logger) (*api.CrosTestFinderResponse, error) {
	req, err := toTestFinderRequest(testPlan)
	if err != nil {
		return nil, fmt.Errorf("failed to Translate Request: %w", err)
	}
	log.Printf("made req: %s", req)

	response, err := test_finder.FindTests(log, req, mdDir)
	if err != nil {
		return nil, fmt.Errorf("unable to run test-finder: %w", err)
	}

	if err != nil {
		return nil, fmt.Errorf("Failed2 to call test-finder: %w", err)
	}

	log.Printf("Got Response: %s", response)

	return response, nil

}

func main() {
	err := servertemplate.Server(executor, binName)
	if err != nil {
		os.Exit(2)
	}

	os.Exit(0)
}
