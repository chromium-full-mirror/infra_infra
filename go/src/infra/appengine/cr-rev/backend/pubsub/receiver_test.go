// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pubsub

import (
	"context"
	"sync"
	"testing"

	"cloud.google.com/go/pubsub"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

type psmObserver struct {
	acked  int
	nacked int
	mu     sync.Mutex
}

func (psmo *psmObserver) observe(ackOrNack bool) {
	psmo.mu.Lock()
	defer psmo.mu.Unlock()
	if ackOrNack {
		psmo.acked++
	} else {
		psmo.nacked++
	}
}

type mockPubsubReceiver struct {
	messages []*pubsub.Message
}

func (m *mockPubsubReceiver) Receive(ctx context.Context, f func(ctx context.Context, m *pubsub.Message)) error {
	for _, message := range m.messages {
		f(ctx, message)
	}
	return nil
}

type mockProcessMessage struct {
	calls int
}

func (m *mockProcessMessage) processPubsubMessage(ctx context.Context,
	event *SourceRepoEvent) error {
	m.calls++
	return nil
}

func TestPubsubSubscribe(t *testing.T) {
	t.Skip("Unsafe memory hacks in mockPubsubReceiver.Receive broke when PubSub library changed its internal structs")

	ftt.Run("no messages", t, func(t *ftt.Test) {
		psmo := &psmObserver{}
		ctx := WithObserver(context.Background(), psmo.observe)
		mReceiver := &mockPubsubReceiver{
			messages: make([]*pubsub.Message, 0),
		}
		mProcess := &mockProcessMessage{}

		err := Subscribe(ctx, mReceiver, mProcess.processPubsubMessage)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, psmo.acked, should.BeZero)
		assert.Loosely(t, psmo.nacked, should.BeZero)
	})

	ftt.Run("invalid message", t, func(t *ftt.Test) {
		psmo := &psmObserver{}
		ctx := WithObserver(context.Background(), psmo.observe)
		mReceiver := &mockPubsubReceiver{
			messages: []*pubsub.Message{
				{
					Data: []byte("foo"),
				},
			},
		}
		mProcess := &mockProcessMessage{}

		err := Subscribe(ctx, mReceiver, mProcess.processPubsubMessage)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, psmo.acked, should.BeZero)
		assert.Loosely(t, psmo.nacked, should.Equal(1))
	})

	ftt.Run("valid message", t, func(t *ftt.Test) {
		psmo := &psmObserver{}
		ctx := WithObserver(context.Background(), psmo.observe)
		mReceiver := &mockPubsubReceiver{
			messages: []*pubsub.Message{
				{
					Data: []byte(`
{
  "name": "projects/chromium-gerrit/repos/chromium/src",
  "url": "http://foo/",
  "eventTime": "2020-08-01T00:01:02.333333Z",
  "refUpdateEvent": {
    "refUpdates": {
      "refs/heads/master": {
        "refName": "refs/heads/master",
        "updateType": "UPDATE_FAST_FORWARD",
        "oldId": "b82e8bfe83fadac69a6cad56c06ec45b85c86e49",
        "newId": "ef279f3d5c617ebae8573a664775381fe0225e63"
      }
    }
  }
}`),
				},
			},
		}
		mProcess := &mockProcessMessage{}

		err := Subscribe(ctx, mReceiver, mProcess.processPubsubMessage)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, psmo.acked, should.Equal(1))
		assert.Loosely(t, psmo.nacked, should.BeZero)
		assert.Loosely(t, mProcess.calls, should.Equal(1))
	})
}
