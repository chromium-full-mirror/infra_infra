// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"errors"

	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
)

var (
	GlobalNonInfraError error
)

// TestRunnerError implements the error interface for Test Runner errors.
type TestRunnerError struct {
	Type skylab_test_runner.TestRunnerErrorType
	Err  error
}

func (t *TestRunnerError) Error() string {
	if t.Err == nil {
		t.Err = errors.New(skylab_test_runner.TestRunnerErrorType_name[int32(t.Type)])
	}
	return t.Err.Error()
}

func (t *TestRunnerError) Unwrap() error {
	return t.Err
}
