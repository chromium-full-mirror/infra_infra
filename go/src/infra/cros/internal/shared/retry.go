// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shared

import (
	"context"
	"log"
	"math"
	"time"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/errors/errtag/stacktag"
)

// Options wraps retry options.
type Options struct {
	BaseDelay   time.Duration // backoff base delay.
	BackoffBase float64       // base for exponential backoff
	Retries     int           // allowed number of retries.
}

// DoFunc is a function type that can be retried by DoWithRetry if the return error is not nil.
type DoFunc func() error

var (
	// ExtremeOpts gives an even longer timeout and more retries (~30 min).
	ExtremeOpts = Options{BaseDelay: 120 * time.Second, BackoffBase: 2.0, Retries: 10}
	// LongerOpts gives a longer timeout than default (~7.5 minutes).
	LongerOpts = Options{BaseDelay: 30 * time.Second, BackoffBase: 2.0, Retries: 5}
	// DefaultOpts is the default timeout (~5 minutes).
	DefaultOpts = Options{BaseDelay: 10 * time.Second, BackoffBase: 2.0, Retries: 5}
	// ShortOpts is for operations that need rapid results.
	ShortOpts = Options{BaseDelay: 500 * time.Millisecond, BackoffBase: 1.0, Retries: 1}
	// NoRetryOpts is for unretriable requests or testing.
	NoRetryOpts = Options{BaseDelay: 0 * time.Second, BackoffBase: 1.0, Retries: 0}
)

// DoWithRetry executes function doFunc. If there is an error, it will retry with a backoff delay
// until max retry times reached or context done.
// If retryOpts.Retries == 0, it will execute doFunc just once without any retries.
// If retryOpts.Retries < 0, it retries an infinite number of times.
func DoWithRetry(ctx context.Context, retryOpts Options, doFunc DoFunc) error {
	var err error
	for i := 0; retryOpts.Retries < 0 || i <= retryOpts.Retries; i++ {
		var d time.Duration
		if i > 0 {
			d = time.Duration(float64(retryOpts.BaseDelay) * math.Pow(retryOpts.BackoffBase, float64(i-1)))
			log.Printf("Sleeping for %s before trying again", d.String())
		}
		select {
		case <-ctx.Done():
			return stacktag.Capture(errors.Join(err, ctx.Err()), 0)
		case <-time.After(d):
			err = doFunc()
			if err == nil {
				return nil
			}
			log.Printf("DoWithRetry [%d]: %v", i, err)
		}
	}
	return errors.WrapIf(err, "failed after %d retries", retryOpts.Retries)
}
