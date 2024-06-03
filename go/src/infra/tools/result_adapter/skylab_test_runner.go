// Copyright 2021 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"strconv"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/resultdb/pbutil"
	pb "go.chromium.org/luci/resultdb/proto/v1"
	sinkpb "go.chromium.org/luci/resultdb/sink/proto/v1"
)

// Following CrOS test_runner's convention, test_case represents a single test
// executed in an Autotest run. Described in
// http://cs/chromeos_public/infra/proto/src/test_platform/skylab_test_runner/result.proto
// Fields not used by ResultSink Test Results are omitted.
type TestRunnerResult struct {
	Autotest TestRunnerAutotest `json:"autotest_result"`
}

type TestRunnerAutotest struct {
	TestCases []TestRunnerTestCase `json:"test_cases"`
}

type TestRunnerTestCase struct {
	Name                 string    `json:"name"`
	Verdict              string    `json:"verdict"`
	HumanReadableSummary string    `json:"human_readable_summary"`
	StartTime            time.Time `json:"start_time"`
	EndTime              time.Time `json:"end_time"`
}

// ConvertFromJSON reads the provided reader into the receiver.
//
// The receiver is cleared and its fields overwritten.
func (r *TestRunnerResult) ConvertFromJSON(reader io.Reader) error {
	*r = TestRunnerResult{}
	if err := json.NewDecoder(reader).Decode(r); err != nil {
		return err
	}
	return nil
}

// ToProtos converts test results in r to []*sinkpb.TestResult.
func (r *TestRunnerResult) ToProtos(ctx context.Context, testMetadataFile string) ([]*sinkpb.TestResult, error) {
	metadata := map[string]*api.TestCaseMetadata{}
	var err error
	if testMetadataFile != "" {
		metadata, err = parseMetadata(testMetadataFile)
		if err != nil {
			return nil, err
		}
	}

	var ret []*sinkpb.TestResult
	for i, c := range r.Autotest.TestCases {
		status := genTestCaseStatus(c)
		tr := &sinkpb.TestResult{
			TestId: c.Name,
			// The status is expected if the test passed or was skipped. The
			// expected skipped will be translated to TEST_NA in Testhaus.
			Expected:     status == pb.TestStatus_PASS || status == pb.TestStatus_SKIP,
			Status:       status,
			TestMetadata: &pb.TestMetadata{},
		}
		if c.HumanReadableSummary != "" {
			// Limits the maximum size of the summary html message with an offset for the additional html tags.
			summaryHtmlFormat := "<pre>%s</pre>"
			tr.SummaryHtml = fmt.Sprintf(summaryHtmlFormat, truncateString(html.EscapeString(c.HumanReadableSummary), maxSummaryHtmlBytes-len(summaryHtmlFormat)))
			errorMessage := truncateString(c.HumanReadableSummary, maxErrorMessageBytes)
			tr.FailureReason = &pb.FailureReason{
				PrimaryErrorMessage: errorMessage,
				Errors: []*pb.FailureReason_Error{
					{Message: errorMessage},
				},
			}
		}

		if !c.StartTime.IsZero() {
			tr.StartTime = timestamppb.New(c.StartTime)
			if !c.EndTime.IsZero() {
				tr.Duration = msToDuration(float64(c.EndTime.Sub(c.StartTime).Milliseconds()))
			}
		}

		// Add Tags to test results.
		tr.Tags = append(tr.Tags, pbutil.StringPair(executionOrderTag,
			strconv.Itoa(i+1)))

		testMetadata, ok := metadata[c.Name]
		if ok {
			tr.Tags = append(tr.Tags, metadataToTags(ctx, testMetadata)...)
			props, err := structpb.NewStruct(tagsToMap(tr.Tags))
			if err != nil {
				logging.Warningf(
					ctx,
					"could not set metadata properties from %v due to %v",
					testMetadata, err)
			} else {
				tr.TestMetadata.Properties = props
				tr.TestMetadata.PropertiesSchema = metadataSchema
			}
			tr.TestMetadata.BugComponent, err = parseBugComponentMetadata(testMetadata)
			if err != nil {
				logging.Errorf(
					ctx,
					"could not parse bug component metadata from: %v due to: %v",
					testMetadata,
					err)
			}
		}

		ret = append(ret, tr)
	}
	return ret, nil
}

// Converts a TestCase Verdict into a ResultSink Status.
func genTestCaseStatus(c TestRunnerTestCase) pb.TestStatus {
	if c.Verdict == "VERDICT_PASS" {
		return pb.TestStatus_PASS
	} else if c.Verdict == "VERDICT_NO_VERDICT" {
		return pb.TestStatus_SKIP
	} else if c.Verdict == "VERDICT_ERROR" {
		return pb.TestStatus_CRASH
	} else if c.Verdict == "VERDICT_ABORT" {
		return pb.TestStatus_ABORT
	}
	return pb.TestStatus_FAIL
}
