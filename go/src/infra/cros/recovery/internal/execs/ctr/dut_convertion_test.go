// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ctr

import (
	"testing"

	"go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/recovery/tlw"
)

func TestToLabDut(t *testing.T) {
	t.Parallel()
	ftt.Run("dut is nil", t, func(t *ftt.Test) {
		ld := toLabDut(nil, nil)
		assert.Loosely(t, ld, should.BeNil)
	})
	ftt.Run("empty dut", t, func(t *ftt.Test) {
		d := &tlw.Dut{}
		ld := toLabDut(d, nil)
		expected := &api.Dut{
			Id: &api.Dut_Id{
				Value: "",
			},
			DutType: &api.Dut_Chromeos{
				Chromeos: &api.Dut_ChromeOS{
					Ssh: &api.IpEndpoint{
						Address: "",
						Port:    22,
					},
					DutModel: &api.DutModel{
						BuildTarget: "",
						ModelName:   "",
					},
					Servo: &api.Servo{},
				},
			},
			CacheServer: &api.CacheServer{
				Address: nil,
			},
		}
		assert.Loosely(t, ld.Id, should.Match(expected.Id))
		assert.Loosely(t, ld.DutType, should.Match(expected.DutType))
		assert.Loosely(t, ld.CacheServer, should.Match(expected.CacheServer))
	})
	ftt.Run("dut with data", t, func(t *ftt.Test) {
		d := &tlw.Dut{
			Name: "d-name",
			Chromeos: &tlw.ChromeOS{
				Board: "my-board",
				Model: "my-model",
			},
		}
		ld := toLabDut(d, nil)
		expected := &api.Dut{
			Id: &api.Dut_Id{
				Value: "d-name",
			},
			DutType: &api.Dut_Chromeos{
				Chromeos: &api.Dut_ChromeOS{
					Ssh: &api.IpEndpoint{
						Address: "d-name",
						Port:    22,
					},
					DutModel: &api.DutModel{
						BuildTarget: "my-board",
						ModelName:   "my-model",
					},
					Servo: &api.Servo{},
				},
			},
			CacheServer: &api.CacheServer{
				Address: nil,
			},
		}
		assert.Loosely(t, ld.Id, should.Match(expected.Id))
		assert.Loosely(t, ld.DutType, should.Match(expected.DutType))
		assert.Loosely(t, ld.CacheServer, should.Match(expected.CacheServer))
	})
	ftt.Run("dut with servo", t, func(t *ftt.Test) {
		d := &tlw.Dut{
			Name: "d-name",
			Chromeos: &tlw.ChromeOS{
				Board: "my-board",
				Model: "my-model",
				Servo: &tlw.ServoHost{
					Name:         "servo-host1",
					SerialNumber: "servo-v4p1",
					ServodPort:   9995,
				},
			},
		}
		ld := toLabDut(d, nil)
		expected := &api.Dut{
			Id: &api.Dut_Id{
				Value: "d-name",
			},
			DutType: &api.Dut_Chromeos{
				Chromeos: &api.Dut_ChromeOS{
					Ssh: &api.IpEndpoint{
						Address: "d-name",
						Port:    22,
					},
					DutModel: &api.DutModel{
						BuildTarget: "my-board",
						ModelName:   "my-model",
					},
					Servo: &api.Servo{
						Present: true,
						Serial:  "servo-v4p1",
						ServodAddress: &api.IpEndpoint{
							Address: "servo-host1",
							Port:    9995,
						},
					},
				},
			},
			CacheServer: &api.CacheServer{
				Address: nil,
			},
		}
		assert.Loosely(t, ld.Id, should.Match(expected.Id))
		assert.Loosely(t, ld.DutType, should.Match(expected.DutType))
		assert.Loosely(t, ld.CacheServer, should.Match(expected.CacheServer))
	})
	ftt.Run("dut with servo container", t, func(t *ftt.Test) {
		d := &tlw.Dut{
			Name: "d-name",
			Chromeos: &tlw.ChromeOS{
				Board: "my-board",
				Model: "my-model",
				Servo: &tlw.ServoHost{
					Name:          "servo-host1",
					SerialNumber:  "servo-v4p1",
					ContainerName: "container1",
					ServodPort:    9996,
				},
			},
		}
		ld := toLabDut(d, nil)
		expected := &api.Dut{
			Id: &api.Dut_Id{
				Value: "d-name",
			},
			DutType: &api.Dut_Chromeos{
				Chromeos: &api.Dut_ChromeOS{
					Ssh: &api.IpEndpoint{
						Address: "d-name",
						Port:    22,
					},
					DutModel: &api.DutModel{
						BuildTarget: "my-board",
						ModelName:   "my-model",
					},
					Servo: &api.Servo{
						Present: true,
						Serial:  "servo-v4p1",
						ServodAddress: &api.IpEndpoint{
							Address: "container1",
							Port:    9996,
						},
					},
				},
			},
			CacheServer: &api.CacheServer{
				Address: nil,
			},
		}
		assert.Loosely(t, ld.Id, should.Match(expected.Id))
		assert.Loosely(t, ld.DutType, should.Match(expected.DutType))
		assert.Loosely(t, ld.CacheServer, should.Match(expected.CacheServer))
	})
	ftt.Run("full set of data", t, func(t *ftt.Test) {
		d := &tlw.Dut{
			Name: "d-name",
			Chromeos: &tlw.ChromeOS{
				Board: "my-board",
				Model: "my-model",
				Servo: &tlw.ServoHost{
					Name:          "servo-host1",
					SerialNumber:  "servo-v4p1",
					ContainerName: "container1",
					ServodPort:    0,
				},
			},
		}
		cachingData := &api.IpEndpoint{
			Address: "cache-name",
			Port:    34,
		}
		ld := toLabDut(d, cachingData)
		expected := &api.Dut{
			Id: &api.Dut_Id{
				Value: "d-name",
			},
			DutType: &api.Dut_Chromeos{
				Chromeos: &api.Dut_ChromeOS{
					Ssh: &api.IpEndpoint{
						Address: "d-name",
						Port:    22,
					},
					DutModel: &api.DutModel{
						BuildTarget: "my-board",
						ModelName:   "my-model",
					},
					Servo: &api.Servo{
						Present: true,
						Serial:  "servo-v4p1",
						ServodAddress: &api.IpEndpoint{
							Address: "container1",
							Port:    0,
						},
					},
				},
			},
			CacheServer: &api.CacheServer{
				Address: cachingData,
			},
		}
		assert.Loosely(t, ld.Id, should.Match(expected.Id))
		assert.Loosely(t, ld.DutType, should.Match(expected.DutType))
		assert.Loosely(t, ld.CacheServer, should.Match(expected.CacheServer))
	})
}
