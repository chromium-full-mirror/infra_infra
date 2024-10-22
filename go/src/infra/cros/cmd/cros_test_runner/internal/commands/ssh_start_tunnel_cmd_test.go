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
	"infra/cros/cmd/cros_test_runner/data"
	"infra/cros/cmd/cros_test_runner/internal/commands"
	"infra/cros/cmd/cros_test_runner/internal/executors"
)

func TestSshStartTunnelCmd_UnsupportedSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Unsupported state keeper", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &UnsupportedStateKeeper{}
		exec := executors.NewSshTunnelExecutor()
		cmd := commands.NewSshStartTunnelCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestSshStartTunnelCmd_UpdateSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd with no updates", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.LocalTestStateKeeper{}
		exec := executors.NewSshTunnelExecutor()
		cmd := commands.NewSshStartTunnelCmd(exec)
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, sk.DutSshAddress, should.BeNil)
	})
}

func TestSshStartTunnelCmd_ExtractDepsSuccess(t *testing.T) {
	t.Parallel()

	hostname := "DUT1234"

	ftt.Run("SshStartTunnelCmd extract deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.LocalTestStateKeeper{HwTestStateKeeper: data.HwTestStateKeeper{HostName: hostname}}
		exec := executors.NewSshTunnelExecutor()
		cmd := commands.NewSshStartTunnelCmd(exec)

		// Extract deps first
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cmd.HostName, should.Equal(hostname))
	})
}

func TestSshStartTunnelCmd_UpdateSKSuccess(t *testing.T) {
	t.Parallel()

	hostname := "DUT1234"

	ftt.Run("SshStartTunnelCmd update SK", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.LocalTestStateKeeper{HwTestStateKeeper: data.HwTestStateKeeper{HostName: hostname}}
		exec := executors.NewSshTunnelExecutor()
		cmd := commands.NewSshStartTunnelCmd(exec)
		cmd.SshTunnelPort = 1234

		// Update SK
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, sk.DutSshAddress, should.NotBeNil)
	})
}
