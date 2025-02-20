// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commoncommands

import (
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
)

// CtrServiceStartAsyncCmd represents ctr service start async command.
type CtrServiceStartAsyncCmd struct {
	*interfaces.SingleCmdByExecutor
}

func NewCtrServiceStartAsyncCmd(executor interfaces.ExecutorInterface) *CtrServiceStartAsyncCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(CtrServiceStartAsyncCmdType, executor)
	cmd := &CtrServiceStartAsyncCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
