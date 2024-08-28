// Copyright (c) 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package android

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/common/tsmon"
)

func TestFileGlobbing(t *testing.T) {
	now := time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC) // Unix timestamp 946782245
	c := context.Background()
	c, _ = testclock.UseTime(c, now)

	ftt.Run("In a temporary directory", t, func(t *ftt.Test) {
		tmpPath, err := ioutil.TempDir("", "android-devicefile-test")
		assert.Loosely(t, err, should.BeNil)
		defer os.RemoveAll(tmpPath)
		err = os.Mkdir(filepath.Join(tmpPath, ".android"), 0777)
		assert.Loosely(t, err, should.BeNil)
		path := filepath.Join(tmpPath, ".android")
		fileNames := []string{
			strings.Replace(fileGlob, "*", "file1", 1),
			strings.Replace(fileGlob, "*", "file2", 1),
			strings.Replace(fileGlob, "*", "file3", 1),
		}
		t.Run("loads a number of empty files", func(t *ftt.Test) {
			for _, fileName := range fileNames {
				assert.Loosely(t, ioutil.WriteFile(filepath.Join(path, fileName), []byte(`{"version": 1, "timestamp": 946782245, "devices": {}}`), 0644), should.BeNil)
			}
			assert.Loosely(t, update(c, tmpPath), should.BeNil)
		})
		t.Run("loads a number of broken files", func(t *ftt.Test) {
			for _, fileName := range fileNames {
				assert.Loosely(t, ioutil.WriteFile(filepath.Join(path, fileName), []byte(`not json`), 0644), should.BeNil)
			}
			assert.Loosely(t, update(c, tmpPath), should.NotBeNil)
		})
	})
}

func TestMetrics(t *testing.T) {
	c := context.Background()
	c, _ = tsmon.WithDummyInMemory(c)

	var cpu float64 = 23
	ftt.Run("Device metrics", t, func(t *ftt.Test) {
		file := deviceStatusFile{
			Devices: map[string]deviceStatus{
				"02eccd9208ead9ab": {
					Battery: battery{
						Level:       100,
						Temperature: 248,
					},
					Build: build{
						ID:    "KTU84P",
						Board: "hammerhead",
					},
					IMEI: "123456789",
					Mem: memory{
						Avail: 1279052,
						Total: 1899548,
					},
					Processes: 179,
					State:     "available",
					Temp: temperature{
						CPUTherm: &cpu,
						MtktsCPU: nil,
						TSensTZ0: nil,
					},
					Uptime: 1159.48,
				},
			},
			Version:   1,
			Timestamp: 9.46782245e+08,
		}

		updateFromFile(c, file)

		assert.That(t, cpuTemp.Get(c, "02eccd9208ead9ab"), should.Equal(23.))
		assert.That(t, battTemp.Get(c, "02eccd9208ead9ab"), should.Equal(24.8))
		assert.That(t, battCharge.Get(c, "02eccd9208ead9ab"), should.Equal(100.))
		assert.That(t, devOS.Get(c, "02eccd9208ead9ab"), should.Equal("KTU84P"))
		assert.That(t, devStatus.Get(c, "02eccd9208ead9ab", "123456789"), should.Equal("good"))
		assert.That(t, devType.Get(c, "02eccd9208ead9ab"), should.Equal("hammerhead"))
		assert.That(t, devUptime.Get(c, "02eccd9208ead9ab"), should.Equal(1159.48))
		assert.Loosely(t, memFree.Get(c, "02eccd9208ead9ab"), should.Equal(1279052))
		assert.Loosely(t, memTotal.Get(c, "02eccd9208ead9ab"), should.Equal(1899548))
		assert.Loosely(t, procCount.Get(c, "02eccd9208ead9ab"), should.Equal(179))
	})
}
