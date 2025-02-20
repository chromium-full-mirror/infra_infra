// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
)

// TestFinderServiceStartCmd represents test service start cmd.
type TestFinderServiceStartCmd struct {
	*interfaces.SingleCmdByExecutor
}

func NewTestFinderServiceStartCmd(executor interfaces.ExecutorInterface) *TestFinderServiceStartCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(TestFinderServiceStartCmdType, executor)
	cmd := &TestFinderServiceStartCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
