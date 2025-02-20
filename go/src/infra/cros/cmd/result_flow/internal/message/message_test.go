// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package message_test

import (
	"context"
	"testing"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"google.golang.org/api/option"
	"google.golang.org/grpc"

	"go.chromium.org/chromiumos/infra/proto/go/test_platform/result_flow"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/cmd/result_flow/internal/message"
)

type received struct {
	buildID                 int64
	parentUID               string
	shouldPollForCompletion bool
}

func TestMessage(t *testing.T) {
	fakeConfig := &result_flow.PubSubConfig{
		Project:              "test-project",
		Topic:                "test-topic",
		Subscription:         "test-sub",
		MaxReceivingMessages: 50,
	}
	ctx := context.Background()
	// Start a fake server running locally.
	srv := pstest.NewServer()
	defer srv.Close()

	setupPubSubServer(ctx, fakeConfig, newConnection(srv.Addr))

	mClient, err := message.NewClient(ctx, fakeConfig, 2, option.WithGRPCConn(newConnection(srv.Addr)))
	if err != nil {
		panic(err)
	}

	cases := []struct {
		description string
		in          map[string]string
		expected    *received
	}{
		{
			"CTP message with Build ID only",
			map[string]string{
				message.BuildIDKey: "8878576942164330944",
			},
			&received{
				buildID: 8878576942164330944,
			},
		},
		{
			"CTP message sent after the execution step",
			map[string]string{
				message.BuildIDKey:                 "8878576942164330944",
				message.ShouldPollForCompletionKey: "True",
			},
			&received{
				buildID:                 8878576942164330944,
				shouldPollForCompletion: true,
			},
		},
		{
			"Test runner message with Build ID and Parent UID",
			map[string]string{
				message.BuildIDKey:   "8878576942164330945",
				message.ParentUIDKey: "TestPlanRun/foo/fake-test",
			},
			&received{
				buildID:   8878576942164330945,
				parentUID: "TestPlanRun/foo/fake-test",
			},
		},
		{
			"Test runner message sent after the execution step",
			map[string]string{
				message.BuildIDKey:                 "8878576942164330945",
				message.ParentUIDKey:               "TestPlanRun/foo/fake-test",
				message.ShouldPollForCompletionKey: "True",
			},
			&received{
				buildID:                 8878576942164330945,
				parentUID:               "TestPlanRun/foo/fake-test",
				shouldPollForCompletion: true,
			},
		},
		{
			"Message with explicit false ShouldPollForCompletion in its attribute",
			map[string]string{
				message.BuildIDKey:                 "8878576942164330945",
				message.ParentUIDKey:               "TestPlanRun/foo/fake-test",
				message.ShouldPollForCompletionKey: "False",
			},
			&received{
				buildID:                 8878576942164330945,
				parentUID:               "TestPlanRun/foo/fake-test",
				shouldPollForCompletion: false,
			},
		},
	}
	for _, c := range cases {
		ftt.Run(c.description, t, func(t *ftt.Test) {
			if err := message.PublishBuild(ctx, c.in, fakeConfig, option.WithGRPCConn(newConnection(srv.Addr))); err != nil {
				panic(err)
			}
			msgs, err := mClient.PullMessages(ctx)
			mClient.AckMessages(ctx, msgs)
			if err != nil {
				panic(err)
			}
			got := message.ExtractBuildIDMap(ctx, msgs)
			assert.Loosely(t, got, should.ContainKey(c.expected.buildID))
			assert.Loosely(t, message.GetParentUID(got[c.expected.buildID]), should.Equal(c.expected.parentUID))
			assert.Loosely(t, message.ShouldPollForCompletion(got[c.expected.buildID]), should.Equal(c.expected.shouldPollForCompletion))
		})
	}
}

// Create a new connection to the server without using TLS.
func newConnection(addr string) *grpc.ClientConn {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	return conn
}

// Setup the fake Topic and Subscription in the local Pub/Sub server.
func setupPubSubServer(ctx context.Context, conf *result_flow.PubSubConfig, conn *grpc.ClientConn) {
	client, err := pubsub.NewClient(ctx, conf.Project, option.WithGRPCConn(conn))
	if err != nil {
		panic(err)
	}
	defer client.Close()
	topic, err := client.CreateTopic(ctx, conf.Topic)
	if err != nil {
		panic(err)
	}
	if _, err := client.CreateSubscription(ctx, conf.Subscription, pubsub.SubscriptionConfig{Topic: topic}); err != nil {
		panic(err)
	}
}
