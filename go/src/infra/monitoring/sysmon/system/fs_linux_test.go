// Copyright (c) 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package system

import (
	"fmt"
	"testing"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestRemoveDiskDevices(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Names []string
		Want  []string
	}{
		{[]string{"sda", "sda1"}, []string{"sda1"}},
		{[]string{"sda1", "sda"}, []string{"sda1"}},
		{[]string{"sda", "sdb1"}, []string{"sda", "sdb1"}},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%d. %s", i, test.Names), func(t *testing.T) {
			assert.That(t, removeDiskDevices(test.Names), should.Match(test.Want))
		})
	}
}

func TestMountpointsAreIgnored(t *testing.T) {
	t.Parallel()

	assert.That(t, shouldIgnoreMountpoint("/var/lib/docker/aufs"), should.BeTrue)
}
