// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tree

import (
	"fmt"

	"go.chromium.org/infra/cros/recovery/config"
)

// Configuration provides the plans to be used by the recovery engine.
type Configuration struct {
	// List of plans provided by configuration.
	Plans []*Plan `json:"plans,omitempty"`
}

// ConvertConfiguration converts Configuration to a tree representation.
func ConvertConfiguration(configuration *config.Configuration, shortVersion bool) *Configuration {
	c := &Configuration{}
	for _, planName := range configuration.GetPlanNames() {
		if planName == config.PlanClosing {
			// Skip it as Closing plan is always last.
			continue
		}
		p := configuration.GetPlans()[planName]
		c.Plans = append(c.Plans, ConvertPlan(planName, p, shortVersion))
	}
	if p, ok := configuration.GetPlans()[config.PlanClosing]; ok {
		c.Plans = append(c.Plans, ConvertPlan(config.PlanClosing, p, shortVersion))
	}
	return c
}

// Plan holds information about actions to execute.
type Plan struct {
	// Name of the Plan.
	Name string `json:"name,omitempty"`
	// Critical actions are actions which have to pass for plan to succeed.
	CriticalActions []*Action `json:"critical_actions,omitempty"`
}

// planConverter holds data to convert actions of single plan.
type planConverter struct {
	srcPlan *config.Plan
}

// ConvertPlan converts Plan to a tree representation.
func ConvertPlan(name string, plan *config.Plan, shortVersion bool) *Plan {
	p := &Plan{
		Name: name,
	}
	if plan.GetAllowFail() {
		p.Name = fmt.Sprintf("%s (Allow to fail)", p.Name)
	}
	converter := &planConverter{srcPlan: plan}
	for _, actionName := range plan.GetCriticalActions() {
		p.CriticalActions = append(p.CriticalActions, converter.convertAcion(actionName, false, shortVersion))
	}
	return p
}

// Action describes how to run the action, including its dependencies,
// conditions, and other attributes.
type Action struct {
	// Name of the Action.
	Name string `yaml:"name,omitempty"`
	// Name of the exec function to use.
	ExecName string `yaml:"exec_name,omitempty"`
	// Extra arguments provided to the exec function.
	ExecArgs []string `yaml:"exec_args,omitempty"`
	// Documentation to describe detail of the action.
	Docs []string `yaml:"docs,omitempty"`
	// List of actions to determine if this action is applicable for the resource.
	Conditions []*Action `yaml:"conditions,omitempty"`
	// List of actions that must pass before executing this action's exec function.
	Dependencies []*Action `yaml:"dependencies,omitempty"`
	// List of actions used to recover this action if exec function fails.
	Recoveries []*Action `yaml:"recoveries,omitempty"`
}

func (t *planConverter) convertAcion(name string, excludeRecoveries, shortVersion bool) *Action {
	action, ok := t.srcPlan.GetActions()[name]
	if !ok {
		return nil
	}
	a := &Action{Name: name}
	if action.GetAllowFailAfterRecovery() {
		a.Name = fmt.Sprintf("%s (Allow to fail)", a.Name)
	}
	if shortVersion {
		if name != action.GetExecName() {
			a.Name = fmt.Sprintf("%s ('%s')", a.Name, action.GetExecName())
		}
	}
	if action.GetRunControl() != config.RunControl_RERUN_AFTER_RECOVERY {
		a.Name = fmt.Sprintf("%s (%s)", a.Name, action.GetRunControl().String())
	}
	if action.GetExecTimeout() != nil {
		a.Name = fmt.Sprintf("%s (time:'%s')", a.Name, action.GetExecTimeout().AsDuration().String())
	}
	if !shortVersion {
		a.Docs = action.GetDocs()
		a.ExecName = action.GetExecName()
		a.ExecArgs = action.GetExecExtraArgs()
	}
	for _, actionName := range action.GetConditions() {
		a.Conditions = append(a.Conditions, t.convertAcion(actionName, true, shortVersion))
	}
	for _, actionName := range action.GetDependencies() {
		a.Dependencies = append(a.Dependencies, t.convertAcion(actionName, excludeRecoveries || false, shortVersion))
	}
	if !excludeRecoveries {
		for _, actionName := range action.GetRecoveryActions() {
			a.Recoveries = append(a.Recoveries, t.convertAcion(actionName, true, shortVersion))
		}
	}
	return a
}
