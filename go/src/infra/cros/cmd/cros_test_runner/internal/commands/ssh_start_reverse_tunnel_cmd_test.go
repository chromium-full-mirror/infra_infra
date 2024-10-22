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

	"infra/cros/cmd/cros_test_runner/data"
	"infra/cros/cmd/cros_test_runner/internal/commands"
	"infra/cros/cmd/cros_test_runner/internal/executors"
)

func TestSshStartReverseTunnelCmd_UnsupportedSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Unsupported state keeper", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &UnsupportedStateKeeper{}
		exec := executors.NewSshTunnelExecutor()
		cmd := commands.NewSshStartReverseTunnelCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestSshStartReverseTunnelCmd_UpdateSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd with no updates", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.LocalTestStateKeeper{}
		exec := executors.NewSshTunnelExecutor()
		cmd := commands.NewSshStartReverseTunnelCmd(exec)
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, sk.DutCacheServerAddress, should.BeNil)
	})
}

func TestSshStartReverseTunnelCmd_ExtractDepsSuccess(t *testing.T) {
	t.Parallel()

	hostname := "DUT1234"
	cacheServerAddress := &labapi.IpEndpoint{Address: "cacheserver", Port: 4321}

	ftt.Run("SshStartReverseTunnelCmd extract deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.LocalTestStateKeeper{HwTestStateKeeper: data.HwTestStateKeeper{HostName: hostname}, CacheServerAddress: cacheServerAddress}
		exec := executors.NewSshTunnelExecutor()
		cmd := commands.NewSshStartReverseTunnelCmd(exec)

		// Extract deps first
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cmd.HostName, should.Equal(hostname))
		assert.Loosely(t, cmd.CacheServerPort, should.Equal(cacheServerAddress.Port))
	})
}

func TestSshStartReverseTunnelCmd_UpdateSKSuccess(t *testing.T) {
	t.Parallel()

	hostname := "DUT1234"
	cacheServerAddress := &labapi.IpEndpoint{Address: "cacheserver", Port: 4321}

	ftt.Run("SshStartReverseTunnelCmd update SK", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.LocalTestStateKeeper{HwTestStateKeeper: data.HwTestStateKeeper{HostName: hostname}, CacheServerAddress: cacheServerAddress}
		exec := executors.NewSshTunnelExecutor()
		cmd := commands.NewSshStartReverseTunnelCmd(exec)
		cmd.SshReverseTunnelPort = 1234

		// Update SK
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, sk.DutCacheServerAddress, should.NotBeNil)
	})
}
