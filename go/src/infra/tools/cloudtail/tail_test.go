// Copyright 2015 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cloudtail

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestTailer(t *testing.T) {
	// This test creates a file, writes a line to it, waits for the line to be
	// read by tailer and sent via mocked client, then writes another one, etc.
	// If something goes wrong the test fails in two possible ways:
	//   * Timeout if number of tailed lines is less than expected.
	//   * ShouldResemble failure when comparing final results if number of tailed
	//     lines is more than expected.
	// The channel is needed to provide synchronization for internal file system
	// buffers: this test invokes full round trip through file system guts
	// (including inotify subsystem, or whatever fsnotify library is using on
	// the current platform).

	ftt.Run("Dumb poller works", t, func(t *ftt.Test) {
		runTest(t, TailerOptions{
			UsePolling:    true,
			PollingPeriod: 10 * time.Millisecond,
		})
	})

	ftt.Run("Fsnotify poller works", t, func(t *ftt.Test) {
		runTest(t, TailerOptions{
			UsePolling: false,
		})
	})
}

func runTest(t testing.TB, opts TailerOptions) {
	t.Helper()

	// This test is more like a smoke test (it uses real file system), do not
	// mock the clock.
	ctx := context.Background()

	// Use buffered channel to avoid deadlocking if something goes wrong.
	client := &fakeClient{ch: make(chan pushEntriesCall, 100)}
	buf := NewPushBuffer(PushBufferOptions{Client: client, FlushTimeout: 1 * time.Millisecond})
	buf.Start(ctx)

	dir, err := os.MkdirTemp("", "cloudtail_test")
	assert.Loosely(t, err, should.BeNil, truth.LineContext())
	defer os.RemoveAll(dir)
	filePath := filepath.Join(dir, "tailed")

	totalExpected := 0
	putData := func(expected int, data string) {
		t.Helper()

		f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
		assert.Loosely(t, err, should.BeNil, truth.LineContext())
		_, err = f.Seek(0, os.SEEK_END)
		assert.Loosely(t, err, should.BeNil, truth.LineContext())
		_, err = f.WriteString(data)
		assert.Loosely(t, err, should.BeNil, truth.LineContext())
		assert.Loosely(t, f.Sync(), should.BeNil, truth.LineContext())
		assert.Loosely(t, f.Close(), should.BeNil, truth.LineContext())

		// Wait for this data to be consumed by tailer.
		totalExpected += expected
		for expected != 0 {
			consumed := <-client.ch
			assert.Loosely(t, len(consumed.entries), should.BeLessThanOrEqual(expected), truth.LineContext())
			expected -= len(consumed.entries)
		}
	}

	truncateFile := func() {
		t.Helper()
		assert.Loosely(t, os.Truncate(filePath, 0), should.BeNil, truth.LineContext())
	}

	rotateFile := func() {
		t.Helper()
		assert.Loosely(t, os.Rename(filePath, filePath+".1"), should.BeNil, truth.LineContext())
	}

	deleteFile := func() {
		t.Helper()
		assert.Loosely(t, os.Remove(filePath), should.BeNil, truth.LineContext())

		// On Windows if deleted file still has open handles, new one can't be
		// created in its place. Wait until tailer closes its handle. Daemons that
		// rotate logs have to deal with it too :-/
		if runtime.GOOS == "windows" {
			deadline := time.Now().Add(2 * time.Second)
			var lastErr error
			for time.Now().Before(deadline) {
				var f *os.File
				f, lastErr = os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
				if f != nil {
					f.Close()
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			assert.Loosely(t, lastErr, should.BeNil, truth.LineContext())
		}
	}

	// Put some lines to be skipped due to 'SeekToEnd: true'.
	putData(0, "skipped line 1\nskipped line 2\n")

	initializedSignal := make(chan struct{})

	// Use small RotationCheckPeriod to detect file truncation faster. This
	// timer is the only mechanism that correctly detects truncation.
	opts.Path = filePath
	opts.Parser = NullParser()
	opts.PushBuffer = buf
	opts.SeekToEnd = true
	opts.RotationCheckPeriod = 50 * time.Millisecond
	opts.initializedSignal = initializedSignal
	tailer, err := NewTailer(opts)
	assert.Loosely(t, err, should.BeNil, truth.LineContext())

	done := make(chan struct{})
	go func() {
		tailer.Run(ctx)
		close(done)
	}()

	<-initializedSignal

	manyLines := strings.Repeat("manylines\n", 1000)
	bigLine := strings.Repeat("bigline", 1000)

	putData(1, "line\n")
	putData(0, "   \n")
	putData(1, "  another\n")
	truncateFile()
	putData(1, "truncated\n")
	putData(0, "\n")
	rotateFile()
	putData(1, "rotated\n")
	deleteFile()
	putData(1, "deleted\n")
	putData(1, "last one\n")
	putData(0, "")
	putData(1000, manyLines)
	putData(1, bigLine+"\n")

	tailer.Stop()
	<-done

	assert.Loosely(t, buf.Stop(ctx), should.BeNil, truth.LineContext())

	text := []string{}
	for _, e := range client.getEntries() {
		text = append(text, e.TextPayload)
	}
	assert.Loosely(t, len(text), should.Equal(totalExpected), truth.LineContext())
	assert.Loosely(t, text[:6], should.Resemble([]string{"line", "another", "truncated", "rotated", "deleted", "last one"}), truth.LineContext())
}
