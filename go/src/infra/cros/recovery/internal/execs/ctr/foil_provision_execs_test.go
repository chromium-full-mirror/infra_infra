// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ctr

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

var androidImagePathCases = []struct {
	board  string
	expect string
	ok     bool
}{
	{"corsola", "android-build/build_explorer/artifacts_list/12645826/corsola-trunk_staging-userdebug/corsola-ota-12645826.zip", true},
	{"dedede", "android-build/build_explorer/artifacts_list/12494948/dedede-trunk_staging-userdebug/dedede-ota-12494948.zip", true},
	{"nissa", "android-build/build_explorer/artifacts_list/12645826/nissa-trunk_staging-userdebug/nissa-ota-12645826.zip", true},
	{"brya", "android-build/build_explorer/artifacts_list/12643288/brya-trunk_staging-userdebug/brya-ota-12643288.zip", true},
	{"", "", false},
	{"some-board", "", false},
}

func TestAndroidImagePath(t *testing.T) {
	t.Parallel()
	for _, c := range androidImagePathCases {
		cs := c
		t.Run(cs.board, func(t *testing.T) {
			got, err := androidImagePath(cs.board)
			if cs.ok {
				if !cmp.Equal(got, cs.expect) {
					t.Errorf("%q ->want: %v\n got: %v", cs.board, cs.expect, got)
				}
			} else if err == nil {
				t.Errorf("%q -> expected to finish with error but passed", cs.board)
			}
		})
	}
}
