// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build windows
// +build windows

package main

import (
	"context"
	"os"

	"go.chromium.org/infra/cros/cmd/labservice/server"
)

var handledSignals = []os.Signal{}

func handleSignal(ctx context.Context, s *server.Server, sig os.Signal) {
	panic("not implemented for windows")
}
