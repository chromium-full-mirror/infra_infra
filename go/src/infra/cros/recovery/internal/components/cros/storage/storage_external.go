// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
)

// IsBootedFromExternalStorage verify that device has been booted from external storage.
func IsBootedFromExternalStorage(ctx context.Context, run components.Runner) error {
	bootStorage, err := run(ctx, time.Minute, "rootdev", "-s", "-d")
	if err != nil {
		return errors.Annotate(err, "booted from external storage").Err()
	} else if bootStorage == "" {
		return errors.Reason("booted from external storage: booted storage not detected").Err()
	}
	mainStorage, err := DeviceMainStoragePath(ctx, run)
	if err != nil {
		return errors.Annotate(err, "booted from external storage").Err()
	}
	metrics.DefaultActionAddObservations(ctx, metrics.NewStringObservation("booted_drive", bootStorage))
	metrics.DefaultActionAddObservations(ctx, metrics.NewStringObservation("internal_drive", mainStorage))
	// If main device is not detected then probably it can be dead or broken
	// but as we gt the boot device then it is external one.
	if mainStorage == "" || bootStorage != mainStorage {
		return nil
	}
	return errors.Reason("booted from external storage: booted from main storage").Err()
}
