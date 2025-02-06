// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

// addEnvActions adds actios for environments.
func addEnvActions(actions map[string]*Action) {
	am := map[string]*Action{
		"Is not a partner side": {
			Docs: []string{
				"Check if the process run for partner.",
				"Detection based on usage namespace of the context.",
			},
			ExecName:   "env_is_not_os_partner_namespace",
			RunControl: RunControl_RUN_ONCE,
		},
		"Execution not on CloudBot": {
			Docs: []string{
				"Check that the process is not running on cloudbot.",
			},
			ExecName:   "env_is_not_cloudbot",
			RunControl: RunControl_RUN_ONCE,
		},
		"Execution on Mobile Harness box": {
			Docs: []string{
				"Check that the process is running on Mobile Harness box.",
			},
			ExecName:   "env_is_mh_box",
			RunControl: RunControl_RUN_ONCE,
		},
		"Execution not on Mobile Harness box": {
			Docs: []string{
				"Check that the process is not running on Mobile Harness box.",
			},
			ExecName:   "env_is_not_mh_box",
			RunControl: RunControl_RUN_ONCE,
		},
	}
	for k, v := range am {
		if _, ok := actions[k]; ok {
			panic("duplicate key:" + k + " in actions map")
		}
		actions[k] = v
	}
}
