// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"testing"
)

// TestStringification tests the zero but not nil string.
//
// The expansion is actually not very friendly, but it *is* comprehensible.
// TODO(gregorynisbet): make the friendly string friendlier.
func TestStringification(t *testing.T) {
	t.Parallel()

	str := (&IndicatorAttribute{}).String()

	if str != "type=INDICATOR_TYPE_UNKNOWN primary= secondary= tertiary= loc=LOCATION_UNKNOWN" {
		t.Errorf("unexpcted friendly string %q", str)
	}
}
