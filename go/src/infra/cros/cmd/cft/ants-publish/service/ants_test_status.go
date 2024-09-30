// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package service provides the API handlers for ants publish.
package service

import (
	"go.chromium.org/chromiumos/config/go/test/api"
)

// Possible values for the TestResult.TestStatus field.
type TestStatus int

const (
	UNKNOWN TestStatus = iota
	PASS
	FAIL
	IGNORED
	ASSUMPTION_FAILURE
	ERROR
	SKIPPED
)

func (ts TestStatus) String() string {
	return [...]string{
		"unknown",
		"pass",
		"fail",
		"ignored",
		"assumptionFailure",
		"testError",
		"testSkipped",
	}[ts-1]
}

func (ts TestStatus) EnumIndex() int {
	return int(ts)
}

func antsTestStatus(result *api.TestCaseResult) string {
	var status TestStatus
	if s := result.GetPass(); s != nil {
		status = PASS
	} else if s := result.GetFail(); s != nil {
		status = FAIL
	} else if s := result.GetAbort(); s != nil {
		status = ERROR
	} else if s := result.GetCrash(); s != nil {
		status = ERROR
	} else if s := result.GetNotRun(); s != nil {
		status = IGNORED
	} else if s := result.GetSkip(); s != nil {
		status = SKIPPED
	}

	return status.String()
}
