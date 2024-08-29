// Copyright (c) 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package system

import (
	"context"
	"runtime"
	"testing"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/common/tsmon"
)

func TestMetrics(t *testing.T) {
	t.Parallel()

	t.Run("Uptime", func(t *testing.T) {
		t.Parallel()

		c, _ := tsmon.WithDummyInMemory(context.Background())
		assert.That(t, updateUptimeMetrics(c), should.ErrLike(nil))
		assert.That(t, uptime.Get(c), should.BeGreaterThan[int64](0))
	})

	t.Run("CPU", func(t *testing.T) {
		t.Parallel()

		c, _ := tsmon.WithDummyInMemory(context.Background())
		if !cgoEnabled && runtime.GOOS == "darwin" {
			t.Skip("Requires CGO_ENABLED=1 on Mac")
		}

		assert.That(t, updateCPUMetrics(c), should.ErrLike(nil))
		assert.That(t, cpuCount.Get(c), should.BeGreaterThan[int64](0))

		// Small fudge factor because sometimes this isn't exact.
		const aBitLessThanZero = -0.001
		const oneHundredAndABit = 100.001

		v := cpuTime.Get(c, "user")
		assert.That(t, v, should.BeGreaterThanOrEqual(aBitLessThanZero))
		assert.That(t, v, should.BeLessThanOrEqual(oneHundredAndABit))

		v = cpuTime.Get(c, "system")
		assert.That(t, v, should.BeGreaterThanOrEqual(aBitLessThanZero))
		assert.That(t, v, should.BeLessThanOrEqual(oneHundredAndABit))

		v = cpuTime.Get(c, "idle")
		assert.That(t, v, should.BeGreaterThanOrEqual(aBitLessThanZero))
		assert.That(t, v, should.BeLessThanOrEqual(oneHundredAndABit))
	})

	t.Run("Disk", func(t *testing.T) {
		t.Parallel()

		c, _ := tsmon.WithDummyInMemory(context.Background())
		assert.That(t, updateDiskMetrics(c).AsError(), should.ErrLike(nil))

		// A disk mountpoint that should always be present.
		path := "/"
		if runtime.GOOS == "windows" {
			path = `C:\`
		}

		free := diskFree.Get(c, path)
		total := diskTotal.Get(c, path)
		assert.That(t, free, should.BeLessThanOrEqual(total))

		iFree := inodesFree.Get(c, path)
		iTotal := inodesTotal.Get(c, path)
		assert.That(t, iFree, should.BeLessThanOrEqual(iTotal))

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
		assert.That(t, device, should.NotEqual(""))

		assert.That(t, diskRead.Get(c, device), should.BeGreaterThan[int64](0))
		assert.That(t, diskReadCount.Get(c, device), should.BeGreaterThan[int64](0))
		assert.That(t, diskReadTimeSpent.Get(c, device), should.BeGreaterThan[int64](0))

		assert.That(t, diskWrite.Get(c, device), should.BeGreaterThan[int64](0))
		assert.That(t, diskWriteCount.Get(c, device), should.BeGreaterThan[int64](0))
		assert.That(t, diskWriteTimeSpent.Get(c, device), should.BeGreaterThan[int64](0))
	})

	t.Run("Memory", func(t *testing.T) {
		t.Parallel()

		c, _ := tsmon.WithDummyInMemory(context.Background())
		assert.That(t, updateMemoryMetrics(c), should.ErrLike(nil))

		free := memFree.Get(c)
		total := memTotal.Get(c)
		assert.That(t, free, should.BeLessThanOrEqual(total))
	})

	t.Run("Network", func(t *testing.T) {
		t.Parallel()

		c, _ := tsmon.WithDummyInMemory(context.Background())
		assert.That(t, updateNetworkMetrics(c), should.ErrLike(nil))

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

	t.Run("Process", func(t *testing.T) {
		t.Parallel()

		c, _ := tsmon.WithDummyInMemory(context.Background())
		assert.That(t, updateProcessMetrics(c), should.ErrLike(nil))
		assert.That(t, procCount.Get(c), should.BeGreaterThan[int64](0))

		if runtime.GOOS != "windows" {
			assert.That(t, loadAverage.Get(c, 1), should.BeGreaterThan[float64](0))
			assert.That(t, loadAverage.Get(c, 5), should.BeGreaterThan[float64](0))
			assert.That(t, loadAverage.Get(c, 15), should.BeGreaterThan[float64](0))
		}
	})

	t.Run("Unix time", func(t *testing.T) {
		t.Parallel()

		c, _ := tsmon.WithDummyInMemory(context.Background())
		assert.That(t, updateUnixTimeMetrics(c), should.ErrLike(nil))
		assert.That(t, unixTime.Get(c), should.BeGreaterThan[int64](1257894000000))
	})

	t.Run("OS information", func(t *testing.T) {
		t.Parallel()

		c, _ := tsmon.WithDummyInMemory(context.Background())
		assert.That(t, updateOSInfoMetrics(c), should.ErrLike(nil))

		assert.That(t, osName.Get(c, ""), should.NotEqual(""))
		assert.That(t, osVersion.Get(c, ""), should.NotEqual(""))
		assert.That(t, osArch.Get(c), should.NotEqual(""))
	})
}
