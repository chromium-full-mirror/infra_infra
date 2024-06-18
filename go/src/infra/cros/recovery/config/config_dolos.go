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
			"Device is pingable",
			"Device is sshable",
			"Update state",
		},
		Actions: map[string]*Action{
			"Device is pingable": {
				ExecName:    "cros_ping",
				ExecTimeout: &durationpb.Duration{Seconds: 15},
			},
			"Device is sshable": {
				ExecName:    "cros_ssh",
				ExecTimeout: &durationpb.Duration{Seconds: 30},
			},
			"Update state": {
				ExecName:    "set_dolos_state",
				ExecTimeout: &durationpb.Duration{Seconds: 30},
			},
		},
	}
}
