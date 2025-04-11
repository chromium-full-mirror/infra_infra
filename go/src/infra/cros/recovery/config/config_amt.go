// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"google.golang.org/protobuf/types/known/durationpb"
)

// amtRepairPlan describes the plan to repair AMT management
func amtRepairPlan() *Plan {
	return &Plan{
		CriticalActions: []string{
			"Set AMT state to BROKEN",
			"Intel AMT is working",
			"Set AMT state to WORKING",
		},
		Actions: map[string]*Action{
			"Intel AMT is working": {
				Conditions: []string{
					"Intel AMT is available",
				},
				ExecName:      "amt_manager_is_healthy",
				ExecTimeout:   &durationpb.Duration{Seconds: 15},
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set AMT state to BROKEN": {
				ExecName: "amt_manager_set_state",
				ExecExtraArgs: []string{
					"state:BROKEN",
				},
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set AMT state to WORKING": {
				ExecName: "amt_manager_set_state",
				ExecExtraArgs: []string{
					"state:WORKING",
				},
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Intel AMT is available": {
				Docs: []string{
					"Ping the AMT interface until it responds to pings.",
				},
				ExecName:      "amt_manager_amt_available",
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
				RunControl:    RunControl_ALWAYS_RUN,
				ExecTimeout:   &durationpb.Duration{Seconds: 91},
			},
		},
	}
}
