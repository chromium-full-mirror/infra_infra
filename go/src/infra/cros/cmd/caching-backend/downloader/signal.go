// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows
// +build !windows

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"golang.org/x/sys/unix"
)

func cancelOnSignals(ctx context.Context, svr *http.Server, gracePeriod time.Duration) context.Context {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, unix.SIGTERM)
	outerCtx, outerCancel := context.WithCancel(ctx)
	// We leak the cancellation resource if we never get a signal.
	go func() {
		sig := <-c
		log.Printf("Caught signal: %s. Gracefully shutting down archive-server", sig)
		ctx, cancel := context.WithTimeout(outerCtx, gracePeriod)
		defer cancel()
		if err := svr.Shutdown(ctx); err != nil {
			log.Printf("archive-server shutdown unsuccesfully: %v", err)
		} else {
			log.Printf("archive-server shutdown successfully!")
		}
		outerCancel()
	}()
	return outerCtx
}
