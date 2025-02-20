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

	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/commands"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/executors"
)

func TestBuildDutTopologyCmd_UnsupportedSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Unsupported state keeper", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &UnsupportedStateKeeper{}
		exec := executors.NewInvServiceExecutor("")
		cmd := commands.NewBuildDutTopologyCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestBuildDutTopologyCmd_UpdateSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd with no updates", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.LocalTestStateKeeper{}
		exec := executors.NewInvServiceExecutor("")
		cmd := commands.NewBuildDutTopologyCmd(exec)
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cmd.DutTopology, should.BeNil)
	})
}

func TestBuildDutTopologyCmd_ExtractDepsSuccess(t *testing.T) {
	t.Parallel()

	board := "kevin"
	dutSshAddress := &labapi.IpEndpoint{Address: "dutssh", Port: 1234}
	cacheServerAddress := &labapi.IpEndpoint{Address: "cacheserver", Port: 4321}

	ftt.Run("BuildInputValidationCmd extract deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.LocalTestStateKeeper{Args: &data.LocalArgs{BuildBoard: board}, DutSshAddress: dutSshAddress, DutCacheServerAddress: cacheServerAddress}
		exec := executors.NewInvServiceExecutor("")
		cmd := commands.NewBuildDutTopologyCmd(exec)

		// Extract deps first
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cmd.Board, should.Equal(board))
		assert.Loosely(t, cmd.DutSshAddress, should.Equal(dutSshAddress))
		assert.Loosely(t, cmd.CacheServerAddress, should.Equal(cacheServerAddress))
	})
}

func TestBuildDutTopologyCmd_UpdateSKSuccess(t *testing.T) {
	t.Parallel()

	board := "kevin"
	dutSshAddress := &labapi.IpEndpoint{Address: "dutssh", Port: 1234}
	cacheServerAddress := &labapi.IpEndpoint{Address: "cacheserver", Port: 4321}

	ftt.Run("BuildInputValidationCmd update SK", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.LocalTestStateKeeper{Args: &data.LocalArgs{BuildBoard: board}, DutSshAddress: dutSshAddress, DutCacheServerAddress: cacheServerAddress}
		exec := executors.NewInvServiceExecutor("")
		cmd := commands.NewBuildDutTopologyCmd(exec)
		cmd.DutTopology = &labapi.DutTopology{}

		// Update SK
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, sk.DutTopology, should.NotBeNil)
	})
}
