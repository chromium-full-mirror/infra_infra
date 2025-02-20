// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executions

import (
	"context"
	"log"

	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/container_uprev/internal"
)

// RevertExecution goes through each container and reverts its sha.
func RevertExecution(configNames []string, isProd bool, firestoreDatabaseName string) {
	ctx := context.Background()

	logCfg := common.LoggerConfig{Out: log.Default().Writer()}
	ctx = logCfg.Use(ctx)

	tag := common.LabelStaging
	if isProd {
		tag = common.LabelProd
	}

	if len(configNames) == 1 && configNames[0] == "all" {
		configNames = []string{}
		configs := internal.GetConfigs()
		for _, config := range configs {
			configNames = append(configNames, config.Name)
		}
	}

	err := internal.RevertShas(ctx, configNames, firestoreDatabaseName, "", tag)
	if err != nil {
		logging.Infof(ctx, "failed to revert some or all SHAs, %s", err)
		return
	}
}
