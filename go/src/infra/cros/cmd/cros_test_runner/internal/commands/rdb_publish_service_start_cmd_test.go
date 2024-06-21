// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands_test

import (
	"context"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"infra/cros/cmd/common_lib/containers"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/cmd/cros_test_runner/data"
	"infra/cros/cmd/cros_test_runner/internal/commands"
	"infra/cros/cmd/cros_test_runner/internal/executors"
)

func TestRdbPublishStartCmd(t *testing.T) {
	t.Parallel()
	Convey("Cmd with no deps", t, func() {
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
		So(err, ShouldBeNil)
		So(cmd.RdbPublishSrcDir, ShouldBeEmpty)
	})

	Convey("Cmd deps", t, func() {
		ctx := context.Background()
		rdbPublishSrcDir := "rdb_publish_src_dir"
		sk := &data.HwTestStateKeeper{RdbPublishSrcDir: rdbPublishSrcDir}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}

		cont := containers.NewCrosPublishTemplatedContainer(containers.CrosRdbPublishTemplatedContainerType, "container/image/path", ctr)
		exec := executors.NewCrosPublishExecutor(cont, executors.CrosRdbPublishExecutorType)
		cmd := commands.NewRdbPublishServiceStartCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)

		So(err, ShouldBeNil)
		So(cmd.RdbPublishSrcDir, ShouldEqual, rdbPublishSrcDir)
	})
}

func TestRdbPublishStartCmd_UpdateSK(t *testing.T) {
	t.Parallel()
	Convey("Cmd with no updates", t, func() {
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
		So(err, ShouldBeNil)
	})
}
