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

var G3MoblyFinderType = common.FinderHarness("G3Mobly")

type G3MoblyFinder struct {
	*common.AbstractFinder
}

func matchTestsforG3Mobly(testSuites []*api.TestSuite, log *log.Logger) ([]*api.TestCaseMetadata, error) {
	src, err := getSourceData(context.Background(), "mobly_priv_artifacts/out")
	if err != nil {
		log.Println("Unable to fetch data from GCS: ", err)
	}
	// The source data will not have direct access to the actual proto bindings; thus is in a loose json format
	// we will translate this into the strict proto format here.
	metadata := translateG3SrcToMetadata(src)

	return finder.MatchedTestsForSuites(metadata, testSuites)
}

func (ex *G3MoblyFinder) FindTestsAB() (*api.InternalTestplan, error) {
	suites, err := TPtoSuite(ex.Testplan)
	if err != nil {
		ex.Logger.Println("unable to convert testplan to suite: ", err)
		return nil, err
	}
	matchingTests, err := matchTestsforG3Mobly(suites, ex.Logger)
	if err != nil {
		ex.Logger.Println("unable to match test:", err)
	}

	// Translate the TC metadata schema into CTP testplan schema.
	ctpTestCases := common.TranslateTCMtoCTPTC(matchingTests)
	ex.Testplan.TestCases = append(ex.Testplan.TestCases, ctpTestCases...)
	common.AddFlexibleTFFlag(ex.Testplan)
	return ex.Testplan, nil
}

// append the magical `moblySuite` key on the suiteArgs to be processed by cros-test.
func addMoblyFlagToSuiteArgs(tp *api.InternalTestplan) {
	existingMD := tp.GetSuiteInfo().GetSuiteMetadata().GetExecutionMetadata()
	if existingMD == nil || len(existingMD.Args) == 0 {
		existingMD = &api.ExecutionMetadata{Args: []*api.Arg{}}
	}
	existingMD.Args = append(existingMD.Args, &api.Arg{
		Flag:  "moblySuite",
		Value: "true",
	})
	tp.SuiteInfo.SuiteMetadata.ExecutionMetadata = existingMD
}

func NewG3MoblyFinder(ctx context.Context, req *api.InternalTestplan, log *log.Logger) *G3MoblyFinder {
	absExec := common.NewAbstractFinder(ctx, req, G3MoblyFinderType, log)
	return &G3MoblyFinder{AbstractFinder: absExec}
}

func getSourceData(ctx context.Context, gcsBasePath string) ([][]byte, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage.NewClient: %w", err)
	}
	defer client.Close()

	bucketName, pf, err := common.ExtractBucketAndPrefixFromPath(gcsBasePath)
	if err != nil {
		return nil, err
	}

	bucket := client.Bucket(bucketName)

	// Currently G3Mobly content is fetched off the latest.
	// TODO, we probably should expose a way to pass in a desired path to allow repeatability on tests here.
	newestDir, err := common.FindNewestDirInGcsBucket(ctx, gcsBasePath, bucket, pf)
	if err != nil {
		return nil, err
	}
	data, err := common.PullAllFilesFromGcsDir(ctx, bucket, newestDir, ".json")
	if err != nil {
		return nil, err
	}
	return data, nil
}

func TPtoSuite(req *api.InternalTestplan) ([]*api.TestSuite, error) {
	testSuites, err := common.TestSuiteFromTestplan(req)
	if err != nil {
		return nil, err
	}
	return testSuites, nil
}
