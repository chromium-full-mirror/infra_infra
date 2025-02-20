// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build linux
// +build linux

package executors

import (
	"context"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/commands"
)

// This test triggers `hostname -I` that is only available on Linux, not macOS.
func TestCacheServerExecutor_StartCacheServerLocalhost(t *testing.T) {
	t.Parallel()

	getCmd := func(exec interfaces.ExecutorInterface) *commands.DutVmCacheServerStartCmd {
		cmd := commands.NewDutVmCacheServerStartCmd(exec)
		duts := []*labapi.Dut{{
			Id: &labapi.Dut_Id{Value: "VM"},
			DutType: &labapi.Dut_Chromeos{
				Chromeos: &labapi.Dut_ChromeOS{},
			}}}
		cmd.DutTopology = &labapi.DutTopology{
			Duts: duts,
		}
		return cmd
	}

	ftt.Run("StartCacheServer success", t, func(t *ftt.Test) {
		ctx := context.Background()
		exec := buildCacheServerExecutor()
		cmd := getCmd(exec)
		exec.Container = &mockContainerApi{
			process: func(ctx context.Context, template *api.Template) (string, error) {
				return "localhost:8080", nil
			},
		}

		err := exec.ExecuteCommand(ctx, cmd)

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cmd.CacheServerAddress.Address, should.NotEqual("localhost"))
	})
}
