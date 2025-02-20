// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
)

// InvServiceStartCmd represents inventory service start cmd.
type InvServiceStartCmd struct {
	*interfaces.SingleCmdByExecutor
}

func NewInvServiceStartCmd(executor interfaces.ExecutorInterface) *InvServiceStartCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(InvServiceStartCmdType, executor)
	cmd := &InvServiceStartCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
