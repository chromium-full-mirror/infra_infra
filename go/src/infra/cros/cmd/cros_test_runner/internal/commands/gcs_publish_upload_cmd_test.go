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
	"infra/cros/cmd/common_lib/containers"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/cmd/cros_test_runner/data"
	"infra/cros/cmd/cros_test_runner/internal/commands"
	"infra/cros/cmd/cros_test_runner/internal/executors"
)

func TestGcsPublishPublishCmd_UnsupportedSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Unsupported state keeper", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &UnsupportedStateKeeper{}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := executors.NewCrosPublishExecutor(
			cont,
			executors.CrosGcsPublishExecutorType)
		cmd := commands.NewGcsPublishUploadCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestGcsPublishPublishCmd_MissingDeps(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd missing deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := executors.NewCrosPublishExecutor(
			cont,
			executors.CrosGcsPublishExecutorType)
		cmd := commands.NewGcsPublishUploadCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestGcsPublishPublishCmd_UpdateSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd with no updates", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{CftTestRequest: nil}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := executors.NewCrosPublishExecutor(
			cont,
			executors.CrosGcsPublishExecutorType)
		cmd := commands.NewGcsPublishUploadCmd(exec)
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestGcsPublishPublishCmd_ExtractDepsSuccess(t *testing.T) {
	t.Parallel()

	ftt.Run("ProvisionStartCmd extract deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantGcsURL := "gs://gcs/url"
		sk := &data.HwTestStateKeeper{GcsURL: wantGcsURL}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := executors.NewCrosPublishExecutor(
			cont,
			executors.CrosGcsPublishExecutorType)
		cmd := commands.NewGcsPublishUploadCmd(exec)

		// Extract deps first
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cmd.GcsURL, should.Equal(wantGcsURL))
	})
}
