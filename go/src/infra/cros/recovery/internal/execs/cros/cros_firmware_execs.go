// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components/cros/firmware"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/logger/metrics"
	"infra/cros/recovery/tlw"
)

// collectFirmwareTargetExec read fw target from DUT to inventory.
//
// If inventory has data then data will not be overwritten.
func collectFirmwareTargetExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	isForceOverride := argsMap.AsBool(ctx, "force_override", false)
	cros := info.GetChromeos()
	if cros == nil {
		return errors.Reason("collect firmware target: only for chromeos devices").Err()
	}
	fi := cros.GetFirmwareInfo()
	if fi == nil {
		fi = &tlw.FirmwareInfo{}
		cros.FirmwareInfo = fi
	}
	log.Debugf(ctx, "Fw targets before update ec:%q, ap:%q", fi.GetEcTarget(), fi.GetApTarget())
	run := info.NewRunner(info.GetDut().Name)
	ec, ap, err := firmware.ReadConfigYAML(ctx, cros.GetModel(), run, log.Get(ctx))
	if err != nil {
		return errors.Annotate(err, "collect firmware target").Err()
	}
	metrics.DefaultActionAddObservations(ctx,
		metrics.NewStringObservation("collected_ec_target", ec),
		metrics.NewStringObservation("collected_ap_target", ap),
	)
	if isForceOverride || fi.GetApTarget() == "" {
		log.Debugf(ctx, "AP target updated from %q to %q.", fi.GetApTarget(), ap)
		fi.ApTarget = ap
	}
	if isForceOverride || fi.GetEcTarget() == "" {
		log.Debugf(ctx, "EC target updated from %q to %q.", fi.GetEcTarget(), ec)
		fi.EcTarget = ec
	}
	return nil
}

// setFirmwareTargetsExec sets fw-targets
func setFirmwareTargetsExec(ctx context.Context, info *execs.ExecInfo) error {
	cros := info.GetChromeos()
	if cros == nil {
		return errors.Reason("set fw-targets: only for chromeos devices").Err()
	}
	fi := cros.GetFirmwareInfo()
	if fi == nil {
		return errors.Reason("set fw-targets: only for override firmware targets").Err()
	}
	argsMap := info.GetActionArgs(ctx)
	if target := argsMap.AsString(ctx, "ec_target", ""); target != "" {
		fi.EcTarget = target
	}
	if target := argsMap.AsString(ctx, "ap_target", ""); target != "" {
		fi.ApTarget = target
	}
	return nil
}

func init() {
	execs.Register("cros_collect_firmware_target", collectFirmwareTargetExec)
	execs.Register("cros_set_fw_targets", setFirmwareTargetsExec)
}
