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

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/cros_test_runner/data"
	"infra/cros/cmd/cros_test_runner/internal/commands"
)

func TestParseEnvInfoCmd_UnsupportedSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Unsupported state keeper", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &UnsupportedStateKeeper{}
		cmd := commands.NewParseEnvInfoCmd()
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestParseEnvInfoCmd_NoDeps(t *testing.T) {
	t.Parallel()
	ftt.Run("No deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{}
		cmd := commands.NewParseEnvInfoCmd()
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestParseEnvInfoCmd_UpdateSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd with updates", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{}
		cmd := commands.NewParseEnvInfoCmd()
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestParseEnvInfoCmd_Execute(t *testing.T) {
	hostName := "DUT-1234"
	ftt.Run("ParseEnvInfoCmd execute", t, func(t *ftt.Test) {
		// Set proper env
		t.Setenv("SWARMING_BOT_ID", hostName)
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{}
		sk.Injectables = common.NewInjectableStorage()
		cmd := commands.NewParseEnvInfoCmd()

		// Extract deps first
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)

		// Execute cmd
		err = cmd.Execute(ctx)
		assert.Loosely(t, err, should.BeNil)

		// Update SK
		err = cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)

		// Check if SK data updated
		assert.Loosely(t, sk.HostName, should.Equal(hostName))
	})
}
