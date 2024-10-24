// Copyright 2020 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"context"
	"strings"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	pb "go.chromium.org/luci/resultdb/proto/v1"
	sinkpb "go.chromium.org/luci/resultdb/sink/proto/v1"
)

func TestSingleConversions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run(`From JSON works`, t, func(t *ftt.Test) {
		str := `{
				"failures": [
					"Failed to run content_shell."
				],
				"valid": true
			}`

		results := &SingleResult{}
		err := results.ConvertFromJSON(strings.NewReader(str))
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, results.Failures, should.Resemble([]string{"Failed to run content_shell."}))
		assert.Loosely(t, results.Valid, should.BeTrue)
	})

	ftt.Run(`ToProtos`, t, func(t *ftt.Test) {
		t.Run("test passes", func(t *ftt.Test) {
			results := &SingleResult{
				Failures: []string{},
				Valid:    true,
			}

			testResults, err := results.ToProtos(ctx)
			assert.Loosely(t, err, should.BeNil)

			expected := &sinkpb.TestResult{
				TestId:   "",
				Expected: true,
				Status:   pb.TestStatus_PASS,
			}
			assert.Loosely(t, testResults, should.HaveLength(1))
			assert.Loosely(t, testResults[0], should.Resemble(expected))
		})

		t.Run("test fails", func(t *ftt.Test) {
			results := &SingleResult{
				Failures: []string{"Failed to run content_shell."},
				Valid:    true,
			}

			testResults, err := results.ToProtos(ctx)
			assert.Loosely(t, err, should.BeNil)

			expected := &sinkpb.TestResult{
				TestId:      "",
				Expected:    false,
				Status:      pb.TestStatus_FAIL,
				SummaryHtml: "<pre>Failed to run content_shell.</pre>",
			}
			assert.Loosely(t, testResults, should.HaveLength(1))
			assert.Loosely(t, testResults[0], should.Resemble(expected))
		})

		t.Run("test result invalid", func(t *ftt.Test) {
			results := &SingleResult{
				Failures: []string{},
				Valid:    false,
			}

			testResults, err := results.ToProtos(ctx)
			assert.Loosely(t, err, should.BeNil)

			expected := &sinkpb.TestResult{
				TestId:   "",
				Expected: false,
				Status:   pb.TestStatus_ABORT,
			}
			assert.Loosely(t, testResults, should.HaveLength(1))
			assert.Loosely(t, testResults[0], should.Resemble(expected))
		})
	})
}
