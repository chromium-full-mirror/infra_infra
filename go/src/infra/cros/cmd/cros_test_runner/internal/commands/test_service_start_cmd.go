// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
)

// TestServiceStartCmd represents test service start cmd.
type TestServiceStartCmd struct {
	*interfaces.SingleCmdByExecutor
}

func NewTestServiceStartCmd(executor interfaces.ExecutorInterface) *TestServiceStartCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(TestServiceStartCmdType, executor)
	cmd := &TestServiceStartCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
