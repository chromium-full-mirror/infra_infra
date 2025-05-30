// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package finders contains the implementations of the abstract finder interface.
package finders

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/api/option"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cft/common/finder"
	commonlib "go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/test-finder/common"
)

const abGCSDir = "mobly_priv_artifacts/aosp_out"

var AndroidBuildMoblyFinderType = common.FinderHarness("AndroidBuildMobly")

// AndroidBuildMoblyFinder fetches test metadata for tests originating from AndroidBuild.
type AndroidBuildMoblyFinder struct {
	*common.AbstractFinder
}

func NewAndroidBuildMoblyFinder(ctx context.Context, req *api.InternalTestplan, log *log.Logger) *AndroidBuildMoblyFinder {
	absExec := common.NewAbstractFinder(ctx, req, AndroidBuildMoblyFinderType, log)
	return &AndroidBuildMoblyFinder{AbstractFinder: absExec}
}

func (ex *AndroidBuildMoblyFinder) FindTestsAB(commonParams *commonlib.CommonFilterParams) (*api.InternalTestplan, error) {
	ex.Logger.Println("Searching for Android Build Mobly Tests...")
	matchingTests, err := matchTestsforAndroidBuildMobly(context.Background(), ex.Testplan, ex.Logger, commonParams)
	if err != nil {
		ex.Logger.Println("unable to match test:", err)
	}
	ex.Logger.Printf("found matching tests: %v", matchingTests)

	// Translate the TC metadata schema into CTP testplan schema.
	ctpTestCases := common.TranslateTCMtoCTPTC(matchingTests)
	ex.Testplan.TestCases = append(ex.Testplan.TestCases, ctpTestCases...)
	common.AddFlexibleTFFlag(ex.Testplan)
	return ex.Testplan, nil
}

func matchTestsforAndroidBuildMobly(ctx context.Context, tp *api.InternalTestplan, log *log.Logger, commonParams *commonlib.CommonFilterParams) ([]*api.TestCaseMetadata, error) {
	suites, err := common.TestSuiteFromTestplan(tp)
	if err != nil {
		return nil, fmt.Errorf("unable to convert testplan to suite: %w", err)
	}

	tokenSource := commonParams.AuthHelper.GetTokenSource([]string{}, commonlib.AllPurposeCloudScope)
	dir, err := fetchBuildDirectory(ctx, tp, log, abGCSDir, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("unable to fetch artifact directory: %w", err)
	}

	metadata, err := GetAndroidBuildMoblySourceData(context.Background(), log, abGCSDir, dir, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("unable to fetch data from GCS: %w", err)
	}

	log.Printf("Parsed AndroidBuild metadata: %v", metadata)
	return finder.MatchedTestsForSuites(metadata, suites)
}

func fetchBuildDirectory(ctx context.Context, tp *api.InternalTestplan, log *log.Logger, gcsBasePath string, clientOpts ...option.ClientOption) (string, error) {
	buildID, err := common.AndroidBuildIDFromTestplan(tp)
	if err != nil {
		// Build ID not available for Kron scheduled tests.
		// Technically undefined, so reset buildID.
		log.Printf("No BuildID present in test suite metadata, falling back to LATEST")
		buildID = ""
	}

	dir, exists, err := PathOrLatest(ctx, gcsBasePath, buildID, clientOpts...)
	if err != nil {
		return "", err
	}

	if !exists {
		log.Printf("Failed to find artifacts for build %q, falling back to %q", buildID, dir)
	}
	return dir, nil
}
