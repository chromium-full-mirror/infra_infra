// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package logger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/memlogger"
	"go.chromium.org/luci/common/logging/teelogger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

// TestBasic tests that log level is passing right.
func TestBasic(t *testing.T) {
	ftt.Run(`A new instance`, t, func(t *ftt.Test) {
		ml := logging.Get(memlogger.Use(context.Background())).(*memlogger.MemLogger)
		l := &loggerImpl{
			log: ml,
			// Expected that logger will increase it for +1
			callDepth: 2,
		}
		for _, entry := range []struct {
			L logging.Level
			F func(string, ...any)
			T string
		}{
			{logging.Debug, l.Debugf, "DEBU"},
			{logging.Info, l.Infof, "INFO"},
			{logging.Warning, l.Warningf, "WARN"},
			{logging.Error, l.Errorf, "ERRO"},
		} {
			t.Run(fmt.Sprintf("Can log to %s", entry.L), func(t *ftt.Test) {
				entry.F("%s", entry.T)
				assert.Loosely(t, len(ml.Messages()), should.Equal(1))
				msg := ml.Get(entry.L, entry.T, map[string]any(nil))
				assert.Loosely(t, msg, should.NotBeNil)
				assert.Loosely(t, msg.CallDepth, should.Equal(3))
			})
		}
	})
}

// TestCreateFileLogger tests that file logger created with expected level and set level is matching.
func TestCreateFileLogger(t *testing.T) {
	ftt.Run(`A new file logger`, t, func(t *ftt.Test) {
		l := &loggerImpl{
			// Expected that logger will increase it for +1
			callDepth: 2,
			logDir:    "my_dir",
		}
		buf := new(bytes.Buffer)
		fc := func(ctx context.Context, dir, name string) (io.Writer, closer, error) {
			return buf, func() {}, nil
		}
		for _, entry := range []struct {
			L logging.Level
		}{
			{logging.Debug},
			{logging.Info},
			{logging.Warning},
			{logging.Error},
		} {
			t.Run(fmt.Sprintf("Can create logger to %s", entry.L), func(t *ftt.Test) {
				ctx := context.Background()
				var fs []teelogger.Filtered
				assert.Loosely(t, len(fs), should.BeZero)

				nfs, err := l.createFileLogger(ctx, fc, fs, entry.L)
				assert.Loosely(t, nfs, should.NotBeNil)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, len(nfs), should.Equal(1))
				assert.Loosely(t, nfs[0].Factory, should.NotBeNil)
				assert.Loosely(t, nfs[0].Level, should.Equal(entry.L))
			})
		}
	})
}
