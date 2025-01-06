// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ctprequest

import (
	"testing"

	suschpb "go.chromium.org/chromiumos/infra/proto/go/testplans"

	"infra/cros/cmd/kron/common"
)

func TestGetGCSImageBucketCrosImageBucketNotNil(t *testing.T) {
	config := &suschpb.SchedulerConfig{
		RunOptions: &suschpb.SchedulerConfig_RunOptions{
			CrosImageBucket: "config123",
		},
	}

	if bucket := getGCSImageBucket(config); bucket != config.GetRunOptions().GetCrosImageBucket() {
		t.Errorf("Expected %s got %s", config.GetRunOptions().GetCrosImageBucket(), bucket)
	}
}

func TestGetGCSImageBucketCrosImageBucketNil(t *testing.T) {
	config := &suschpb.SchedulerConfig{}

	if bucket := getGCSImageBucket(config); bucket != DefaultImageBucket {
		t.Errorf("Expected %s got %s", DefaultImageBucket, bucket)
	}

}

func TestFormGCSPathDefault(t *testing.T) {
	config := &suschpb.SchedulerConfig{}

	fakeItems := []string{
		"test123",
		"abc",
	}
	expected := "gs://chromeos-image-archive/test123/abc"

	if path := formGCSPath(config, fakeItems...); path != expected {
		t.Errorf("Expected %s got %s", expected, path)
	}
}

func TestFormGCSPathPartner(t *testing.T) {
	config := &suschpb.SchedulerConfig{
		RunOptions: &suschpb.SchedulerConfig_RunOptions{
			CrosImageBucket: "partner-archive",
		},
	}

	fakeItems := []string{
		"test123",
		"abc",
	}
	expected := "gs://partner-archive/test123/abc"

	if path := formGCSPath(config, fakeItems...); path != expected {
		t.Errorf("Expected %s got %s", expected, path)
	}
}

func TestGenerateStagingConfig(t *testing.T) {
	config := &suschpb.SchedulerConfig{
		Name: "TSEStagingTest",
		RunOptions: &suschpb.SchedulerConfig_RunOptions{
			TimeoutMins: 39 * 60,
		},
	}

	request := BuildCTPRequest(config, "", "", "", "", "", "")

	if request.GetParams().GetTime().GetMaximumDuration().GetSeconds() != common.MaxStagingSeconds {
		t.Errorf("expected %d seconds got %d", int64(common.MaxStagingSeconds), request.GetParams().GetTime().GetMaximumDuration().GetSeconds())
	}
}
