// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"strings"

	"infra/cros/cmd/common_lib/ants/androidbuildinternal/v3"
)

func InvocationSealed(inv *androidbuildinternal.Invocation) bool {
	state := strings.ToUpper(inv.SchedulerState)
	if state == "CANCELLED" || state == "ERROR" || state == "COMPLETED" {
		return true
	}
	return false
}
