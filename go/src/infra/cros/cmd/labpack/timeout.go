// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"
)

// ctxFunc is a function that takes no arguments and returns an error indicating
// whether it was successful or not.
type ctxFunc = func(context.Context) error

const (
	// completed means that the task ran to completion.
	completed = "completed"
	// interrupted means that the task did not run to completion.
	interrupted = "interrupted"
)

// callFuncWithTimeout synchronously calls a unary function with a timeout.
// It returns whatever error was produced by the unary function in question, or a dedicated error
// if the deadline was exceeded.
// The status unambiguously indicates whether the function ran to completion or not.
func callFuncWithTimeout(ctx context.Context, timeout time.Duration, cb ctxFunc) (status string, err error) {
	ctxTimeout, cancelHandle := context.WithTimeout(ctx, timeout)
	defer cancelHandle()
	ch := make(chan error, 1)
	go func() {
		ch <- cb(ctxTimeout)
	}()
	select {
	case <-ctxTimeout.Done():
		return interrupted, errors.WrapIf(ctxTimeout.Err(), "deadline exceeded")
	case e := <-ch:
		return completed, e
	}
}
