// Copyright 2022 The Chrmium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package metrics gathers and report drone host performance data.
package metrics

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	stats "github.com/containerd/cgroups/stats/v1"

	"go.chromium.org/luci/common/tsmon"
	"go.chromium.org/luci/common/tsmon/metric"
	"go.chromium.org/luci/common/tsmon/target"
	"go.chromium.org/luci/common/tsmon/types"
)

var (
	cpuLimit = metric.NewFloat("chromeos/skylab/drone_agent/cpu/limit",
		"The CPU count of a drone can use (e.g. 2 means 2 CPU). '-1' means no limit.",
		nil)
	cpuThrottledCount = metric.NewCounter("chromeos/skylab/drone_agent/cpu/throttled_count",
		"The count that drone CPUs have been throttled.",
		nil)
	cpuThrottledTimeSecond = metric.NewFloatCounter("chromeos/skylab/drone_agent/cpu/throttled_time",
		"The total time duration (in seconds) that drone CPUs have been throttled.",
		&types.MetricMetadata{Units: types.Seconds})
	cpuTimeSecond = metric.NewFloatCounter("chromeos/skylab/drone_agent/cpu/time",
		"The total time duration (in seconds) spent by drone CPUs in user+sys state.",
		&types.MetricMetadata{Units: types.Seconds})

	memLimitBytes = metric.NewInt("chromeos/skylab/drone_agent/memory/limit",
		"The memory limit of a drone (in bytes). '-1' means no limit.",
		&types.MetricMetadata{Units: types.Bytes})
	memFailCount = metric.NewCounter("chromeos/skylab/drone_agent/memory/fail_count",
		"The count of memory usage hits limits.",
		nil)
	memUsageBytes = metric.NewInt("chromeos/skylab/drone_agent/memory/usage",
		"The memory usage of a drone (in bytes).",
		&types.MetricMetadata{Units: types.Bytes})
)

// Setup sets up the metrics.
func Setup(ctx context.Context, tsmonEndpoint, tsmonCredentialPath string) error {
	log.Printf("Setting up tsmon...")
	fl := tsmon.NewFlags()
	fl.Endpoint = tsmonEndpoint
	fl.Credentials = tsmonCredentialPath
	fl.Flush = tsmon.FlushAuto
	fl.Target.SetDefaultsFromHostname()
	fl.Target.TargetType = target.TaskType
	fl.Target.TaskServiceName = "drone-agent"
	fl.Target.TaskJobName = "drone-agent"

	if err := tsmon.InitializeFromFlags(ctx, &fl); err != nil {
		return fmt.Errorf("metrics: setup tsmon: %w", err)
	}

	// The registered functions run automatically when tsmon flush every time.
	tsmon.RegisterCallback(func(c context.Context) {
		m, err := cgroupStats()
		if err != nil {
			log.Printf("Failed to get cgroup stats: %s", err)
			return
		}
		if err := updateCPUMetrics(c, m); err != nil {
			log.Printf("Failed to update drone CPU metrics: %s", err)
		}
		if err := updateMemoryMetrics(c, m); err != nil {
			log.Printf("Failed to update drone memory metrics: %s", err)
		}
	})
	return nil
}

// Shutdown stops the metrics.
func Shutdown(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	log.Printf("Shutting down metrics...")
	tsmon.Shutdown(ctx)
}

// updateCPUMetrics updates the drone CPU metrics.
func updateCPUMetrics(c context.Context, m *stats.Metrics) error {
	cpu := m.CPU
	if cpu == nil {
		return fmt.Errorf("update CPU metrics: no CPU metrics available")
	}
	l, err := cgroupCPULimit()
	if err != nil {
		return fmt.Errorf("update CPU metrics: %w", err)
	}
	cpuLimit.Set(c, l)
	cpuThrottledCount.Set(c, int64(cpu.Throttling.ThrottledPeriods))
	cpuThrottledTimeSecond.Set(c, float64(cpu.Throttling.ThrottledTime)*time.Nanosecond.Seconds())
	cpuTimeSecond.Set(c, float64(cpu.Usage.Total)*time.Nanosecond.Seconds())
	return nil
}

// cgroupCPULimit gets the CPU limit.
//
// The module of github.com/containerd/cgroups doesn't provide the support of
// CPU limit by Jun 2022, so we do it by ourselves.
//
// The limit value is the ratio of quota / period.
// It returns -1 to indicate there's no limit.
func cgroupCPULimit() (float64, error) {
	quota, err := parseIntFromCgroupFile("cpu", "cpu.cfs_quota_us")
	if err != nil {
		return 0, fmt.Errorf("cgroup CPU limit: %w", err)
	}
	if quota == -1 { // No limit on CPU.
		return -1, nil
	}
	period, err := parseIntFromCgroupFile("cpu", "cpu.cfs_period_us")
	if err != nil {
		return 0, fmt.Errorf("cgroup CPU limit: %w", err)
	}
	return float64(quota) / float64(period), nil
}

const baseDir = "/sys/fs/cgroup"

func parseIntFromCgroupFile(controller, controlFile string) (int64, error) {
	f := path.Join(baseDir, controller, controlFile)
	c, err := os.ReadFile(f)
	if err != nil {
		return 0, fmt.Errorf("parse int from cgroup file '%s/%s': %w", controller, controlFile, err)
	}

	v, err := strconv.ParseInt(strings.TrimRight(string(c), "\n"), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse int from cgroup file '%s/%s': %w", controller, controlFile, err)
	}
	return v, nil
}

// updateMemoryMetrics updates the drone memory metrics.
func updateMemoryMetrics(c context.Context, m *stats.Metrics) error {
	mem := m.Memory
	if mem == nil {
		return fmt.Errorf("update memory metrics: no memory metrics available")
	}
	usage := mem.Usage
	if usage == nil {
		return fmt.Errorf("update memory metrics: no usage information available")
	}
	memLimitBytes.Set(c, int64(usage.Limit))
	memFailCount.Set(c, int64(usage.Failcnt))
	// We need to subtract the cache memory size from the total usage to get the
	// working set memory usage. This is also how 'docker stat' and K8s metrics
	// does.
	memUsageBytes.Set(c, int64(usage.Usage-mem.TotalInactiveFile))
	return nil
}
