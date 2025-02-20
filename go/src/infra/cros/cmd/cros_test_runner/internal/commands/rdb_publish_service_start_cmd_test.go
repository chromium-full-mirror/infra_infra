// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands_test

import (
	"context"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/cmd/common_lib/containers"
	"go.chromium.org/infra/cros/cmd/common_lib/tools/crostoolrunner"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/commands"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/executors"
)

func TestRdbPublishStartCmd(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd with no deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosRdbPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := executors.NewCrosPublishExecutor(
			cont,
			executors.CrosRdbPublishExecutorType)
		cmd := commands.NewRdbPublishServiceStartCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cmd.RdbPublishSrcDir, should.BeEmpty)
	})

	ftt.Run("Cmd deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		rdbPublishSrcDir := "rdb_publish_src_dir"
		sk := &data.HwTestStateKeeper{RdbPublishSrcDir: rdbPublishSrcDir}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}

		cont := containers.NewCrosPublishTemplatedContainer(containers.CrosRdbPublishTemplatedContainerType, "container/image/path", ctr)
		exec := executors.NewCrosPublishExecutor(cont, executors.CrosRdbPublishExecutorType)
		cmd := commands.NewRdbPublishServiceStartCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cmd.RdbPublishSrcDir, should.Equal(rdbPublishSrcDir))
	})
}

func TestRdbPublishStartCmd_UpdateSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd with no updates", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{CftTestRequest: nil}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosRdbPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := executors.NewCrosPublishExecutor(cont, executors.CrosRdbPublishExecutorType)
		cmd := commands.NewRdbPublishServiceStartCmd(exec)
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}
