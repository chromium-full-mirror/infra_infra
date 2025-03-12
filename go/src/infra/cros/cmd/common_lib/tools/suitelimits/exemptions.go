// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package suitelimits

import (
	"time"
)

type suiteFilter struct {
	suiteName  string
	expiration time.Time
}

var (
	permanentExemption = time.Date(2050, time.July, 30, 0, 0, 0, 0, time.UTC)
)

// exemptions stores all granted exemptions from the SuiteLimits project. go/sl-tracking-sheet for more information.
var exemptions = []suiteFilter{
	{
		suiteName:  "arc-cts-long",
		expiration: permanentExemption,
	},
	{
		suiteName:  "arc-cts-camera-opendut",
		expiration: permanentExemption,
	},
	{
		suiteName:  "arc-cts-hardware",
		expiration: permanentExemption,
	},
	{
		suiteName:  "arc-cts-qual-long",
		expiration: permanentExemption,
	},
	{
		suiteName:  "arc-cts-vm-stable",
		expiration: permanentExemption,
	},
	{
		suiteName:  "arc-cts-vm-stable-long",
		expiration: permanentExemption,
	},
	{
		suiteName:  "arc-gts-long",
		expiration: permanentExemption,
	},
	{
		suiteName:  "arc-gts-qual-long",
		expiration: permanentExemption,
	},
	{
		suiteName:  "arc-sts-full",
		expiration: permanentExemption,
	},
	{
		suiteName:  "arc-sts-full-r",
		expiration: permanentExemption,
	},
	{
		suiteName:  "arc-sts-full-t",
		expiration: permanentExemption,
	},
	{
		suiteName:  "bvt-perbuild",
		expiration: permanentExemption,
	},
	{
		suiteName:  "bvt-tast-arc",
		expiration: permanentExemption,
	},
	{
		suiteName:  "bvt-tast-cq",
		expiration: permanentExemption,
	},
	{
		suiteName:  "bvt-tast-cq-cft-crostini",
		expiration: permanentExemption,
	},
	{
		suiteName:  "bvt-tast-cq-crostini",
		expiration: permanentExemption,
	},
	{
		suiteName:  "bvt-tast-cq-hw",
		expiration: permanentExemption,
	},
	{
		suiteName:  "bvt-tast-criticalstaging",
		expiration: permanentExemption,
	},
	{
		suiteName:  "bvt-tast-informational",
		expiration: permanentExemption,
	},
	{
		suiteName:  "bvt-tast-cq-non-arc-non-crostini",
		expiration: permanentExemption,
	},
	{
		suiteName:  "bvt-tast-parallels-informational",
		expiration: permanentExemption,
	},
	{
		suiteName:  "fieldtrial-testing-config-on-weekly",
		expiration: permanentExemption,
	},
	{
		suiteName:  "crosbolt_perf_nightly",
		expiration: permanentExemption,
	},
	{
		suiteName:  "crosbolt_perf_perbuild",
		expiration: permanentExemption,
	},
	{
		suiteName:  "crosbolt_perf_weekly",
		expiration: permanentExemption,
	},
	{
		suiteName:  "flex-perbuild",
		expiration: permanentExemption,
	},
	{
		suiteName:  "chrome-uprev-hw",
		expiration: permanentExemption,
	},
	{
		suiteName:  "graphics_per-build",
		expiration: permanentExemption,
	},
	{
		suiteName:  "graphics_per-day",
		expiration: permanentExemption,
	},
	{
		suiteName:  "graphics_per-week",
		expiration: permanentExemption,
	},
	{
		suiteName:  "dma-per-build",
		expiration: permanentExemption,
	},
	// Release specific exemptions, giving an extra year of time so the
	// exemption doesn't unexpectedly expire.
	{
		suiteName:  "paygen_au_stable",
		expiration: permanentExemption,
	},
	{
		suiteName:  "paygen_au_dev",
		expiration: permanentExemption,
	},
	{
		suiteName:  "paygen_au_beta",
		expiration: permanentExemption,
	},
	{
		suiteName:  "paygen_au_canary",
		expiration: permanentExemption,
	},
	{
		suiteName:  "cq-medium",
		expiration: permanentExemption,
	},
}
