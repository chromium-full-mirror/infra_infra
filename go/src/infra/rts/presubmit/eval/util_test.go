// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package eval

import (
	"bytes"
	"testing"
	"time"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"google.golang.org/protobuf/types/known/durationpb"

	evalpb "infra/rts/presubmit/eval/proto"
)

func TestPSURL(t *testing.T) {
	t.Parallel()
	ftt.Run(`psURL`, t, func(t *ftt.Test) {
		patchSet := &evalpb.GerritPatchset{
			Change: &evalpb.GerritChange{
				Host:   "example.googlesource.com",
				Number: 123,
			},
			Patchset: 4,
		}
		assert.Loosely(t, psURL(patchSet), should.Equal("https://example.googlesource.com/c/123/4"))
	})
}

func TestPrintResults(t *testing.T) {
	t.Parallel()

	ftt.Run(`PrintResults`, t, func(t *ftt.Test) {
		r := &evalpb.Results{
			TotalRejections:   100,
			TotalTestFailures: 100,
			TotalDuration:     durationpb.New(time.Hour * 24 * 365),
			Thresholds: []*evalpb.Threshold{
				{
					Savings: 1,
				},
				{
					MaxDistance:  10,
					ChangeRecall: 0.99,
					TestRecall:   0.99,
					Savings:      0.25,
				},
				{
					MaxDistance:  40,
					ChangeRecall: 1,
					TestRecall:   1,
					Savings:      0.5,
				},
			},
		}

		buf := &bytes.Buffer{}
		PrintResults(r, buf, 0)
		assert.Loosely(t, buf.String(), should.Equal(`
ChangeRecall | Savings | TestRecall | Distance
----------------------------------------------
  0.00%      | 100.00% |   0.00%    |  0.000
 99.00%      |  25.00% |  99.00%    | 10.000
100.00%      |  50.00% | 100.00%    | 40.000

based on 100 rejections, 100 test failures, 1 years 0 days 0 hours 0 minutes 0 seconds testing time
`[1:]))
	})

	ftt.Run(`PrintResultsNoDuplicates`, t, func(t *ftt.Test) {
		r := &evalpb.Results{
			TotalRejections:   100,
			TotalTestFailures: 100,
			TotalDuration:     durationpb.New(time.Hour),
			Thresholds: []*evalpb.Threshold{
				{
					Savings: 1,
				},
				{
					MaxDistance:  10,
					ChangeRecall: 0.8,
					TestRecall:   0.8,
					Savings:      0.25,
				},
				{
					MaxDistance:  40,
					ChangeRecall: 0.9,
					TestRecall:   0.9,
					Savings:      0.5,
				},
				{
					MaxDistance:  40,
					ChangeRecall: 0.9,
					TestRecall:   0.9,
					Savings:      0.5,
				},
				{
					MaxDistance:  41,
					ChangeRecall: 1,
					TestRecall:   0.9,
					Savings:      0.5,
				},
			},
		}

		buf := &bytes.Buffer{}
		PrintResults(r, buf, 0)
		assert.Loosely(t, buf.String(), should.Equal(`
ChangeRecall | Savings | TestRecall | Distance
----------------------------------------------
  0.00%      | 100.00% |   0.00%    |  0.000
 80.00%      |  25.00% |  80.00%    | 10.000
 90.00%      |  50.00% |  90.00%    | 40.000
100.00%      |  50.00% |  90.00%    | 41.000

based on 100 rejections, 100 test failures, 1 hours 0 minutes 0 seconds testing time
`[1:]))
	})
}

func TestPrintSpecificResults(t *testing.T) {
	t.Parallel()

	ftt.Run(`PrintSpecificResults`, t, func(t *ftt.Test) {
		r := &evalpb.Results{
			TotalRejections:   100,
			TotalTestFailures: 100,
			TotalDuration:     durationpb.New(time.Hour),
			Thresholds: []*evalpb.Threshold{
				{
					Savings: 1,
				},
				{
					MaxDistance:  10,
					ChangeRecall: 0.99,
					TestRecall:   0.99,
					Savings:      0.25,
				},
				{
					MaxDistance:  40,
					ChangeRecall: 1,
					TestRecall:   1,
					Savings:      0.5,
				},
			},
		}

		buf := &bytes.Buffer{}
		PrintSpecificResults(r, buf, 0, true, true)
		assert.Loosely(t, buf.String(), should.Equal(`
ChangeRecall | Savings | TestRecall | Distance
----------------------------------------------
  0.00%      | 100.00% |   0.00%    |  0.000
 99.00%      |  25.00% |  99.00%    | 10.000
100.00%      |  50.00% | 100.00%    | 40.000

based on 100 rejections, 100 test failures, 1 hours 0 minutes 0 seconds testing time
`[1:]))

		buf = &bytes.Buffer{}
		PrintSpecificResults(r, buf, 0, true, false)
		assert.Loosely(t, buf.String(), should.Equal(`
ChangeRecall | Savings | TestRecall
-----------------------------------
  0.00%      | 100.00% |   0.00%    
 99.00%      |  25.00% |  99.00%    
100.00%      |  50.00% | 100.00%    

based on 100 rejections, 100 test failures, 1 hours 0 minutes 0 seconds testing time
`[1:]))

		buf = &bytes.Buffer{}
		PrintSpecificResults(r, buf, 0, false, true)
		assert.Loosely(t, buf.String(), should.Equal(`
ChangeRecall | Savings | Distance
---------------------------------
  0.00%      | 100.00% |  0.000
 99.00%      |  25.00% | 10.000
100.00%      |  50.00% | 40.000

based on 100 rejections, 100 test failures, 1 hours 0 minutes 0 seconds testing time
`[1:]))

		buf = &bytes.Buffer{}
		PrintSpecificResults(r, buf, 0, false, false)
		assert.Loosely(t, buf.String(), should.Equal(`
ChangeRecall | Savings
----------------------
  0.00%      | 100.00% 
 99.00%      |  25.00% 
100.00%      |  50.00% 

based on 100 rejections, 100 test failures, 1 hours 0 minutes 0 seconds testing time
`[1:]))
	})
}
