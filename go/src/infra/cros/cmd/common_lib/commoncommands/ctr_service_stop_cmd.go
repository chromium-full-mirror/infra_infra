// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commoncommands

import (
	"infra/cros/cmd/common_lib/interfaces"
)

// CtrServiceStopCmd represents ctr service stop command.
type CtrServiceStopCmd struct {
	*interfaces.SingleCmdByExecutor
}

func NewCtrServiceStopCmd(executor interfaces.ExecutorInterface) *CtrServiceStopCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(CtrServiceStopCmdType, executor)
	cmd := &CtrServiceStopCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
