// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cft initialize CTR service to manage CFT containers.
package cft

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/recovery/ctr"
	"infra/cros/recovery/logger"
	"infra/cros/recovery/logger/metrics"
)

type Info struct {
	UnitName   string
	CreateStep bool

	// Task root directory.
	RootDir string

	// Task identifiers.
	SwarmingTaskID string
	BBID           string
}

type CftCloser func(context.Context) error

// Prepare prepares CFT tools.
func Prepare(ctx context.Context, in *Info, mt metrics.Metrics, lg logger.Logger) (_ ctr.ServiceInfo, _ CftCloser, rErr error) {
	lg.Infof("Start preparing CFT tools...")
	if in.CreateStep {
		step, sCtx := build.StartStep(ctx, "Prepare CFT")
		// Creating context which cannot be canceled.
		// The end of the step can trigger cancel of the context.
		ctx = common.IgnoreCancel(sCtx)
		defer func() { step.End(rErr) }()
	}
	if mt != nil {
		mta := &metrics.Action{
			ActionKind:     "Init CFT",
			StartTime:      time.Now(),
			SwarmingTaskID: in.SwarmingTaskID,
			BuildbucketID:  in.BBID,
			Hostname:       in.UnitName,
		}
		defer (func() {
			mta.UpdateStatus(rErr)
			mta.StopTime = time.Now()
			if mErr := mt.Create(ctx, mta); mErr != nil {
				lg.Debugf("Fail to save task metric: %s", mErr)
			}
		})()
	}

	ctrInfo, err := ctr.Init(ctx, in.RootDir)
	if err != nil {
		return nil, nil, errors.Annotate(err, "prepare cft").Err()
	}
	closer := func(nCtx context.Context) error {
		return stop(nCtx, in, mt, lg)
	}
	return ctrInfo, closer, nil
}

// Stop stop CFT tools.
func stop(ctx context.Context, in *Info, mt metrics.Metrics, lg logger.Logger) (rErr error) {
	lg.Infof("Start stopping CFT tools...")
	if in.CreateStep {
		var step *build.Step
		step, ctx = build.StartStep(ctx, "Stop CFT")
		defer func() { step.End(rErr) }()
	}
	if mt != nil {
		mta := &metrics.Action{
			ActionKind:     "Stop CFT",
			StartTime:      time.Now(),
			SwarmingTaskID: in.SwarmingTaskID,
			BuildbucketID:  in.BBID,
			Hostname:       in.UnitName,
		}
		defer (func() {
			mta.UpdateStatus(rErr)
			mta.StopTime = time.Now()
			if mErr := mt.Create(ctx, mta); mErr != nil {
				lg.Debugf("Fail to save task metric: %s", mErr)
			}
		})()
	}
	info, ok := ctr.Get(ctx)
	if !ok {
		lg.Infof("CTF Was not started, so nothing to stop!")
		return nil
	}
	if err := info.Stop(ctx); err != nil {
		lg.Debugf("CTF stop failed: %s", err)
		return errors.Annotate(err, "stop cft").Err()
	}
	lg.Infof("CTR stopped!")
	return nil
}
