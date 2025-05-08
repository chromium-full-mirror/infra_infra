// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package metrics

import (
	"fmt"

	"go.chromium.org/infra/cros/recovery/logger"
)

// helpers_test.go contains utilities for creating fake versions of various
// types for use in tests. For example, it contains a fake logger, which can
// be used in tests to ensure that specific log messages were written at
// specific severity levels.

// FakeLogger is an implementation of the logger interface
// that is suitable for use in tests. It records calls as
// necessary to support tests.
type fakeLogger struct {
	messages map[string][]string
}

// NewFakeLogger creates a new logger that's suitable for
// use in tests.
func newFakeLogger() logger.Logger {
	return &fakeLogger{
		messages: make(map[string][]string),
	}
}

// Debugf intercepts a debug-level message.
func (l *fakeLogger) Debugf(format string, args ...any) {
	l.messages["debug"] = append(l.messages["debug"], fmt.Sprintf(format, args...))
}

// Infof intercepts an info-level message.
func (l *fakeLogger) Infof(format string, args ...any) {
	l.messages["info"] = append(l.messages["info"], fmt.Sprintf(format, args...))
}

// Warningf intercepts a warning-level message.
func (l *fakeLogger) Warningf(format string, args ...any) {
	l.messages["warning"] = append(l.messages["warning"], fmt.Sprintf(format, args...))
}

// Errorf intercepts an error-level message.
func (l *fakeLogger) Errorf(format string, args ...any) {
	l.messages["error"] = append(l.messages["error"], fmt.Sprintf(format, args...))
}
