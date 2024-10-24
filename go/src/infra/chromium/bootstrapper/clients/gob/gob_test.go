// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gob

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

type fakeClient struct {
	err   error
	max   int
	count int
}

func (c *fakeClient) op() error {
	c.count += 1
	if c.max < 0 || c.count <= c.max {
		return c.err
	}
	return nil
}

func messageForCode(code codes.Code) string {
	return fmt.Sprintf("fake %s error", code)
}

func TestExecute(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctx = UseTestClock(ctx)

	ftt.Run("Execute", t, func(t *ftt.Test) {

		var retriableErrors = []codes.Code{
			codes.NotFound,
			codes.Unavailable,
			codes.DeadlineExceeded,
			codes.ResourceExhausted,
		}

		t.Run("does not retry on errors without status code", func(t *ftt.Test) {
			client := &fakeClient{
				err: errors.New("fake error without code"),
				max: 1,
			}

			err := Execute(ctx, "fake op", client.op)

			assert.Loosely(t, err, should.ErrLike("fake error without code"))
		})

		t.Run("does not retry on errors with non-retriable code", func(t *ftt.Test) {
			client := &fakeClient{
				err: status.Error(codes.InvalidArgument, "fake error with non-retriable code"),
				max: 1,
			}

			err := Execute(ctx, "fake op", client.op)

			assert.Loosely(t, err, should.ErrLike("fake error with non-retriable code"))
		})

		for _, code := range retriableErrors {
			t.Run(fmt.Sprintf("retries on %s errors", code), func(t *ftt.Test) {
				message := messageForCode(code)
				client := &fakeClient{
					err: status.Error(code, message),
					max: 1,
				}

				err := Execute(ctx, "fake op", client.op)

				assert.Loosely(t, err, should.BeNil)

				t.Run("unless retries are disabled", func(t *ftt.Test) {
					client.count = 0
					ctx := DisableRetries(ctx)

					err := Execute(ctx, "fake op", client.op)

					assert.Loosely(t, err, should.ErrLike(message))
				})

				t.Run("when retries are re-enabled", func(t *ftt.Test) {
					client.count = 0
					ctx := EnableRetries(DisableRetries(ctx))

					err := Execute(ctx, "fake op", client.op)

					assert.Loosely(t, err, should.BeNil)
				})
			})
		}

		t.Run("fails if operation does not succeed within max time", func(t *ftt.Test) {
			code := retriableErrors[0]
			message := messageForCode(code)
			client := &fakeClient{
				err: status.Error(code, message),
				max: -1,
			}

			err := Execute(ctx, "fake op", client.op)

			assert.Loosely(t, err, should.ErrLike(message))
		})

	})
}
