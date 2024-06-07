// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executions

import (
	"context"
	"log"

	"go.chromium.org/luci/common/logging"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/container_uprev/internal"
)

// RevertExecution goes through each container and reverts its sha.
func RevertExecution(containerNames []string, isProd bool) {
	ctx := context.Background()

	logCfg := common.LoggerConfig{Out: log.Default().Writer()}
	ctx = logCfg.Use(ctx)

	tag := common.LabelStaging
	if isProd {
		tag = common.LabelProd
	}

	err := internal.RevertShas(ctx, containerNames, "", tag)
	if err != nil {
		logging.Infof(ctx, "failed to revert some or all SHAs, %s", err)
		return
	}
}
