// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testlogger implements a logging.Logger for use in tests.
package testlogger

import (
	"context"
	"testing"

	"go.chromium.org/luci/common/logging"
)

// Use adds a logging.Logger implementation to the context which logs for a test.
func Use(ctx context.Context, t *testing.T) context.Context {
	return logging.SetFactory(ctx, func(ctx context.Context, lc *logging.LogContext) logging.Logger {
		return loggerImpl{ctx: ctx, level: lc.Level, t: t}
	})
}

type loggerImpl struct {
	ctx   context.Context
	level logging.Level
	t     *testing.T
}

func (gl loggerImpl) Debugf(format string, args ...any) {
	gl.LogCall(logging.Debug, 1, format, args)
}
func (gl loggerImpl) Infof(format string, args ...any) {
	gl.LogCall(logging.Info, 1, format, args)
}
func (gl loggerImpl) Warningf(format string, args ...any) {
	gl.LogCall(logging.Warning, 1, format, args)
}
func (gl loggerImpl) Errorf(format string, args ...any) {
	gl.LogCall(logging.Error, 1, format, args)
}

func (gl loggerImpl) LogCall(l logging.Level, calldepth int, format string, args []any) {
	if l >= gl.level {
		gl.t.Logf(format, args...)
	}
}
