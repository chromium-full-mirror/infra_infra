// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executors

import (
	"context"
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"infra/cros/cmd/common_lib/containers"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/cmd/cros_test_runner/internal/commands"
	vmlabapi "infra/libs/vmlab/api"
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

	Convey("GetImage success", t, func() {
		expected := &vmlabapi.GceImage{Name: "image-1", Project: "project-1"}
		ctx := context.Background()
		exec := buildCrosDutVmExecutor()
		exec.ImageApi = &mockImageApi{
			getImage: func(builderPath string, wait bool) (*vmlabapi.GceImage, error) {
				So(builderPath, ShouldEqual, "betty/R101")
				So(wait, ShouldBeTrue)
				return expected, nil
			},
		}
		cmd := getCmd(exec)

		err := exec.ExecuteCommand(ctx, cmd)

		So(cmd.DutVmGceImage, ShouldResemble, expected)
		So(err, ShouldBeNil)
	})

	Convey("GetImage error", t, func() {
		ctx := context.Background()
		exec := buildCrosDutVmExecutor()
		exec.ImageApi = &mockImageApi{
			getImage: func(builderPath string, wait bool) (*vmlabapi.GceImage, error) {
				return nil, fmt.Errorf("vmlab lib error")
			},
		}
		cmd := getCmd(exec)

		err := exec.ExecuteCommand(ctx, cmd)

		So(cmd.DutVmGceImage, ShouldBeNil)
		So(err, ShouldNotBeNil)
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

	Convey("StartCrosDut success", t, func() {
		expected := &labapi.IpEndpoint{Address: "localhost", Port: 44355}
		ctx := context.Background()
		exec := buildCrosDutVmExecutor()
		cmd := getCmd(exec)
		exec.Container = &mockContainerApi{
			process: func(ctx context.Context, template *api.Template) (string, error) {
				So(template, ShouldNotBeNil)
				return "localhost:44355", nil
			},
		}

		err := exec.ExecuteCommand(ctx, cmd)

		So(err, ShouldBeNil)
		So(cmd.DutServerAddress, ShouldResemble, expected)
	})

	Convey("StartCrosDut error", t, func() {
		ctx := context.Background()
		exec := buildCrosDutVmExecutor()
		exec.Container = &mockContainerApi{
			process: func(ctx context.Context, template *api.Template) (string, error) {
				return "", fmt.Errorf("ctr error")
			},
		}
		cmd := getCmd(exec)

		err := exec.ExecuteCommand(ctx, cmd)

		So(err, ShouldNotBeNil)
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
