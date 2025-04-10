// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"fmt"
	"testing"

	"google.golang.org/api/googleapi"
)

func TestIsTransientError_NoWrapping(t *testing.T) {
	e := &googleapi.Error{Code: 429, Message: "rateLimitExceeded"}
	if !isTransientError(e) {
		t.Errorf("should be transient error")
	}
}

func TestIsTransientError_NoWrapping_Invalid(t *testing.T) {
	e := &googleapi.Error{Code: 404, Message: "notFound"}
	if isTransientError(e) {
		t.Errorf("should not be transient error")
	}
}

func TestIsTransientError_Wrapped(t *testing.T) {
	oe := &googleapi.Error{Code: 429, Message: "rateLimitExceeded"}
	e := fmt.Errorf("failed to close: %w", oe)
	if !isTransientError(e) {
		t.Errorf("should be transient error")
	}
}

func TestIsTransientError_MultipleWrappers(t *testing.T) {
	oe := &googleapi.Error{Code: 429, Message: "rateLimitExceeded"}
	we := fmt.Errorf("failed to close: %w", oe)
	e := fmt.Errorf("cannot upload: %w", we)
	if !isTransientError(e) {
		t.Errorf("should be transient error")
	}
}
