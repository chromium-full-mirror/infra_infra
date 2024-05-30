// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package external

import (
	"context"
	"testing"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Test_verifyTopics ensures verifyTopics checks that topics are created or
// absent properly.
func Test_verifyTopics(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("pass; created all topics", func(t *testing.T) {
		srv := pstest.NewServer()
		defer func() {
			err := srv.Close()
			if err != nil {
				t.Logf("failed to close fake pubsub server: %s", err)
			}
		}()

		conn, err := grpc.Dial(srv.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Fatalf("could not start fake pubsub server")
		}
		defer func() {
			err = conn.Close()
			if err != nil {
				t.Logf("failed to close fake pubsub connection: %s", err)
			}
		}()

		psClient, err := pubsub.NewClient(ctx, "project", option.WithGRPCConn(conn))
		if err != nil {
			t.Fatalf("could not connect to fake pubsub server")
		}
		defer func() {
			err = psClient.Close()
			if err != nil {
				t.Logf("failed to close fake pubsub client: %s", err)
			}
		}()

		for _, topic := range allDMTopics {
			_, err = psClient.CreateTopic(ctx, topic)
			if err != nil {
				t.Fatalf("failed to create fake pubsub topic")
			}
		}

		if err = verifyTopics(ctx, psClient); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	})

	t.Run("fail; missing topics", func(t *testing.T) {
		srv := pstest.NewServer()
		defer func() {
			err := srv.Close()
			if err != nil {
				t.Logf("failed to close fake pubsub server: %s", err)
			}
		}()

		conn, err := grpc.Dial(srv.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Fatalf("could not start fake pubsub server")
		}
		defer func() {
			err = conn.Close()
			if err != nil {
				t.Logf("failed to close fake pubsub connection: %s", err)
			}
		}()

		psClient, err := pubsub.NewClient(ctx, "project", option.WithGRPCConn(conn))
		if err != nil {
			t.Fatalf("could not connect to fake pubsub server")
		}
		defer func() {
			err = psClient.Close()
			if err != nil {
				t.Logf("failed to close fake pubsub client: %s", err)
			}
		}()

		if err = verifyTopics(ctx, psClient); err == nil {
			t.Fatalf("unexpected success")
		}
	})
}
