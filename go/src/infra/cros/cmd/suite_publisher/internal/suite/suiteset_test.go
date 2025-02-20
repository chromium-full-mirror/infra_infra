// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package suite

import (
	"testing"

	"go.chromium.org/infra/cros/cmd/suite_publisher/test"
)

func TestNewSuiteSet(t *testing.T) {
	s := NewSuiteSet(test.ExampleSuiteSet())
	if s == nil {
		t.Errorf("NewSuiteSet() returned nil")
	}
	if s.ID() != "example_suite_set" {
		t.Errorf("NewSuiteSet() SuiteSet ID got: %q want %q", s.ID(), "example_suite")
	}
	if suiteSets := s.SuiteSets(); len(suiteSets) == 0 {
		t.Errorf("s.SuiteSets() expect non zero test length, got: %v", len(suiteSets))
	}
	if suites := s.Suites(); len(suites) == 0 {
		t.Errorf("s.Suites() expect non zero test length, got: %v", len(suites))
	}
	if metadata := s.Metadata(); metadata == nil {
		t.Errorf("s.Metadata() returned nil")
	}
}
