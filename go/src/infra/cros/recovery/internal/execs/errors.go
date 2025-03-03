// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package execs

import (
	"go.chromium.org/luci/common/errors/errtag"
)

var (
	// Error tag to track error with request to start critical actions over.
	PlanStartOverTag = errtag.Make("plan-start-over", true)

	// Error tag to track error with request to stop execution of the current plan.
	PlanAbortTag = errtag.Make("plan-abort", true)
)
