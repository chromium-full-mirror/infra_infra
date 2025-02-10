// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package finders contains the implementations of the abstract finder interface.
package finders

import (
	"context"
	"log"

	"go.chromium.org/chromiumos/config/go/test/api"
	finder "go.chromium.org/chromiumos/test/util/finder"

	"infra/cros/cmd/ctpv2-filters/test-finder/common"
)

var G3MoblyFinderType = common.FinderHarness("G3Mobly")

type G3MoblyFinder struct {
	*common.AbstractFinder
}

func (ex *G3MoblyFinder) FindTestsAB() (*api.InternalTestplan, error) {
	src := getSourceData()
	metadata := translateSrcToMetadata(src)
	matchingTests, _ := matchTests(metadata, ex.Testplan)
	ctpTestCases := translateTC(matchingTests)
	ex.Testplan.TestCases = append(ex.Testplan.TestCases, ctpTestCases...)
	return ex.Testplan, nil
}

func NewG3MoblyFinder(ctx context.Context, req *api.InternalTestplan, log *log.Logger) *G3MoblyFinder {
	absExec := common.NewAbstractFinder(ctx, req, G3MoblyFinderType, log)
	return &G3MoblyFinder{AbstractFinder: absExec}
}

// TODO implement; currently just building up the logical flow + signatures.
func getSourceData() []byte {
	return nil
}

func matchTests(metadata []*api.TestCaseMetadata, req *api.InternalTestplan) ([]*api.TestCaseMetadata, error) {
	testSuites := testSuiteFromTestplan(req)
	return finder.MatchedTestsForSuites(metadata, testSuites)
}

// TODO implement; currently just building up the logical flow + signatures.
func translateTC(matchingTests []*api.TestCaseMetadata) []*api.CTPTestCase {
	return nil
}

// TODO implement; currently just building up the logical flow + signatures.
func testSuiteFromTestplan(req *api.InternalTestplan) []*api.TestSuite {
	return nil
}
