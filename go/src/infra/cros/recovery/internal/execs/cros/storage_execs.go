// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"strconv"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/cros/storage"
	"go.chromium.org/infra/cros/recovery/internal/components/linux"
	"go.chromium.org/infra/cros/recovery/internal/execs"
)

// auditStorageSMARTExec confirms that it is able to audit
// smartStorage info and mark the DUT if it needs replacement.
func auditStorageSMARTExec(ctx context.Context, info *execs.ExecInfo) error {
	if err := storage.AuditStorageSMART(ctx, info.DefaultRunner(), info.GetChromeos().GetStorage(), info.GetDut()); err != nil {
		return errors.Annotate(err, "audit storage smart").Err()
	}
	return nil
}

// auditStorageBadblocksExec confirms that it is able to audit storage
// using badblocks, and mark the DUT if it needs replacement.
func auditStorageBadblocksExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	bbMode := storage.AuditMode(argsMap.AsString(ctx, "badblocks_mode", "auto"))
	timeoutRO := argsMap.AsDuration(ctx, "rw_badblocks_timeout", 5400, time.Second)
	timeoutRW := argsMap.AsDuration(ctx, "ro_badblocks_timeout", 3600, time.Second)
	bbArgs := storage.BadBlocksArgs{
		AuditMode: bbMode,
		Run:       info.DefaultRunner(),
		Storage:   info.GetChromeos().GetStorage(),
		Dut:       info.GetDut(),
		Metrics:   info.GetMetrics(),
		TimeoutRW: timeoutRW,
		TimeoutRO: timeoutRO,
		NewMetric: info.NewMetric,
	}
	if err := storage.CheckBadblocks(ctx, &bbArgs); err != nil {
		return errors.Annotate(err, "audit storage bad blocks").Err()
	}
	return nil
}

// hasEnoughStorageSpaceExec confirms the given path has at least the amount of free space specified by the actionArgs arguments.
// provides arguments should be in the formart of:
// ["path:x"]
// x is the number of GB of the disk space.
// input will only consist of one path and its corresponding value for storage.
func hasEnoughStorageSpaceExec(ctx context.Context, info *execs.ExecInfo) error {
	// TODO(gregorynisbet): recheck it and simplify. Also do it for hasEnoughStoragePercentageExec
	if len(info.GetExecArgs()) != 1 {
		return errors.Reason("has enough storage space: input in wrong format").Err()
	}
	inputs := strings.Split(info.GetExecArgs()[0], ":")
	if len(inputs) != 2 {
		return errors.Reason("has enough storage space: input in wrong format").Err()
	}
	path := inputs[0]
	pathMinSpaceInGB, convertErr := strconv.ParseFloat(inputs[1], 64)
	if convertErr != nil {
		return errors.Annotate(convertErr, "has enough storage space: convert stateful path min space").Err()
	}
	if err := linux.PathHasEnoughValue(ctx, info.DefaultRunner(), info.GetActiveResource(), path, linux.SpaceTypeDisk, pathMinSpaceInGB); err != nil {
		return errors.Annotate(err, "has enough storage space").Err()
	}
	return nil
}

// hasEnoughFreeIndexNodesExec confirms the given path has at least the amount of free index nodes specified by the actionArgs arguments.
// provides arguments should be in the formart of:
// ["path:x"]
// x is the number of kilos of index nodes.
// input will only consist of one path and its corresponding value for storage.
func hasEnoughFreeIndexNodesExec(ctx context.Context, info *execs.ExecInfo) error {
	if len(info.GetExecArgs()) != 1 {
		return errors.Reason("has enough index nodes: input in wrong format").Err()
	}
	inputs := strings.Split(info.GetExecArgs()[0], ":")
	if len(inputs) != 2 {
		return errors.Reason("has enough index nodes: input in wrong format").Err()
	}
	path := inputs[0]
	pathMinKiloIndexNodes, convertErr := strconv.ParseFloat(inputs[1], 64)
	if convertErr != nil {
		return errors.Annotate(convertErr, "has enough storage index nodes: convert stateful path min kilo nodes").Err()
	}
	err := linux.PathHasEnoughValue(ctx, info.DefaultRunner(), info.GetActiveResource(), path, linux.SpaceTypeInode, pathMinKiloIndexNodes*1000)
	return errors.Annotate(err, "has enough storage index nodes").Err()
}

// hasEnoughStorageSpacePercentageExec confirms the given path has at least the percentage of free space specified by the actionArgs arguments.
// provides arguments should be in the formart of:
// ["path:x"]
// x is the percentage of the disk space.
// input will only consist of one path and its corresponding percentage for storage.
func hasEnoughStorageSpacePercentageExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	path := argsMap.AsString(ctx, "path", "")
	pathMinSpaceInPercentage := argsMap.AsFloat64(ctx, "expected", -1)
	if path == "" {
		return errors.Reason("has enough storage space percentage: missing path argument").Err()
	}
	if pathMinSpaceInPercentage < 0 || pathMinSpaceInPercentage > 100 {
		return errors.Reason("has enough storage space percentage: invalid value for expected argument %e", pathMinSpaceInPercentage).Err()
	}
	if occupiedSpacePercentage, err := linux.PathOccupiedSpacePercentage(ctx, info.DefaultRunner(), path); err != nil {
		return errors.Annotate(err, "has enough storage space percentage").Err()
	} else if actualFreePercentage := (100 - occupiedSpacePercentage); pathMinSpaceInPercentage > actualFreePercentage {
		return errors.Reason("path have enough free space percentage: %s/%s, expect %v%%, actual %v%%", info.GetActiveResource(), path, pathMinSpaceInPercentage, actualFreePercentage).Err()
	}
	return nil
}

func init() {
	execs.Register("cros_audit_storage_smart", auditStorageSMARTExec)
	execs.Register("cros_audit_storage_bad_blocks", auditStorageBadblocksExec)
	execs.Register("cros_has_enough_storage_space", hasEnoughStorageSpaceExec)
	execs.Register("cros_has_enough_storage_space_percentage", hasEnoughStorageSpacePercentageExec)
	execs.Register("cros_has_enough_index_nodes", hasEnoughFreeIndexNodesExec)
}
