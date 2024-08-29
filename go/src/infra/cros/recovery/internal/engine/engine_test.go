// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package engine

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"infra/cros/recovery/config"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/logger/metrics"
)

// Predefined exec functions.
const (
	exec_pass = "sample_pass"
	exec_fail = "sample_fail"
)

var planTestCases = []struct {
	name       string
	got        *config.Plan
	expSuccess bool
}{
	{
		"simple",
		&config.Plan{},
		true,
	},
	{
		"critical action fail",
		&config.Plan{
			CriticalActions: []string{
				"a1",
				"a2",
			},
			Actions: map[string]*config.Action{
				"a1": {
					ExecName: exec_pass,
				},
				"a2": {
					ExecName: exec_fail,
				},
			},
		},
		false,
	},
	{
		"allowed critical action fail",
		&config.Plan{
			AllowFail: true,
			CriticalActions: []string{
				"a1",
				"a2",
			},
			Actions: map[string]*config.Action{
				"a1": {
					ExecName: exec_pass,
				},
				"a2": {
					ExecName: exec_fail,
				},
			},
		},
		true,
	},
	{
		"skip fail action as not applicable",
		&config.Plan{
			CriticalActions: []string{
				"a1",
			},
			Actions: map[string]*config.Action{
				"a1": {
					ExecName:   exec_fail,
					Conditions: []string{"c1"},
				},
				"c1": {
					ExecName: exec_fail,
				},
			},
		},
		true,
	},
	{
		"skip fail dependency as not applicable",
		&config.Plan{
			CriticalActions: []string{
				"a1",
			},
			Actions: map[string]*config.Action{
				"a1": {
					ExecName:     exec_pass,
					Dependencies: []string{"d1"},
				},
				"d1": {
					ExecName:   exec_fail,
					Conditions: []string{"c1"},
				},
				"c1": {
					ExecName: exec_fail,
				},
			},
		},
		true,
	},
	{
		"fail action by dependencies",
		&config.Plan{
			CriticalActions: []string{
				"a1",
			},
			Actions: map[string]*config.Action{
				"a1": {
					ExecName:     exec_pass,
					Dependencies: []string{"d1"},
				},
				"d1": {
					ExecName: exec_fail,
				},
			},
		},
		false,
	},
	{
		"success run",
		&config.Plan{
			CriticalActions: []string{
				"a1",
			},
			Actions: map[string]*config.Action{
				"a1": {
					ExecName:     exec_pass,
					Conditions:   []string{"c1"},
					Dependencies: []string{"d1"},
				},
				"c1": {
					ExecName:     exec_pass,
					Dependencies: []string{"d2"},
				},
				"d1": {
					ExecName:     exec_pass,
					Dependencies: []string{"d2"},
				},
				"d2": {
					ExecName: exec_pass,
				},
			},
		},
		true,
	},
	{
		"skip fail action when allowed to fail",
		&config.Plan{
			CriticalActions: []string{
				"a1",
			},
			Actions: map[string]*config.Action{
				"a1": {
					ExecName:               exec_fail,
					AllowFailAfterRecovery: true,
				},
			},
		},
		true,
	},
	{
		"skip fail action by dependencies when allowed to fail",
		&config.Plan{
			CriticalActions: []string{
				"a1",
			},
			Actions: map[string]*config.Action{
				"a1": {
					ExecName:               exec_pass,
					Dependencies:           []string{"d1"},
					AllowFailAfterRecovery: true,
				},
				"d1": {
					ExecName: exec_fail,
				},
			},
		},
		true,
	},
}

func TestRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, c := range planTestCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			// t.Parallel() -- TODO(gregorynisbet): Consider parallelizing.
			args := &execs.RunArgs{
				EnableRecovery: true,
			}
			err := Run(ctx, c.name, c.got, args, nil)
			if c.expSuccess {
				if err != nil {
					t.Errorf("Case %q fail but expected to pass. Received error: %s", c.name, err)
				}
			} else {
				if err == nil {
					t.Errorf("Case %q expected to fail but pass", c.name)
				}
			}
		})
	}
}

func TestRunPlanDoNotRunActionAsResultInCache(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := recoveryEngine{
		plan: &config.Plan{
			CriticalActions: []string{"a"},
			Actions: map[string]*config.Action{
				"a": {},
			},
		},
		args: &execs.RunArgs{},
	}
	r.initCache()
	r.cacheActionResult("a", nil)
	err := r.runPlan(ctx)
	if err != nil {
		t.Errorf("Expected plan pass as single action cached with result=nil. Received error: %s", err)
	}
}

var recoveryTestCases = []struct {
	name         string
	got          map[string]*config.Action
	expStartOver bool
}{
	{
		"no recoveries",
		map[string]*config.Action{
			"a": {
				RecoveryActions: nil,
			},
		},
		false,
	},
	{
		"recoveries stopped on passed r2 and create request to start over",
		map[string]*config.Action{
			"a": {
				RecoveryActions: []string{"r1", "r2", "r3"},
			},
			"r1": {
				ExecName: exec_fail,
			},
			"r2": {
				ExecName: exec_pass,
			},
			"r3": {}, //Should not reached
		},
		true,
	},
	{
		"recoveries fail but the process still pass",
		map[string]*config.Action{
			"a": {
				RecoveryActions: []string{"r1", "r2", "r3"},
			},
			"r1": {
				ExecName: exec_fail,
			},
			"r2": {
				ExecName: exec_fail,
			},
			"r3": {
				ExecName: exec_fail,
			},
		},
		false,
	},
}

func TestRunRecovery(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, c := range recoveryTestCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			r := recoveryEngine{
				plan: &config.Plan{
					Actions: c.got,
				},
			}
			r.initCache()
			err := r.runRecoveries(ctx, "a", nil)
			if c.expStartOver {
				if !execs.PlanStartOverTag.In(err) {
					t.Errorf("Case %q expected to get request to start over. Received: %s", c.name, err)
				}
			} else {
				if err != nil {
					t.Errorf("Case %q expected to receive nil. Received error: %s", c.name, err)
				}
			}
		})
	}
}

var runExecTestCases = []struct {
	name           string
	enableRecovery bool
	got            map[string]*config.Action
	expError       bool
	expStartOver   bool
}{
	{
		"do not start recovery flow if action passed",
		true,
		map[string]*config.Action{
			"a": {
				ExecName: exec_pass,
				// Will fail if reached any recovery actions.
				RecoveryActions: []string{"r11"},
			},
		},
		false,
		false,
	},
	{
		"do not start recovery flow if it is not allowed",
		false,
		map[string]*config.Action{
			"a": {
				ExecName: exec_fail,
				// Will fail if reached any recovery actions.
				RecoveryActions: []string{"r21"},
			},
		},
		true,
		false,
	},
	{
		"receive start over request after run successful recovery action",
		true,
		map[string]*config.Action{
			"a": {
				ExecName:        exec_fail,
				RecoveryActions: []string{"r31"},
			},
			"r31": {
				ExecName: exec_pass,
			},
		},
		true,
		true,
	},
	{
		"receive error after try recovery action",
		true,
		map[string]*config.Action{
			"a": {
				ExecName:        exec_fail,
				RecoveryActions: []string{"r41"},
			},
			"r41": {
				ExecName: exec_fail,
			},
		},
		true,
		false,
	},
}

func TestActionExec(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, c := range runExecTestCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			r := recoveryEngine{
				plan: &config.Plan{
					Actions: c.got,
				},
			}
			r.initCache()
			err := r.runActionExec(ctx, "a", nil, c.enableRecovery)
			if c.expError && c.expStartOver {
				if !execs.PlanStartOverTag.In(err) {
					t.Errorf("Case %q expected to get request to start over. Received error: %s", c.name, err)
				}
			} else if c.expError {
				if err == nil {
					t.Errorf("Case %q expected to fail by passed", c.name)
				}
			} else {
				if err != nil {
					t.Errorf("Case %q expected to receive nil. Received error: %s", c.name, err)
				}
			}
		})
	}
}

var actionResultsCacheTestCases = []struct {
	name       string
	got        map[string]*config.Action
	expInCashe bool
	expError   bool
}{
	{
		"set pass to the cache",
		map[string]*config.Action{
			"a": {
				ExecName: exec_pass,
			},
		},
		true,
		false,
	},
	{
		"do not set pass to the cache when run_control:run_always",
		map[string]*config.Action{
			"a": {
				ExecName:   exec_pass,
				RunControl: config.RunControl_ALWAYS_RUN,
			},
		},
		false,
		false,
	},
	{
		"set fail to the cache",
		map[string]*config.Action{
			"a": {
				ExecName: exec_fail,
			},
		},
		true,
		true,
	},
	{
		"do not set if recovery finished with success",
		map[string]*config.Action{
			"a": {
				ExecName:        exec_fail,
				RecoveryActions: []string{"r"},
			},
			"r": {
				ExecName: exec_pass,
			},
		},
		false,
		false,
	},
	{
		"set fail when all recoveries failed",
		map[string]*config.Action{
			"a": {
				ExecName:        exec_fail,
				RecoveryActions: []string{"r"},
			},
			"r": {
				ExecName: exec_fail,
			},
		},
		true,
		true,
	},
	{
		"do not set pass to cache when all recoveries failed and action has run_control:run_always",
		map[string]*config.Action{
			"a": {
				ExecName:        exec_fail,
				RecoveryActions: []string{"r"},
				RunControl:      config.RunControl_ALWAYS_RUN,
			},
			"r": {
				ExecName: exec_fail,
			},
		},
		false,
		false,
	},
}

func TestActionExecCache(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, c := range actionResultsCacheTestCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			r := recoveryEngine{
				plan: &config.Plan{
					Actions: c.got,
				},
			}
			r.initCache()
			r.runActionExec(ctx, "a", nil, true)
			err, ok := r.actionResultFromCache("a")
			if c.expInCashe {
				if !ok {
					t.Errorf("Case %q: action result in not in the cache", c.name)
				}
				if c.expError && err == nil {
					t.Errorf("Case %q: expected has error as action result but got nil", c.name)
				} else if !c.expError && err != nil {
					t.Errorf("Case %q: expected do not have error as action result but got it: %s", c.name, err)
				}
			} else {
				if ok {
					t.Errorf("Case %q: does not expected result in the cache", c.name)
				}
			}
		})
	}
}

var resetCacheTestCases = []struct {
	name    string
	got     map[string]config.RunControl
	present []string
	removed []string
}{
	{
		"clean all",
		map[string]config.RunControl{
			"a1": config.RunControl_RERUN_AFTER_RECOVERY,
			"a2": config.RunControl_RERUN_AFTER_RECOVERY,
			"a3": config.RunControl_RERUN_AFTER_RECOVERY,
			"a4": config.RunControl_RERUN_AFTER_RECOVERY,
		},
		nil,
		[]string{"a1", "a2", "a3", "a4"},
	},
	{
		"partially clean up",
		map[string]config.RunControl{
			"a1": config.RunControl_RUN_ONCE,
			"a2": config.RunControl_RUN_ONCE,
			"a3": config.RunControl_RERUN_AFTER_RECOVERY,
			"a4": config.RunControl_RERUN_AFTER_RECOVERY,
		},
		[]string{"a1", "a2"},
		[]string{"a3", "a4"},
	},
}

func TestResetCacheAfterSuccessfulRecoveryAction(t *testing.T) {
	t.Parallel()
	for _, c := range resetCacheTestCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			actions := make(map[string]*config.Action)
			for name, rc := range c.got {
				actions[name] = &config.Action{
					RunControl: rc,
				}
			}
			r := recoveryEngine{
				plan: &config.Plan{
					Actions: actions,
				},
			}
			r.initCache()
			for name := range c.got {
				r.cacheActionResult(name, nil)
			}
			r.resetCacheAfterSuccessfulRecoveryAction()
			for _, name := range c.present {
				if _, ok := r.actionResultFromCache(name); !ok {
					t.Errorf("Case %q: expected to have result for action %q in the cache", c.name, name)
				}
			}
			for _, name := range c.removed {
				if _, ok := r.actionResultFromCache(name); ok {
					t.Errorf("Case %q: not expected to have result for action %q in the cache", c.name, name)
				}
			}
		})
	}
}

var setCacheTestCases = []struct {
	name string
	got  config.RunControl
	exp  bool
}{
	{
		"run once",
		config.RunControl_RUN_ONCE,
		true,
	},
	{
		"rerun after recovery",
		config.RunControl_RERUN_AFTER_RECOVERY,
		true,
	},
	{
		"always run",
		config.RunControl_ALWAYS_RUN,
		false,
	},
}

func TestCacheActionResult(t *testing.T) {
	t.Parallel()
	for _, c := range setCacheTestCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			r := recoveryEngine{
				plan: &config.Plan{
					Actions: map[string]*config.Action{
						"a": {
							RunControl: c.got,
						},
					},
				},
			}
			r.initCache()
			r.cacheActionResult("a", nil)
			_, ok := r.actionResultFromCache("a")
			if c.exp {
				if !ok {
					t.Errorf("Case %q: expected to have result but not present in cache", c.name)
				}
			} else {
				if ok {
					t.Errorf("Case %q: not expected to have result but present in cache", c.name)
				}
			}
		})
	}
}

var isRecoveryUsageTestCases = []struct {
	name          string
	actionCache   []string
	recoveryCache []recoveryUsageKey
	used          bool
}{
	{
		"not used",
		[]string{"a", "b"},
		[]recoveryUsageKey{
			{
				action:   "a",
				recovery: "a",
			},
			{
				action:   "a",
				recovery: "b",
			},
			{
				action:   "b",
				recovery: "a",
			},
			{
				action:   "b",
				recovery: "r",
			},
		},
		false,
	},
	{
		"used by action result",
		[]string{"r"},
		nil,
		true,
	},
	{
		"used by recovery result from other action",
		nil,
		[]recoveryUsageKey{
			{
				action:   "a",
				recovery: "r",
			},
		},
		true,
	},
}

func TestRecoveryCachePersistence(t *testing.T) {
	t.Parallel()
	for _, c := range isRecoveryUsageTestCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			r := recoveryEngine{
				plan: &config.Plan{
					Actions: map[string]*config.Action{
						"a": {},
						"b": {},
						"r": {},
					},
				},
			}
			r.initCache()
			for _, name := range c.actionCache {
				r.cacheActionResult(name, nil)
			}
			for _, k := range c.recoveryCache {
				r.registerRecoveryUsage(k.action, k.recovery, nil)
			}
			if r.isRecoveryUsed("a", "r") != c.used {
				t.Errorf("Case %q before rest: expectaton did not matche expectations: Expected: %v, Got: %v", c.name, c.used, !c.used)
			}
			r.resetCacheAfterSuccessfulRecoveryAction()
			if r.isRecoveryUsed("a", "r") != c.used {
				t.Errorf("Case %q after reset: expectaton did not matche expectations: Expected: %v, Got: %v", c.name, c.used, !c.used)
			}
		})
	}
}

// TestCallMetricsInSimplePlan tests that calling a simple plan with a fake implementation of a metrics interface calls the metrics implementation.
func TestCallMetricsInEmptyPlan(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := newFakeMetrics()
	r := &recoveryEngine{
		planName: "2e9aa66a-5fa1-4eaa-933c-eee8e4337823",
		metricSaver: func(metric *metrics.Action) error {
			return m.Create(ctx, metric)
		},
	}
	var zero time.Time
	expected := []*metrics.Action{
		{
			ActionKind: "plan:2e9aa66a-5fa1-4eaa-933c-eee8e4337823",
			Status:     "success",
			Observations: []*metrics.Observation{
				{MetricKind: "restarts", ValueType: "number", Value: "0"},
				{MetricKind: "started_recoveries", ValueType: "number", Value: "0"},
			},
		},
	}
	r.plan = &config.Plan{
		Actions: map[string]*config.Action{},
	}
	r.args = &execs.RunArgs{
		Metrics: m,
	}
	err := r.runPlan(ctx)
	// TODO(gregorynisbet): Mock the time.Now() function everywhere instead of removing times
	// from test cases.
	for i := range m.actions {
		m.actions[i].StartTime = zero
	}
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if diff := cmp.Diff(expected, m.actions); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
}

// TestCallMetricsWithNonexistentAction tests that calling a simple plan with nonexistent action and a fake implementation of a metrics interface calls the metrics implementation.
func TestCallMetricsWithNonexistentAction(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := newFakeMetrics()
	r := &recoveryEngine{
		planName: "2e9aa66a-5fa1-4eaa-933c-eee8e4337823",
		metricSaver: func(metric *metrics.Action) error {
			return m.Create(ctx, metric)
		},
	}
	var zero time.Time
	expected := []*metrics.Action{
		{
			Name:       "unsaved:a",
			ActionKind: "action:a",
			Status:     "fail",
			StartTime:  zero,
			StopTime:   zero,
			Observations: []*metrics.Observation{
				{MetricKind: "action_type", ValueType: "string", Value: "verifier"},
				{MetricKind: "parent_action_name", ValueType: "string", Value: "plan"},
				{MetricKind: "action_level", ValueType: "number", Value: "0"},
				{MetricKind: "exec_execution_sec", ValueType: "number", Value: "1"},
				{MetricKind: "plan_run_tally", ValueType: "number", Value: "0"},
			},
			AllowFail:  "no-allow-fail",
			Type:       "verifier",
			FailReason: `run action "a": run action "a" exec: run exec "a" with timeout 1m0s: exec "a": not found`,
		},
		{
			ActionKind: "plan:2e9aa66a-5fa1-4eaa-933c-eee8e4337823",
			Status:     "fail",
			Observations: []*metrics.Observation{
				{MetricKind: "restarts", ValueType: "number", Value: "0"},
				{MetricKind: "started_recoveries", ValueType: "number", Value: "0"},
			},
			FailReason: `run plan "2e9aa66a-5fa1-4eaa-933c-eee8e4337823": run actions: run action "a": run action "a" exec: run exec "a" with timeout 1m0s: exec "a": not found`,
		},
	}
	r.plan = &config.Plan{
		CriticalActions: []string{
			"a",
		},
		Actions: map[string]*config.Action{
			"a": {ExecName: "a"},
		},
	}
	r.args = &execs.RunArgs{
		Metrics: m,
	}
	r.initCache()
	err := r.runPlan(ctx)
	// TODO(gregorynisbet): Mock the time.Now() function everywhere instead of removing times
	// from test cases.
	for i := range m.actions {
		m.actions[i].StartTime = zero
		for _, o := range m.actions[i].Observations {
			// Set time observation if present as they can be flaky.
			switch o.MetricKind {
			case "exec_execution_sec":
				o.Value = "1"
			case "exec_execution":
				o.Value = "2"
			}
		}
	}
	expectedErrorMessage := "run plan \"2e9aa66a-5fa1-4eaa-933c-eee8e4337823\": run actions: run action \"a\": run action \"a\" exec: run exec \"a\" with timeout 1m0s: exec \"a\": not found"
	if err == nil {
		t.Errorf("expected error but not received: %s", err)
	} else if err.Error() != expectedErrorMessage {
		t.Errorf("error message does not match: %s", cmp.Diff(expectedErrorMessage, err.Error()))
	}
	if diff := cmp.Diff(expected, m.actions); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
}

// TestCallMetricsWithExistentAction tests that calling a simple plan with action and a fake implementation of a metrics interface calls the metrics implementation.
func TestCallMetricsWithExistentAction(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := newFakeMetrics()
	r := &recoveryEngine{
		planName: "2e9aa66a-5fa1-4eaa-933c-eee8e4337823",
		metricSaver: func(metric *metrics.Action) error {
			return m.Create(ctx, metric)
		},
	}
	var zero time.Time
	expected := []*metrics.Action{
		{
			Name:       "unsaved:r",
			ActionKind: "action:r",
			StartTime:  zero,
			StopTime:   zero,
			Status:     "success",
			FailReason: "",
			Observations: []*metrics.Observation{
				{MetricKind: "action_type", ValueType: "string", Value: "recovery"},
				{MetricKind: "parent_action_name", ValueType: "string", Value: "a"},
				{MetricKind: "action_level", ValueType: "number", Value: "0"},
				{MetricKind: "exec_execution_sec", ValueType: "number", Value: "1"},
				{MetricKind: "plan_run_tally", ValueType: "number", Value: "0"},
			},
			AllowFail: "no-allow-fail",
			PlanName:  "",
			Type:      "recovery",
		},
		{
			Name:       "unsaved:a",
			ActionKind: "action:a",
			Status:     "fail",
			FailReason: `run action "a": run action "a" exec: run recoveries: recovery "r" requested to start over`,
			StartTime:  zero,
			StopTime:   zero,
			Observations: []*metrics.Observation{
				{MetricKind: "action_type", ValueType: "string", Value: "verifier"},
				{MetricKind: "parent_action_name", ValueType: "string", Value: "plan"},
				{MetricKind: "action_level", ValueType: "number", Value: "0"},
				{MetricKind: "exec_execution_sec", ValueType: "number", Value: "1"},
				{MetricKind: "plan_run_tally", ValueType: "number", Value: "0"},
			},
			RecoveredBy: "r",
			AllowFail:   "allow-fail",
			Type:        "verifier",
		},
		{
			Name:       "unsaved:a",
			ActionKind: "action:a",
			StartTime:  zero,
			StopTime:   zero,
			Status:     "fail",
			FailReason: `run action "a": run action "a" exec: run exec "sample_fail" with timeout 1m0s: failed`,
			Observations: []*metrics.Observation{
				{MetricKind: "action_type", ValueType: "string", Value: "verifier"},
				{MetricKind: "parent_action_name", ValueType: "string", Value: "plan"},
				{MetricKind: "action_level", ValueType: "number", Value: "0"},
				{MetricKind: "exec_execution_sec", ValueType: "number", Value: "1"},
				{MetricKind: "plan_run_tally", ValueType: "number", Value: "1"},
			},
			AllowFail: "allow-fail",
			PlanName:  "",
			Type:      "verifier",
		},
		{
			ActionKind: "plan:2e9aa66a-5fa1-4eaa-933c-eee8e4337823",
			Status:     "success",
			Observations: []*metrics.Observation{
				{MetricKind: "restarts", ValueType: "number", Value: "1"},
				{MetricKind: "started_recoveries", ValueType: "number", Value: "1"},
			},
			Restarts: 1,
		},
	}
	r.plan = &config.Plan{
		CriticalActions: []string{
			"a",
		},
		Actions: map[string]*config.Action{
			"a": {
				ExecName:               exec_fail,
				RecoveryActions:        []string{"r"},
				AllowFailAfterRecovery: true,
			},
			"r": {
				ExecName: exec_pass,
			},
		},
	}
	r.args = &execs.RunArgs{
		Metrics:        m,
		EnableRecovery: true,
	}
	r.initCache()
	err := r.runPlan(ctx)
	// TODO(gregorynisbet): Mock the time.Now() function everywhere instead of removing times
	// from test cases.
	for i := range m.actions {
		m.actions[i].StartTime = zero
		for _, o := range m.actions[i].Observations {
			// Set time observation if present as they can be flaky.
			switch o.MetricKind {
			case "exec_execution_sec":
				o.Value = "1"
			case "exec_execution":
				o.Value = "2"
			}
		}
	}
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if diff := cmp.Diff(expected, m.actions); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
}
