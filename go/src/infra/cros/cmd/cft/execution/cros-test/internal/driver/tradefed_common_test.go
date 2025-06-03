// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package driver

import (
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
)

func TestExtractRetryConfig(t *testing.T) {

	metadata := &api.ExecutionMetadata{
		Args: []*api.Arg{
			{
				Flag:  "driverArg:retry-strategy",
				Value: "RETRY_ANY_FAILURE",
			},
			{
				Flag:  "driverArg:retry-isolation-grade",
				Value: "FULLY_ISOLATED",
			},
			{
				Flag:  "driverArg:retry-max-attempts",
				Value: "3",
			},
		},
	}
	if !extractRetryConfig(metadata) {
		t.Errorf("Retry config expected true")
	}
}

func TestGenerateDriverArgsCmds(t *testing.T) {
	metadata := &api.ExecutionMetadata{
		Args: []*api.Arg{
			{
				Flag:  "driverArg:retry-strategy",
				Value: "RETRY_ANY_FAILURE",
			},
			{
				Flag:  "driverArg:retry-isolation-grade",
				Value: "FULLY_ISOLATED",
			},
			{
				Flag:  "driverArg:retry-max-attempts",
				Value: "3",
			},
		},
	}
	cmd := generateDriverArgsCmds(metadata)
	if len(cmd) != 6 {
		t.Errorf("Retry config count incorrect: got %d, want 6", len(cmd))
	}
}
