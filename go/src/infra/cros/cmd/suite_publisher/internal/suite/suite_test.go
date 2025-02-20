// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package suite

import (
	"testing"

	"go.chromium.org/infra/cros/cmd/suite_publisher/test"
)

func TestNewSuite(t *testing.T) {
	s := NewSuite(test.ExampleSuite())
	if s == nil {
		t.Errorf("NewSuite() returned nil")
	}
	if s.ID() != "example_suite" {
		t.Errorf("NewSuite() Suite ID got: %q want %q", s.ID(), "example_suite")
	}
	if tests := s.Tests(); len(tests) == 0 {
		t.Errorf("s.Tests() expect non zero test length, got: %v", len(tests))
	}
	if metadata := s.Metadata(); metadata == nil {
		t.Errorf("s.Metadata() returned nil")
	}
}
