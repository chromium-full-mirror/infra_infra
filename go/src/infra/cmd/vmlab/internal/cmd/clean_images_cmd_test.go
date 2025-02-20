// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmd

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/infra/libs/vmlab/api"
)

type mockImageApi struct {
	getImageFunc    func(buildPath string, wait bool) (*api.GceImage, error)
	listImagesFunc  func(filter string) ([]*api.GceImage, error)
	deleteImageFunc func(imageName string, wait bool) error
}

func (m *mockImageApi) GetImage(buildPath string, wait bool) (*api.GceImage, error) {
	return m.getImageFunc(buildPath, wait)
}

func (m *mockImageApi) ListImages(filter string) ([]*api.GceImage, error) {
	return m.listImagesFunc(filter)
}

func (m *mockImageApi) DeleteImage(imageName string, wait bool) error {
	return m.deleteImageFunc(imageName, wait)
}

// isEquivalentTo compares if two `cleanImagesResult` have the same number of
// total images and the same items in deleted, failed, unknown images. Ordering
// of items doesn't matter.
func (a *cleanImagesResult) isEquivalentTo(b *cleanImagesResult) bool {
	less := func(a, b string) bool { return a < b }
	if a.Total != b.Total ||
		!cmp.Equal(a.Deleted, b.Deleted, cmpopts.SortSlices(less)) ||
		!cmp.Equal(a.Failed, b.Failed, cmpopts.SortSlices(less)) ||
		!cmp.Equal(a.Unknown, b.Unknown, cmpopts.SortSlices(less)) {
		return false
	}
	return true
}

func TestNoDeleteNoExpired(t *testing.T) {
	gceImages := []*api.GceImage{
		{
			Name: "cq-noexpire",
			Labels: map[string]string{
				"build-type": "cq",
			},
			TimeCreated: timestamppb.New(time.Now().Add(-imageRetentionCQ + time.Hour)),
		},
		{
			Name: "postsubmit-noexpire",
			Labels: map[string]string{
				"build-type": "postsubmit",
			},
			TimeCreated: timestamppb.New(time.Now().Add(-imageRetentionPostsubmit + time.Hour)),
		},
		{
			Name: "release-noexpire",
			Labels: map[string]string{
				"build-type": "release",
			},
			TimeCreated: timestamppb.New(time.Now().Add(-imageRetentionRelease + time.Hour)),
		},
		{
			Name: "snapshot-noexpire",
			Labels: map[string]string{
				"build-type": "snapshot",
			},
			TimeCreated: timestamppb.New(time.Now().Add(-imageRetentionDefault + time.Hour)),
		},
		{
			Name: "unknown-noexpire",
			Labels: map[string]string{
				"build-type": "unknown",
			},
			TimeCreated: timestamppb.New(time.Now().Add(-imageRetentionDefault + time.Hour)),
		},
		{
			Name:        "unknown-noexpire-nolabel",
			Labels:      map[string]string{},
			TimeCreated: timestamppb.New(time.Now().Add(-imageRetentionDefault + time.Hour)),
		},
	}

	imageApi := &mockImageApi{
		listImagesFunc: func(filter string) ([]*api.GceImage, error) {
			return gceImages, nil
		},
		deleteImageFunc: func(imageName string, wait bool) error {
			return nil
		},
	}

	result, err := cleanUpImages(imageApi, 1000, false)

	if err != nil {
		t.Fatalf("cleanUpImages() returned error: %v", err)
	}

	expectedResult := cleanImagesResult{
		Total:   len(gceImages),
		Deleted: []string{},
		Failed:  []string{},
		Unknown: []string{"unknown-noexpire", "unknown-noexpire-nolabel"},
	}

	if !expectedResult.isEquivalentTo(&result) {
		t.Errorf("Expected result to be %v, but is %v", expectedResult, result)
	}
}

func TestDeleteExpired(t *testing.T) {
	gceImages := []*api.GceImage{
		{
			Name: "cq-noexpire",
			Labels: map[string]string{
				"build-type": "cq",
			},
			TimeCreated: timestamppb.New(time.Now().Add(-imageRetentionCQ + time.Hour)),
		},
		{
			Name: "cq-expire",
			Labels: map[string]string{
				"build-type": "cq",
			},
			TimeCreated: timestamppb.New(time.Now().Add(-imageRetentionCQ - time.Hour)),
		},
		{
			Name: "postsubmit-expire",
			Labels: map[string]string{
				"build-type": "postsubmit",
			},
			TimeCreated: timestamppb.New(time.Now().Add(-imageRetentionPostsubmit - time.Hour)),
		},
		{
			Name: "release-expire",
			Labels: map[string]string{
				"build-type": "release",
			},
			TimeCreated: timestamppb.New(time.Now().Add(-imageRetentionRelease - time.Hour)),
		},
		{
			Name: "snapshot-expire",
			Labels: map[string]string{
				"build-type": "snapshot",
			},
			TimeCreated: timestamppb.New(time.Now().Add(-imageRetentionDefault - time.Hour)),
		},
		{
			Name: "unknown-expire",
			Labels: map[string]string{
				"build-type": "unknown",
			},
			TimeCreated: timestamppb.New(time.Now().Add(-imageRetentionDefault - time.Hour)),
		},
		{
			Name:        "unknown-expire-nolabel",
			Labels:      map[string]string{},
			TimeCreated: timestamppb.New(time.Now().Add(-imageRetentionDefault - time.Hour)),
		},
	}

	imageApi := &mockImageApi{
		listImagesFunc: func(filter string) ([]*api.GceImage, error) {
			return gceImages, nil
		},
		deleteImageFunc: func(imageName string, wait bool) error {
			return nil
		},
	}

	result, err := cleanUpImages(imageApi, 1000, false)

	if err != nil {
		t.Fatalf("cleanUpImages() returned error: %v", err)
	}

	expectedResult := cleanImagesResult{
		Total: len(gceImages),
		Deleted: []string{
			"cq-expire", "postsubmit-expire", "release-expire", "snapshot-expire", "unknown-expire", "unknown-expire-nolabel",
		},
		Failed:  []string{},
		Unknown: []string{"unknown-expire", "unknown-expire-nolabel"},
	}

	if !expectedResult.isEquivalentTo(&result) {
		t.Errorf("Expected result to be %v, but is %v", expectedResult, result)
	}
}

func TestDeleteExpiredError(t *testing.T) {
	gceImages := []*api.GceImage{
		{
			Name: "cq-expire-fail",
			Labels: map[string]string{
				"build-type": "cq",
			},
			TimeCreated: timestamppb.New(time.Now().Add(-imageRetentionCQ - time.Hour)),
		},
		{
			Name: "cq-noexpire",
			Labels: map[string]string{
				"build-type": "cq",
			},
			TimeCreated: timestamppb.New(time.Now().Add(-imageRetentionCQ + time.Hour)),
		},
		{
			Name: "cq-expire",
			Labels: map[string]string{
				"build-type": "cq",
			},
			TimeCreated: timestamppb.New(time.Now().Add(-imageRetentionCQ - time.Hour)),
		},
	}

	imageApi := &mockImageApi{
		listImagesFunc: func(filter string) ([]*api.GceImage, error) {
			return gceImages, nil
		},
		deleteImageFunc: func(imageName string, wait bool) error {
			if imageName == "cq-expire-fail" {
				return fmt.Errorf("Error deleting")
			}
			return nil
		},
	}

	result, err := cleanUpImages(imageApi, 1000, false)

	if err != nil {
		t.Fatalf("cleanUpImages() returned error: %v", err)
	}

	expectedResult := cleanImagesResult{
		Total: len(gceImages),
		Deleted: []string{
			"cq-expire",
		},
		Failed: []string{
			"cq-expire-fail",
		},
		Unknown: []string{},
	}

	if !expectedResult.isEquivalentTo(&result) {
		t.Errorf("Expected result to be %v, but is %v", expectedResult, result)
	}
}

func TestDeleteExpiredDryRun(t *testing.T) {
	gceImages := []*api.GceImage{
		{
			Name: "cq-expire-fail",
			Labels: map[string]string{
				"build-type": "cq",
			},
			TimeCreated: timestamppb.New(time.Now().Add(-imageRetentionCQ - time.Hour)),
		},
		{
			Name: "cq-noexpire",
			Labels: map[string]string{
				"build-type": "cq",
			},
			TimeCreated: timestamppb.New(time.Now().Add(-imageRetentionCQ + time.Hour)),
		},
		{
			Name: "cq-expire",
			Labels: map[string]string{
				"build-type": "cq",
			},
			TimeCreated: timestamppb.New(time.Now().Add(-imageRetentionCQ - time.Hour)),
		},
	}

	imageApi := &mockImageApi{
		listImagesFunc: func(filter string) ([]*api.GceImage, error) {
			return gceImages, nil
		},
		deleteImageFunc: func(imageName string, wait bool) error {
			t.Fatalf("Should not reach here")
			return fmt.Errorf("Should not reach here")
		},
	}

	result, err := cleanUpImages(imageApi, 1000, true)

	if err != nil {
		t.Fatalf("cleanUpImages() returned error: %v", err)
	}

	expectedResult := cleanImagesResult{
		Total: len(gceImages),
		Deleted: []string{
			"cq-expire",
			"cq-expire-fail",
		},
		Failed:  []string{},
		Unknown: []string{},
	}

	if !expectedResult.isEquivalentTo(&result) {
		t.Errorf("Expected result to be %v, but is %v", expectedResult, result)
	}
}
