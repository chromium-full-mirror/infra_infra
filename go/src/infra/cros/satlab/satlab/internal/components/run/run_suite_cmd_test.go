// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package run

import (
	"testing"
)

func TestValidateArgs(t *testing.T) {
	t.Parallel()

	type test struct {
		inputCommand *run
	}
	tests := []test{
		{
			&run{ // no test no suite
				runFlags: runFlags{
					board:     "zork",
					model:     "gumboz",
					milestone: "111",
					build:     "15329.6.0",
					satlabId:  "satlab-0wgatfqi21118003",
					pool:      "pool"},
			},
		},
		{
			&run{ // test and suite
				runFlags: runFlags{
					test:      "rlz_CheckPing.should_send_rlz_ping_missing",
					suite:     "rlz",
					harness:   "tauto",
					board:     "zork",
					model:     "gumboz",
					milestone: "111",
					build:     "15329.6.0",
					pool:      "pool"},
			},
		},
		{
			&run{ // test and testplan
				runFlags: runFlags{
					test:      "rlz_CheckPing.should_send_rlz_ping_missing",
					testplan:  "testplan.json",
					harness:   "tauto",
					board:     "zork",
					model:     "gumboz",
					milestone: "111",
					build:     "15329.6.0",
					pool:      "pool"},
			},
		},
		{
			&run{ // 'cft' test without harness
				runFlags: runFlags{
					test:      "rlz_CheckPing.should_send_rlz_ping_missing",
					board:     "zork",
					model:     "gumboz",
					milestone: "111",
					build:     "15329.6.0",
					satlabId:  "satlab-0wgatfqi21118003",
					pool:      "pool",
					cft:       true},
			},
		},
		{
			&run{ // 'non-cft' test with TRv2
				runFlags: runFlags{
					test:      "rlz_CheckPing.should_send_rlz_ping_missing",
					board:     "zork",
					model:     "gumboz",
					milestone: "111",
					build:     "15329.6.0",
					satlabId:  "satlab-0wgatfqi21118003",
					pool:      "pool",
					harness:   "tast",
					trv2:      true,
					cft:       false},
			},
		},
		{
			&run{ // 'non-cft' test with DynamicTRv2
				runFlags: runFlags{
					test:        "rlz_CheckPing.should_send_rlz_ping_missing",
					board:       "zork",
					model:       "gumboz",
					milestone:   "111",
					build:       "15329.6.0",
					satlabId:    "satlab-0wgatfqi21118003",
					pool:        "pool",
					harness:     "tast",
					dynamicTrv2: true,
					cft:         false},
			},
		},
		{
			&run{ // tagIncludes not with -suite
				runFlags: runFlags{
					test:        "rlz_CheckPing.should_send_rlz_ping_missing",
					board:       "zork",
					model:       "gumboz",
					milestone:   "111",
					build:       "15329.6.0",
					satlabId:    "satlab-0wgatfqi21118003",
					pool:        "pool",
					harness:     "tast",
					dynamicTrv2: true,
					tagIncludes: []string{"group:pasit"}},
			},
		},
		{
			&run{ // tagExcludes not with suite
				runFlags: runFlags{
					test:        "rlz_CheckPing.should_send_rlz_ping_missing",
					board:       "zork",
					model:       "gumboz",
					milestone:   "111",
					build:       "15329.6.0",
					satlabId:    "satlab-0wgatfqi21118003",
					pool:        "pool",
					harness:     "tast",
					dynamicTrv2: true,
					tagExcludes: []string{"group:pasit"}},
			},
		},
		{
			&run{ // testNameIncludes not with suite
				runFlags: runFlags{
					test:             "rlz_CheckPing.should_send_rlz_ping_missing",
					board:            "zork",
					model:            "gumboz",
					milestone:        "111",
					build:            "15329.6.0",
					satlabId:         "satlab-0wgatfqi21118003",
					pool:             "pool",
					harness:          "tast",
					dynamicTrv2:      true,
					testNameIncludes: []string{"tast.example.Pass"}},
			},
		},
		{
			&run{ // testNameExcludes not with suite
				runFlags: runFlags{
					test:             "rlz_CheckPing.should_send_rlz_ping_missing",
					board:            "zork",
					model:            "gumboz",
					milestone:        "111",
					build:            "15329.6.0",
					satlabId:         "satlab-0wgatfqi21118003",
					pool:             "pool",
					harness:          "tast",
					dynamicTrv2:      true,
					testNameExcludes: []string{"tast.example.Pass"}},
			},
		},
		{
			&run{ // no board
				runFlags: runFlags{
					suite:     "rlz",
					model:     "gumboz",
					milestone: "111",
					build:     "15329.6.0",
					satlabId:  "satlab-0wgatfqi21118003",
					pool:      "pool"},
			},
		},
		{
			&run{ // no pool
				runFlags: runFlags{
					suite:     "rlz",
					board:     "zork",
					model:     "gumboz",
					milestone: "111",
					build:     "15329.6.0",
					satlabId:  "satlab-0wgatfqi21118003"},
			},
		},
		{
			&run{ // drone passed as dim
				runFlags: runFlags{
					suite:     "rlz",
					board:     "zork",
					model:     "gumboz",
					milestone: "111",
					build:     "15329.6.0",
					satlabId:  "satlab-0wgatfqi21118003",
					pool:      "pool",
					addedDims: map[string]string{"drone": "not allowed"},
				},
			},
		},
	}

	for _, tc := range tests {
		err := tc.inputCommand.validateArgs()
		if err == nil {
			t.Errorf("Expected command to error")
		}
	}
}
