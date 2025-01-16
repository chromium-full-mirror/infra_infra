// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows
// +build !windows

package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"golang.org/x/sys/unix"
)

func ctxWithSignal(ctx context.Context) context.Context {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, unix.SIGTERM)

	ctx, cancel := context.WithCancel(ctx)
	go func() {
		sig := <-c
		log.Printf("Caught signal: %s.", sig)
		cancel()
	}()
	return ctx
}
