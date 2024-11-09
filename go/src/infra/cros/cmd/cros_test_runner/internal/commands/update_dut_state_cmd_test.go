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
	"infra/cros/dutstate"
)

func TestUpdateDutStateCmd_UnsupportedSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Unsupported state keeper", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &UnsupportedStateKeeper{}
		cmd := commands.NewUpdateDutStateCmd()
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestUpdateDutStateCmd_UpdateSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Update SK", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{CurrentDutState: dutstate.Ready}
		cmd := commands.NewUpdateDutStateCmd()
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cmd.CurrentDutState, should.Equal(""))
	})
}

func TestUpdateDutStateCmd_Execute(t *testing.T) {
	t.Parallel()
	ftt.Run("TestUpdateDutState execute", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{
			HostName: "host",
		}
		cmd := commands.NewUpdateDutStateCmd()

		// Extract deps first
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)

		// Execute cmd
		err = cmd.Execute(ctx)
		assert.Loosely(t, err, should.BeNil)

		// Update SK
		err = cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})

}
