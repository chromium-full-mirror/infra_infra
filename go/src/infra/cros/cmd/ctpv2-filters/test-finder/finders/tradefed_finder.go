// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package finders contains the implementations of the abstract finder interface.
package finders

import (
	"context"
	"fmt"
	"log"

	"cloud.google.com/go/storage"

	"go.chromium.org/chromiumos/config/go/test/api"
	finder "go.chromium.org/chromiumos/test/util/finder"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/test-finder/common"
)

var TradefedFinderType = common.FinderHarness("Tradefed")

type TradefedFinder struct {
	*common.AbstractFinder
}

func matchTestsForTradefed(testSuites []*api.TestSuite, log *log.Logger) ([]*api.TestCaseMetadata, error) {
	src, err := GetTFSourceData(context.Background(), "cros-xts-metadata")
	if err != nil {
		log.Println("Unable to fetch data from GCS: ", err)
	}
	// The source data will not have direct access to the actual proto bindings; thus is in a loose json format
	// we will translate this into the strict proto format here.
	metadata := TranslateTFSrcToMetadata(src)
	log.Println("Looked for test cases in: ", metadata)

	return finder.MatchedTestsForSuites(metadata, testSuites)
}

func (ex *TradefedFinder) FindTestsAB() (*api.InternalTestplan, error) {
	ex.Logger.Println("Looking for TF Tests!")
	suites, err := TPtoSuite(ex.Testplan)
	if err != nil {
		ex.Logger.Println("unable to convert testplan to suite: ", err)
		return nil, err
	}
	matchingTests, err := matchTestsForTradefed(suites, ex.Logger)
	if err != nil {
		ex.Logger.Println("unable to match test:", err)
	}
	if len(matchingTests) == 0 {
		ex.Logger.Println("Found no test cases for: ", suites)

	}

	// Translate the TC metadata schema into CTP testplan schema.
	ctpTestCases := common.TranslateTCMtoCTPTC(matchingTests)
	ex.Testplan.TestCases = append(ex.Testplan.TestCases, ctpTestCases...)
	common.AddFlexibleTFFlag(ex.Testplan)
	return ex.Testplan, nil
}

func NewTradefedFinder(ctx context.Context, req *api.InternalTestplan, log *log.Logger) *TradefedFinder {
	absExec := common.NewAbstractFinder(ctx, req, TradefedFinderType, log)
	return &TradefedFinder{AbstractFinder: absExec}
}

func GetTFSourceData(ctx context.Context, gcsBasePath string) ([][]byte, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage.NewClient: %w", err)
	}
	defer client.Close()

	bucket := client.Bucket(gcsBasePath)

	data, err := common.PullAllFilesFromGcsDir(ctx, bucket, "", "latest.json")
	if err != nil {
		return nil, err
	}
	return data, nil
}
