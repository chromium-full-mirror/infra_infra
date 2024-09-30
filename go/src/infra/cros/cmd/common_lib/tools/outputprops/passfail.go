// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package outputprops sets up the infrastructure to write custom types to the
// LUCIEXE output properties.
package outputprops

import (
	"go.chromium.org/luci/luciexe/build"
)

// The luciexe/build API will handle the actual definition of these functions.
// We just need to define a function pointer to pass in.

// SummaryItem is used in the tracking map to aggregate results for test run
// totals.
type SummaryItem struct {
	TotalTestCount          int
	TotalFailedTestCount    int
	TotalFailedTestRunCount int
}

type SummaryMap map[string]*SummaryItem

var CTPv2PassFail = build.RegisterOutputProperty[SummaryMap]("$ctpv2/passFail")
