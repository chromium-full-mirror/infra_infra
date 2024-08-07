// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"testing"
)

func TestTcDepsToSwarmingLabels(t *testing.T) {
	tcDeps := []string{
		"wificell",
		"wifi_state:NORMAL",
		"peripheral_wifi_state:WORKING",
		"bluetooth",
		"bluetooth_state:NORMAL",
	}

	swDeps := tcDepsToSwarmingLabels(tcDeps)

	expected := []string{
		"wificell:True",
		"wifi_state:NORMAL",
		"peripheral_wifi_state:WORKING",
		"bluetooth:True",
		"bluetooth_state:NORMAL",
	}
	for i, k := range swDeps {
		if expected[i] != k {
			t.Fatalf("Transformed swLabels didn't match. Expected: %v, Got: %v", expected[i], k)
		}
	}
}
