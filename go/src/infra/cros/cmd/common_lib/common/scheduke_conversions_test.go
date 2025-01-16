// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	schedukepb "go.chromium.org/chromiumos/config/go/test/scheduling"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
)

var testSchedukePriorityData = []struct {
	build        *buildbucketpb.Build
	wantPriority int64
}{
	{
		build: &buildbucketpb.Build{
			Tags: []*buildbucketpb.StringPair{
				{Key: "qs_account", Value: "release_low_prio"},
			}},
		wantPriority: 3,
	},
	{
		build:        &buildbucketpb.Build{},
		wantPriority: 10,
	},
}

func TestSchedukePriority(t *testing.T) {
	t.Parallel()
	for _, tt := range testSchedukePriorityData {
		tt := tt
		t.Run(fmt.Sprintf("(%s)", tt.build.GetTags()), func(t *testing.T) {
			t.Parallel()
			gotPriority := trPriority(tt.build.GetTags())
			if gotPriority != tt.wantPriority {
				t.Errorf("got %d, want %d", gotPriority, tt.wantPriority)
			}
		})
	}
}

var testQuotaAccountData = []struct {
	build       *buildbucketpb.Build
	wantAccount string
}{
	{
		build: &buildbucketpb.Build{
			Tags: []*buildbucketpb.StringPair{
				{Key: "foo", Value: "bar"},
				{Key: "qs_account", Value: "the account"},
			}},
		wantAccount: "the account",
	},
	{
		build: &buildbucketpb.Build{
			Tags: []*buildbucketpb.StringPair{
				// Misspelled key; shouldn't return a value.
				{Key: "qs-account-with-typo", Value: "the account"},
			}},
		wantAccount: "",
	},
	{
		build:       &buildbucketpb.Build{},
		wantAccount: "",
	},
}

func TestQuotaAccount(t *testing.T) {
	t.Parallel()
	for _, tt := range testQuotaAccountData {
		tt := tt
		t.Run(fmt.Sprintf("(%s)", tt.build.GetTags()), func(t *testing.T) {
			t.Parallel()
			gotAccount := trQSAccount(tt.build.GetTags())
			if gotAccount != tt.wantAccount {
				t.Errorf("got %v, want %v", gotAccount, tt.wantAccount)
			}
		})
	}
}

var testPeriodicData = []struct {
	build           *buildbucketpb.Build
	wantPeriodicity bool
}{
	{
		build:           &buildbucketpb.Build{},
		wantPeriodicity: false,
	},
	{
		build: &buildbucketpb.Build{
			Tags: []*buildbucketpb.StringPair{
				{Key: "foo", Value: "bar"},
			},
		},
		wantPeriodicity: false,
	},
	{
		build: &buildbucketpb.Build{
			Tags: []*buildbucketpb.StringPair{
				{Key: "foo", Value: "bar"},
				{Key: "analytics_name", Value: "baz"},
			},
		},
		wantPeriodicity: true,
	},
}

func TestPeriodic(t *testing.T) {
	t.Parallel()
	for _, tt := range testPeriodicData {
		tt := tt
		t.Run(fmt.Sprintf("(%s)", tt.build.GetTags()), func(t *testing.T) {
			t.Parallel()
			gotPeriodicity := trBuildIsPeriodic(tt.build.GetTags())
			if gotPeriodicity != tt.wantPeriodicity {
				t.Errorf("got %v, want %v", gotPeriodicity, tt.wantPeriodicity)
			}
		})
	}
}

func TestTimeFromTimestampPBString(t *testing.T) {
	t.Parallel()
	timestampPBString := "2023-01-23T14:23:05.808538180Z"
	wantTime := time.Date(2023, 1, 23, 14, 23, 5, 808538180, time.UTC)
	gotTime, err := timeFromTimestampPBString(timestampPBString)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if gotTime != wantTime {
		t.Errorf("got %v, want %v", gotTime, wantTime)
	}
}

var testAsapData = []struct {
	qsAccount string
	periodic  bool
	wantAsap  bool
}{
	{
		qsAccount: "pcq",
		periodic:  false,
		wantAsap:  true,
	},
	{
		qsAccount: "pcq",
		periodic:  true,
		wantAsap:  false,
	},
	{
		qsAccount: "not pcq",
		periodic:  true,
		wantAsap:  false,
	},
	{
		qsAccount: "not pcq",
		periodic:  false,
		wantAsap:  false,
	},
}

func TestAsap(t *testing.T) {
	t.Parallel()
	for _, tt := range testAsapData {
		tt := tt
		t.Run(fmt.Sprintf("(%s/%v)", tt.qsAccount, tt.periodic), func(t *testing.T) {
			t.Parallel()
			gotAsap := asap(tt.qsAccount, tt.periodic)
			if gotAsap != tt.wantAsap {
				t.Errorf("got %v, want %v", gotAsap, tt.wantAsap)
			}
		})
	}
}

var testDimensionsDeviceNameAndPoolData = []struct {
	bbDims           []*buildbucketpb.RequestedDimension
	wantSchedukeDims *schedukepb.SwarmingDimensions
	wantDeviceName   string
	wantPool         string
}{
	{
		bbDims: []*buildbucketpb.RequestedDimension{},
		wantSchedukeDims: &schedukepb.SwarmingDimensions{
			DimsMap: map[string]*schedukepb.DimValues{},
		},
		wantDeviceName: "",
		wantPool:       "",
	},
	{
		bbDims: []*buildbucketpb.RequestedDimension{
			{
				Key:   "foo",
				Value: "val",
			},
			{
				Key:   "bar",
				Value: "val1|val2",
			},
		},
		wantSchedukeDims: &schedukepb.SwarmingDimensions{
			DimsMap: map[string]*schedukepb.DimValues{
				"foo": {Values: []string{"val"}},
				"bar": {Values: []string{"val1", "val2"}},
			},
		},
		wantDeviceName: "",
		wantPool:       "",
	},
	{
		bbDims: []*buildbucketpb.RequestedDimension{
			{
				Key:   "foo",
				Value: "val",
			},
			{
				Key:   "bar",
				Value: "val1|val2",
			},
			{
				Key:   "bar",
				Value: "val3",
			},
			{
				Key:   "bar",
				Value: "val4|val5",
			},
			{
				Key:   "label-pool",
				Value: "pool1|pool2",
			},
			{
				Key:   "dut_name",
				Value: "name1|name2",
			},
		},
		wantSchedukeDims: &schedukepb.SwarmingDimensions{
			DimsMap: map[string]*schedukepb.DimValues{
				"foo":        {Values: []string{"val"}},
				"bar":        {Values: []string{"val1", "val2", "val3", "val4", "val5"}},
				"label-pool": {Values: []string{"pool1", "pool2"}},
				"dut_name":   {Values: []string{"name1", "name2"}},
			},
		},
		wantDeviceName: "name1|name2",
		wantPool:       "pool1|pool2",
	},
	{
		bbDims: []*buildbucketpb.RequestedDimension{
			{
				Key:   "foo",
				Value: "val",
			},
			{
				Key:   "bar",
				Value: "val1|val2",
			},
			{
				Key:   "label-pool",
				Value: "baz pool",
			},
			{
				Key:   "dut_name",
				Value: "haha name",
			},
		},
		wantSchedukeDims: &schedukepb.SwarmingDimensions{
			DimsMap: map[string]*schedukepb.DimValues{
				"foo":        {Values: []string{"val"}},
				"bar":        {Values: []string{"val1", "val2"}},
				"label-pool": {Values: []string{"baz pool"}},
				"dut_name":   {Values: []string{"haha name"}},
			},
		},
		wantDeviceName: "haha name",
		wantPool:       "baz pool",
	},
	{
		bbDims: []*buildbucketpb.RequestedDimension{
			{
				Key:   "foo",
				Value: "val",
			},
			{
				Key:   "bar",
				Value: "val1|val2",
			},
			{
				Key:   "label-pool",
				Value: "schedukeTest",
			},
			{
				Key:   "dut_name",
				Value: "lol name",
			},
		},
		wantSchedukeDims: &schedukepb.SwarmingDimensions{
			DimsMap: map[string]*schedukepb.DimValues{
				"foo":        {Values: []string{"val"}},
				"bar":        {Values: []string{"val1", "val2"}},
				"label-pool": {Values: []string{"schedukeTest"}},
				"dut_name":   {Values: []string{"lol name"}},
			},
		},
		wantDeviceName: "lol name",
		wantPool:       "schedukeTest",
	},
}

func TestDimensionsDeviceNameAndPool(t *testing.T) {
	t.Parallel()
	cmpOpts := cmpopts.IgnoreUnexported(
		schedukepb.DimValues{},
		schedukepb.SwarmingDimensions{})
	for _, tt := range testDimensionsDeviceNameAndPoolData {
		tt := tt
		t.Run(fmt.Sprintf("(%s)", tt.bbDims), func(t *testing.T) {
			t.Parallel()
			gotSchedukeDims, gotDeviceName, gotPool := dimensionsDeviceNameAndPool(tt.bbDims)
			if diff := cmp.Diff(gotSchedukeDims, tt.wantSchedukeDims, cmpOpts); diff != "" {
				t.Errorf("unexpected diff (%s)", diff)
			}
			if gotDeviceName != tt.wantDeviceName {
				t.Errorf("got %v, want %v", gotDeviceName, tt.wantDeviceName)
			}
			if gotPool != tt.wantPool {
				t.Errorf("got %v, want %v", gotPool, tt.wantPool)
			}
		})
	}
}

var testCompressAndEncodeBBReqData = []struct {
	src                           []byte
	wantCompressedEncodedBuildStr string
}{
	{
		src: []byte(`{
	RequestId: "foo",
	Priority:  20
}`),
		wantCompressedEncodedBuildStr: "eJyq5uIMSi0sTS0u8UyxUlBKy89X0uHiDCjKzC/KLKm0UlAwMuCqBQQAAP//2HYLCw==",
	},
	{
		src: []byte(`{
	RequestId: "bar",
	Priority:  20
}`),
		wantCompressedEncodedBuildStr: "eJyq5uIMSi0sTS0u8UyxUlBKSixS0uHiDCjKzC/KLKm0UlAwMuCqBQQAAP//1zQK/A==",
	},
}

func TestCompressAndEncodeBBReq(t *testing.T) {
	t.Parallel()
	for _, tt := range testCompressAndEncodeBBReqData {
		tt := tt
		t.Run(fmt.Sprintf("(%s)", tt.src), func(t *testing.T) {
			t.Parallel()
			gotBuildStr, err := compressAndEncodeBBReq(tt.src)
			if err != nil {
				t.Fatalf("error found: %s", err)
			}
			if diff := cmp.Diff(gotBuildStr, tt.wantCompressedEncodedBuildStr); diff != "" {
				t.Errorf("unexpected diff (%s)", diff)
			}
		})
	}
}

var testResolvePoolsData = []struct {
	req, wantResolvedReq *schedukepb.TaskRequestEvent
}{
	{
		req:             &schedukepb.TaskRequestEvent{},
		wantResolvedReq: &schedukepb.TaskRequestEvent{},
	},
	{
		req: &schedukepb.TaskRequestEvent{
			Pool: "foo pool",
			RequestedDimensions: &schedukepb.SwarmingDimensions{
				DimsMap: map[string]*schedukepb.DimValues{
					"label-pool": {Values: []string{"bar pool", "baz pool"}},
					"label-foo":  {Values: []string{"MANAGED_POOL_QUOTA", "quota"}},
				},
			},
		},
		wantResolvedReq: &schedukepb.TaskRequestEvent{
			Pool: "foo pool",
			RequestedDimensions: &schedukepb.SwarmingDimensions{
				DimsMap: map[string]*schedukepb.DimValues{
					"label-pool": {Values: []string{"bar pool", "baz pool"}},
					"label-foo":  {Values: []string{"MANAGED_POOL_QUOTA", "quota"}},
				},
			},
		},
	},
	{
		req: &schedukepb.TaskRequestEvent{
			Pool: "MANAGED_POOL_QUOTA",
			RequestedDimensions: &schedukepb.SwarmingDimensions{
				DimsMap: map[string]*schedukepb.DimValues{
					"label-pool": {Values: []string{"bar pool", "baz pool"}},
				},
			},
		},
		wantResolvedReq: &schedukepb.TaskRequestEvent{
			Pool: "DUT_POOL_QUOTA",
			RequestedDimensions: &schedukepb.SwarmingDimensions{
				DimsMap: map[string]*schedukepb.DimValues{
					"label-pool": {Values: []string{"bar pool", "baz pool"}},
				},
			},
		},
	},
	{
		req: &schedukepb.TaskRequestEvent{
			Pool: "foo pool",
			RequestedDimensions: &schedukepb.SwarmingDimensions{
				DimsMap: map[string]*schedukepb.DimValues{
					"label-pool": {Values: []string{"MANAGED_POOL_QUOTA", "bar pool"}},
				},
			},
		},
		wantResolvedReq: &schedukepb.TaskRequestEvent{
			Pool: "foo pool",
			RequestedDimensions: &schedukepb.SwarmingDimensions{
				DimsMap: map[string]*schedukepb.DimValues{
					"label-pool": {Values: []string{"DUT_POOL_QUOTA", "bar pool"}},
				},
			},
		},
	},
	{
		req: &schedukepb.TaskRequestEvent{
			Pool: "quota",
			RequestedDimensions: &schedukepb.SwarmingDimensions{
				DimsMap: map[string]*schedukepb.DimValues{
					"label-pool": {Values: []string{"MANAGED_POOL_QUOTA", "quota"}},
				},
			},
		},
		wantResolvedReq: &schedukepb.TaskRequestEvent{
			Pool: "DUT_POOL_QUOTA",
			RequestedDimensions: &schedukepb.SwarmingDimensions{
				DimsMap: map[string]*schedukepb.DimValues{
					"label-pool": {Values: []string{"DUT_POOL_QUOTA", "DUT_POOL_QUOTA"}},
				},
			},
		},
	},
}

func TestRespolvePoolName(t *testing.T) {
	t.Parallel()
	cmpOpts := cmpopts.IgnoreUnexported(
		schedukepb.DimValues{},
		schedukepb.SwarmingDimensions{},
		schedukepb.TaskRequestEvent{})
	for _, tt := range testResolvePoolsData {
		tt := tt
		t.Run(fmt.Sprintf("(%s)", tt.req), func(t *testing.T) {
			t.Parallel()
			resolvePool(tt.req)
			if diff := cmp.Diff(tt.req, tt.wantResolvedReq, cmpOpts); diff != "" {
				t.Errorf("unexpected diff (%s)", diff)
			}
		})
	}
}

func TestTrOSType(t *testing.T) {
	// cros
	tags := []*buildbucketpb.StringPair{
		{Key: "os_type", Value: "chromeos"},
		{Key: "analytics_name", Value: "baz"},
	}
	osType := trOSType(tags)
	if osType != schedukepb.OsType_CHROMEOS {
		t.Errorf("got %v, want %v", osType, schedukepb.OsType_CHROMEOS)
	}
	// al
	tags = []*buildbucketpb.StringPair{
		{Key: "os_type", Value: "al"},
		{Key: "analytics_name", Value: "baz"},
	}
	osType = trOSType(tags)
	if osType != schedukepb.OsType_ANDROID {
		t.Errorf("got %v, want %v", osType, schedukepb.OsType_ANDROID)
	}
	// cros
	tags = []*buildbucketpb.StringPair{
		{Key: "analytics_name", Value: "baz"},
	}
	osType = trOSType(tags)
	if osType != schedukepb.OsType_CHROMEOS {
		t.Errorf("got %v, want %v", osType, schedukepb.OsType_CHROMEOS)
	}
}
