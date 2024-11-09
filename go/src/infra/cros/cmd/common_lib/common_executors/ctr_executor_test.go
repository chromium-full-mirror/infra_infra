// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common_executors_test

import (
	"context"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/cmd/common_lib/common_commands"
	"infra/cros/cmd/common_lib/common_executors"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
)

type UnsupportedCmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor
}

func NewUnsupportedCmd() interfaces.CommandInterface {
	absCmd := interfaces.NewAbstractCmd(common_commands.UnSupportedCmdType)
	absSingleCmdByNoExec := &interfaces.AbstractSingleCmdByNoExecutor{AbstractCmd: absCmd}
	return &UnsupportedCmd{AbstractSingleCmdByNoExecutor: absSingleCmdByNoExec}
}

func TestCtrServiceStartAsync(t *testing.T) {
	t.Parallel()

	ftt.Run("ctr initialization error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		exec := common_executors.NewCtrExecutor(ctr)
		err := exec.StartAsync(ctx)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestGcloudAuth(t *testing.T) {
	t.Parallel()

	ftt.Run("gcloud auth error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		exec := common_executors.NewCtrExecutor(ctr)
		err := exec.GcloudAuth(ctx, "", false)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestCtrStop(t *testing.T) {
	t.Parallel()

	ftt.Run("gcloud auth error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		exec := common_executors.NewCtrExecutor(ctr)
		err := exec.Stop(ctx)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestCtrExecuteCommand(t *testing.T) {
	t.Parallel()

	ftt.Run("unsupported cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		exec := common_executors.NewCtrExecutor(ctr)
		err := exec.ExecuteCommand(ctx, NewUnsupportedCmd())
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("start async cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		exec := common_executors.NewCtrExecutor(ctr)
		cmd := common_commands.NewCtrServiceStartAsyncCmd(exec)
		err := exec.ExecuteCommand(ctx, cmd)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("stop cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		exec := common_executors.NewCtrExecutor(ctr)
		cmd := common_commands.NewCtrServiceStopCmd(exec)
		err := exec.ExecuteCommand(ctx, cmd)
		assert.Loosely(t, err, should.BeNil)
	})

	ftt.Run("gcloud auth cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		exec := common_executors.NewCtrExecutor(ctr)
		cmd := common_commands.NewGcloudAuthCmd(exec)
		err := exec.ExecuteCommand(ctx, cmd)
		assert.Loosely(t, err, should.NotBeNil)
	})
}
