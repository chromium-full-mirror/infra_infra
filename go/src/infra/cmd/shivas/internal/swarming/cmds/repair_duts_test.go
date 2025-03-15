// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tasks

import (
	"testing"

	"go.chromium.org/infra/libs/skylab/buildbucket"
)

func TestGetBuilderAndTaskName(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name                string
		cmd                 *repairDuts
		expectedBuilderName string
		expectedTaskName    string
	}{
		{
			"normal repair job",
			&repairDuts{
				bbBuilder: "repair",
			},
			"repair",
			string(buildbucket.Recovery),
		},
		{
			"verify job",
			&repairDuts{
				bbBuilder:  "repair",
				onlyVerify: true,
			},
			"verify",
			string(buildbucket.Recovery),
		},
		{
			"deep repair job",
			&repairDuts{
				bbBuilder:  "repair",
				deepRepair: true,
			},
			"repair",
			string(buildbucket.DeepRecovery),
		},
		{
			"Verify with deep-repair enabled",
			&repairDuts{
				bbBuilder:  "repair",
				onlyVerify: true,
				deepRepair: true,
			},
			"verify",
			string(buildbucket.DeepRecovery),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualBuilderName, actualTaskName := tc.cmd.getBuilderAndTaskName()
			if actualBuilderName != tc.expectedBuilderName {
				t.Errorf("unexpected buildername %s (expected %s) for cmd %v", actualBuilderName, tc.expectedBuilderName, tc.cmd)
			}

			if actualTaskName != tc.expectedTaskName {
				t.Errorf("unexpected taskname %s (expected %s) for cmd %v", actualTaskName, tc.expectedTaskName, tc.cmd)
			}
		})
	}
}
