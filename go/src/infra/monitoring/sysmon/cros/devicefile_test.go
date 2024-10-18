// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestLoadfile(t *testing.T) {
	now := time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)
	c := context.Background()
	c, _ = testclock.UseTime(c, now)

	ftt.Run("Using tmp directory", t, func(t *ftt.Test) {
		// Use tmp dir to create a mock file
		path, err := ioutil.TempDir("", "cros-devicefile-test")
		assert.Loosely(t, err, should.BeNil)
		defer os.RemoveAll(path)

		fileName := filepath.Join(path, "file.json")

		t.Run("loads a valid file", func(t *ftt.Test) {
			err := ioutil.WriteFile(fileName, []byte(`{
			  "container_hostname": "b1_b2",
			  "status": "online",
			  "timestamp": 1559855998.093489
			}`), 0644)
			assert.Loosely(t, err, should.BeNil)
			f, err := loadfile(c, fileName)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, f, should.Match(deviceStatusFile{
				ContainerHostname: "b1_b2",
				Timestamp:         1559855998.093489,
				Status:            "online",
			}))
		})

		t.Run("file not found", func(t *ftt.Test) {
			_, err := loadfile(c, "/file/not/found")
			assert.Loosely(t, err, should.NotBeNil)
		})

		t.Run("invalid json", func(t *ftt.Test) {
			err := ioutil.WriteFile(fileName,
				[]byte(`not valid json`), 0644)
			assert.Loosely(t, err, should.BeNil)

			_, err = loadfile(c, fileName)
			assert.Loosely(t, err, should.NotBeNil)
		})

	})
}
