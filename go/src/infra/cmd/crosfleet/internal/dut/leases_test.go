// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"

	"go.chromium.org/infra/cmd/crosfleet/internal/buildbucket"
	"go.chromium.org/infra/cmd/crosfleet/internal/site"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

var testLeaseInfoAsBashVariablesData = []struct {
	info         *common.LeaseInfo
	wantBashVars string
}{
	{ // All variables found
		&common.LeaseInfo{
			Build: &buildbucketpb.Build{
				Id:     12345,
				Status: buildbucketpb.Status_SCHEDULED,
				Input: &buildbucketpb.Build_Input{
					Properties: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"lease_length_minutes": structpb.NewNumberValue(10.0),
						},
					},
				},
			},
			Device: &common.DeviceInfo{
				Name: "sample-hostname",
			},
		},
		`LEASE_TASK=https://ci.chromium.org/ui/p/chromeos/builders/test_runner/dut_leaser/b12345
STATUS=SCHEDULED
MINS_REMAINING=10
DUT_HOSTNAME=sample-hostname`,
	},
	{ // Only lease build variables found
		&common.LeaseInfo{
			Build: &buildbucketpb.Build{
				Id:     12345,
				Status: buildbucketpb.Status_SCHEDULED,
				Input: &buildbucketpb.Build_Input{
					Properties: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"lease_length_minutes": structpb.NewNumberValue(10.0),
						},
					},
				},
			},
		},
		`LEASE_TASK=https://ci.chromium.org/ui/p/chromeos/builders/test_runner/dut_leaser/b12345
STATUS=SCHEDULED
MINS_REMAINING=10`,
	},
	{ // Only DUT variables found
		&common.LeaseInfo{
			Device: &common.DeviceInfo{
				Name: "sample-hostname",
			},
		},
		"DUT_HOSTNAME=sample-hostname",
	},
	{ // No variables found
		&common.LeaseInfo{},
		"",
	},
}

func TestLeaseInfoAsBashVariables(t *testing.T) {
	t.Parallel()
	for _, tt := range testLeaseInfoAsBashVariablesData {
		tt := tt
		fakeLeaseBBClient := buildbucket.NewClientForTesting(site.Prod.DUTLeaserBuilder)
		t.Run(fmt.Sprintf("(%s)", tt.wantBashVars), func(t *testing.T) {
			t.Parallel()
			gotBashVars := leaseInfoAsBashVariables(tt.info, fakeLeaseBBClient)
			if diff := cmp.Diff(tt.wantBashVars, gotBashVars); diff != "" {
				t.Errorf("unexpected diff (%s)", diff)
			}
		})
	}
}

var testGetRemainingMinsData = []struct {
	build             *buildbucketpb.Build
	wantRemainingMins int64
}{
	{ // Scheduled build
		&buildbucketpb.Build{
			Status:    buildbucketpb.Status_SCHEDULED,
			StartTime: timestamppb.New(time.Now().Add(-3 * time.Minute)),
			Input: &buildbucketpb.Build_Input{
				Properties: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"lease_length_minutes": structpb.NewNumberValue(10.5),
					},
				},
			},
		},
		10,
	},
	{ // Started build
		&buildbucketpb.Build{
			Status:    buildbucketpb.Status_STARTED,
			StartTime: timestamppb.New(time.Now().Add(-3 * time.Minute)),
			Input: &buildbucketpb.Build_Input{
				Properties: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"lease_length_minutes": structpb.NewNumberValue(10.5),
					},
				},
			},
		},
		7,
	},
	{ // Finished build
		&buildbucketpb.Build{
			Status:    buildbucketpb.Status_ENDED_MASK,
			StartTime: timestamppb.New(time.Now().Add(-3 * time.Minute)),
			Input: &buildbucketpb.Build_Input{
				Properties: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"lease_length_minutes": structpb.NewNumberValue(10.5),
					},
				},
			},
		},
		0,
	},
}

func TestGetRemainingMins(t *testing.T) {
	t.Parallel()
	for _, tt := range testGetRemainingMinsData {
		tt := tt
		t.Run(fmt.Sprintf("(%d)", tt.wantRemainingMins), func(t *testing.T) {
			t.Parallel()
			gotRemainingMins := getRemainingMins(tt.build)
			if gotRemainingMins != tt.wantRemainingMins {
				t.Errorf("unexpected error: wanted %d, got %d", tt.wantRemainingMins, gotRemainingMins)
			}
		})
	}
}
