// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicelabel

import (
	ufslabconfigpb "infra/unifiedfleet/api/v1/models/chromeos/lab"
)

const trueString = "True"
const falseString = "False"

func peripheralStateToString(ps ufslabconfigpb.PeripheralState) string {
	if ps == ufslabconfigpb.PeripheralState_UNKNOWN || ps == ufslabconfigpb.PeripheralState_NOT_CONNECTED {
		return falseString
	}
	return trueString
}

func boolToSwarmingString(b bool) string {
	if b {
		return trueString
	}
	return falseString
}
