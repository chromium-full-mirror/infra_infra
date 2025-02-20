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

	"go.chromium.org/infra/cros/cmd/common_lib/containers"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/common_lib/tools/crostoolrunner"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/commands"
	vmlabapi "go.chromium.org/infra/libs/vmlab/api"
)

type mockImageApi struct {
	vmlabapi.ImageApi
	getImage func(builderPath string, wait bool) (*vmlabapi.GceImage, error)
}

func (m *mockImageApi) GetImage(builderPath string, wait bool) (*vmlabapi.GceImage, error) {
	return m.getImage(builderPath, wait)
}

func TestCrosDutVmExecutor_GetImage(t *testing.T) {
	t.Parallel()

	getCmd := func(exec interfaces.ExecutorInterface) *commands.DutVmGetImageCmd {
		cmd := commands.NewDutVmGetImageCmd(exec)
		cmd.Build = "betty/R101"
		return cmd
	}

	ftt.Run("GetImage success", t, func(t *ftt.Test) {
		expected := &vmlabapi.GceImage{Name: "image-1", Project: "project-1"}
		ctx := context.Background()
		exec := buildCrosDutVmExecutor()
		exec.ImageApi = &mockImageApi{
			getImage: func(builderPath string, wait bool) (*vmlabapi.GceImage, error) {
				assert.Loosely(t, builderPath, should.Equal("betty/R101"))
				assert.Loosely(t, wait, should.BeTrue)
				return expected, nil
			},
		}
		cmd := getCmd(exec)

		err := exec.ExecuteCommand(ctx, cmd)

		assert.Loosely(t, cmd.DutVmGceImage, should.Resemble(expected))
		assert.Loosely(t, err, should.BeNil)
	})

	ftt.Run("GetImage error", t, func(t *ftt.Test) {
		ctx := context.Background()
		exec := buildCrosDutVmExecutor()
		exec.ImageApi = &mockImageApi{
			getImage: func(builderPath string, wait bool) (*vmlabapi.GceImage, error) {
				return nil, fmt.Errorf("vmlab lib error")
			},
		}
		cmd := getCmd(exec)

		err := exec.ExecuteCommand(ctx, cmd)

		assert.Loosely(t, cmd.DutVmGceImage, should.BeNil)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

type mockInstanceApi struct {
	vmlabapi.InstanceApi
	create func(*vmlabapi.CreateVmInstanceRequest) (*vmlabapi.VmInstance, error)
	delete func(*vmlabapi.VmInstance) error
}

func (m *mockInstanceApi) Create(ctx context.Context, req *vmlabapi.CreateVmInstanceRequest) (*vmlabapi.VmInstance, error) {
	return m.create(req)
}

func (m *mockInstanceApi) Delete(ctx context.Context, ins *vmlabapi.VmInstance) error {
	return m.delete(ins)
}

func TestCrosDutVmExecutor_StartCrosDut(t *testing.T) {
	t.Parallel()

	getCmd := func(exec interfaces.ExecutorInterface) *commands.DutServiceStartCmd {
		cmd := commands.NewDutServiceStartCmd(exec)
		cmd.CacheServerAddress = &labapi.IpEndpoint{Address: "4.3.2.1", Port: 8080}
		cmd.DutSshAddress = &labapi.IpEndpoint{Address: "1.2.3.4", Port: 22}
		return cmd
	}

	ftt.Run("StartCrosDut success", t, func(t *ftt.Test) {
		expected := &labapi.IpEndpoint{Address: "localhost", Port: 44355}
		ctx := context.Background()
		exec := buildCrosDutVmExecutor()
		cmd := getCmd(exec)
		exec.Container = &mockContainerApi{
			process: func(ctx context.Context, template *api.Template) (string, error) {
				assert.Loosely(t, template, should.NotBeNil)
				return "localhost:44355", nil
			},
		}

		err := exec.ExecuteCommand(ctx, cmd)

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cmd.DutServerAddress, should.Resemble(expected))
	})

	ftt.Run("StartCrosDut error", t, func(t *ftt.Test) {
		ctx := context.Background()
		exec := buildCrosDutVmExecutor()
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

type mockContainerApi struct {
	interfaces.ContainerInterface
	process func(context.Context, *api.Template) (string, error)
}

func (m *mockContainerApi) ProcessContainer(ctx context.Context, t *api.Template) (string, error) {
	return m.process(ctx, t)
}

func buildCrosDutVmExecutor() *CrosDutVmExecutor {
	ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
	ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
	cont := containers.NewCrosDutTemplatedContainer("container/image/path", ctr)
	exec := NewCrosDutVmExecutor(cont)
	return exec
}
