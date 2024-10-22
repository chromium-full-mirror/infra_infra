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

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/cros_test_runner/data"
	"infra/cros/cmd/cros_test_runner/internal/commands"
	"infra/cros/cmd/cros_test_runner/internal/executors"
)

func TestLoadDutTopologyCmd_UnsupportedSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Unsupported state keeper", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &UnsupportedStateKeeper{}
		exec := executors.NewInvServiceExecutor("")
		cmd := commands.NewLoadDutTopologyCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestLoadDutTopologyCmd_MissingDeps(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd missing deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{}
		exec := executors.NewInvServiceExecutor("")
		cmd := commands.NewLoadDutTopologyCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestLoadDutTopologyCmd_UpdateSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd with no updates", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{CftTestRequest: nil}
		exec := executors.NewInvServiceExecutor("")
		cmd := commands.NewLoadDutTopologyCmd(exec)
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestLoadDutTopologyCmd_ExtractDepsSuccess(t *testing.T) {
	t.Parallel()

	hostName := "DUT-1234"

	ftt.Run("BuildInputValidationCmd extract deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{HostName: hostName}
		exec := executors.NewInvServiceExecutor("")
		cmd := commands.NewLoadDutTopologyCmd(exec)

		// Extract deps first
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cmd.HostName, should.Equal(hostName))
	})
}

func TestLoadDutTopologyCmd_UpdateSKSuccess(t *testing.T) {
	t.Parallel()
	ftt.Run("BuildInputValidationCmd update SK", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{
			HostName:    "DUT-1234",
			Injectables: common.NewInjectableStorage(),
		}
		exec := executors.NewInvServiceExecutor("")
		cmd := commands.NewLoadDutTopologyCmd(exec)
		cmd.DutTopology = &labapi.DutTopology{}

		// Update SK
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, sk.DutTopology, should.NotBeNil)
	})
}
