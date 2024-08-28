// Copyright (c) 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package system

import (
	"context"
	"runtime"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/common/tsmon"
)

func TestMetrics(t *testing.T) {

	ftt.Run("Uptime", t, func(t *ftt.Test) {
		c, _ := tsmon.WithDummyInMemory(context.Background())
		assert.Loosely(t, updateUptimeMetrics(c), should.BeNil)
		assert.Loosely(t, uptime.Get(c), should.BeGreaterThan(0))
	})

	ftt.Run("CPU", t, func(t *ftt.Test) {
		c, _ := tsmon.WithDummyInMemory(context.Background())
		if !cgoEnabled && runtime.GOOS == "darwin" {
			t.Skip("Requires CGO_ENABLED=1 on Mac")
		}

		assert.Loosely(t, updateCPUMetrics(c), should.BeNil)
		assert.Loosely(t, cpuCount.Get(c), should.BeGreaterThan(0))

		// Small fudge factor because sometimes this isn't exact.
		const aBitLessThanZero = -0.001
		const oneHundredAndABit = 100.001

		v := cpuTime.Get(c, "user")
		assert.Loosely(t, v, should.BeGreaterThanOrEqual(aBitLessThanZero))
		assert.Loosely(t, v, should.BeLessThanOrEqual(oneHundredAndABit))

		v = cpuTime.Get(c, "system")
		assert.Loosely(t, v, should.BeGreaterThanOrEqual(aBitLessThanZero))
		assert.Loosely(t, v, should.BeLessThanOrEqual(oneHundredAndABit))

		v = cpuTime.Get(c, "idle")
		assert.Loosely(t, v, should.BeGreaterThanOrEqual(aBitLessThanZero))
		assert.Loosely(t, v, should.BeLessThanOrEqual(oneHundredAndABit))
	})

	ftt.Run("Disk", t, func(t *ftt.Test) {
		c, _ := tsmon.WithDummyInMemory(context.Background())
		assert.Loosely(t, updateDiskMetrics(c), should.BeEmpty)

		// A disk mountpoint that should always be present.
		path := "/"
		if runtime.GOOS == "windows" {
			path = `C:\`
		}

		free := diskFree.Get(c, path)
		total := diskTotal.Get(c, path)
		assert.Loosely(t, free, should.BeLessThanOrEqual(total))

		iFree := inodesFree.Get(c, path)
		iTotal := inodesTotal.Get(c, path)
		assert.Loosely(t, iFree, should.BeLessThanOrEqual(iTotal))

		if runtime.GOOS == "darwin" && !cgoEnabled {
			// gopsutil/v3/disk relies on CGO to get disk device stats.
			t.Skip("skipping disk write tests on darwin with CGO_ENABLED=0")
		}

		// Try to get a device from reported metrics. There might be multiple
		// devices reported, pick the one that has Non-Zero value for verification.
		var device string
		for _, cell := range tsmon.Store(c).GetAll(c) {
			if cell.MetricInfo.Name == diskWrite.Info().Name && cell.CellData.Value.(int64) > 0 {
				device = cell.CellData.FieldVals[0].(string)
				break
			}
		}
		assert.Loosely(t, device, should.NotEqual(""))

		assert.Loosely(t, diskRead.Get(c, device), should.BeGreaterThan(0))
		assert.Loosely(t, diskReadCount.Get(c, device), should.BeGreaterThan(0))
		assert.Loosely(t, diskReadTimeSpent.Get(c, device), should.BeGreaterThan(0))

		assert.Loosely(t, diskWrite.Get(c, device), should.BeGreaterThan(0))
		assert.Loosely(t, diskWriteCount.Get(c, device), should.BeGreaterThan(0))
		assert.Loosely(t, diskWriteTimeSpent.Get(c, device), should.BeGreaterThan(0))
	})

	ftt.Run("Memory", t, func(t *ftt.Test) {
		c, _ := tsmon.WithDummyInMemory(context.Background())
		assert.Loosely(t, updateMemoryMetrics(c), should.BeNil)

		free := memFree.Get(c)
		total := memTotal.Get(c)
		assert.Loosely(t, free, should.BeLessThanOrEqual(total))
	})

	ftt.Run("Network", t, func(t *ftt.Test) {
		c, _ := tsmon.WithDummyInMemory(context.Background())
		assert.Loosely(t, updateNetworkMetrics(c), should.BeNil)

		// A network interface that should always be present.
		iface := "lo"
		if runtime.GOOS == "windows" {
			return // TODO(dsansome): Figure out what this is on Windows.
		} else if runtime.GOOS == "darwin" {
			iface = "en0"
		}

		netUp.Get(c, iface)
		netDown.Get(c, iface)
	})

	ftt.Run("Process", t, func(t *ftt.Test) {
		c, _ := tsmon.WithDummyInMemory(context.Background())
		assert.Loosely(t, updateProcessMetrics(c), should.BeNil)
		assert.Loosely(t, procCount.Get(c), should.BeGreaterThan(0))

		if runtime.GOOS != "windows" {
			assert.That(t, loadAverage.Get(c, 1), should.BeGreaterThan[float64](0))
			assert.That(t, loadAverage.Get(c, 5), should.BeGreaterThan[float64](0))
			assert.That(t, loadAverage.Get(c, 15), should.BeGreaterThan[float64](0))
		}
	})

	ftt.Run("Unix time", t, func(t *ftt.Test) {
		c, _ := tsmon.WithDummyInMemory(context.Background())
		assert.Loosely(t, updateUnixTimeMetrics(c), should.BeNil)
		assert.Loosely(t, unixTime.Get(c), should.BeGreaterThan(int64(1257894000000)))
	})

	ftt.Run("OS information", t, func(t *ftt.Test) {
		c, _ := tsmon.WithDummyInMemory(context.Background())
		assert.Loosely(t, updateOSInfoMetrics(c), should.BeNil)

		assert.Loosely(t, osName.Get(c, ""), should.NotEqual(""))
		assert.Loosely(t, osVersion.Get(c, ""), should.NotEqual(""))
		assert.Loosely(t, osArch.Get(c), should.NotEqual(""))
	})
}
