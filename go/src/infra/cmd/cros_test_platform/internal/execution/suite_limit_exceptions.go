// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package execution

import (
	"time"
)

type suiteFilter struct {
	suiteName  string
	expiration time.Time
}

var (
	// After end of Q3
	standardExemption = time.Date(2025, time.January, 20, 0, 0, 0, 0, time.UTC)

	// Date beyond the lifetime of this builder to ensure no lapse in coverage
	releaseLongTerm = time.Date(2025, time.January, 30, 0, 0, 0, 0, time.UTC)
)

// exceptions stores all granted exceptions from the SuiteLimits project. go/sl-tracking-sheet for more information.
var exceptions = []suiteFilter{
	{
		suiteName:  "arc-cts-long",
		expiration: standardExemption,
	},
	{
		suiteName:  "arc-cts-camera-opendut",
		expiration: standardExemption,
	},
	{
		suiteName:  "arc-cts-hardware",
		expiration: standardExemption,
	},
	{
		suiteName:  "arc-cts-qual-long",
		expiration: standardExemption,
	},
	{
		suiteName:  "arc-cts-vm-stable",
		expiration: standardExemption,
	},
	{
		suiteName:  "arc-cts-vm-stable-long",
		expiration: standardExemption,
	},
	{
		suiteName:  "arc-gts-long",
		expiration: standardExemption,
	},
	{
		suiteName:  "arc-gts-qual-long",
		expiration: standardExemption,
	},
	{
		suiteName:  "arc-sts-full",
		expiration: standardExemption,
	},
	{
		suiteName:  "arc-sts-full-r",
		expiration: standardExemption,
	},
	{
		suiteName:  "arc-sts-full-t",
		expiration: standardExemption,
	},
	{
		suiteName:  "bvt-perbuild",
		expiration: standardExemption,
	},
	{
		suiteName:  "bvt-tast-arc",
		expiration: standardExemption,
	},
	{
		suiteName:  "bvt-tast-cq",
		expiration: standardExemption,
	},
	{
		suiteName:  "bvt-tast-cq-cft-crostini",
		expiration: standardExemption,
	},
	{
		suiteName:  "bvt-tast-cq-crostini",
		expiration: standardExemption,
	},
	{
		suiteName:  "bvt-tast-cq-hw",
		expiration: standardExemption,
	},
	{
		suiteName:  "bvt-tast-criticalstaging",
		expiration: standardExemption,
	},
	{
		suiteName:  "bvt-tast-informational",
		expiration: standardExemption,
	},
	{
		suiteName:  "bvt-tast-cq-non-arc-non-crostini",
		expiration: standardExemption,
	},
	{
		suiteName:  "bvt-tast-parallels-informational",
		expiration: standardExemption,
	},
	{
		suiteName:  "fieldtrial-testing-config-on-weekly",
		expiration: standardExemption,
	},
	{
		suiteName:  "crosbolt_perf_nightly",
		expiration: standardExemption,
	},
	{
		suiteName:  "crosbolt_perf_perbuild",
		expiration: standardExemption,
	},
	{
		suiteName:  "crosbolt_perf_weekly",
		expiration: standardExemption,
	},
	{
		suiteName:  "flex-perbuild",
		expiration: standardExemption,
	},
	{
		suiteName:  "chrome-uprev-hw",
		expiration: standardExemption,
	},
	{
		suiteName:  "graphics_per-build",
		expiration: standardExemption,
	},
	{
		suiteName:  "graphics_per-day",
		expiration: standardExemption,
	},
	{
		suiteName:  "graphics_per-week",
		expiration: standardExemption,
	},
	{
		suiteName:  "dma-per-build",
		expiration: standardExemption,
	},
	// Release specific exemptions, giving an extra year of time so the
	// exemption doesn't unexpectedly expire.
	{
		suiteName:  "paygen_au_stable",
		expiration: releaseLongTerm,
	},
	{
		suiteName:  "paygen_au_dev",
		expiration: releaseLongTerm,
	},
	{
		suiteName:  "paygen_au_beta",
		expiration: releaseLongTerm,
	},
	{
		suiteName:  "paygen_au_canary",
		expiration: releaseLongTerm,
	},
	{
		suiteName:  "cq-medium",
		expiration: standardExemption,
	},
}
