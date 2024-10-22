// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands_test

import (
	"context"
	"testing"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/cmd/common_lib/containers"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/cmd/cros_test_runner/data"
	"infra/cros/cmd/cros_test_runner/internal/commands"
	"infra/cros/cmd/cros_test_runner/internal/executors"
)

func TestCacheServerStartCmd_UpdateSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd with no updates", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.LocalTestStateKeeper{Args: &data.LocalArgs{}}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCacheServerTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCacheServerExecutor(cont)
		cmd := commands.NewCacheServerStartCmd(exec)
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestCacheServerStartCmd_ExtractDepsSuccess(t *testing.T) {
	t.Parallel()

	ftt.Run("CacheServerStartCmd extract deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.LocalTestStateKeeper{Args: &data.LocalArgs{}}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCacheServerTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCacheServerExecutor(cont)
		cmd := commands.NewCacheServerStartCmd(exec)

		// Extract deps first
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestCacheServerStartCmd_UpdateSKSuccess(t *testing.T) {
	t.Parallel()
	ftt.Run("CacheServerStartCmd update SK", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.LocalTestStateKeeper{Args: &data.LocalArgs{}}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCacheServerTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCacheServerExecutor(cont)
		cmd := commands.NewCacheServerStartCmd(exec)
		cmd.CacheServerAddress = &labapi.IpEndpoint{}

		// Update SK
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, sk.CacheServerAddress, should.NotBeNil)
	})
}
