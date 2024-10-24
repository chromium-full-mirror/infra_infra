// Copyright 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"testing"

	"google.golang.org/grpc/grpclog"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/memlogger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestDisableGRPCLogging(t *testing.T) {
	ftt.Run(`LogDog executions suppress gRPC print-level logging`, t, func(t *ftt.Test) {
		var (
			ctx = context.Background()
			ml  memlogger.MemLogger
		)

		// Install our memory logger.
		ctx = logging.SetFactory(ctx, func(context.Context) logging.Logger { return &ml })

		// Call "runWithLogdogButler". This should panic, but, more importantly for
		// this test, should also install our gRPC log suppression. Note that this
		// is GLOBAL, so we cannot run this in parallel.
		t.Run(`When log level is Info, does not log Prints.`, func(t *ftt.Test) {
			ctx = logging.SetLevel(ctx, logging.Info)
			disableGRPCLogging(ctx)

			grpclog.Println("TEST!")
			assert.Loosely(t, ml.Messages(), should.HaveLength(0))
		})

		t.Run(`When log level is Debug, does log Prints.`, func(t *ftt.Test) {
			ctx = logging.SetLevel(ctx, logging.Debug)
			disableGRPCLogging(ctx)

			grpclog.Println("TEST!")
			assert.Loosely(t, ml.Messages(), should.HaveLength(1))
		})
	})
}
