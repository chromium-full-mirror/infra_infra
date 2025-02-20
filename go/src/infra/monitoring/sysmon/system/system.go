// Copyright (c) 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package system

import (
	"context"
	"fmt"
	"runtime"
	"sync"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/tsmon"
	"go.chromium.org/luci/common/tsmon/field"
	"go.chromium.org/luci/common/tsmon/metric"
	"go.chromium.org/luci/common/tsmon/types"
)

var (
	cpuCount = metric.NewInt("dev/cpu/count",
		"Number of CPU cores.",
		nil)
	cpuTime = metric.NewFloat("dev/cpu/time",
		"percentage of time spent by the CPU in different states.",
		nil,
		field.String("mode"))

	diskFree = metric.NewInt("dev/disk/free",
		"Available bytes on disk partition.",
		&types.MetricMetadata{Units: types.Bytes},
		field.String("path"))
	diskTotal = metric.NewInt("dev/disk/total",
		"Total bytes on disk partition.",
		&types.MetricMetadata{Units: types.Bytes},
		field.String("path"))

	inodesFree = metric.NewInt("dev/inodes/free",
		"Number of available inodes on disk partition (unix only).",
		nil,
		field.String("path"))
	inodesTotal = metric.NewInt("dev/inodes/total",
		"Number of possible inodes on disk partition (unix only).",
		nil,
		field.String("path"))

	diskRead = metric.NewCounter("dev/disk/read",
		"Number of Bytes read on disk.",
		&types.MetricMetadata{Units: types.Bytes},
		field.String("disk"))
	diskWrite = metric.NewCounter("dev/disk/write",
		"Number of Bytes written on disk.",
		&types.MetricMetadata{Units: types.Bytes},
		field.String("disk"))
	diskReadCount = metric.NewCounter("dev/disk/io/read_count",
		"The total number of read operations.",
		nil,
		field.String("disk"))
	diskReadTimeSpent = metric.NewCounter("dev/disk/io/read_time_spent",
		"The total number of milliseconds spent by all reads.",
		&types.MetricMetadata{Units: types.Milliseconds},
		field.String("disk"))
	diskWriteCount = metric.NewCounter("dev/disk/io/write_count",
		"The total number of write operations.",
		nil,
		field.String("disk"))
	diskWriteTimeSpent = metric.NewCounter("dev/disk/io/write_time_spent",
		"The total number of milliseconds spent by all writes.",
		&types.MetricMetadata{Units: types.Milliseconds},
		field.String("disk"))

	memFree = metric.NewInt("dev/mem/free",
		"Amount of memory available to a process (in Bytes). Buffers are considered free memory.",
		&types.MetricMetadata{Units: types.Bytes})
	memTotal = metric.NewInt("dev/mem/total",
		"Total physical memory in Bytes.",
		&types.MetricMetadata{Units: types.Bytes})

	netUp = metric.NewCounter("dev/net/bytes/up",
		"Number of bytes sent on interface.",
		&types.MetricMetadata{Units: types.Bytes},
		field.String("interface"))
	netDown = metric.NewCounter("dev/net/bytes/down",
		"Number of Bytes received on interface.",
		&types.MetricMetadata{Units: types.Bytes},
		field.String("interface"))
	netErrUp = metric.NewCounter("dev/net/err/up",
		"Total number of errors when sending (per interface).",
		nil,
		field.String("interface"))
	netErrDown = metric.NewCounter("dev/net/err/down",
		"Total number of errors when receiving (per interface).",
		nil,
		field.String("interface"))
	netDropUp = metric.NewCounter("dev/net/drop/up",
		"Total number of outgoing packets that have been dropped.",
		nil,
		field.String("interface"))
	netDropDown = metric.NewCounter("dev/net/drop/down",
		"Total number of incoming packets that have been dropped.",
		nil,
		field.String("interface"))

	uptime = metric.NewInt("dev/uptime",
		"Machine uptime, in seconds.",
		&types.MetricMetadata{Units: types.Seconds})
	procCount = metric.NewInt("dev/proc/count",
		"Number of processes currently running.",
		nil)
	loadAverage = metric.NewFloat("dev/proc/load_average",
		"Number of processes currently in the system run queue.",
		nil,
		field.Int("minutes"))

	tempAmbient = metric.NewFloat("dev/temperature/ambient",
		"Ambient temperature as reported by the machine.",
		&types.MetricMetadata{Units: types.DegreeCelsiusUnit})
	tempBattery = metric.NewFloat("dev/temperature/battery",
		"Temperature of the machine's battery (if it has one).",
		&types.MetricMetadata{Units: types.DegreeCelsiusUnit})
	tempCPU = metric.NewFloat("dev/temperature/cpu",
		"Temperature of each CPU core.",
		&types.MetricMetadata{Units: types.DegreeCelsiusUnit},
		field.String("core"))

	// tsmon pipeline uses backend clocks when assigning timestamps to metric
	// points. By comparing point timestamp to the point value (i.e. time by
	// machine's local clock), we can potentially detect some anomalies (clock
	// drift, unusually high metrics pipeline delay, completely wrong clocks,
	// etc).
	//
	// It is important to gather this metric right before the flush.
	unixTime = metric.NewInt("dev/unix_time",
		"Number of milliseconds since epoch based on local machine clock.",
		&types.MetricMetadata{Units: types.Milliseconds})

	osName = metric.NewString("proc/os/name",
		"OS name on the machine",
		nil,
		field.String("hostname")) // Legacy hostname field, still required.
	osVersion = metric.NewString("proc/os/version",
		"OS version on the machine",
		nil,
		field.String("hostname")) // Legacy hostname field, still required.
	osArch = metric.NewString("proc/os/arch",
		"OS architecture on this machine",
		nil)
	osKernelVersion = metric.NewString("proc/os/kernel_version",
		"Kernel version on the machine",
		nil)

	lastCPUTimes *cpu.TimesStat

	sysCountersLock      sync.Mutex
	sysCountersInit      = map[sysCounterKey]int64{}
	sysCountersSkipShift = false // set only in tests
)

type sysCounterKey struct {
	counter metric.Counter
	field   string
}

// Register adds tsmon callbacks to set system metrics.
func Register() {
	tsmon.RegisterCallback(func(c context.Context) {
		if err := updateCPUMetrics(c); err != nil {
			logging.Warningf(c, "Failed to update system cpu metrics: %v", err)
		}
		if err := updateDiskMetrics(c); err != nil {
			logging.Warningf(c, "Failed to update system disk metrics: %v", err)
		}
		if err := updateMemoryMetrics(c); err != nil {
			logging.Warningf(c, "Failed to update system memory metrics: %v", err)
		}
		if err := updateNetworkMetrics(c); err != nil {
			logging.Warningf(c, "Failed to update system network metrics: %v", err)
		}
		if err := updateUptimeMetrics(c); err != nil {
			logging.Warningf(c, "Failed to update system uptime metrics: %v", err)
		}
		if err := updateProcessMetrics(c); err != nil {
			logging.Warningf(c, "Failed to update system process metrics: %v", err)
		}
		if err := updateOSInfoMetrics(c); err != nil {
			logging.Warningf(c, "Failed to update OS info metrics: %v", err)
		}
		if err := updateSystemTemps(c); err != nil {
			logging.Warningf(c, "Failed to update system temperatures: %v", err)
		}
		if err := updateUnixTimeMetrics(c); err != nil {
			logging.Warningf(c, "Failed to update unix time metrics: %v", err)
		}
	})
}

func updateCPUMetrics(c context.Context) error {
	if !cgoEnabled && runtime.GOOS == "darwin" {
		panic("CGO_ENABLED=1 is required for sysmon on Mac")
	}

	cpuCounts, err := cpu.Counts(true)
	if err != nil {
		return err
	}
	cpuCount.Set(c, int64(cpuCounts))

	cpuTimes, err := cpu.Times(false)
	if err != nil {
		return err
	}
	if len(cpuTimes) < 1 {
		return status.Errorf(codes.OutOfRange, "cpu.Times(false) returned no entries")
	}

	if lastCPUTimes == nil {
		// Initialize on the first call. Don't report any bogus 0 values though.
		// `cpuTime` is a gauge, not a counter. We'll need to report a value only
		// after we calculate the delta on the next measurement.
		cpy := cpuTimes[0]
		lastCPUTimes = &cpy
		return nil
	}

	user := cpuTimes[0].User - lastCPUTimes.User
	system := cpuTimes[0].System - lastCPUTimes.System
	idle := cpuTimes[0].Idle - lastCPUTimes.Idle
	if total := cpuTimes[0].Total() - lastCPUTimes.Total(); total != 0 {
		user = user / total * 100
		system = system / total * 100
		idle = idle / total * 100
	} else {
		user = 0
		system = 0
		idle = 0
	}
	*lastCPUTimes = cpuTimes[0]

	cpuTime.Set(c, user, "user")
	cpuTime.Set(c, system, "system")
	cpuTime.Set(c, idle, "idle")

	return nil
}

func updateDiskMetrics(c context.Context) errors.MultiError {
	var ret errors.MultiError

	partitions, err := disk.Partitions(false)
	if err != nil {
		ret = append(ret, fmt.Errorf("failed to get list of partitions: %w", err))
	} else {
		for _, part := range partitions {
			if part.Mountpoint == "" || part.Device == "none" || shouldIgnoreFstype(part.Fstype) || shouldIgnoreMountpoint(part.Mountpoint) {
				continue
			}

			usage, err := disk.Usage(part.Mountpoint)
			if err != nil {
				ret = append(ret, fmt.Errorf(
					"failed to get disk usage for partition '%s': %w", part.Mountpoint, err))
				continue
			}
			diskFree.Set(c, int64(usage.Free), part.Mountpoint)
			diskTotal.Set(c, int64(usage.Total), part.Mountpoint)
			inodesFree.Set(c, int64(usage.InodesFree), part.Mountpoint)
			inodesTotal.Set(c, int64(usage.InodesTotal), part.Mountpoint)
		}
	}

	io, err := disk.IOCounters()
	switch {
	case err != nil && err.Error() == "not implemented yet": // ErrNotImplementedError is in an internal package.
		// Not implemented on Darwin.
	case err != nil:
		ret = append(ret, fmt.Errorf("failed to get disk IO counters: %w", err))
	default:
		var devices []string
		for device := range io {
			devices = append(devices, device)
		}
		// Remove, for example, sda if sda1 is in the list.
		devices = removeDiskDevices(devices)

		for _, device := range devices {
			counters := io[device]
			setSysCounter(c, diskRead, int64(counters.ReadBytes), device)
			setSysCounter(c, diskReadCount, int64(counters.ReadCount), device)
			setSysCounter(c, diskReadTimeSpent, int64(counters.ReadTime), device)
			setSysCounter(c, diskWrite, int64(counters.WriteBytes), device)
			setSysCounter(c, diskWriteCount, int64(counters.WriteCount), device)
			setSysCounter(c, diskWriteTimeSpent, int64(counters.WriteTime), device)
		}
	}

	return ret
}

func updateMemoryMetrics(c context.Context) error {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return err
	}
	memFree.Set(c, int64(vm.Available))
	memTotal.Set(c, int64(vm.Total))
	return nil
}

func updateNetworkMetrics(c context.Context) error {
	counts, err := net.IOCounters(true)
	if err != nil {
		return err
	}
	for _, count := range counts {
		setSysCounter(c, netUp, int64(count.BytesSent), count.Name)
		setSysCounter(c, netDown, int64(count.BytesRecv), count.Name)
		setSysCounter(c, netErrUp, int64(count.Errout), count.Name)
		setSysCounter(c, netErrDown, int64(count.Errin), count.Name)
		setSysCounter(c, netDropUp, int64(count.Dropout), count.Name)
		setSysCounter(c, netDropDown, int64(count.Dropin), count.Name)
	}
	return nil
}

func updateUptimeMetrics(c context.Context) error {
	ut, err := host.Uptime()
	if err != nil {
		return err
	}
	uptime.Set(c, int64(ut))
	return nil
}

func updateProcessMetrics(c context.Context) error {
	procs, err := process.Pids()
	if err != nil {
		return err
	}
	procCount.Set(c, int64(len(procs)))

	avg, err := load.Avg()
	switch {
	case err != nil && err.Error() == "not implemented yet": // ErrNotImplementedError is in an internal package.
		// Not implemented on Windows.
	case err != nil:
		return fmt.Errorf("failed to get load average: %w", err)
	default:
		loadAverage.Set(c, avg.Load1, 1)
		loadAverage.Set(c, avg.Load5, 5)
		loadAverage.Set(c, avg.Load15, 15)
	}
	return nil
}

func updateUnixTimeMetrics(c context.Context) error {
	unixTime.Set(c, clock.Get(c).Now().UnixMilli())
	return nil
}

func updateOSInfoMetrics(c context.Context) error {
	platform, version, err := osInformation()
	if err != nil {
		logging.WithError(err).Errorf(c, "Failed to get platform information")
		// Carry on since we still have a useful platform metric to report.
	}

	kernelVersion, err := host.KernelVersionWithContext(c)
	if err != nil {
		logging.WithError(err).Errorf(c, "Failed to get kernel version information")
	}

	osName.Set(c, platform, "")
	osVersion.Set(c, version, "")
	osArch.Set(c, runtime.GOARCH)
	osKernelVersion.Set(c, kernelVersion)
	return err
}

func updateSystemTemps(c context.Context) error {
	model, err := model(c)
	if err != nil {
		return err
	}
	t, err := getTemps(c, model)
	if err != nil {
		return err
	}
	if t.Ambient != nil {
		tempAmbient.Set(c, *t.Ambient)
	}
	if t.Battery != nil {
		tempBattery.Set(c, *t.Battery)
	}
	for _, cpu := range t.CPUs {
		tempCPU.Set(c, cpu.Temperature, cpu.Core)
	}
	return nil
}

// setSysCounter reports a value of an OS counter (a cumulative integer).
//
// It makes sure values start from 0 to conform to tsmon rules for cumulative
// metrics. The monitoring backend will use the reset timestamp (set when the
// sysmon process starts) to correctly integrate values in presence of
// discontinuities.
func setSysCounter(ctx context.Context, counter metric.Counter, val int64, field string) {
	var initVal int64

	if !sysCountersSkipShift {
		var ok bool
		sysCountersLock.Lock()
		if initVal, ok = sysCountersInit[sysCounterKey{counter, field}]; !ok {
			sysCountersInit[sysCounterKey{counter, field}] = val
			initVal = val
		}
		sysCountersLock.Unlock()
	}

	counter.Set(ctx, val-initVal, field)
}
