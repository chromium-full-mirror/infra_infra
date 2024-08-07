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

func Prepare(ctx context.Context, in *Info, mt metrics.Metrics, lg logger.Logger) (_ ctr.Info, rErr error) {
	lg.Infof("Pull CTR CIPD...")
	if in.CreateStep {
		step, sCtx := build.StartStep(ctx, "Prepare CTR")
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
		return nil, errors.Annotate(err, "prepare cft").Err()
	}
	return ctrInfo, nil
}
