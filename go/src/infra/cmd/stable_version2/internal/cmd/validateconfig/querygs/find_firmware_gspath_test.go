// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package querygs

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/luci/common/gcloud/gs"
)

func TestMilestonesInOrder(t *testing.T) {
	expected := []int{5, 6, 7, 8, 9, 10, 4, 3, 2, 1}
	actual := milestonesInOrder(5)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("unexpected diff (-want, +got):\n%s", diff)
	}
}

func TestFindFirmwarePath(t *testing.T) {
	t.Parallel()
	var r Reader
	r.exst = func(gsPath gs.Path) error {
		if gsPath == "gs://chromeos-image-archive/a-release/R10-11.12.13-aaaaaa/firmware_from_source.tar.bz2" {
			return nil
		}
		return fmt.Errorf("Unexpected")
	}
	expected := &FindFirmwarePathResult{
		Image:    "a-release/R10-11.12.13-aaaaaa",
		FullPath: "gs://chromeos-image-archive/a-release/R10-11.12.13-aaaaaa/firmware_from_source.tar.bz2",
	}

	actual, err := r.FindFirmwarePath("a", 10, 11, 12, "13-aaaaaa")
	if err != nil {
		t.Error(err)
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("unexpected diff (-want, +got):\n%s", diff)
	}
}
