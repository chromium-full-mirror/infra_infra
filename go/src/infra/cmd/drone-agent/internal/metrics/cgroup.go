// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// The module of 'github.com/containerd/cgroups' doesn't work on both Mac and
// Windows

//go:build linux
// +build linux

package metrics

import (
	"fmt"

	"github.com/containerd/cgroups"
	stats "github.com/containerd/cgroups/stats/v1"
)

// cgroupStats gets the stats for the root cgroup of the drone.
func cgroupStats() (*stats.Metrics, error) {
	control, err := cgroups.Load(cgroups.V1, cgroups.StaticPath("/"))
	if err != nil {
		return nil, fmt.Errorf("loading root cgroup status: %w", err)
	}
	stats, err := control.Stat(cgroups.IgnoreNotExist)
	if err != nil {
		return nil, fmt.Errorf("loading root cgroup status: %w", err)
	}
	return stats, nil
}
