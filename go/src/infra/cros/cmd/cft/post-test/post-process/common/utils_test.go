// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"io"
	"log"
	"testing"

	_go "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/artifact"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestTestLevelFiles(t *testing.T) {
	t.Parallel()

	emptyLogger := log.New(io.Discard, "", 0)
	gscInfoFileName := "gsc_info.json"

	ftt.Run("GSCInfo works", t, func(t *ftt.Test) {
		tastFirstClassTest := "tast.storage.FirstClass"
		tastSecondClassTest := "tast.storage.SecondClass"
		tautoTest := "tauto.foo.bar"
		tastResultDirPath := "/tmp/test/results/tast/tests/" + tastFirstClassTest
		tautoResultDirPath := "/tmp/test/tauto/results-1-storage_testing_v3_part_perf"
		testResult := &artifact.TestResult{
			TestRuns: []*artifact.TestRun{
				// For Tast first class test
				{
					TestCaseInfo: &artifact.TestCaseInfo{
						TestCaseResult: &api.TestCaseResult{
							TestCaseId: &api.TestCase_Id{
								Value: tastFirstClassTest,
							},
							ResultDirPath: &_go.StoragePath{
								HostType: _go.StoragePath_LOCAL,
								Path:     tastResultDirPath,
							},
						},
					},
				},
				// For Tast second class test wrapped by Tauto test suite
				{
					TestCaseInfo: &artifact.TestCaseInfo{
						TestCaseResult: &api.TestCaseResult{
							TestCaseId: &api.TestCase_Id{
								Value: tastSecondClassTest,
							},
							ResultDirPath: &_go.StoragePath{
								HostType: _go.StoragePath_LOCAL,
								Path:     tautoResultDirPath,
							},
						},
					},
				},
				// For Tauto test
				{
					TestCaseInfo: &artifact.TestCaseInfo{
						TestCaseResult: &api.TestCaseResult{
							TestCaseId: &api.TestCase_Id{
								Value: tautoTest,
							},
							ResultDirPath: &_go.StoragePath{
								HostType: _go.StoragePath_LOCAL,
								Path:     tautoResultDirPath,
							},
						},
					},
				},
			},
		}
		want := map[string]string{
			tastFirstClassTest:  tastResultDirPath + "/" + gscInfoFileName,
			tastSecondClassTest: tautoResultDirPath + "/" + TastTestsDir + "/storage.SecondClass/" + gscInfoFileName,
			tautoTest:           tautoResultDirPath + "/foo.bar/" + gscInfoFileName,
		}

		got := TestLevelFiles(emptyLogger, testResult, gscInfoFileName)

		assert.Loosely(t, got, should.Match(want))
	})

	ftt.Run("Return empty if no test result", t, func(t *ftt.Test) {
		got := TestLevelFiles(emptyLogger, nil, gscInfoFileName)

		assert.Loosely(t, got, should.Match(map[string]string{}))
	})

	ftt.Run("Return empty if no test case result", t, func(t *ftt.Test) {
		testResult := &artifact.TestResult{
			TestRuns: []*artifact.TestRun{
				{
					TestCaseInfo: &artifact.TestCaseInfo{},
				},
			},
		}

		got := TestLevelFiles(emptyLogger, testResult, gscInfoFileName)

		assert.Loosely(t, got, should.Match(map[string]string{}))

	})
}
