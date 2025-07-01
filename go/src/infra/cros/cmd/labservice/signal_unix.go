// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows
// +build !windows

package main

import (
	"context"
	"os"

	"golang.org/x/sys/unix"

	"go.chromium.org/infra/cros/cmd/labservice/internal/log"
	"go.chromium.org/infra/cros/cmd/labservice/server"
)

var handledSignals = []os.Signal{unix.SIGINT, unix.SIGHUP, unix.SIGTERM, unix.SIGQUIT}

func handleSignal(ctx context.Context, s *server.Server, sig os.Signal) {
	log.Infof(ctx, "Got signal %s", sig)
	var graceful bool
	switch sig {
	case unix.SIGINT, unix.SIGHUP:
		graceful = true
	}
	s.Stop(graceful)
}
