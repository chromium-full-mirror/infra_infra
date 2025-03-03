// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/types/known/durationpb"
)

var testValidateCases = []struct {
	name string
	got  *Configuration
	exp  *Configuration
}{
	{
		"simple",
		&Configuration{
			Plans: map[string]*Plan{
				"plan1": {},
				"plan2": {
					AllowFail: true,
				},
			},
		},
		&Configuration{
			Plans: map[string]*Plan{
				"plan1": {
					AllowFail:       false,
					CriticalActions: nil,
					Actions:         nil,
				},
				"plan2": {
					AllowFail:       true,
					CriticalActions: nil,
					Actions:         nil,
				},
			},
		},
	},
	{
		"full",
		&Configuration{
			Plans: map[string]*Plan{
				"full": {
					AllowFail: true,
					CriticalActions: []string{
						"a1-full",
						"missing_critical_action",
					},
					Actions: map[string]*Action{
						"a1-full": {
							ExecName:               "a1",
							Conditions:             []string{"c1", "c2"},
							Dependencies:           []string{"d1"},
							RecoveryActions:        []string{"r2"},
							AllowFailAfterRecovery: true,
							RunControl:             RunControl_RUN_ONCE,
						},
						"c1": {
							ExecName:        "c1",
							RecoveryActions: []string{"a1-full", "c1"},
						},
						"d1": {
							Dependencies:    []string{"d2"},
							RecoveryActions: []string{"r1"},
						},
						"d2": {
							ExecName:               "d2-exec",
							AllowFailAfterRecovery: true,
						},
						"r2": {
							ExecName:     "r2-exec",
							Dependencies: []string{"d2"},
							ExecTimeout:  &durationpb.Duration{Seconds: 1000},
							RunControl:   RunControl_ALWAYS_RUN,
						},
					},
				},
			},
		},
		&Configuration{
			Plans: map[string]*Plan{
				"full": {
					AllowFail: true,
					CriticalActions: []string{
						"a1-full",
						"missing_critical_action",
					},
					Actions: map[string]*Action{
						"a1-full": {
							ExecName:               "a1",
							Conditions:             []string{"c1", "c2"},
							Dependencies:           []string{"d1"},
							RecoveryActions:        []string{"r2"},
							AllowFailAfterRecovery: true,
							RunControl:             RunControl_RUN_ONCE,
						},
						"c1": {
							ExecName:        "c1",
							RecoveryActions: []string{"a1-full", "c1"},
						},
						"c2": {
							ExecName: "c2",
						},
						"d1": {
							ExecName:        "d1",
							Dependencies:    []string{"d2"},
							ExecTimeout:     nil,
							RecoveryActions: []string{"r1"},
						},
						"d2": {
							ExecName:               "d2-exec",
							AllowFailAfterRecovery: true,
						},
						"r1": {
							ExecName: "r1",
						},
						"r2": {
							ExecName:     "r2-exec",
							Dependencies: []string{"d2"},
							ExecTimeout:  &durationpb.Duration{Seconds: 1000},
							RunControl:   RunControl_ALWAYS_RUN,
						},
						"missing_critical_action": {
							ExecName: "missing_critical_action",
						},
					},
				},
			},
		},
	},
}

func TestValidate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, c := range testValidateCases {
		t.Run(c.name, func(t *testing.T) {
			// t.Parallel() Test cannot be parallel because it modifies a global variable.
			execsExist := func(string) bool {
				return true
			}
			cNew, err := Validate(ctx, c.got, execsExist)
			if err != nil {
				t.Errorf("unmarshal fail: %s", err)
			}
			loadedJson, _ := json.MarshalIndent(cNew, "", "\t")
			expectedJson, _ := json.MarshalIndent(c.exp, "", "\t")
			if diff := cmp.Diff(string(loadedJson), string(expectedJson)); diff != "" {
				t.Errorf("Receive diff: %v \ngot:\n %s", diff, loadedJson)
			}
		})
	}
}

var cycleTestCases = []struct {
	testName     string
	in           *Plan
	errorActions []string
}{
	{
		"Bad1: A_dependency -> A",
		&Plan{
			Actions: map[string]*Action{
				"A": {Dependencies: []string{"A"}},
			},
		},
		[]string{"A"},
	},
	{
		"Bad2: A_dependency -> B_condition -> A",
		&Plan{
			Actions: map[string]*Action{
				"A": {Dependencies: []string{"B"}},
				"B": {Conditions: []string{"A"}},
			},
		},
		[]string{"B", "A"},
	},
	{
		"Good3: A_dependency -> B_condition -> C : recovery ignored for conditions",
		&Plan{
			Actions: map[string]*Action{
				"A": {Dependencies: []string{"B"}},
				"B": {Conditions: []string{"C"}},
				"C": {RecoveryActions: []string{"A"}},
			},
		},
		nil,
	},
	{
		"Bad4: A_dependency -> B_dependency -> C_dependency -> A",
		&Plan{
			Actions: map[string]*Action{
				"A": {Dependencies: []string{"B"}},
				"B": {Dependencies: []string{"C"}},
				"C": {Dependencies: []string{"A"}},
			},
		},
		[]string{"B", "C", "A"},
	},
	{
		"Bad5: C_dependency -> B_condition -> A_recovery -> C",
		&Plan{
			Actions: map[string]*Action{
				"A": {RecoveryActions: []string{"C"}},
				"B": {Conditions: []string{"A"}},
				"C": {Dependencies: []string{"B"}},
			},
		},
		[]string{"C", "B", "A"},
	},
	{
		"Bad6: A_dependency -> B_condition -> C_dependency -> D_dependency -> E_dependency -> F_condition -> B",
		&Plan{
			Actions: map[string]*Action{
				"A": {Dependencies: []string{"B"}},
				"B": {Conditions: []string{"C"}},
				"C": {Dependencies: []string{"D"}},
				"D": {Dependencies: []string{"E"}},
				"E": {Dependencies: []string{"F"}},
				"F": {Conditions: []string{"B"}},
			},
		},
		[]string{"B", "C", "D", "E", "F", "B"},
	},
	// Test Case: Cycle in actions, but not reachable by critical actions.
	{
		"Good: A_dependency -> B_condition -> C_recovery; D_recovery -> E_dependency -> F_recovery -> D",
		&Plan{
			Actions: map[string]*Action{
				"A": {Dependencies: []string{"B"}},
				"B": {Conditions: []string{"C"}},
				"C": {},
				"D": {Conditions: []string{"F"}, RecoveryActions: []string{"E"}},
				"E": {Dependencies: []string{"F"}},
				"F": {RecoveryActions: []string{"D"}},
			},
		},
		[]string{},
	},
	{
		"Bad9: Cycle in recovery actions",
		&Plan{
			Actions: map[string]*Action{
				"A":           {RecoveryActions: []string{"sample_pass"}},
				"sample_pass": {Dependencies: []string{"sample_pass"}, ExecName: "sample_fail"},
			},
		},
		[]string{"sample_pass", "sample_pass"},
	},
}

func TestVerifyPlanAcyclic(t *testing.T) {
	t.Parallel()
	for _, tt := range cycleTestCases {
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			// Assume "A" as the critical action.
			tt.in.CriticalActions = []string{"A"}
			if err := verifyPlanAcyclic(tt.in); err != nil {
				m := strings.Replace(err.Error(), ": found loop", "", -1)
				errMessages := strings.Split(m, ":")
				raiseError := false
				if len(tt.errorActions) != len(errMessages) {
					t.Errorf("got %d, want %d errors", len(errMessages), len(tt.errorActions))
					t.Errorf("got %q, want %q", err.Error(), tt.errorActions)
				} else {
					for i, eachAction := range tt.errorActions {
						if !strings.Contains(errMessages[i], fmt.Sprintf("%q", eachAction)) {
							raiseError = true
						}
					}
					if raiseError {
						t.Errorf("got %q, want %q", err.Error(), tt.errorActions)
					}
				}
			} else if len(tt.errorActions) != 0 {
				t.Errorf("got nil, want %q", tt.errorActions)
			}
		})
	}
}

var createMissingActionsCases = []struct {
	name      string
	inPlan    *Plan
	inActions []string
	outPlan   *Plan
}{
	{
		"init Actions and set action if missed in actions map",
		&Plan{
			Actions: nil,
		},
		[]string{"a"},
		&Plan{
			Actions: map[string]*Action{
				"a": {},
			},
		},
	},
	{
		"do not replace if action is present in the plan",
		&Plan{
			Actions: map[string]*Action{
				"a": {Dependencies: []string{"F"}},
			},
		},
		[]string{"a"},
		&Plan{
			Actions: map[string]*Action{
				"a": {Dependencies: []string{"F"}},
			},
		},
	},
}

func TestCreateMissingActions(t *testing.T) {
	t.Parallel()
	for _, tt := range createMissingActionsCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			createMissingActions(tt.inPlan, tt.inActions)
			if !reflect.DeepEqual(tt.inPlan, tt.outPlan) {
				t.Errorf("case %q: did not updated in expecteds struct", tt.name)
			}
		})
	}
}
