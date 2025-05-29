// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ctprequest

import (
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	requestpb "go.chromium.org/chromiumos/infra/proto/go/test_platform"
	suschpb "go.chromium.org/chromiumos/infra/proto/go/testplans"

	"go.chromium.org/infra/cros/cmd/kron/common"
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

func TestAddTagToRequest(t *testing.T) {
	request := &requestpb.Request{
		Params: &requestpb.Request_Params{
			Decorations: &requestpb.Request_Params_Decorations{},
		},
	}

	modifiedRequest := AddTagToRequest("key", "value", request)
	expectedTag := "key:value"

	tags := modifiedRequest.GetParams().GetDecorations().GetTags()
	if len(tags) == 0 {
		t.Errorf("request tags were expected to be populated, a zero length was returned")
		return
	}

	if len(tags) != 1 {
		t.Errorf("expected %d tag, got %d tags", 1, len(tags))
		return
	}

	if tags[0] != expectedTag {
		t.Errorf("expected tag (%s), got (%s)", expectedTag, tags[0])
		return
	}

}

func TestGetTestPlanIterations(t *testing.T) {
	config := &suschpb.SchedulerConfig{
		RunOptions: &suschpb.SchedulerConfig_RunOptions{
			Iterations: &api.Iterations{NumRetries: 5},
		},
	}
	testPlan := getTestPlan(config)
	expectedRetries := 5
	actualRetries := int(testPlan.GetIterations().NumRetries)
	if actualRetries != expectedRetries {
		t.Fatalf("Testplan iterations field was expected to be populated with the value of %d, actual value %d", expectedRetries, actualRetries)
	}
	config = &suschpb.SchedulerConfig{
		RunOptions: &suschpb.SchedulerConfig_RunOptions{},
	}
	testPlan = getTestPlan(config)
	if testPlan.GetIterations() != nil {
		t.Fatal("Testplan iterations field was expected to be empty, the field was populated")
	}
}

func TestGetTestPlanIgnoreVariantCategory(t *testing.T) {
	type IgnoreVariantCategoryTest struct {
		name           string
		input          *suschpb.SchedulerConfig
		expectedResult bool
	}
	ignoreVcTests := []IgnoreVariantCategoryTest{
		{
			name:           "Flag True",
			input:          &suschpb.SchedulerConfig{RunOptions: &suschpb.SchedulerConfig_RunOptions{IgnoreVariantCategory: true}},
			expectedResult: true,
		},
		{
			name:           "Flag False",
			input:          &suschpb.SchedulerConfig{RunOptions: &suschpb.SchedulerConfig_RunOptions{IgnoreVariantCategory: false}},
			expectedResult: false,
		},
		{
			name:           "Empty Flag",
			input:          &suschpb.SchedulerConfig{RunOptions: &suschpb.SchedulerConfig_RunOptions{}},
			expectedResult: false,
		},
	}

	for _, test := range ignoreVcTests {
		testPlan := getTestPlan(test.input)
		actual := testPlan.GetIgnoreVariantCategory()
		if testPlan.GetIgnoreVariantCategory() != test.expectedResult {
			t.Fatalf("Test %s: EXPECTED - %t , ACTUAL -  %t", test.name, test.expectedResult, actual)
		}
	}
}
