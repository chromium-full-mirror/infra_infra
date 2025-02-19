// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import "infra/fleetconsole/api/fleetconsolerpc"

func NewDateOnly(year int32, month int32, day int32) *fleetconsolerpc.DateOnly {
	return &fleetconsolerpc.DateOnly{
		Year:  year,
		Month: month,
		Day:   day,
	}
}
