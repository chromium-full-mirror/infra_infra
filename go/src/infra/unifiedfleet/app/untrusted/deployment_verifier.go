// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package untrusted

import (
	"context"
	"time"

	"cloud.google.com/go/pubsub"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/server/router"

	"go.chromium.org/infra/unifiedfleet/app/config"
	"go.chromium.org/infra/unifiedfleet/app/controller"
	"go.chromium.org/infra/unifiedfleet/app/external"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

// VerifierEndpoint is the POST endpoint for use by pubsub service.
const VerifierEndpoint = "/pubsub/verify"

// verifierSubscription is the name of the subscription
const verifierSubscription = "ufs-verify"

// EnsureVerifierSubscription ensures that the topic for verification is created
// and the subscription is assigned.
func EnsureVerifierSubscription(ctx context.Context) error {
	// Ensure that our verifier topic is created
	topic, err := controller.CreatePubSubTopicClient(ctx, "verify")
	if err != nil {
		logging.Errorf(ctx, "Cannot create topic. %v", err)
		return nil
	}
	client := external.GetPubSub(ctx)
	if client == nil {
		logging.Errorf(ctx, "Cannot get pubsub client")
		return nil
	}
	sub := client.Subscription(verifierSubscription)
	exists, err := sub.Exists(ctx)
	if err != nil {
		logging.Errorf(ctx, "Can't figure out if %s subscription exists. %v", verifierSubscription, err)
		return nil
	}
	// Don't create one if it exists
	if exists {
		return nil
	}
	_, err = client.CreateSubscription(ctx, verifierSubscription, pubsub.SubscriptionConfig{
		Topic:       topic,
		AckDeadline: 600 * time.Second,
		PushConfig: pubsub.PushConfig{
			Endpoint: "https://" + config.Get(ctx).GetHostname() + VerifierEndpoint,
		},
	})
	if err != nil {
		logging.Errorf(ctx, "Cannot create subscription. %v", err)
		return nil
	}
	return nil
}

// DeploymentVerifier is the handler for VerifierEndpoint.
func DeploymentVerifier(routerCtx *router.Context) {
	res, err := util.NewPSRequest(routerCtx.Request)
	if err != nil {
		logging.Errorf(routerCtx.Request.Context(), "DeploymentVerifier - Failed to read push req %v", err)
		return
	}
	data, err := res.DecodeMessage()
	if err != nil {
		logging.Errorf(routerCtx.Request.Context(), "DeploymentVerifier - Failed to read data %v", err)
		return
	}
	logging.Debugf(routerCtx.Request.Context(), "Got data %x", data)
}
