// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
)

// AndroidProvisionServiceStartCmd represents android-provision service start cmd.
type AndroidProvisionServiceStartCmd struct {
	*interfaces.SingleCmdByExecutor
}

// NewAndroidProvisionServiceStartCmd returns an object of AndroidProvisionServiceStartCmd
func NewAndroidProvisionServiceStartCmd(executor interfaces.ExecutorInterface) *AndroidProvisionServiceStartCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(AndroidProvisionServiceStartCmdType, executor)
	cmd := &AndroidProvisionServiceStartCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
