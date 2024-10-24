// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !linux
// +build !linux

package profile

import (
	"context"

	"github.com/pkg/profile"

	"go.chromium.org/luci/common/logging"
)

// Register creates a handler to catch SIGUSR1 and SIGUSR2 signals to start and
// stop profiling, respectively.
func Register(options ...func(*profile.Profile)) {
	logging.Errorf(context.Background(), "Profiling isn't supported on this platform")
}
