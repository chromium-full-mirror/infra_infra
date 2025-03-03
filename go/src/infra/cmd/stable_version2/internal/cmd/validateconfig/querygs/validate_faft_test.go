// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package querygs

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var validateFaftUseCases = []struct {
	name        string
	in          string
	expectError bool
	want        string
}{
	{
		"happy path - path without specifying the filename",
		"a-firmware/R10-11.12.13-aaaaaa",
		false,
		"gs://chromeos-image-archive/a-firmware/R10-11.12.13-aaaaaa/firmware_from_source.tar.bz2",
	},
	{
		"happy path - path with specified filename",
		"firmware-b-12345.B-branch-firmware/R10-11.12.13-aaaaaa/b/test2.tar.bz2",
		false,
		"gs://chromeos-image-archive/firmware-b-12345.B-branch-firmware/R10-11.12.13-aaaaaa/b/test2.tar.bz2",
	},
	{
		"happy path - branch path without specifying the filename ",
		"firmware-c-12345.B-branch-firmware/R10-11.12.13-aaaaaa",
		false,
		"gs://chromeos-image-archive/firmware-c-12345.B-branch-firmware/R10-11.12.13-aaaaaa/firmware_from_source.tar.bz2",
	},
	{
		"has gs in the path - type 1 path",
		"gs://chromeos-image-archive/a-firmware/R10-11.12.13-aaaaaa/firmware_from_source.tar.bz2",
		true,
		"",
	},
	{
		"has gs in the path - type 2 path",
		"gs://chromeos-image-archive/firmware-b-12345.B-branch-firmware/R10-11.12.13-aaaaaa/b/firmware_from_source.tar.bz2",
		true,
		"",
	},
	{
		"has gs in the path - type 3 path",
		"gs://chromeos-image-archive/firmware-c-12345.B-branch-firmware/R10-11.12.13-aaaaaa/firmware_from_source.tar.bz2",
		true,
		"",
	},
}

func TestValidateFaft(t *testing.T) {
	t.Parallel()
	for _, uc := range validateFaftUseCases {
		ctx := context.Background()
		var r Reader
		r.dld = fakeDownloader
		r.exst = fakeExistenceChecker
		got, err := r.validateFaft(ctx, uc.in)
		if uc.expectError && err == nil {
			t.Errorf("case %q: Expected error but got nil", uc.name)
		}
		if diff := cmp.Diff(uc.want, got); diff != "" {
			t.Errorf("case %q: unexpected diff: %s \n(-want: %q \n+got: %q)", uc.name, diff, uc.want, got)
		}
	}
}
