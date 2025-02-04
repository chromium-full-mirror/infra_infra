// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commoncommands

import (
	"infra/cros/cmd/common_lib/interfaces"
)

// ContainerCloseLogsCmd represents container close logs command.
type ContainerCloseLogsCmd struct {
	*interfaces.SingleCmdByExecutor
}

func NewContainerCloseLogsCmd(executor interfaces.ExecutorInterface) *ContainerCloseLogsCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(ContainerCloseLogsCmdType, executor)
	cmd := &ContainerCloseLogsCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
