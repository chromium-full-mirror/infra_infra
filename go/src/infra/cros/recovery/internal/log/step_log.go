// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package log

import (
	"context"

	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/recovery/logger"
)

// AddStepLog created and adds step's log to logger.
func AddStepLog(ctx context.Context, log logger.Logger, step *build.Step, logName string) logger.StepLogCloser {
	var closers []logger.StepLogCloser
	// Create closer which will work for any call to reduce complexity of usage.
	closer := func() {
		for _, c := range closers {
			c()
		}
	}
	if step == nil || log == nil {
		return closer
	}
	if i, ok := log.(logger.StepLogRegister); ok {
		// If the name is not specified then we set default name
		if logName == "" {
			logName = "logs"
		}
		stepLog := step.Log(logName)
		if logCloser, err := i.RegisterStepLog(ctx, stepLog); err != nil {
			log.Debugf("Fail to register step logger!")
		} else {
			log.Debugf("Step logger created!")
			// Only if registration passed without issues.
			closers = append(closers, logCloser)
		}
	}
	return closer
}
