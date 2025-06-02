// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"strings"

	pb "go.chromium.org/luci/resultdb/proto/v1"
	sinkpb "go.chromium.org/luci/resultdb/sink/proto/v1"
)

// SingleResult represents result format for a test suite with a single test.
type SingleResult struct {
	Failures []string `json:"failures"`
	Valid    bool     `json:"valid"`
}

// ConvertFromJSON reads the provided reader into the receiver.
//
// The receiver is cleared and its fields overwritten.
func (r *SingleResult) ConvertFromJSON(reader io.Reader) error {
	*r = SingleResult{}
	if err := json.NewDecoder(reader).Decode(r); err != nil {
		return err
	}

	return nil
}

// ToProtos converts test results in r to []*sinkpb.TestResult.
func (r *SingleResult) ToProtos(ctx context.Context) ([]*sinkpb.TestResult, error) {
	tr := &sinkpb.TestResult{
		// For a test suite with a single test, the suite itself is one test.
		TestId: "",
		TestIdStructured: &sinkpb.TestIdentifier{
			CaseNameComponents: []string{"*fixture"},
		},
	}

	switch {
	case !r.Valid:
		tr.StatusV2 = pb.TestResult_FAILED
		tr.FailureReason = &pb.FailureReason{
			Kind: pb.FailureReason_TIMEOUT,
		}
	case len(r.Failures) == 0:
		tr.StatusV2 = pb.TestResult_PASSED
	default:
		tr.StatusV2 = pb.TestResult_FAILED

		errs, truncated := truncateErrorsToResultDBLimits(toErrors(r.Failures))
		tr.FailureReason = &pb.FailureReason{
			Kind:                 pb.FailureReason_ORDINARY,
			Errors:               errs,
			TruncatedErrorsCount: int32(truncated),
		}

		tr.SummaryHtml = fmt.Sprintf("<pre>%s</pre>", html.EscapeString(strings.Join(r.Failures, "\n")))
	}

	return []*sinkpb.TestResult{tr}, nil
}

// toErrors converts failures to a ResultDB FailureReason_Errors collection.
func toErrors(failures []string) []*pb.FailureReason_Error {
	errors := make([]*pb.FailureReason_Error, 0, len(failures))
	for _, f := range failures {
		errors = append(errors, &pb.FailureReason_Error{
			Message: f,
		})
	}
	return errors
}
