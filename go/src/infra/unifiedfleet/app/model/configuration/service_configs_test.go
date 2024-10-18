// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configuration

import (
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func mockServiceConfig() *ServiceConfig {
	return &ServiceConfig{
		LastCheckedVMMacAddress: "000000",
	}
}

func TestGetLastCheckedVMMacAddress(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	sc := mockServiceConfig()
	ftt.Parallel("GetLastCheckedVMMacAddress", t, func(t *ftt.Test) {
		err := UpdateServiceConfig(ctx, sc)
		assert.Loosely(t, err, should.BeNil)
		resp, err := GetServiceConfig(ctx)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp.LastCheckedVMMacAddress, should.Match("000000"))
	})
}
