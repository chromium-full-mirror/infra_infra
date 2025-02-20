// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pubsub wraps all the pubsub API interactions that will be required by Kron.
package pubsub

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"

	"go.chromium.org/infra/cros/cmd/kron/common"
)

const (
	// MaxIdleTime is the maximum amount of time we will let the Pub/Sub receive
	// client sit idle.
	MaxIdleSeconds = 5

	loopDuration = 100 * time.Millisecond
)

// ReceiveClient defines the minimum requires that this project will need of a
// Pub/Sub API.
type ReceiveClient interface {
	initClient(projectID string) error
	initSubscription(subscriptionID string) error
	ingestMessage(ctx context.Context, msg *pubsub.Message)
	PullMessages() error
	// Deprecate: The finalize feature is now implemented by the build ingestion
	// handler.
	PullAllMessagesForProcessing(finalize func()) error
}

// ReceiveTimer defines an interface with for an auto-decrementing timer.
type ReceiveTimer interface {
	Start(receiveCtxCancel context.CancelFunc, closeHandlerChan func())
	// Deprecate: The finalize feature is now implemented by the build ingestion
	// handler.
	FinalizeBeforeContextCancel(receiveCtxCancel context.CancelFunc, finalize func())
	Refresh()
	Decrement(duration time.Duration)
	checkMillisecondsLeft() int64
}

// Timer implements the ReceiveTimer interface with thread-safe functionality.
type Timer struct {
	mutex            sync.Mutex
	maxSeconds       int
	millisecondsLeft int64
}

// checkMillisecondsLeft returns the amount of milliseconds left in a thread-safe manner.
func (t *Timer) checkMillisecondsLeft() int64 {
	t.mutex.Lock()
	retMilliseconds := t.millisecondsLeft
	t.mutex.Unlock()

	return retMilliseconds
}

// Start is a busy loop that will auto decrement the timer and call the provided
// cancel function when it has fully expired.
func (t *Timer) Start(receiveCtxCancel context.CancelFunc, closeHandlerChan func()) {
	lastTick := time.Now()

	common.Stdout.Printf("Starting the Pub/Sub timer with a max of %d seconds\n", t.maxSeconds)
	for {
		timeSince := time.Since(lastTick)
		t.Decrement(time.Duration(timeSince))
		lastTick = time.Now()

		if t.checkMillisecondsLeft() < 0 {
			// Cancel the parent context controlling this timer.
			receiveCtxCancel()

			// Cancel the handler ctx to trigger the
			closeHandlerChan()

			common.Stdout.Println("No time left, cancelling the timer context to end receiving from Pub/Sub")
			return
		}
		time.Sleep(loopDuration)
	}
}

// FinalizeBeforeContextCancel starts a busy loop that will auto decrement the timer and executes
// finalize before cancelling the context.
//
// Deprecate: The finalize feature is now implemented by the build ingestion
// handler.
func (t *Timer) FinalizeBeforeContextCancel(receiveCtxCancel context.CancelFunc, finalize func()) {
	lastTick := time.Now()

	common.Stdout.Printf("Starting the Pub/Sub timer with a max of %d seconds\n", t.maxSeconds)
	for {
		timeSince := time.Since(lastTick)
		t.Decrement(time.Duration(timeSince))
		lastTick = time.Now()

		if t.checkMillisecondsLeft() < 0 {
			common.Stdout.Println("No time left, calling finalize before cancelling the context to end receiving from Pub/Sub")
			finalize()
			// Cancel the parent context controlling this timer.
			receiveCtxCancel()
			common.Stdout.Println("Finalize complete, cancelling the timer context to end receiving from Pub/Sub")
			return
		}
		time.Sleep(loopDuration)
	}
}

// Decrement is a thread-safe function to reduce the amount of time left in the
// timer.
func (t *Timer) Decrement(duration time.Duration) {
	t.mutex.Lock()
	t.millisecondsLeft = t.millisecondsLeft - duration.Milliseconds()
	t.mutex.Unlock()
}

// Refresh sets the timer to the maximum amount of allotted time.
func (t *Timer) Refresh() {
	t.mutex.Lock()
	timerCeiling := time.Second * time.Duration(t.maxSeconds)
	t.millisecondsLeft = timerCeiling.Milliseconds()
	t.mutex.Unlock()
}

// InitTimer returns a waiting Timer set to the maximum amount of milliseconds
// provided.
func InitTimer(maxSeconds int) *Timer {
	t := &Timer{
		mutex:      sync.Mutex{},
		maxSeconds: maxSeconds,
	}

	// Set the time left using a thread-safe function.
	t.Refresh()

	return t
}

// ReceiveWithTimer implements the ReceiveClient interface with an
// auto-decrementing timer to cap idle time.
//
// NOTE: An idle timer is being implemented because the build reporting Pub/Sub
// feed is not a high QPS service so once we flush the channel, we do not expect
// more to arrive within the next hour(s). If any unexpectedly arrive after the
// receive is closed then they will be picked up in the next run.
type ReceiveWithTimer struct {
	ctx              context.Context
	receiveCancel    context.CancelFunc
	closeHandlerChan func()
	client           *pubsub.Client
	subscription     *pubsub.Subscription
	handleMessage    func(*pubsub.Message) error
	idleTimer        ReceiveTimer
}

// InitReceiveClientWithTimer returns a newly created Pub/Sub Client interface.
func InitReceiveClientWithTimer(ctx context.Context, projectID, subscriptionID string, closeHandlerChan func(), handleMessage func(*pubsub.Message) error) (ReceiveClient, error) {
	psClient := &ReceiveWithTimer{
		handleMessage:    handleMessage,
		closeHandlerChan: closeHandlerChan,
	}

	psClient.ctx, psClient.receiveCancel = context.WithCancel(ctx)

	err := psClient.initClient(projectID)
	if err != nil {
		return nil, err
	}

	err = psClient.initSubscription(subscriptionID)
	if err != nil {
		return nil, err
	}

	psClient.idleTimer = InitTimer(MaxIdleSeconds)

	return psClient, nil
}

// initClient creates the client interface for the current Pub/Sub Client.
func (r *ReceiveWithTimer) initClient(projectID string) error {
	common.Stdout.Printf("Initializing Pub/Sub client to %s GCP project\n", projectID)
	if r.client != nil {
		return fmt.Errorf("client is already initialized")
	}

	var err error
	r.client, err = pubsub.NewClient(r.ctx, projectID)
	if err != nil {
		return err
	}
	return nil
}

// initSubscription creates the client interface for the current Pub/Sub Client.
func (r *ReceiveWithTimer) initSubscription(subscriptionID string) error {
	common.Stdout.Printf("Initializing Pub/Sub subscription to %s \n", subscriptionID)
	if r.subscription != nil {
		return fmt.Errorf("subscription is already initialized")
	}

	// NOTE: A negative value here means that no limit is set for outstanding
	// messages. The default is 1000.
	rSettings := pubsub.ReceiveSettings{
		MaxOutstandingMessages: -1,
	}

	r.subscription = r.client.Subscription(subscriptionID)
	r.subscription.ReceiveSettings = rSettings

	return nil
}

// ingestMessage places all messages into a channel buffer where they will wait
// to be processed.
func (r *ReceiveWithTimer) ingestMessage(ctx context.Context, msg *pubsub.Message) {
	r.idleTimer.Refresh()
	err := r.handleMessage(msg)
	if err != nil {
		common.Stdout.Println(err)
		msg.Nack()
		return
	}
}

// PullMessages does a streaming pull of all messages in the release pubsub
// feed.
func (r *ReceiveWithTimer) PullMessages() error {
	// Begin the timer. When it expires it'll cancel the Receive client's
	// context ending the blocking receive.
	go r.idleTimer.Start(r.receiveCancel, r.closeHandlerChan)

	// Blocking pull all messages in the feed.
	common.Stdout.Printf("Begin receiving from Pub/Sub Subscription %s on project %s\n", r.subscription.ID(), r.client.Project())
	err := r.subscription.Receive(r.ctx, r.ingestMessage)
	common.Stdout.Printf("Done receiving from Pub/Sub Subscription %s on project %s\n", r.subscription.ID(), r.client.Project())
	if err != nil {
		return err
	}

	return nil
}

// PullAllMessagesForProcessing pulls all messages from the Pub/Sub Subscription associated
// with the ReceiveWithTimer instance for processing. It begins a timer, and when it expires,
// it executes the provided "finalize" function and cancels the Receive client's context,
// ending the blocking receive operation.
func (r *ReceiveWithTimer) PullAllMessagesForProcessing(finalize func()) error {
	// Begin the timer. When it expires it'll execute "finalize" and cancel the Receive
	// client's context ending the blocking receive.
	go r.idleTimer.FinalizeBeforeContextCancel(r.receiveCancel, finalize)

	// Blocking pull all messages in the feed.
	common.Stdout.Printf("Begin receiving from Pub/Sub Subscription %s on project %s\n", r.subscription.ID(), r.client.Project())
	err := r.subscription.Receive(r.ctx, r.ingestMessage)
	common.Stdout.Printf("Done receiving from Pub/Sub Subscription %s on project %s\n", r.subscription.ID(), r.client.Project())
	if err != nil {
		return err
	}

	return nil
}
