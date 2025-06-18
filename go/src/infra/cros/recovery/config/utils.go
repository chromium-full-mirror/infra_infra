// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

// removeRecoveries clean up all recovery actions for the all plans.
func removeRecoveries(c *Configuration) {
	for _, p := range c.GetPlans() {
		for _, a := range p.GetActions() {
			a.RecoveryActions = nil
		}
	}
}
