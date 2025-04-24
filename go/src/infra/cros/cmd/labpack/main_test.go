// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"testing"
	"time"

	"go.chromium.org/infra/cros/recovery/logger"
)

func TestCheckLeaseAlreadyExpired(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	lg := logger.NewLogger()

	extenderOverrider := func(context.Context, string, time.Duration) (time.Time, error) {
		return time.Now().UTC(), nil
	}

	if err := checkLeaseAlreadyExpired(ctx, lg, "some-lease-id", "some-pool", extenderOverrider); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
