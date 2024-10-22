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

func TestInvServiceStopCmd_NoDeps(t *testing.T) {
	t.Parallel()
	ftt.Run("No deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{}
		exec := executors.NewInvServiceExecutor("")
		cmd := commands.NewInvServiceStopCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestInvServiceStopCmd_NoUpdates(t *testing.T) {
	t.Parallel()
	ftt.Run("No updates", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{}
		exec := executors.NewInvServiceExecutor("")
		cmd := commands.NewInvServiceStopCmd(exec)
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}
