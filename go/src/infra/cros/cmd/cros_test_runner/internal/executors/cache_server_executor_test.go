// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executors

import (
	"context"
	"fmt"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/cmd/common_lib/containers"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/cmd/cros_test_runner/internal/commands"
)

func TestCacheServerExecutor_StartCacheServer(t *testing.T) {
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
		expected := &labapi.IpEndpoint{Address: "4.3.2.1", Port: 8080}
		ctx := context.Background()
		exec := buildCacheServerExecutor()
		cmd := getCmd(exec)
		exec.Container = &mockContainerApi{
			process: func(ctx context.Context, template *api.Template) (string, error) {
				assert.Loosely(t, template, should.NotBeNil)
				return "4.3.2.1:8080", nil
			},
		}

		err := exec.ExecuteCommand(ctx, cmd)

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cmd.CacheServerAddress, should.Resemble(expected))
	})

	ftt.Run("StartCacheServer error", t, func(t *ftt.Test) {
		ctx := context.Background()
		exec := buildCacheServerExecutor()
		exec.Container = &mockContainerApi{
			process: func(ctx context.Context, template *api.Template) (string, error) {
				return "", fmt.Errorf("ctr error")
			},
		}
		cmd := getCmd(exec)

		err := exec.ExecuteCommand(ctx, cmd)

		assert.Loosely(t, err, should.NotBeNil)
	})
}

func buildCacheServerExecutor() *CacheServerExecutor {
	ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
	ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
	cont := containers.NewCacheServerTemplatedContainer("container/image/path", ctr)
	exec := NewCacheServerExecutor(cont)
	return exec
}
