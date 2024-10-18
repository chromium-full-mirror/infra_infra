// Copyright (c) 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package android

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

func TestLoadFile(t *testing.T) {
	now := time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC) // Unix timestamp 946782245
	ctx := context.Background()
	ctx, _ = testclock.UseTime(ctx, now)

	ftt.Run("In a temporary directory", t, func(t *ftt.Test) {
		path, err := ioutil.TempDir("", "android-devicefile-test")
		assert.Loosely(t, err, should.BeNil)
		defer os.RemoveAll(path)

		fileName := filepath.Join(path, "file.json")
		t.Run("loads a valid file", func(t *ftt.Test) {
			err := ioutil.WriteFile(fileName, []byte(`
        {
          "version": 1,
          "timestamp": 946782245,
          "devices": {
            "02eccd9208ead9ab": {
              "battery": {
                "health": 2,
                "level": 100,
                "power": [
                  "USB"
                ],
                "status": 5,
                "temperature": 248,
                "voltage": 4352
              },
              "build": {
                "board.platform": "msm8974",
                "build.fingerprint": "google/hammerhead/hammerhead:4.4.4/KTU84P/1227136:userdebug/dev-keys",
                "build.id": "KTU84P",
                "build.version.sdk": "19",
                "product.board": "hammerhead",
                "product.cpu.abi": "armeabi-v7a"
              },
              "cpu": {
                "cur": "300000",
                "governor": "powersave"
              },
              "disk": {
                "cache": {
                  "free_mb": 677.3,
                  "size_mb": 689.8
                },
                "data": {
                  "free_mb": 12512.4,
                  "size_mb": 12853.7
                },
                "system": {
                  "free_mb": 369.1,
                  "size_mb": 1009.3
                }
              },
              "imei": "358239051612770",
              "ip": {},
              "max_uid": 10073,
              "mem": {
                "avail": 1279052,
                "buffers": 59668,
                "cached": 341656,
                "free": 877728,
                "total": 1899548,
                "used": 620496
              },
              "other_packages": [],
              "port_path": "2/28",
              "processes": 179,
              "state": "available",
              "temp": {
                "emmc_therm": 23.0,
                "pa_therm0": 23.0,
                "pm8841_tz": 37.0,
                "pm8941_tz": 24.541,
                "tsens_tz_sensor0": 25
              },
              "uptime": 1159.48
            }
          }
        }
      `), 0644)
			assert.Loosely(t, err, should.BeNil)

			f, st, _, err := loadFile(ctx, fileName)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, st, should.Equal(status("good")))
			var cpu float64 = 25
			assert.Loosely(t, f, should.Match(deviceStatusFile{
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
						IMEI: "358239051612770",
						Mem: memory{
							Avail: 1279052,
							Total: 1899548,
						},
						Processes: 179,
						State:     "available",
						Temp: temperature{
							TSensTZ0: &cpu,
							MtktsCPU: nil,
							CPUTherm: nil,
						},
						Uptime: 1159.48,
					},
				},
				Version:   1,
				Timestamp: 9.46782245e+08,
			}))
		})

		t.Run("loads a valid file, no CPUs", func(t *ftt.Test) {
			err := ioutil.WriteFile(fileName, []byte(`
        {
          "version": 1,
          "timestamp": 946782245,
          "devices": {
            "02eccd9208ead9ab": {
              "state": "available",
              "temp": {
                "omg_sensor": 23.0
              }
            }
          }
        }
      `), 0644)
			assert.Loosely(t, err, should.BeNil)

			f, st, _, err := loadFile(ctx, fileName)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, st, should.Equal(status("good")))
			assert.Loosely(t, f, should.Match(deviceStatusFile{
				Devices: map[string]deviceStatus{
					"02eccd9208ead9ab": {
						State: "available",
						Temp: temperature{
							TSensTZ0: nil,
							MtktsCPU: nil,
							CPUTherm: nil,
						},
					},
				},
				Version:   1,
				Timestamp: 9.46782245e+08,
			}))
		})

		t.Run("file not found", func(t *ftt.Test) {
			_, st, _, err := loadFile(ctx, "/file/not/found")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, st, should.Equal(status("not_found")))
		})

		t.Run("invalid json", func(t *ftt.Test) {
			err := ioutil.WriteFile(fileName, []byte(`not valid json`), 0644)
			assert.Loosely(t, err, should.BeNil)

			_, st, _, err := loadFile(ctx, fileName)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, st, should.Equal(status("invalid_json")))
		})

		t.Run("invalid version", func(t *ftt.Test) {
			err := ioutil.WriteFile(fileName, []byte(`
        {
          "version": 42
        }
      `), 0644)
			assert.Loosely(t, err, should.BeNil)

			_, st, _, err := loadFile(ctx, fileName)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, st, should.Equal(status("invalid_version")))
		})

		t.Run("previous version", func(t *ftt.Test) {
			err := ioutil.WriteFile(fileName, []byte(`
        {
          "version": 0,
          "timestamp": 946782245
        }
      `), 0644)
			assert.Loosely(t, err, should.BeNil)

			_, st, _, err := loadFile(ctx, fileName)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, st, should.Equal(status("good")))
		})

		t.Run("stale timestamp", func(t *ftt.Test) {
			// 946782084 == 946782245 - 161
			err := ioutil.WriteFile(fileName, []byte(`
        {
          "version": 1,
          "timestamp": 946782084
        }
      `), 0644)
			assert.Loosely(t, err, should.BeNil)

			_, st, staleness, err := loadFile(ctx, fileName)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, staleness, should.Equal(161.))
			assert.Loosely(t, st, should.Equal(status("stale_file")))
		})
	})
}
