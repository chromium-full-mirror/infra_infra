// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build windows
// +build windows

package main

import (
	"context"
	"net/http"
	"time"
)

func cancelOnSignals(ctx context.Context, svr *http.Server, gracePeriod time.Duration) context.Context {
	panic("windows not supported")
}
