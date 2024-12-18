// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"google.golang.org/protobuf/types/known/durationpb"
)

// hmrRepairPlan describe the plan to repair human motion robot
func hmrRepairPlan() *Plan {
	return &Plan{
		CriticalActions: []string{
			"Set state:BROKEN",
			"Device is pingable",
			"Device is sshable",
			"Check HMR state with XMLRPC call GetErrors",
			"Set state:WORKING",
		},
		Actions: map[string]*Action{
			"Device is pingable": {
				ExecName:    "cros_ping",
				ExecTimeout: &durationpb.Duration{Seconds: 15},
				// TODO: Recovery with RPM power cycle on the touchhost.
			},
			"Device is sshable": {
				ExecName:    "cros_ssh",
				ExecTimeout: &durationpb.Duration{Seconds: 60},
			},
			"Set state:BROKEN": {
				ExecName:    "set_hmr_state",
				ExecTimeout: &durationpb.Duration{Seconds: 15},
				ExecExtraArgs: []string{
					"state:BROKEN",
				},
				RunControl:    RunControl_RUN_ONCE,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Set state:WORKING": {
				ExecName:    "set_hmr_state",
				ExecTimeout: &durationpb.Duration{Seconds: 15},
				ExecExtraArgs: []string{
					"state:WORKING",
				},
				RunControl:    RunControl_RUN_ONCE,
				MetricsConfig: &MetricsConfig{UploadPolicy: MetricsConfig_SKIP_ALL},
			},
			"Check HMR state with XMLRPC call GetErrors": {
				Docs: []string{
					"Check the state of Touchhost of HMR with XMLRPC API call GetErrors.",
				},
				ExecName: "check_hmr_state",
			},
		},
	}
}
