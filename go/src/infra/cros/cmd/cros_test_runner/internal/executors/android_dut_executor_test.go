// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executors

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/cmd/common_lib/containers"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/cmd/cros_test_runner/internal/commands"
	"infra/cros/cmd/cros_test_runner/internal/mocked_services"
)

func TestAndroidDutServiceStart(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	ftt.Run("Android Android dut service start with no cache server address", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewAndroidDutTemplatedContainer("container/image/path", ctr)
		exec := NewAndroidDutExecutor(cont)
		err := exec.Start(ctx, nil, nil)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Android dut service start with no dut ssh address", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewAndroidDutTemplatedContainer("container/image/path", ctr)
		exec := NewAndroidDutExecutor(cont)
		err := exec.Start(ctx, &labapi.IpEndpoint{}, nil)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Android dut service start without starting ctr", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewAndroidDutTemplatedContainer("container/image/path", ctr)
		exec := NewAndroidDutExecutor(cont)
		err := exec.Start(ctx, &labapi.IpEndpoint{}, &labapi.IpEndpoint{})
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Android dut service start process container fails", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		mocked_client := mocked_services.NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mocked_client
		getMockedStartTemplatedContainer(mocked_client).Return(nil, fmt.Errorf("some error"))
		cont := containers.NewAndroidDutTemplatedContainer("container/image/path", ctr)
		exec := NewAndroidDutExecutor(cont)
		err := exec.Start(ctx, &labapi.IpEndpoint{}, &labapi.IpEndpoint{})
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Android dut service start process address fails", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		mocked_client := mocked_services.NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mocked_client
		getMockedStartTemplatedContainer(mocked_client).Return(&testapi.StartContainerResponse{}, nil)
		getMockedGetContainer(mocked_client).Return(&testapi.GetContainerResponse{
			Container: &testapi.Container{
				PortBindings: []*testapi.Container_PortBinding{
					{},
				},
			},
		},
			nil)
		cont := containers.NewAndroidDutTemplatedContainer("container/image/path", ctr)
		exec := NewAndroidDutExecutor(cont)
		err := exec.Start(ctx, &labapi.IpEndpoint{}, &labapi.IpEndpoint{})
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestAndroidDutServiceExecuteCommand(t *testing.T) {
	t.Parallel()

	ftt.Run("Android dut service unsupported cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewAndroidDutTemplatedContainer("container/image/path", ctr)
		exec := NewAndroidDutExecutor(cont)
		err := exec.ExecuteCommand(ctx, NewUnsupportedCmd())
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Android dut service start cmd process container execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewAndroidDutTemplatedContainer("container/image/path", ctr)
		exec := NewAndroidDutExecutor(cont)
		err := exec.ExecuteCommand(ctx, commands.NewDutServiceStartCmd(exec))
		assert.Loosely(t, err, should.NotBeNil)
	})
}
