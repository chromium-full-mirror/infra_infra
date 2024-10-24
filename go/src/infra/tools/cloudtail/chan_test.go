// Copyright 2015 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cloudtail

import (
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"testing"
	"time"
)

func TestDrainChannel(t *testing.T) {
	ftt.Run("Works", t, func(t *ftt.Test) {
		ctx := testContext()
		client := &fakeClient{}
		buf := NewPushBuffer(PushBufferOptions{Client: client})

		ch := make(chan string)
		go func() {
			ch <- "normal line"
			ch <- "  to be trimmed  "
			// Empty lines are skipped.
			ch <- ""
			ch <- "   "
			close(ch)
		}()

		buf.Start(ctx)
		drainChannel(ctx, ch, NullParser(), buf)
		assert.Loosely(t, buf.Stop(ctx), should.BeNil)

		text := []string{}
		for _, e := range client.getEntries() {
			text = append(text, e.TextPayload)
		}
		assert.Loosely(t, text, should.Resemble([]string{"normal line", "to be trimmed"}))
	})

	ftt.Run("Rejects unparsed lines", t, func(t *ftt.Test) {
		ctx := testContext()
		client := &fakeClient{}
		buf := NewPushBuffer(PushBufferOptions{Client: client})
		parser := &callbackParser{
			cb: func(string) *Entry { return nil },
		}

		ch := make(chan string)
		go func() {
			ch <- "normal line"
			close(ch)
		}()

		buf.Start(ctx)
		drainChannel(ctx, ch, parser, buf)
		assert.Loosely(t, buf.Stop(ctx), should.BeNil)
		assert.Loosely(t, len(client.getEntries()), should.BeZero)
	})
}

func TestComputeInsertID(t *testing.T) {
	ftt.Run("Text with TS", t, func(t *ftt.Test) {
		id, err := computeInsertID(&Entry{
			Timestamp:   time.Unix(1435788505, 12345),
			TextPayload: "Hi",
		})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, id, should.Equal("1435788505000012345:lN2eCMEpx4X38lbo"))
	})

	ftt.Run("Text without TS", t, func(t *ftt.Test) {
		id, err := computeInsertID(&Entry{
			TextPayload: "Hi",
		})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, id, should.Equal(":lN2eCMEpx4X38lbo"))
	})

	ftt.Run("JSON", t, func(t *ftt.Test) {
		id, err := computeInsertID(&Entry{
			JSONPayload: struct {
				A int
				B int
			}{10, 20},
		})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, id, should.Equal(":dJ9ZWHLGN9BWUyLG"))
	})
}
