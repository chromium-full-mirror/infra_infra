// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package execs provides collection of execution functions for actions and ability to execute them.
package execs

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/dev"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/logger"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// ExecFunction represents an execution function of the action.
// The single exec can be associated with one or more actions.
type ExecFunction func(ctx context.Context, i *ExecInfo) error

var (
	// Map of known exec functions used by recovery engine.
	// Use Register() function to add to this map.
	knownExecMap = make(map[string]ExecFunction)
)

// Register registers new exec function to be used with recovery engine.
// We panic if a name is reused.
func Register(name string, f ExecFunction) {
	if _, ok := knownExecMap[name]; ok {
		panic(fmt.Sprintf("Register exec %q: already registered", name))
	}
	if f == nil {
		panic(fmt.Sprintf("register exec %q: exec function is not provided", name))
	}
	knownExecMap[name] = f
}

// RunArgs holds plan input arguments.
//
// Keep this type up to date with recovery.go:RunArgs .
// Also update recovery.go:runDUTPlans .
type RunArgs struct {
	// Resource name targeted by plan.
	ResourceName string
	DUT          *tlw.Dut
	Access       tlw.Access
	// Logger prints message to the logs.
	Logger logger.Logger
	// Provide option to stop use steps.
	ShowSteps bool
	// Metrics records actions and observations.
	Metrics metrics.Metrics
	// EnableRecovery tells if recovery actions are enabled.
	EnableRecovery bool
	// SwarmingTaskID is the ID of the swarming task we're running under.
	SwarmingTaskID string
	// BuildbucketID is the ID of the buildbucket build we're running under.
	BuildbucketID string
	// LogRoot is an absolute path to a directory that contains logs.
	LogRoot string
}

// ExecInfo holds all data required to run exec.
// The struct created every time new for each exec run.
type ExecInfo struct {
	runArgs *RunArgs
	// Name of exec.
	name string
	// Extra arguments specified per action for exec.
	actionArgs []string
	// Timeout specified per action.
	actionTimeout time.Duration
	// Metric assigned for the exec.
	metric *metrics.Action
	// Additional metrics created by the exec.
	additionalMetrics []*metrics.Action
}

// NewExecInfo creates new instance of ExecInfo.
func NewExecInfo(runArgs *RunArgs, name string, actionArgs []string, actionTimeout time.Duration, metric *metrics.Action) *ExecInfo {
	return &ExecInfo{
		runArgs:       runArgs,
		name:          name,
		actionArgs:    actionArgs,
		actionTimeout: actionTimeout,
		metric:        metric,
	}
}

// GetActiveResource returns what is active resource.
func (ei *ExecInfo) GetActiveResource() string {
	if ei == nil || ei.runArgs == nil {
		return ""
	}
	return ei.runArgs.ResourceName
}

// GetDut returns Dut.
func (ei *ExecInfo) GetDut() *tlw.Dut {
	if ei == nil || ei.runArgs == nil {
		return nil
	}
	return ei.runArgs.DUT
}

// GetAndroid returns Android device from DUT.
func (ei *ExecInfo) GetAndroid() *tlw.Android {
	if ei == nil || ei.runArgs == nil || ei.runArgs.DUT == nil {
		return nil
	}
	return ei.runArgs.DUT.GetAndroid()
}

// GetChromeos returns ChromeOS device from DUT.
func (ei *ExecInfo) GetChromeos() *tlw.ChromeOS {
	if ei == nil || ei.runArgs == nil || ei.runArgs.DUT == nil {
		return nil
	}
	return ei.runArgs.DUT.GetChromeos()
}

// GetMetrics returns Metrics.
func (ei *ExecInfo) GetMetrics() metrics.Metrics {
	return ei.runArgs.Metrics
}

// GetExecArgs returns list of arguments provided for an exec.
func (ei *ExecInfo) GetExecArgs() []string {
	return ei.actionArgs
}

// GetExecTimeout returns timeout specified for an exec.
func (ei *ExecInfo) GetExecTimeout() time.Duration {
	return ei.actionTimeout
}

// GetAccess returns access to main interface.
func (ei *ExecInfo) GetAccess() tlw.Access {
	return ei.runArgs.Access
}

// Run runs exec function provided by this package by name.
func Run(ctx context.Context, ei *ExecInfo) (rErr error) {

	defer func() {
		// Do not recove from panic only in dev mode.
		if dev.IsActive(ctx) {
			return
		}
		if r := recover(); r != nil {
			log.Debugf(ctx, "Received panic: %v", r)
			rErr = errors.Reason("panic: %v", r).Err()
		}
	}()
	e, ok := knownExecMap[ei.name]
	if !ok {
		return errors.Reason("exec %q: not found", ei.name).Err()
	}
	return e(ctx, ei)
}

// Exist check if exec function with name is present.
func Exist(name string) bool {
	_, ok := knownExecMap[name]
	return ok
}
