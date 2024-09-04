// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package run

import (
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	suschpb "go.chromium.org/chromiumos/infra/proto/go/testplans"

	"infra/cros/cmd/kron/common"
)

func TestIsBuildTooOldDaily(t *testing.T) {
	tooOldCreateTime := time.Now().Add(common.Day * (-2))
	buildCreateTime := timestamppb.New(tooOldCreateTime)
	tooOld := isBuildTooOld(buildCreateTime, suschpb.SchedulerConfig_LaunchCriteria_DAILY, 0)
	if !tooOld {
		t.Errorf("Expected %t, got %t", true, tooOld)
		return
	}

	validCreateTime := time.Now().Add(time.Hour * (-2))
	buildCreateTime = timestamppb.New(validCreateTime)
	tooOld = isBuildTooOld(buildCreateTime, suschpb.SchedulerConfig_LaunchCriteria_DAILY, 0)
	if tooOld {
		t.Errorf("Expected %t, got %t", false, tooOld)
	}
}

func TestIsBuildTooOldWeekly(t *testing.T) {
	tooOldCreateTime := time.Now().Add(common.Week * (-2))
	buildCreateTime := timestamppb.New(tooOldCreateTime)
	tooOld := isBuildTooOld(buildCreateTime, suschpb.SchedulerConfig_LaunchCriteria_WEEKLY, 0)
	if !tooOld {
		t.Errorf("Expected %t, got %t", true, tooOld)
		return
	}

	validCreateTime := time.Now().Add(time.Hour * (-2))
	buildCreateTime = timestamppb.New(validCreateTime)
	tooOld = isBuildTooOld(buildCreateTime, suschpb.SchedulerConfig_LaunchCriteria_WEEKLY, 0)
	if tooOld {
		t.Errorf("Expected %t, got %t", false, tooOld)
	}
}

func TestIsBuildTooOldFortnightly(t *testing.T) {
	tooOldCreateTime := time.Now().Add(common.Fortnight * (-2))
	buildCreateTime := timestamppb.New(tooOldCreateTime)
	tooOld := isBuildTooOld(buildCreateTime, suschpb.SchedulerConfig_LaunchCriteria_FORTNIGHTLY, 0)
	if !tooOld {
		t.Errorf("Expected %t, got %t", true, tooOld)
		return
	}

	validCreateTime := time.Now().Add(time.Hour * (-2))
	buildCreateTime = timestamppb.New(validCreateTime)
	tooOld = isBuildTooOld(buildCreateTime, suschpb.SchedulerConfig_LaunchCriteria_FORTNIGHTLY, 0)
	if tooOld {
		t.Errorf("Expected %t, got %t", false, tooOld)
	}
}

func TestIsBuildTooOldNDay(t *testing.T) {
	tooOldCreateTime := time.Now().Add(common.Fortnight * (-2))
	buildCreateTime := timestamppb.New(tooOldCreateTime)
	tooOld := isBuildTooOld(buildCreateTime, suschpb.SchedulerConfig_LaunchCriteria_N_DAYS, 16)
	if !tooOld {
		t.Errorf("Expected %t, got %t", true, tooOld)
		return
	}

	validCreateTime := time.Now().Add(time.Hour * (-2))
	buildCreateTime = timestamppb.New(validCreateTime)
	tooOld = isBuildTooOld(buildCreateTime, suschpb.SchedulerConfig_LaunchCriteria_N_DAYS, 1)

	if tooOld {
		t.Errorf("Expected %t, got %t", false, tooOld)
	}
}
