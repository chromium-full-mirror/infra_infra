// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"fmt"

	"cloud.google.com/go/pubsub"

	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/unifiedfleet/app/external"
)

// CreatePubSubTopicClient returns back an instance of a Pub/Sub client for the
// given target.
func CreatePubSubTopicClient(ctx context.Context, topicID string) (*pubsub.Topic, error) {
	logging.Debugf(ctx, "pubsub_stream: Creating topic %s for pubsub publishing.", topicID)
	// Create client for Pub/Sub publishing.
	client := external.GetPubSub(ctx)
	if client == nil {
		return nil, fmt.Errorf("pubsub_stream: Pubsub client is nil")
	}

	// Associate the Pub/Sub client with the correct topicID.
	topic := client.Topic(topicID)

	// Check if the topic exists on the project.go test.
	exists, err := topic.Exists(ctx)
	if err != nil {
		return nil, fmt.Errorf("pubsub_stream: Error when checking if topic %s exist error: %s", topicID, err.Error())
	}

	// Attempt to create the topic if it doesn't exist.
	if !exists {
		topic, err = client.CreateTopic(ctx, topicID)
		if err != nil {
			return nil, fmt.Errorf("pubsub_stream: Topic %s doesn't exist and failed to be created. error: %s", topicID, err.Error())
		}
	}
	logging.Debugf(ctx, "pubsub_stream: Topic for %s successfully made.", topicID)
	return topic, nil
}

// publish wraps all the steps required to send a message to a Pub/Sub topic.
func publish(ctx context.Context, topicId string, msgs [][]byte) error {
	logging.Debugf(ctx, "pubsub_stream: attempting to publish %d messages.", len(msgs))
	topic, err := CreatePubSubTopicClient(ctx, topicId)
	if err != nil {
		return err
	}

	var results []*pubsub.PublishResult
	for _, msg := range msgs {
		// Asynchronously publish the message.
		// NOTE: By default publish waits for up-to 1000 messages for batching.
		// NOTE: By default publish spins up a maximum of GOMAXPROCS goroutines.
		result := topic.Publish(ctx, &pubsub.Message{Data: msg})
		results = append(results, result)
	}

	// Block and wait to check for publishing errors.
	logging.Debugf(ctx, "pubsub_stream: waiting on results from %d messages.", len(msgs))
	for _, result := range results {
		_, err = result.Get(ctx)
		// Rather than flood errors, fail fast.
		if err != nil {
			return err
		}
	}

	logging.Debugf(ctx, "pubsub_stream: published %d messages.", len(results))
	return nil
}
