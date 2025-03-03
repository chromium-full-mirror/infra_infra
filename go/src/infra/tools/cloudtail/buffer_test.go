// Copyright 2015 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cloudtail

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/retry/transient"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/common/tsmon"
)

func TestPushBuffer(t *testing.T) {
	ftt.Run("with mocked time", t, func(t *ftt.Test) {
		ctx := testContext()
		cl := clock.Get(ctx).(testclock.TestClock)
		cl.SetTimerCallback(func(d time.Duration, t clock.Timer) {
			if testclock.HasTags(t, "retry-timer") {
				cl.Add(d)
			}
		})

		t.Run("Noop run", func(t *ftt.Test) {
			buf := NewPushBuffer(PushBufferOptions{})
			buf.Start(ctx)
			assert.Loosely(t, buf.Stop(ctx), should.BeNil)
		})

		t.Run("Send one, then stop", func(t *ftt.Test) {
			client := &fakeClient{}
			buf := NewPushBuffer(PushBufferOptions{Client: client})
			buf.Start(ctx)
			buf.Send(ctx, Entry{})
			assert.Loosely(t, buf.Stop(ctx), should.BeNil)
			assert.Loosely(t, len(client.getCalls()), should.Equal(1))
		})

		t.Run("Send a big chunk to trigger immediate flush, then stop.", func(t *ftt.Test) {
			cl := clock.Get(ctx).(testclock.TestClock)
			client := &fakeClient{ch: make(chan pushEntriesCall)}
			buf := NewPushBuffer(PushBufferOptions{
				Client:         client,
				FlushThreshold: 2,
			})
			buf.Start(ctx)
			for range 4 {
				buf.Send(ctx, Entry{})
			}
			client.drain(t, 4, 30*time.Second)
			cl.Add(time.Second) // to be able to distinguish flushes done during Stop
			assert.Loosely(t, buf.Stop(ctx), should.BeNil)
			assert.Loosely(t, len(client.getCalls()), should.Equal(2))
			assert.Loosely(t, client.getCalls()[0].ts, should.Resemble(testclock.TestRecentTimeUTC)) // i.e. before Stop
		})

		t.Run("Send an entry, wait for flush", func(t *ftt.Test) {
			cl := clock.Get(ctx).(testclock.TestClock)
			cl.SetTimerCallback(func(d time.Duration, t clock.Timer) {
				if testclock.HasTags(t, "flush-timer") {
					cl.Add(d)
				}
			})

			client := &fakeClient{ch: make(chan pushEntriesCall)}
			buf := NewPushBuffer(PushBufferOptions{Client: client})
			buf.Start(ctx)
			buf.Send(ctx, Entry{})

			// Wait until flush is called.
			<-client.ch

			// Make sure it happened by timer.
			assert.Loosely(t, cl.Now().Sub(testclock.TestRecentTimeUTC), should.Equal(DefaultFlushTimeout))

			assert.Loosely(t, buf.Stop(ctx), should.BeNil)
			assert.Loosely(t, len(client.getCalls()), should.Equal(1))
		})

		t.Run("Send some entries that should be merged into one", func(t *ftt.Test) {
			client := &fakeClient{}
			buf := NewPushBuffer(PushBufferOptions{Client: client})
			buf.Start(ctx)
			buf.Send(ctx, Entry{TextPayload: "a", ParsedBy: NullParser()})
			buf.Send(ctx, Entry{TextPayload: "b"})
			assert.Loosely(t, buf.Stop(ctx), should.BeNil)
			assert.Loosely(t, len(client.getCalls()), should.Equal(1))
			assert.Loosely(t, client.getEntries()[0].TextPayload, should.Equal("a\nb"))
		})

		t.Run("Retry works", func(t *ftt.Test) {
			client := &fakeClient{transientErrors: 4}
			buf := NewPushBuffer(PushBufferOptions{
				Client:          client,
				MaxPushAttempts: 10,
				PushRetryDelay:  500 * time.Millisecond,
			})
			buf.Start(ctx)
			buf.Send(ctx, Entry{})
			assert.Loosely(t, buf.Stop(ctx), should.BeNil)
			assert.Loosely(t, len(client.getCalls()), should.Equal(5)) // 4 failures, 1 success
		})

		t.Run("Gives up retrying after N attempts", func(t *ftt.Test) {
			client := &fakeClient{transientErrors: 10000}
			buf := NewPushBuffer(PushBufferOptions{
				Client:          client,
				MaxPushAttempts: 5,
				PushRetryDelay:  500 * time.Millisecond,
			})
			buf.Start(ctx)
			buf.Send(ctx, Entry{})
			assert.Loosely(t, buf.Stop(ctx), should.NotBeNil)
			assert.Loosely(t, len(client.calls), should.Equal(5))
		})

		t.Run("Gives up retrying on fatal errors", func(t *ftt.Test) {
			client := &fakeClient{transientErrors: 5, fatalErrors: 1}
			buf := NewPushBuffer(PushBufferOptions{
				Client:          client,
				MaxPushAttempts: 500,
				PushRetryDelay:  500 * time.Millisecond,
			})
			buf.Start(ctx)
			buf.Send(ctx, Entry{})
			assert.Loosely(t, buf.Stop(ctx), should.NotBeNil)
			assert.Loosely(t, len(client.calls), should.Equal(6))
		})

		t.Run("Stop timeout works", func(t *ftt.Test) {
			withDeadline, _ := clock.WithTimeout(ctx, 20*time.Second)

			cl.SetTimerCallback(func(d time.Duration, t clock.Timer) {
				// "Freeze" time after deadline is reached. Otherwise the retry loop can
				// spin really fast (since the time is mocked) between 'buf.Send' and
				// 'buf.Stop'.
				runtime := clock.Now(ctx).Sub(testclock.TestRecentTimeUTC)
				if runtime <= 20*time.Second && testclock.HasTags(t, "retry-timer") {
					cl.Add(d)
				}
			})

			client := &fakeClient{transientErrors: 100000000}
			buf := NewPushBuffer(PushBufferOptions{
				Client:          client,
				MaxPushAttempts: 100000000,
				PushRetryDelay:  100 * time.Millisecond,
			})
			buf.Start(ctx)
			buf.Send(ctx, Entry{})
			assert.Loosely(t, buf.Stop(withDeadline), should.NotBeNil)
			assert.Loosely(t, clock.Now(ctx).Sub(testclock.TestRecentTimeUTC), should.BeLessThan(30*time.Second))
		})
	})
}

func testContext() context.Context {
	ctx := context.Background()
	ctx, _ = testclock.UseTime(ctx, testclock.TestRecentTimeUTC)
	ctx, _ = tsmon.WithDummyInMemory(ctx)
	return ctx
}

type pushEntriesCall struct {
	ts      time.Time
	entries []Entry
}

type fakeClient struct {
	lock            sync.Mutex
	calls           []pushEntriesCall
	ch              chan pushEntriesCall
	transientErrors int
	fatalErrors     int
}

func (c *fakeClient) PushEntries(ctx context.Context, entries []*Entry) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	cpy := make([]Entry, len(entries))
	for i, e := range entries {
		cpy[i] = *e
	}
	call := pushEntriesCall{entries: cpy}
	call.ts = clock.Now(ctx)
	c.calls = append(c.calls, call)
	if c.ch != nil {
		c.ch <- call
	}
	if c.transientErrors > 0 {
		c.transientErrors--
		return errors.New("transient error", transient.Tag)
	}
	if c.fatalErrors > 0 {
		c.fatalErrors--
		return fmt.Errorf("fatal error")
	}
	return nil
}

func (c *fakeClient) getCalls() []pushEntriesCall {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.calls
}

func (c *fakeClient) getEntries() []Entry {
	c.lock.Lock()
	defer c.lock.Unlock()
	out := []Entry{}
	for _, call := range c.calls {
		out = append(out, call.entries...)
	}
	return out
}

func (c *fakeClient) drain(t testing.TB, count int, timeout time.Duration) {
	t.Helper()

	deadline := time.After(timeout) // use real clock to detect stuck test
	total := 0
	for {
		select {
		case call := <-c.ch:
			total += len(call.entries)
			if total >= count {
				if total != count {
					t.Fatalf("Expected to process %d entries, processed %d", count, total)
				}
				return
			}
		case <-deadline:
			t.Fatalf("Timeout while waiting for a flush")
		}
	}
}
