// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scheduling

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"infra/libs/fleet/scheduling/api"
	"infra/libs/fleet/scheduling/internal/scheduke"
)

func TestNewTaskSchedulingApi_unimplemented(t *testing.T) {
	ts, err := NewTaskSchedulingAPI(api.ProviderId_PROVIDER_ID_UNSPECIFIED)
	if ts != nil {
		t.Errorf("TaskSchedulingAPI = %v, but want nil", ts)
	}
	if err == nil {
		t.Errorf("error should not be nil")
	}
}

func TestNewTaskSchedulingApi_scheduke(t *testing.T) {
	want, _ := scheduke.New()
	ts, err := NewTaskSchedulingAPI(api.ProviderId_PROVIDER_ID_SCHEDUKE)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cmp.Equal(ts, want) {
		t.Errorf("TaskSchedulingAPI = %v, but want %v", ts, want)
	}
}
