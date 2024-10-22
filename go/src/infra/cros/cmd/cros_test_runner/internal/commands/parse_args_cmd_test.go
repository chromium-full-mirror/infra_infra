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
)

func TestParseArgsCmd_UnsupportedSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Unsupported state keeper", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &UnsupportedStateKeeper{}
		cmd := commands.NewParseArgsCmd()
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestParseArgsCmd_NoDeps(t *testing.T) {
	t.Parallel()
	ftt.Run("No deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.PreLocalTestStateKeeper{}
		cmd := commands.NewParseArgsCmd()
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestParseArgsCmd_UpdateSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd with updates", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.PreLocalTestStateKeeper{}
		cmd := commands.NewParseArgsCmd()
		cmd.Tests = []string{"test"}
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, sk.Tests, should.Resemble(cmd.Tests))
	})
}

func TestParseArgsCmd_Execute(t *testing.T) {
	ftt.Run("ParseArgsCmd execute", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.PreLocalTestStateKeeper{Args: &data.LocalArgs{Tests: "test1,test2"}}
		cmd := commands.NewParseArgsCmd()

		// Extract deps first
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)

		// Execute cmd
		err = cmd.Execute(ctx)
		assert.Loosely(t, err, should.BeNil)

		// Update SK
		err = cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)

		// Check if SK data is expected
		assert.Loosely(t, sk.Tests, should.Resemble([]string{"test1", "test2"}))
	})
}
