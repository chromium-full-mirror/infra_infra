// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package datastore

import (
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestGetDevices(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	ftt.Run("Get devices from an empty datastore", t, func(t *ftt.Test) {
		t.Run("Get all", func(t *ftt.Test) {
			devs, err := GetAllDevices(ctx)
			assert.Loosely(t, devs, should.BeEmpty)
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Get by hostnames", func(t *ftt.Test) {
			result := GetDevicesByHostnames(ctx, []string{"dut1", "labstation2"})
			assert.Loosely(t, result.Passed(), should.BeEmpty)
			assert.Loosely(t, result.Failed(), should.HaveLength(2))
		})
	})
}
