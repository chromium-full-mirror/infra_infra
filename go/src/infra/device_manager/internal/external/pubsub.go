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

const DeviceEventsPubSubTopic string = "device-events-v1"

var allDMTopics = []string{
	DeviceEventsPubSubTopic,
}

// NewPubSubClient creates a PubSub client based on cloud project.
func NewPubSubClient(ctx context.Context, cloudProject string) (*pubsub.Client, error) {
	tokenSource, err := auth.GetTokenSource(ctx, auth.AsSelf, auth.WithScopes(auth.CloudOAuthScopes...))
	if err != nil {
		return nil, errors.Annotate(err, "NewPubSubClient: failed to get AsSelf credentails").Err()
	}
	client, err := pubsub.NewClient(
		ctx, cloudProject,
		option.WithTokenSource(tokenSource),
	)
	if err != nil {
		logging.Errorf(ctx, "NewPubSubClient: cannot set up PubSub client: %s", err)
		return nil, err
	}

	err = verifyTopics(ctx, client)
	if err != nil {
		logging.Errorf(ctx, "NewPubSubClient: cannot set up PubSub client: %s", err)
		return nil, err
	}
	return client, nil
}

// verifyTopics verifies that a list of PubSub topics exist.
func verifyTopics(ctx context.Context, psClient *pubsub.Client) error {
	for _, t := range allDMTopics {
		topic := psClient.Topic(t)
		defer topic.Stop()

		ok, err := topic.Exists(ctx)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("verifyTopics: topic %s not found", t)
		}
	}
	return nil
}
