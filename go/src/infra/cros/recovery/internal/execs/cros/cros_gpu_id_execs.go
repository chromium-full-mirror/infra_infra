// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

const (
	// command to grab the gpu_id from dut
	gpuIDCmd = `/usr/local/graphics/hardware_probe --labels-reporting | jq -r .gpu_id`
)

// collectGpuIDExec read gpu_id from dut to inventory.
//
// Find the gpu_id with hardware_probe command and set it on ChromeOS struct
func collectGpuIDExec(ctx context.Context, info *execs.ExecInfo) error {
	r := info.DefaultRunner()

	cros := info.GetChromeos()
	if cros == nil {
		return errors.Reason("collect gpu_id: only for chromeos devices").Err()
	}

	log.Debugf(ctx, "gpu_id before update: %s", cros.GetGpuId())

	gpuID, err := r(ctx, time.Minute, gpuIDCmd)
	if err != nil {
		return errors.Annotate(err, "collect gpu_id").Err()
	}

	log.Debugf(ctx, "gpu_id to set: %s", gpuID)
	cros.GpuId = gpuID
	return nil
}

func init() {
	execs.Register("cros_collect_gpu_id", collectGpuIDExec)
}
