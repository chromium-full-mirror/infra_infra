// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands_test

import (
	"context"
	"testing"

	"go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/commands"
)

func TestUpdateContainerImagesLocallyCmd_UnsupportedSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Unsupported state keeper", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &UnsupportedStateKeeper{}
		cmd := commands.NewUpdateContainerImagesLocallyCmd()
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestUpdateContainerImagesLocallyCmd_NoDeps(t *testing.T) {
	t.Parallel()
	ftt.Run("No deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.PreLocalTestStateKeeper{Args: &data.LocalArgs{}}
		cmd := commands.NewUpdateContainerImagesLocallyCmd()
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestUpdateContainerImagesLocallyCmd_UpdateSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd with updates", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.PreLocalTestStateKeeper{}
		cmd := commands.NewUpdateContainerImagesLocallyCmd()
		cmd.Containers = map[string]*api.ContainerImageInfo{
			"test": {
				Name: "testcontainer",
			},
		}
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, sk.ContainerImages, should.Match(cmd.Containers))
	})
}
