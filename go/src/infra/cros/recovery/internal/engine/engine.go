// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package engine provides struts and functionality of recovery engine.
// For more details please read go/paris-recovery-engine.
package engine

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/recovery/config"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/logger"
	"infra/cros/recovery/logger/metrics"
)

// recoveryEngine holds info required for running a recovery plan.
type recoveryEngine struct {
	planName    string
	plan        *config.Plan
	args        *execs.RunArgs
	metricSaver metrics.MetricSaver
	// Caches
	actionResultsCache map[string]error
	recoveryUsageCache map[recoveryUsageKey]error
	// Tracker the plan iterations.
	planRunTally int32
	// Track how many recoevry actin was used in the plan.
	startedRecoveries int32
}

// Run runs the recovery plan.
func Run(ctx context.Context, planName string, plan *config.Plan, args *execs.RunArgs, metricSaver metrics.MetricSaver) error {
	r := &recoveryEngine{
		planName:    planName,
		plan:        plan,
		args:        args,
		metricSaver: metricSaver,
	}
	r.initCache()
	defer func() { r.close() }()
	log.Debugf(ctx, "Received plan %s for %s", r.planName, r.args.ResourceName)
	return r.runPlan(ctx)
}

// close free up used resources.
func (r *recoveryEngine) close() {
	if r.actionResultsCache != nil {
		r.actionResultsCache = nil
	}
	if r.recoveryUsageCache != nil {
		r.recoveryUsageCache = nil
	}
}

// runPlan executes recovery plan with critical-actions.
func (r *recoveryEngine) runPlan(ctx context.Context) (rErr error) {
	log.Infof(ctx, "Plan %q: started", r.planName)
	// The step and metrics need to know about error but if we need to stop from return then it is here.
	forgiveError := false
	defer func() {
		if forgiveError {
			log.Debugf(ctx, "Plan %q: forgiven error: %v", r.planName, rErr)
			rErr = nil
		}
	}()
	// TODO(gregorynisbet): Generate metrics for plan closing.
	if r.args.ShowSteps {
		var step *build.Step
		step, ctx = build.StartStep(ctx, fmt.Sprintf("Plan: %s", r.planName))
		if r.plan.GetAllowFail() {
			step.Log("Allowed to fail!")
		}
		defer func() { step.End(rErr) }()
	}
	if i, ok := r.args.Logger.(logger.LogIndenter); ok {
		i.Indent()
		defer func() { i.Dedent() }()
	}
	if r.args != nil && r.metricSaver != nil {
		metric := r.args.NewMetricsAction(fmt.Sprintf("plan:%s", r.planName))
		defer func() {
			metric.Observations = append(
				metric.Observations,
				metrics.NewInt64Observation("restarts", int64(r.planRunTally)),
				metrics.NewInt64Observation("started_recoveries", int64(r.startedRecoveries)),
			)
			metric.Restarts = r.planRunTally
			metric.UpdateStatus(rErr)
			if err := r.metricSaver(metric); err != nil {
				log.Debugf(ctx, "Fail to save plan %q metrics with error: %s", r.planName, err)
			}
		}()
	}
	for {
		if err := r.runCriticalActionsAttempt(ctx); err != nil {
			if execs.PlanStartOverTag.In(err) {
				log.Infof(ctx, "Plan %q for %s: received the request to start over!", r.planName, r.args.ResourceName)
				r.resetCacheAfterSuccessfulRecoveryAction()
				r.planRunTally++
				continue
			}
			if execs.PlanAbortTag.In(err) {
				log.Infof(ctx, "Plan %q received the request for abort plan!", r.planName)
				return errors.Annotate(err, "run plan %q: abort", r.planName).Err()
			}
			if r.plan.GetAllowFail() {
				log.Debugf(ctx, "Plan %q for %s: failed with error: %s.", r.planName, r.args.ResourceName, err)
				log.Infof(ctx, "Plan %q for %s: is allowed to fail, continue.", r.planName, r.args.ResourceName)
				forgiveError = true
			}
			log.Infof(ctx, "Plan %q: fail", r.planName)
			return errors.Annotate(err, "run plan %q", r.planName).Err()
		}
		break
	}
	log.Infof(ctx, "Plan %q: finished successfully.", r.planName)
	log.Debugf(ctx, "Plan %q: recorded %d restarts during execution.", r.planName, r.planRunTally)
	log.Debugf(ctx, "Plan %q: is forgiven:%v failures during execution.", r.planName, forgiveError)
	return nil
}

// runCriticalActionsAttempt runs critical action of the plan with wrapper step to show plan restart attempts.
func (r *recoveryEngine) runCriticalActionsAttempt(ctx context.Context) (err error) {
	if r.args.ShowSteps {
		var step *build.Step
		stepName := fmt.Sprintf("First run of critical actions for %s", r.planName)
		if r.planRunTally > 0 {
			stepName = fmt.Sprintf("Attempt %d to run critical actions for %q", r.planRunTally, r.planName)
		}
		step, ctx = build.StartStep(ctx, stepName)
		defer func() { step.End(err) }()
	}
	// Critical actions represent the highest level (0).
	level := int64(0)
	for _, actionName := range r.plan.GetCriticalActions() {
		if _, _, err := r.runAction(ctx, actionName, "plan", r.args.EnableRecovery, "Critical Action", metrics.ActionTypeVerifier, level); err != nil {
			return errors.Annotate(err, "run actions").Err()
		}
	}
	return nil
}

// Single action status per run.
type actionRunStatus string

const (
	actionPass      actionRunStatus = "pass"
	actionFail      actionRunStatus = "fail"
	actionPassCache actionRunStatus = "pass_by_cache"
	actionFailCache actionRunStatus = "fail_by_cache"
	actionSkip      actionRunStatus = "skip"
)

// runAction runs single action.
// Execution steps:
// 1) Check action's result in cache.
// 2) Check if the action is applicable based on conditions. Skip if any fail.
// 3) Run dependencies of the action. Fail if any fails.
// 4) Run action exec function. Fail if any fail.
func (r *recoveryEngine) runAction(ctx context.Context, actionName, parentAction string, enableRecovery bool, stepNamePrefix string, actionType metrics.ActionType, actionLevel int64) (metric *metrics.Action, status actionRunStatus, rErr error) {
	// The step and metrics need to know about error but if we need to stop from return then it is here.
	forgiveError := false
	defer func() {
		if forgiveError {
			log.Debugf(ctx, "Action %q: forgiven error: %v", actionName, rErr)
			rErr = nil
		}
	}()
	var step *build.Step
	act := r.getAction(actionName)
	if r.args != nil {
		if r.args.ShowSteps {
			stepName := fmt.Sprintf("%s: %s", stepNamePrefix, actionName)
			step, ctx = build.StartStep(ctx, stepName)
			defer func() { step.End(rErr) }()
			stepLogCloser := log.AddStepLog(ctx, r.args.Logger, step, "execution details")
			defer func() { stepLogCloser() }()
		}
		if i, ok := r.args.Logger.(logger.LogIndenter); ok {
			i.Indent()
			defer func() { i.Dedent() }()
		}
	}
	defer func() {
		if rErr != nil {
			log.Debugf(ctx, "Action %q: finished with error %s.", actionName, rErr)
		} else {
			log.Debugf(ctx, "Action %q: finished.", actionName)
		}
	}()
	if len(act.GetDocs()) > 0 && step != nil {
		docLog := step.Log("docs")
		if _, err := io.WriteString(docLog, strings.Join(act.GetDocs(), "\n")); err != nil {
			log.Debugf(ctx, "Fail write docs for %q, Error: %s", actionName, err)
		}
	}
	// Please do not create metrics for cached actions as it will lead for creating too many repeated metrics.
	if aErr, ok := r.actionResultFromCache(actionName); ok {
		if aErr == nil {
			log.Infof(ctx, "Action %q (level: %d): pass (cached).", actionName, actionLevel)
			// Return nil error so we can continue execution of next actions...
			return nil, actionPassCache, nil
		}
		if act.GetAllowFailAfterRecovery() {
			log.Infof(ctx, "Action %q (level: %d): fail (cached). Error: %s", actionName, actionLevel, aErr)
			log.Debugf(ctx, "Action %q: error ignored as action is allowed to fail.", actionName)
			// Return error to report for step and metrics but stop from return to parent.
			forgiveError = true
		}
		return nil, actionFailCache, errors.Annotate(aErr, "run action %q: (cached)", actionName).Err()
	}
	if r.args != nil && r.metricSaver != nil {
		var metricKind string
		if act.GetMetricsConfig().GetCustomKind() != "" {
			metricKind = act.GetMetricsConfig().GetCustomKind()
		} else {
			metricKind = fmt.Sprintf("action:%s", actionName)
		}
		metric = r.args.NewMetricsAction(metricKind)
		metric.Type = actionType
		metric.Observations = append(metric.Observations,
			metrics.NewStringObservation("action_type", string(actionType)),
			metrics.NewStringObservation("parent_action_name", parentAction),
		)
		switch act.GetAllowFailAfterRecovery() {
		case true:
			metric.AllowFail = metrics.YesAllowFail
		case false:
			metric.AllowFail = metrics.NoAllowFail
		}
		metric.Observations = append(metric.Observations, metrics.NewInt64Observation("action_level", actionLevel))
		defer func() {
			metric.UpdateStatus(rErr)
			policy := act.GetMetricsConfig().GetUploadPolicy()
			switch policy {
			case config.MetricsConfig_DEFAULT_UPLOAD_POLICY:
				if err := r.metricSaver(metric); err != nil {
					log.Debugf(ctx, "Fail to save %q metrics with error: %s", actionName, err)
				}
			case config.MetricsConfig_UPLOAD_ON_ERROR:
				log.Debugf(ctx, "Action %q requires save metrics only when fail.", actionName)
				if rErr != nil {
					if err := r.metricSaver(metric); err != nil {
						log.Debugf(ctx, "Fail to save %q metrics with error: %s", actionName, err)
					}
				}
			case config.MetricsConfig_SKIP_ALL:
				log.Debugf(ctx, "Action %q: requires skipp metrics upload.", actionName)
			default:
				panic(fmt.Sprintf("Bad metrics upload policy %q %d", policy.String(), policy.Number()))
			}
			if metric != nil && metric.Name == "" {
				metric.Name = fmt.Sprintf("unsaved:%s", actionName)
			}
		}()
	}
	log.Infof(ctx, "Action %q (level: %d): started.", actionName, actionLevel)
	conditionName, err := r.runActionConditions(ctx, actionName, actionLevel+1)
	if err != nil {
		log.Infof(ctx, "Action %q: skipping, one of conditions %q failed.", actionName, conditionName)
		if step != nil {
			stepLog := step.Log("Skipped")
			if _, err := io.WriteString(stepLog, fmt.Sprintf("The condition %q failed!", conditionName)); err != nil {
				log.Debugf(ctx, "Fail to write reason why action skipped: %v.", err)
			}
		}
		log.Debugf(ctx, "Action %q: conditions fail with %s", actionName, err)
		if metric != nil {
			metric.Status = metrics.ActionStatusSkip
		}
		// Return nil error so we can continue execution of next actions...
		return metric, actionSkip, nil
	}
	if err := r.runDependencies(ctx, actionName, actionType, enableRecovery, actionLevel+1); err != nil {
		if execs.PlanStartOverTag.In(err) {
			return metric, actionFail, errors.Annotate(err, "run action %q", actionName).Err()
		}
		if act.GetAllowFailAfterRecovery() {
			log.Infof(ctx, "Action %q: one of dependencies fail. Error: %s", actionName, err)
			log.Debugf(ctx, "Action %q: error ignored as action is allowed to fail.", actionName)
			// Return error to report for step and metrics but stop from return to parent.
			forgiveError = true
		}
		return metric, actionFail, errors.Annotate(err, "run action %q", actionName).Err()
	}
	if err := r.runActionExec(ctx, actionName, metric, enableRecovery); err != nil {
		if execs.PlanStartOverTag.In(err) {
			return metric, actionFail, errors.Annotate(err, "run action %q", actionName).Err()
		}
		if act.GetAllowFailAfterRecovery() {
			log.Infof(ctx, "Action %q: fail. Error: %s", actionName, err)
			log.Debugf(ctx, "Action %q: error ignored as action is allowed to fail.", actionName)
			// Return error to report for step and metrics but stop from return to parent.
			forgiveError = true
		}
		return metric, actionFail, errors.Annotate(err, "run action %q", actionName).Err()
	}
	// Return nil error so we can continue execution of next actions...
	log.Infof(ctx, "Action %q: finished successfully.", actionName)
	return metric, actionPass, nil
}

// runActionExec runs action's exec function and initiates recovery flow if exec fails.
// The recover flow start only recoveries is enabled.
func (r *recoveryEngine) runActionExec(ctx context.Context, actionName string, metric *metrics.Action, enableRecovery bool) error {
	startExec := time.Now()
	err := r.runActionExecWithTimeout(ctx, actionName, metric)
	durationExec := time.Since(startExec)
	log.Debugf(ctx, "Action %q exec execution time: %v", actionName, durationExec)
	if metric != nil {
		metric.Observations = append(metric.Observations,
			metrics.NewInt64Observation("exec_execution_sec", int64(durationExec.Seconds())),
			metrics.NewInt64Observation("plan_run_tally", int64(r.planRunTally)),
		)
	}
	if err != nil {
		a := r.getAction(actionName)
		if enableRecovery && len(a.GetRecoveryActions()) > 0 {
			log.Infof(ctx, "Action %q: starting recovery actions.", actionName)
			log.Debugf(ctx, "Action %q: fail. Error: %s", actionName, err)
			if rErr := r.runRecoveries(ctx, actionName, metric); rErr != nil {
				return errors.Annotate(rErr, "run action %q exec", actionName).Err()
			}
			log.Infof(ctx, "Run action %q exec: no recoveries left to try", actionName)
		}
		// Cache the action error only after running recoveries.
		// If no recoveries were run, we still cache the action.
		r.cacheActionResult(actionName, err)
		return errors.Annotate(err, "run action %q exec", actionName).Err()
	}
	r.cacheActionResult(actionName, nil)
	return nil
}

// Default time limit per action exec function.
const defaultExecTimeout = 60 * time.Second

func actionExecTimeout(a *config.Action) time.Duration {
	if a.ExecTimeout != nil {
		return a.ExecTimeout.AsDuration()
	}
	return defaultExecTimeout
}

// runActionExecWithTimeout runs action's exec function with timeout.
func (r *recoveryEngine) runActionExecWithTimeout(ctx context.Context, actionName string, metric *metrics.Action) (rErr error) {
	a := r.getAction(actionName)
	if r.args != nil && r.args.ShowSteps {
		var step *build.Step
		step, ctx = build.StartStep(ctx, fmt.Sprintf("Execution: %q", actionName))
		defer func() { step.End(rErr) }()
		stepLogCloser := log.AddStepLog(ctx, r.args.Logger, step, "execution details")
		defer func() { stepLogCloser() }()
	}
	timeout := actionExecTimeout(a)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer func() { cancel() }()
	execInfo := execs.NewExecInfo(r.args, a.ExecName, a.GetExecExtraArgs(), timeout, metric)
	if r.metricSaver != nil {
		// Try to save additional metrics if an exec could not finished in time.
		defer func() {
			for _, additionalMetric := range execInfo.GetAdditionalMetrics() {
				additionalMetric.Observations = append(additionalMetric.Observations,
					metrics.NewInt64Observation("plan_run_tally", int64(r.planRunTally)),
				)
				if err := r.metricSaver(additionalMetric); err != nil {
					log.Debugf(ctx, "Fail to save %q additional metrics error: %s", r, r.planName, err)
				}
			}
		}()
	}
	cw := make(chan error, 1)
	go func() {
		// Populate default metric action to be available by context.
		ctx = metrics.WithAction(ctx, metric)
		err := execs.Run(ctx, execInfo)
		cw <- err
	}()
	defer func() {
		if rErr != nil {
			log.Debugf(ctx, "Action %q: finished with error %s.", actionName, rErr)
		} else {
			log.Debugf(ctx, "Action %q: finished.", actionName)
		}
	}()
	select {
	case err := <-cw:
		return errors.Annotate(err, "run exec %q with timeout %s", a.ExecName, timeout).Err()
	case <-ctx.Done():
		log.Infof(ctx, "Run exec %q with timeout %s: exited due to timeout", a.ExecName, timeout)
		return errors.Reason("run exec %q with timeout %s: exited due to timeout", a.ExecName, timeout).Err()
	}
}

// runActionConditions checks if action is applicable based on condition actions.
// If return err then not applicable, if nil then applicable.
func (r *recoveryEngine) runActionConditions(ctx context.Context, actionName string, actionLevel int64) (conditionName string, err error) {
	a := r.getAction(actionName)
	if len(a.GetConditions()) == 0 {
		log.Debugf(ctx, "Action %q: no conditions.", actionName)
		return "", nil
	}
	log.Debugf(ctx, "Action %q: starting running conditions...", actionName)
	enableRecovery := false
	for _, condition := range a.GetConditions() {
		if _, _, err := r.runAction(ctx, condition, actionName, enableRecovery, "Condition", metrics.ActionTypeCondition, actionLevel); err != nil {
			log.Debugf(ctx, "Action %q: condition %q fails. Error: %s", actionName, condition, err)
			return condition, errors.Annotate(err, "run conditions").Err()
		}
	}
	log.Debugf(ctx, "Action %q: all conditions passed.", actionName)
	return "", nil
}

// runDependencies runs action's dependencies.
func (r *recoveryEngine) runDependencies(ctx context.Context, actionName string, actionType metrics.ActionType, enableRecovery bool, actionLevel int64) error {
	a := r.getAction(actionName)
	if len(a.GetDependencies()) == 0 {
		log.Debugf(ctx, "Action %q: no dependencies.", actionName)
		return nil
	}
	log.Debugf(ctx, "Action %q: starting running dependencies...", actionName)
	for _, dependencyName := range a.GetDependencies() {
		if _, _, err := r.runAction(ctx, dependencyName, actionName, enableRecovery, "Dependency", actionType, actionLevel); err != nil {
			log.Debugf(ctx, "Action %q: dependency %q fails. Errors: %s", actionName, dependencyName, err)
			return errors.Annotate(err, "dependencies").Err()
		}
	}
	log.Debugf(ctx, "Action %q: all dependencies passed.", actionName)
	return nil
}

// runRecoveries runs an action's recoveries.
//
// Note that we mutate the metric action!
//
// Recovery actions are expected to fail. If recovery action fails then next will be attempted.
// Finishes with nil if no recovery action provided or nether succeeded.
// Finishes with start-over request if any recovery succeeded.
// Recovery action will skip if used before.
func (r *recoveryEngine) runRecoveries(ctx context.Context, actionName string, metric *metrics.Action) (rErr error) {
	a := r.getAction(actionName)
	// Recovery restarts the action level as it's a new starting point.
	recoveryLevel := int64(0)
	for _, recoveryName := range a.GetRecoveryActions() {
		if r.isRecoveryUsed(actionName, recoveryName) {
			// Engine allows to use each recovery action only once in scope of the action.
			log.Infof(ctx, "Recovery %q skipped as already used before for %q.", recoveryName, actionName)
			continue
		}
		r.startedRecoveries += 1
		recoveryMetric, status, err := r.runAction(ctx, recoveryName, actionName, false, "Recovery", metrics.ActionTypeRecovery, recoveryLevel)
		if err != nil {
			log.Infof(ctx, "Recovery %q: fail", recoveryName)
			log.Debugf(ctx, "Recovery %q: fail. Error: %s", recoveryName, err)
			r.registerRecoveryUsage(actionName, recoveryName, err)
			if execs.PlanStartOverTag.In(err) {
				log.Infof(ctx, "Recovery %q: requested to start plan %q over.", recoveryName, r.planName)
				return errors.Annotate(err, "run recoveries").Err()
			}
			if execs.PlanAbortTag.In(err) {
				log.Infof(ctx, "Recovery %q: requested an abort of the %q plan.", recoveryName, r.planName)
				return errors.Annotate(err, "run recoveries").Err()
			}
			continue
		}
		if status == actionSkip {
			log.Infof(ctx, "Recovery %q: skipped ", recoveryName)
			r.registerRecoveryUsage(actionName, recoveryName, nil)
			continue
		}
		r.registerRecoveryUsage(actionName, recoveryName, nil)
		log.Infof(ctx, "Recovery action %q: request to start-over.", recoveryName)
		// A non-nil request to start over is a success.
		if metric != nil {
			metric.RecoveredBy = recoveryName
			if recoveryMetric != nil {
				log.Infof(ctx, "Successful recovery: %q (%s) recovered %q", recoveryName, recoveryMetric.Name, actionName)
			} else {
				log.Infof(ctx, "Successful recovery: %q recovered %q", recoveryName, actionName)
			}
		}
		return errors.Reason("run recoveries: recovery %q requested to start over", recoveryName).Tag(execs.PlanStartOverTag).Err()
	}
	return nil
}

// getAction finds and provides action from the plan collection.
func (r *recoveryEngine) getAction(name string) *config.Action {
	if a, ok := r.plan.Actions[name]; ok {
		return a
	}
	// If we reach this place then we have issues with plan validation logic.
	panic(fmt.Sprintf("action %q not found in the plan", name))
}

// initCache initializes cache on engine.
// The function extracted to supported testing.
func (r *recoveryEngine) initCache() {
	r.actionResultsCache = make(map[string]error, len(r.plan.GetActions()))
	r.recoveryUsageCache = make(map[recoveryUsageKey]error)
}

// actionResultFromCache reads action's result from cache.
func (r *recoveryEngine) actionResultFromCache(actionName string) (err error, ok bool) {
	err, ok = r.actionResultsCache[actionName]
	return err, ok
}

// cacheActionResult sets action's result to the cache.
func (r *recoveryEngine) cacheActionResult(actionName string, err error) {
	switch r.getAction(actionName).GetRunControl() {
	case config.RunControl_RERUN_AFTER_RECOVERY, config.RunControl_RUN_ONCE:
		r.actionResultsCache[actionName] = err
	case config.RunControl_ALWAYS_RUN:
		// Do not cache the value
	}
}

// resetCacheAfterSuccessfulRecoveryAction resets cache for actions
// with run-control=RERUN_AFTER_RECOVERY.
func (r *recoveryEngine) resetCacheAfterSuccessfulRecoveryAction() {
	for name, a := range r.plan.GetActions() {
		if a.GetRunControl() == config.RunControl_RERUN_AFTER_RECOVERY {
			delete(r.actionResultsCache, name)
		}
	}
}

// isRecoveryUsed checks if recovery action is used in plan or action level scope.
func (r *recoveryEngine) isRecoveryUsed(actionName, recoveryName string) bool {
	k := recoveryUsageKey{
		action:   actionName,
		recovery: recoveryName,
	}
	// If the recovery has been used in previous actions then it can be in
	// the action result cache.
	if err, ok := r.actionResultsCache[recoveryName]; ok {
		r.recoveryUsageCache[k] = err
	}
	_, ok := r.recoveryUsageCache[k]
	return ok
}

// registerRecoveryUsage sets recovery action usage to the cache.
func (r *recoveryEngine) registerRecoveryUsage(actionName, recoveryName string, err error) {
	r.recoveryUsageCache[recoveryUsageKey{
		action:   actionName,
		recovery: recoveryName,
	}] = err
}

// recoveryUsageKey holds action and action's recovery name as key for recovery-usage cache.
type recoveryUsageKey struct {
	action   string
	recovery string
}
