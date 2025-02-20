// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commonexecutors_test

import (
	"context"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/cmd/common_lib/commoncommands"
	"go.chromium.org/infra/cros/cmd/common_lib/commonexecutors"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/common_lib/tools/crostoolrunner"
)

type UnsupportedCmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor
}

func NewUnsupportedCmd() interfaces.CommandInterface {
	absCmd := interfaces.NewAbstractCmd(commoncommands.UnSupportedCmdType)
	absSingleCmdByNoExec := &interfaces.AbstractSingleCmdByNoExecutor{AbstractCmd: absCmd}
	return &UnsupportedCmd{AbstractSingleCmdByNoExecutor: absSingleCmdByNoExec}
}

func TestCtrServiceStartAsync(t *testing.T) {
	t.Parallel()

	ftt.Run("ctr initialization error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		exec := commonexecutors.NewCtrExecutor(ctr)
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
		exec := commonexecutors.NewCtrExecutor(ctr)
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
		exec := commonexecutors.NewCtrExecutor(ctr)
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
		exec := commonexecutors.NewCtrExecutor(ctr)
		err := exec.ExecuteCommand(ctx, NewUnsupportedCmd())
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("start async cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		exec := commonexecutors.NewCtrExecutor(ctr)
		cmd := commoncommands.NewCtrServiceStartAsyncCmd(exec)
		err := exec.ExecuteCommand(ctx, cmd)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("stop cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		exec := commonexecutors.NewCtrExecutor(ctr)
		cmd := commoncommands.NewCtrServiceStopCmd(exec)
		err := exec.ExecuteCommand(ctx, cmd)
		assert.Loosely(t, err, should.BeNil)
	})

	ftt.Run("gcloud auth cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		exec := commonexecutors.NewCtrExecutor(ctr)
		cmd := commoncommands.NewGcloudAuthCmd(exec)
		err := exec.ExecuteCommand(ctx, cmd)
		assert.Loosely(t, err, should.NotBeNil)
	})
}
