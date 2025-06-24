// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Binary bbsched emits a stream of ScheduleBuild requests at a constant QPS.
package main

import (
	"context"
	"flag"
	"math/rand"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/luci/auth"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/common/system/signals"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/hardcoded/chromeinfra"
)

var (
	qps         = flag.Int("qps", 1, "QPS to call ScheduleBuild at")
	duration    = flag.Duration("duration", time.Minute, "How long to run this")
	request     = flag.String("request", "", "Path to a JSON request to send")
	buildbucket = flag.String("buildbucket", "cr-buildbucket-dev.appspot.com", "Buildbucket host to send requests to")
)

type Result struct {
	Build    *buildbucketpb.Build
	Err      error
	Duration time.Duration
}

func main() {
	flag.Parse()

	ctx := context.Background()
	ctx = gologger.StdConfig.Use(ctx)

	ctx, done := context.WithCancel(ctx)
	signals.HandleInterrupt(done)

	if err := run(ctx); err != nil {
		errors.Log(ctx, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	if *request == "" {
		return errors.Reason("-request is required").Err()
	}
	blob, err := os.ReadFile(*request)
	if err != nil {
		return errors.Annotate(err, "failed to read -request file").Err()
	}

	var req buildbucketpb.ScheduleBuildRequest
	if err := protojson.Unmarshal(blob, &req); err != nil {
		return errors.Annotate(err, "failed to unmarshal ScheduleBuildRequest").Err()
	}

	httpC, err := auth.NewAuthenticator(ctx, auth.SilentLogin, chromeinfra.DefaultAuthOptions()).Client()
	if err != nil {
		return errors.Annotate(err, "creating auth client").Err()
	}

	prpcC := prpc.Client{C: httpC, Host: *buildbucket}
	bbC := buildbucketpb.NewBuildsClient(&prpcC)

	ctx, done := context.WithTimeout(ctx, *duration)
	defer done()

	// Channel with results, so they can be printed nicely in a single loop.
	res := make(chan Result, 10000)

	// This is kind of dumb, but should "scale" well and produce relatively even
	// load.
	wg := sync.WaitGroup{}
	wg.Add(*qps)
	for range *qps {
		go func() {
			defer wg.Done()
			sendOneQPS(ctx, res, bbC, &req)
		}()
	}

	// Close `res` when all senders are done. This will unblock the loop below.
	go func() {
		wg.Wait()
		close(res)
	}()

	total := 0
	for r := range res {
		handleResult(ctx, r)
		total += 1
	}

	logging.Infof(ctx, "Sent %d requests total", total)
	return nil
}

func sendOneQPS(ctx context.Context, res chan Result, bbC buildbucketpb.BuildsClient, req *buildbucketpb.ScheduleBuildRequest) {
	// Randomize start time to desynchronize goroutines.
	rnd := rand.Int63n(1000)
	next := time.Now().Add(time.Duration(rnd) * time.Millisecond)

	for {
		// How long we need to sleep to get ~= 1 QPS since the last *send* time.
		dt := time.Until(next)

		// Randomize sleep a bit to avoid all goroutines sending requests in
		// a lockstep.
		rnd := rand.Int63n(50) - 100
		dt += time.Duration(rnd) * time.Millisecond
		if dt < 0 {
			dt = 0
		}

		// Actual sleep.
		select {
		case <-ctx.Done():
			return
		case <-time.After(dt):
		}

		// This is when we need to star the next send operation to get 1 QPS.
		next = time.Now().Add(time.Second)

		// Now do the actual call.
		start := time.Now()
		build, err := bbC.ScheduleBuild(ctx, req)
		if ctx.Err() != nil {
			return // don't report canceled calls
		}
		res <- Result{
			Build:    build,
			Err:      err,
			Duration: time.Since(start),
		}
	}
}

func handleResult(ctx context.Context, res Result) {
	if res.Err == nil {
		logging.Infof(ctx, "%5dms OK luci-milo-dev.appspot.com/b/%d", res.Duration.Milliseconds(), res.Build.Id)
	} else {
		logging.Warningf(ctx, "%5dms %s", res.Duration.Milliseconds(), status.Code(res.Err))
	}
}
