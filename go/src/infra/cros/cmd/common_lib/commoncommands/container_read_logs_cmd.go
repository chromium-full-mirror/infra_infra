// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commoncommands

import (
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
)

// ContainerReadLogsCmd represents container close logs command.
type ContainerReadLogsCmd struct {
	*interfaces.SingleCmdByExecutor
}

func NewContainerReadLogsCmd(executor interfaces.ExecutorInterface) *ContainerReadLogsCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(ContainerReadLogsCmdType, executor)
	cmd := &ContainerReadLogsCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
