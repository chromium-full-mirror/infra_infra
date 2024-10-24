// Copyright 2015 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cloudtail

import (
	"context"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"io"
	"strings"
	"sync"
	"testing"
)

type blockingPushBuffer struct {
	start, finish chan struct{}

	lock     sync.Mutex
	blocking bool
	entries  []Entry
}

func (b *blockingPushBuffer) Start(ctx context.Context) {
}

func (b *blockingPushBuffer) Send(ctx context.Context, e Entry) {
	b.lock.Lock()
	b.entries = append(b.entries, e)
	blocking := b.blocking
	b.lock.Unlock()
	if blocking {
		b.start <- struct{}{}
		<-b.finish
	}
}

func (b *blockingPushBuffer) Stop(ctx context.Context) error {
	return nil
}

func (b *blockingPushBuffer) setBlocking(blocking bool) {
	b.lock.Lock()
	b.blocking = blocking
	b.lock.Unlock()
}

func (b *blockingPushBuffer) getEntries() (out []Entry) {
	b.lock.Lock()
	out = append(out, b.entries...)
	b.lock.Unlock()
	return
}

func TestPipeReader(t *testing.T) {
	ftt.Run("PipeReader", t, func(t *ftt.Test) {
		ctx := testContext()

		id := ClientID{"foo", "bar", "baz"}

		t.Run("works", func(t *ftt.Test) {
			client := &fakeClient{}
			buf := NewPushBuffer(PushBufferOptions{Client: client})
			body := `
				line
						another

				last one
				`

			buf.Start(ctx)
			pipeReader := PipeReader{
				ClientID:   id,
				Source:     strings.NewReader(body),
				PushBuffer: buf,
				Parser:     NullParser(),
			}
			assert.Loosely(t, pipeReader.Run(ctx), should.BeNil)
			assert.Loosely(t, buf.Stop(ctx), should.BeNil)

			text := []string{}
			for _, e := range client.getEntries() {
				text = append(text, e.TextPayload)
			}
			assert.Loosely(t, text, should.Resemble([]string{"line", "another", "last one"}))
		})

		t.Run("is buffered and non-blocking", func(c *ftt.Test) {
			buf := blockingPushBuffer{
				blocking: true,
				start:    make(chan struct{}),
				finish:   make(chan struct{}),
			}

			dropped := make(chan struct{}, 10)

			reader, writer := io.Pipe()
			go func() {
				_, err := writer.Write([]byte("first line\n"))
				assert.Loosely(c, err, should.BeNil)

				// Wait for drainChannel to call Send for the first time and then block.
				<-buf.start

				// Send another two lines while drainChannel is blocked on Send.
				// PipeFromReader will send the first one but drop the second one.
				_, err = writer.Write([]byte("second line\nthird line\n"))
				assert.Loosely(c, err, should.BeNil)

				// Wait for the dropped line to be reported.
				<-dropped

				// Exit from Send and don't block the next time.
				buf.setBlocking(false)
				buf.finish <- struct{}{}

				assert.Loosely(c, writer.Close(), should.BeNil)
			}()

			pipeReader := PipeReader{
				ClientID:       id,
				Source:         reader,
				PushBuffer:     &buf,
				Parser:         NullParser(),
				LineBufferSize: 1,
				OnLineDropped:  func() { dropped <- struct{}{} },
			}
			err := pipeReader.Run(ctx)

			assert.Loosely(c, err, should.NotBeNil)
			assert.Loosely(c, err.Error(), should.Equal(
				"1 lines in total were dropped due to insufficient line buffer size"))

			entries := buf.getEntries()
			assert.Loosely(c, len(entries), should.Equal(2))
			assert.Loosely(c, entries[0].TextPayload, should.Equal("first line"))
			assert.Loosely(c, entries[1].TextPayload, should.Equal("second line"))

			assert.Loosely(c, droppedCounter.Get(ctx, "baz", "foo", "bar"), should.Equal(1))
		})
	})
}
