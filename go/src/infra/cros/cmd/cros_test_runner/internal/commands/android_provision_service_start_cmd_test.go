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

	"go.chromium.org/infra/cros/cmd/common_lib/containers"
	"go.chromium.org/infra/cros/cmd/common_lib/tools/crostoolrunner"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/commands"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/executors"
)

func TestAndroidProvisionServiceStartCmd_NoDeps(t *testing.T) {
	t.Parallel()
	ftt.Run("No deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewGenericProvisionTemplatedContainer("android-provision", "container/image/path", ctr)
		exec := executors.NewAndroidProvisionExecutor(cont)
		cmd := commands.NewAndroidProvisionServiceStartCmd(exec)
		sk := &data.HwTestStateKeeper{}
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestAndroidProvisionServiceStartCmd_NoUpdates(t *testing.T) {
	t.Parallel()
	ftt.Run("No updates", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewGenericProvisionTemplatedContainer("android-provision", "container/image/path", ctr)
		exec := executors.NewAndroidProvisionExecutor(cont)
		cmd := commands.NewAndroidProvisionServiceStartCmd(exec)
		sk := &data.HwTestStateKeeper{}
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}
