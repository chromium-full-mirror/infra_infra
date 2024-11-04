// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package executions contains the various executions for the uprev service.
package executions

import (
	"context"
	"log"

	"go.chromium.org/luci/common/errors"

	"infra/cros/cmd/common_lib/common"
)

// ManualExecution represents manual executions.
func ManualExecution(name, firestoreDatabaseName string, containerItem *common.ContainerInfoItem, isProd bool) {
	ctx := context.Background()

	logCfg := common.LoggerConfig{Out: log.Default().Writer()}
	ctx = logCfg.Use(ctx)

	tag := common.LabelStaging
	if isProd {
		tag = common.LabelProd
	}

	containerInfos := map[string]*common.ContainerInfoItem{
		name: containerItem,
	}

	if shaErr := UpdateShaStorage(ctx, firestoreDatabaseName, containerInfos, "", tag); shaErr != nil {
		shaErr = errors.Annotate(shaErr, "failed to update SHAs").Err()
		return
	}
}
