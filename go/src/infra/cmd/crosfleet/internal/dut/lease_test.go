// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"flag"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/infra/cmd/crosfleet/internal/common"
)

var testValidateData = []struct {
	leaseFlags
	wantValidationErrString string
}{
	{ // All flags raise errors
		leaseFlags{
			durationMins: 0,
			reason:       "this desc is barely too long!!!",
			host:         "",
			model:        "",
			board:        "",
		},
		`must specify DUT dimensions (-board/-model/-dim(s)) or DUT hostname (-host), but not both
duration should be greater than 0
reason cannot exceed 30 characters`,
	},
	{ // Some flags raise errors
		leaseFlags{
			durationMins: 1441,
			reason:       "this desc is just short enough",
			host:         "sample-host",
			model:        "sample-model",
			board:        "sample-board",
		},
		`must specify DUT dimensions (-board/-model/-dim(s)) or DUT hostname (-host), but not both
duration cannot exceed 1440 minutes (24 hours)`,
	},
	{ // No flags raise errors
		leaseFlags{
			durationMins: 1440,
			reason:       "this desc is just short enough",
			host:         "",
			model:        "sample-model",
			board:        "sample-board",
			freeformDims: map[string]string{"foo": "bar"},
		},
		"",
	},
}

func TestValidate(t *testing.T) {
	t.Parallel()
	for _, tt := range testValidateData {
		t.Run(fmt.Sprintf("(%s)", tt.wantValidationErrString), func(t *testing.T) {
			t.Parallel()
			gotValidationErr := tt.leaseFlags.validate(&flag.FlagSet{})
			gotValidationErrString := common.ErrToString(gotValidationErr)
			if tt.wantValidationErrString != gotValidationErrString {
				t.Errorf("unexpected error: wanted %s, got %s", tt.wantValidationErrString, gotValidationErrString)
			}
		})
	}
}

// We avoid testing this function for a host-based lease since we'd have to
// fake a Swarming API call.
var testBotDimsAndBuildTagsData = []struct {
	leaseFlags
	wantDims, wantTags map[string]string
}{
	{ // Model-based lease with added dims
		leaseFlags{
			model:        "sample-model",
			reason:       "sample reason",
			freeformDims: map[string]string{"added-key": "added-val"},
		},
		map[string]string{
			"added-key":   "added-val",
			"dut_state":   "ready",
			"label-model": "sample-model",
		},
		map[string]string{
			"added-key":      "added-val",
			"crosfleet-tool": "lease",
			"lease-reason":   "sample reason",
			"label-model":    "sample-model",
			"qs_account":     "leases",
		},
	},
	{ // Board-based lease without added dims
		leaseFlags{
			board:        "sample-board",
			reason:       "sample reason",
			freeformDims: nil,
		},
		map[string]string{
			"dut_state":   "ready",
			"label-board": "sample-board",
		},
		map[string]string{
			"crosfleet-tool": "lease",
			"lease-reason":   "sample reason",
			"label-board":    "sample-board",
			"qs_account":     "leases",
		},
	},
	{ // Freeform dims that override defaults
		leaseFlags{
			board:        "b1",
			model:        "m1",
			pool:         "p1",
			reason:       "sample reason 2",
			freeformDims: map[string]string{"label-board": "b2", "label-pool": "p2"},
		},
		map[string]string{
			"dut_state":   "ready",
			"label-board": "b2",
			"label-model": "m1",
			"label-pool":  "p2",
		},
		map[string]string{
			"crosfleet-tool": "lease",
			"lease-reason":   "sample reason 2",
			"label-board":    "b2",
			"label-model":    "m1",
			"label-pool":     "p2",
			"qs_account":     "leases",
		},
	},
	{ // Hostname-based lease
		leaseFlags{
			board:        "b1",
			model:        "m1",
			pool:         "p1",
			host:         "sample hostname",
			reason:       "sample reason 3",
			freeformDims: map[string]string{"label-pool": "p2"},
			dutID:        "C12345",
		},
		map[string]string{
			"dut_id":     "C12345",
			"label-pool": "p2",
		},
		map[string]string{
			"crosfleet-tool": "lease",
			"lease-reason":   "sample reason 3",
			"dut_name":       "sample hostname",
			"label-pool":     "p2",
			"qs_account":     "leases",
			"lease-by":       "dut_id",
		},
	},
}

func TestBotDimsAndBuildTagsData(t *testing.T) {
	t.Parallel()
	for _, tt := range testBotDimsAndBuildTagsData {
		t.Run(fmt.Sprintf("(%s, %s)", tt.wantDims, tt.wantTags), func(t *testing.T) {
			gotDims, gotTags, err := botDimsAndBuildTags(tt.leaseFlags)
			if err != nil {
				t.Fatalf("unexpected error calling botDimsAndBuildTags: %v", err)
			}
			if dimDiff := cmp.Diff(tt.wantDims, gotDims); dimDiff != "" {
				t.Errorf("unexpected bot dimension diff (%s)", dimDiff)
			}
			if tagDiff := cmp.Diff(tt.wantTags, gotTags); tagDiff != "" {
				t.Errorf("unexpected build tag diff (%s)", tagDiff)
			}
		})
	}
}

var testLeaseStartStepName = []struct {
	durationMins int64
	wantStepName string
}{
	{
		59,
		"lease DUT for 0 hr 59 min",
	},
	{
		60,
		"lease DUT for 1 hr 0 min",
	},
	{
		61,
		"lease DUT for 1 hr 1 min",
	},
}

func TestLeaseStartStepName(t *testing.T) {
	t.Parallel()
	for _, tt := range testLeaseStartStepName {
		t.Run(fmt.Sprintf("(%s)", tt.wantStepName), func(t *testing.T) {
			t.Parallel()
			leaseRun := &leaseRun{}
			leaseRun.durationMins = tt.durationMins
			gotStepName := leaseRun.leaseStartStepName()
			if diff := cmp.Diff(tt.wantStepName, gotStepName); diff != "" {
				t.Errorf("unexpected diff (%s)", diff)
			}
		})
	}
}
