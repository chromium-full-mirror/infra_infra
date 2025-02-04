// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"fmt"

	"cloud.google.com/go/pubsub"
)

// PublishToTestJobEventPubSub publishes provided test job event msg to ATP pubsub
func PublishToTestJobEventPubSub(ctx context.Context, client *pubsub.Client, msg *TestJobEventMessage) (string, error) {
	if msg == nil {
		return "", fmt.Errorf("cannot publish empty msg")
	}

	if client == nil {
		return "", fmt.Errorf("cannot publish to nil client")
	}

	msgBytes, err := msg.MarshalJSON()
	if err != nil {
		return "", fmt.Errorf("failed to marshal json: %w", err)
	}

	return PublishToPubSub(ctx, client, ATPSwitcherTestJobEventTopicID, msgBytes)
}

// PublishToPubSub publishes provided input to provided topic and client
func PublishToPubSub(ctx context.Context, client *pubsub.Client, topicID string, msgBytes []byte) (string, error) {
	// Publish
	t := client.Topic(topicID)
	result := t.Publish(ctx, &pubsub.Message{
		Data: msgBytes,
	})

	// Block until the result is returned and a server-generated
	// ID is returned for the published message.
	id, err := result.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("err while publishing to %s pub/sub: %w", topicID, err)
	}
	fmt.Printf("Published a message to %s with ID: %s\n", topicID, id)
	return id, nil
}
