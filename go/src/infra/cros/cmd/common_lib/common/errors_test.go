// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"errors"
	"fmt"
	"testing"

	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	lucierrs "go.chromium.org/luci/common/errors"
)

func TestIsTestRunnerError(t *testing.T) {
	original := errors.New("foo error")
	tre := &TestRunnerError{
		Type: skylab_test_runner.TestRunnerErrorType_DUT_CONNECTION,
		Err:  original,
	}

	if !errors.Is(tre.Unwrap(), original) {
		t.Errorf("TestRunnerError [%v] won't unwrap correctly (got [%v], want [%v])", tre, tre.Unwrap(), original)
	}

	wrapped := fmt.Errorf("wrapped again: %w", lucierrs.Annotate(lucierrs.Append(errors.New("bar error"), fmt.Errorf("wrapped error: %w", tre)), "annotated err").Err())
	var unwrappedTRE *TestRunnerError
	if ok := lucierrs.As(wrapped, &unwrappedTRE); !ok {
		t.Errorf("wrapped error [%v] not unwrapping to TestRunnerError", wrapped)
	}
	if unwrappedTRE != tre {
		t.Errorf("wrapped TestRunnerError [%v] won't unwrap correctly (got [%v], want [%v])", wrapped, unwrappedTRE, tre)
	}

	treNoErr := &TestRunnerError{
		Type: skylab_test_runner.TestRunnerErrorType_DUT_CONNECTION,
	}
	wantErrString := "DUT_CONNECTION"
	if treNoErr.Error() != wantErrString {
		t.Errorf("TestRunnerError with no err populated didn't return the corect default error (got [%v], want [%v])", treNoErr.Error(), wantErrString)
	}
}
