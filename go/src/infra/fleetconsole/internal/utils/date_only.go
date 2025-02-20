// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"cloud.google.com/go/civil"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
)

func NewDateOnly(year int32, month int32, day int32) *fleetconsolerpc.DateOnly {
	return &fleetconsolerpc.DateOnly{
		Year:  year,
		Month: month,
		Day:   day,
	}
}

func FromCivilDate(civilDate civil.Date) *fleetconsolerpc.DateOnly {
	return &fleetconsolerpc.DateOnly{
		Year:  int32(civilDate.Year),
		Month: int32(civilDate.Month),
		Day:   int32(civilDate.Day),
	}
}
