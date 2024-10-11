// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package external

import (
	"context"
	"fmt"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/option"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/server/auth"
)

type PubSubClient struct {
	Client                  *pubsub.Client
	DeviceEventsPubSubTopic *pubsub.Topic
}

const DeviceEventsTopicName string = "device-events-v1"

// NewPubSubClient creates a PubSub client based on cloud project.
func NewPubSubClient(ctx context.Context, cloudProject string) (PubSubClient, error) {
	psClient := PubSubClient{}
	tokenSource, err := auth.GetTokenSource(ctx, auth.AsSelf, auth.WithScopes(auth.CloudOAuthScopes...))
	if err != nil {
		return psClient, errors.Annotate(err, "NewPubSubClient: failed to get AsSelf credentails").Err()
	}
	client, err := pubsub.NewClient(
		ctx, cloudProject,
		option.WithTokenSource(tokenSource),
	)
	if err != nil {
		logging.Errorf(ctx, "NewPubSubClient: cannot set up PubSub client: %s", err)
		return psClient, err
	}

	det, err := setTopic(ctx, client, DeviceEventsTopicName)
	if err != nil {
		logging.Errorf(ctx, "NewPubSubClient: cannot set up PubSub client: %s", err)
		return psClient, err
	}

	psClient.Client = client
	psClient.DeviceEventsPubSubTopic = det

	return psClient, nil
}

// setTopic verifies that a list of PubSub topics exist.
func setTopic(ctx context.Context, psClient *pubsub.Client, name string) (*pubsub.Topic, error) {
	topic := psClient.Topic(name)
	ok, err := topic.Exists(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("setTopic: topic %s not found", name)
	}
	return topic, nil
}
