// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"google.golang.org/protobuf/types/known/durationpb"
)

func dolosRepairPlan() *Plan {
	return &Plan{
		CriticalActions: []string{
			"Device is sshable",
			"Update state",
		},
		Actions: map[string]*Action{
			"Device is pingable": {
				ExecName:    "cros_ping",
				ExecTimeout: &durationpb.Duration{Seconds: 15},
			},
			"Device is sshable": {
				Dependencies: []string{
					"Set dolos state:NO_SSH",
					"Device is pingable",
				},
				ExecName:    "cros_ssh",
				ExecTimeout: &durationpb.Duration{Seconds: 30},
			},
			"Update state": {
				ExecName:    "dolos_determine_and_set_dolos_state",
				ExecTimeout: &durationpb.Duration{Seconds: 30},
			},
			"Set dolos state:NO_SSH": {
				ExecName: "dolos_set_dolos_state",
				ExecExtraArgs: []string{
					"state:NO_SSH",
				},
				RunControl:    RunControl_ALWAYS_RUN,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
		},
	}
}
