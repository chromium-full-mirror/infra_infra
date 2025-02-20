// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/cros"
	"go.chromium.org/infra/cros/recovery/internal/components/cros/storage"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
	"go.chromium.org/infra/cros/recovery/scopes"
)

// isBootedFromExternalStorageExec verify that device has been booted from external storage.
func isBootedFromExternalStorageExec(ctx context.Context, info *execs.ExecInfo) error {
	err := storage.IsBootedFromExternalStorage(ctx, info.NewRunner(info.GetDut().Name))
	return errors.Annotate(err, "is booted from external storage").Err()
}

// readBootIdExec reads bootId of the host.
//
// Publish bootId to config scope if required.
// Compare bootId to config scope if required.
func readBootIdExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	publishBootId := argsMap.AsBool(ctx, "publish", false)
	compareBootId := argsMap.AsBool(ctx, "compare", false)
	skipEmpty := argsMap.AsBool(ctx, "skip_empty", false)
	timeout := argsMap.AsDuration(ctx, "timeout", 10, time.Second)
	run := info.DefaultRunner()
	bootId, err := cros.BootID(ctx, timeout, run)
	if err != nil {
		return errors.Annotate(err, "read bootId").Err()
	}
	log.Debugf(ctx, "Host has bootID: %q", bootId)
	if skipEmpty && bootId == "" {
		info.AddObservation(metrics.NewStringObservation("boot_id_empty", "skipped"))
		log.Debugf(ctx, "BootId is empty!")
		return nil
	}
	scopeKey := fmt.Sprintf("boot_id_%s", info.GetActiveResource())
	if compareBootId {
		if oldBootId, ok := scopes.ReadConfigParam(ctx, scopeKey); ok {
			log.Debugf(ctx, "Previous BootId: %q", oldBootId)
			if oldBootId != bootId {
				return errors.Reason("read bootId: expected %q but got %q", oldBootId, bootId).Err()
			}
			log.Debugf(ctx, "BootIds equal, we are good!")
		} else {
			log.Debugf(ctx, "No previous bootId value. Assume we are good.")
		}
	}
	if publishBootId {
		scopes.PutConfigParam(ctx, scopeKey, bootId)
	}
	return nil
}

func init() {
	execs.Register("cros_booted_from_external_storage", isBootedFromExternalStorageExec)
	execs.Register("cros_read_bootid", readBootIdExec)
}
