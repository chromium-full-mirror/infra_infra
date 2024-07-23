// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configuration

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/golang/protobuf/jsonpb"

	"go.chromium.org/chromiumos/config/go/test/dut"
	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestDeviceStability(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	ftt.Run("Test Device Stability", t, func(t *ftt.Test) {
		b, err := ioutil.ReadFile("test_device_stability.cfg")
		assert.Loosely(t, err, should.BeNil)
		unmarshaller := &jsonpb.Unmarshaler{AllowUnknownFields: false}
		var dsList dut.DeviceStabilityList
		err = unmarshaller.Unmarshal(bytes.NewBuffer(b), &dsList)
		assert.Loosely(t, err, should.BeNil)
		for _, ds := range dsList.GetValues() {
			for _, id := range ds.GetDutCriteria()[0].GetValues() {
				err := UpdateDeviceStability(ctx, id, ds)
				assert.Loosely(t, err, should.BeNil)
			}
		}
		t.Run("GetDeviceStability", func(t *ftt.Test) {
			resp, err := GetDeviceStability(ctx, "milkyway")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetStability().String(), should.Equal("UNSTABLE"))
		})
	})
}
