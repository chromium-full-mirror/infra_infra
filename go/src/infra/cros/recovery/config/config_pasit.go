// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

// pasitRepairPlan includes actions to repair PASIT testbeds.
func pasitRepairPlan() *Plan {
	return &Plan{
		CriticalActions: []string{"sample_pass"},
		Actions: map[string]*Action{
			"sample_pass": {
				ExecName: "sample_pass",
			},
		},
	}
}
