// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dumper

import (
	"bufio"
	"context"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"os"
	"testing"
)

func TestParseGoldenEyeJsonData(t *testing.T) {
	t.Parallel()

	ftt.Run("Parse Data", t, func(t *ftt.Test) {
		t.Run("happy path", func(t *ftt.Test) {
			file, err := os.Open("test.json")
			assert.Loosely(t, err, should.BeNil)
			reader := bufio.NewReader(file)

			devices, err := parseGoldenEyeData(context.Background(), reader)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, devices.Devices, should.NotBeNil)
		})
		t.Run("parse for non existent file", func(t *ftt.Test) {
			file, err := os.Open("test2.json")
			assert.Loosely(t, err, should.NotBeNil)
			reader := bufio.NewReader(file)

			devices, err := parseGoldenEyeData(context.Background(), reader)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.Equal("unmarshal chunk failed while reading golden eye data for devices: invalid argument"))
			assert.Loosely(t, devices, should.BeNil)
		})
	})
}
