// Copyright (c) 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package system

import (
	"fmt"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestRemoveDiskDevices(t *testing.T) {
	tests := []struct {
		Names []string
		Want  []string
	}{
		{[]string{"sda", "sda1"}, []string{"sda1"}},
		{[]string{"sda1", "sda"}, []string{"sda1"}},
		{[]string{"sda", "sdb1"}, []string{"sda", "sdb1"}},
	}

	for i, test := range tests {
		ftt.Run(fmt.Sprintf("%d. %s", i, test.Names), t, func(t *ftt.Test) {
			assert.Loosely(t, removeDiskDevices(test.Names), should.Resemble(test.Want))
		})
	}
}

func TestMountpointsAreIgnored(t *testing.T) {
	t.Parallel()

	ftt.Run("Docker mountpoints are ignored", t, func(t *ftt.Test) {
		assert.Loosely(t, shouldIgnoreMountpoint("/var/lib/docker/aufs"), should.BeTrue)
	})
}
