// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"context"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// RunInstallOSCommand run chromeos-install command on the host.
func RunInstallOSCommand(ctx context.Context, timeout time.Duration, run components.Runner, dst string) error {
	var startTime time.Time
	install := func() (string, error) {
		startTime = time.Now()
		args := []string{"--yes"}
		if dst != "" {
			log.Infof(ctx, "Received %q as destination device!", dst)
			args = append(args, "--dst", dst)
		}
		return run(ctx, timeout, "chromeos-install", args...)
	}
	out, err := install()
	if StorageIssuesExist(ctx, err) == tlw.DutStateReasonInternalStorageCannotDetected {
		if dst = DetectInternalStorage(ctx, run); dst != "" {
			log.Infof(ctx, "Retry install due manually detect destination device as %q.", dst)
			out, err = install()
		}
	}
	execTime := time.Since(startTime)
	log.Debugf(ctx, "Execution time: %s", execTime.Seconds())
	log.Debugf(ctx, "Install OS:\n%s", out)
	if err != nil {
		metrics.DefaultActionAddObservations(ctx,
			metrics.NewFloat64Observation("fail_chromeos_install_exec_time_sec", execTime.Seconds()),
		)
		return errors.Annotate(err, "install OS").Err()
	}
	metrics.DefaultActionAddObservations(ctx,
		metrics.NewFloat64Observation("success_chromeos_install_exec_time_sec", execTime.Seconds()),
	)
	return nil
}

// storageErrors are all the possible key parts of error messages that can be
// generated if ChromeOS install process fails due to errors with the
// storage device.
var storageErrors = map[string]tlw.DutStateReason{
	"No space left on device":                    tlw.DutStateReasonInternalStorageNoSpaceLeft,
	"I/O error when trying to write primary GPT": tlw.DutStateReasonInternalStorageIOError,
	"Input/output error while writing out":       tlw.DutStateReasonInternalStorageIOError,
	"cannot read GPT header":                     tlw.DutStateReasonInternalStorageIOError,
	"can not determine destination device":       tlw.DutStateReasonInternalStorageCannotDetected,
	"wrong fs type":                              tlw.DutStateReasonInternalStorageIOError,
	"bad superblock on":                          tlw.DutStateReasonInternalStorageUncategorizedError,
}

// StorageIssuesExist checks is error indicate issue with storage.
func StorageIssuesExist(ctx context.Context, err error) tlw.DutStateReason {
	if err == nil {
		return tlw.DutStateReasonEmpty
	}
	stdErr, ok := errors.TagValueIn(components.StdErrTag, err)
	if !ok {
		log.Debugf(ctx, "Check storage error: stderr not found.")
		return tlw.DutStateReasonEmpty
	}
	stdErrStr := stdErr.(string)
	// Check if the error message contains any message indicating a problem with the storage.
	for storageError, reason := range storageErrors {
		if strings.Contains(stdErrStr, storageError) {
			log.Debugf(ctx, "Failed to install ChromeOS due to the specified storage error: %q", storageError)
			return reason
		}
	}
	return tlw.DutStateReasonEmpty
}
