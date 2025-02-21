// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gob

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"

	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/retry"
	"go.chromium.org/luci/grpc/grpcutil"
)

// ErrorIsRetriable determines whether an error would be retried by Execute.
func ErrorIsRetriable(err error) bool {
	switch grpcutil.Code(err) {
	case codes.NotFound, codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted:
		return true
	}
	return false
}

type retryIterator struct {
	backoff retry.ExponentialBackoff
}

func (i *retryIterator) Next(ctx context.Context, err error) time.Duration {
	if ErrorIsRetriable(err) {
		return i.backoff.Next(ctx, err)
	}
	return retry.Stop
}

var ctxKey = "infra/chromium/bootstrapper/clients/gob.ExecuteRetriesEnabled"

// EnableRetries enables retries for Execute (the default behavior).
func EnableRetries(ctx context.Context) context.Context {
	return context.WithValue(ctx, &ctxKey, true)
}

// DisableRetries disables retries for Execute.
func DisableRetries(ctx context.Context) context.Context {
	return context.WithValue(ctx, &ctxKey, false)
}

// Execute attempts a GoB operation with retries.
//
// Execute mitigates the effects of short-lived outages and replication lag by retrying operations
// with certain error codes. The service client's error should be returned in order to correctly
// detect this situation. The retries will use exponential backoff with a context with the clock
// tagged with "gob-retry". When performing retries, a log will be emitted that uses opName to
// identify the operation that is being retried.
//
// Retries will not be performed in the following cases:
//   - The provided context is one that has had DisableRetries called on it more recently than
//     EnableRetries
//   - The error returned from the operation is tagged with DontRetry
func Execute(ctx context.Context, opName string, fn func() error) error {
	ctxVal := ctx.Value(&ctxKey)
	retriesEnabled := true
	if ctxVal != nil {
		retriesEnabled, _ = ctxVal.(bool)
	}
	var retryFactory retry.Factory
	if retriesEnabled {
		retryFactory = func() retry.Iterator {
			return &retryIterator{
				backoff: retry.ExponentialBackoff{
					Limited: retry.Limited{
						Delay:    time.Second,
						MaxTotal: 10 * time.Minute,
						// Don't limit the number of retries, just use the MaxTotal
						Retries: -1,
					},
					Multiplier: 2,
					MaxDelay:   30 * time.Second,
				},
			}
		}
	}
	ctx = clock.Tag(ctx, "gob-retry")
	return retry.Retry(ctx, retryFactory, fn, retry.LogCallback(ctx, opName))
}

func UseTestClock(ctx context.Context) context.Context {
	tc, ok := clock.Get(ctx).(testclock.TestClock)
	if !ok {
		ctx, tc = testclock.UseTime(ctx, testclock.TestTimeUTC)
	}

	tc.SetTimerCallback(func(d time.Duration, t clock.Timer) {
		if testclock.HasTags(t, "gob-retry") {
			tc.Add(d) // Fast-forward through sleeps in the test.
		}
	})
	return ctx
}
