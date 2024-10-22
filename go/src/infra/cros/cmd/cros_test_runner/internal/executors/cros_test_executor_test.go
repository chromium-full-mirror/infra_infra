// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executors

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/longrunning"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/cmd/common_lib/containers"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/cmd/cros_test_runner/internal/commands"
	"infra/cros/cmd/cros_test_runner/internal/mocked_services"
)

func TestTestServiceStart(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	ftt.Run("Test service start without starting ctr", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosTestTemplatedContainer("container/image/path", ctr)
		exec := NewCrosTestExecutor(cont)
		err := exec.Start(ctx)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Test service start process container fails", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		mocked_client := mocked_services.NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mocked_client
		getMockedStartTemplatedContainer(mocked_client).Return(nil, fmt.Errorf("some error"))
		cont := containers.NewCrosTestTemplatedContainer("container/image/path", ctr)
		exec := NewCrosTestExecutor(cont)
		err := exec.Start(ctx)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestTestServiceExecuteTests(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	ftt.Run("Test service test execution with nil request", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosTestTemplatedContainer("container/image/path", ctr)
		exec := NewCrosTestExecutor(cont)
		resp, err := exec.ExecuteTests(ctx, nil)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("Test service test execution with no established client", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosTestTemplatedContainer("container/image/path", ctr)
		exec := NewCrosTestExecutor(cont)
		resp, err := exec.ExecuteTests(ctx, &testapi.CrosTestRequest{})
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("Test service test execution with run tests error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosTestTemplatedContainer("container/image/path", ctr)
		exec := NewCrosTestExecutor(cont)
		mocked_client := mocked_services.NewMockExecutionServiceClient(ctrl)
		exec.CrosTestServiceClient = mocked_client
		getMockedExecuteTests(mocked_client).Return(nil, fmt.Errorf("some_error"))
		resp, err := exec.ExecuteTests(ctx, &testapi.CrosTestRequest{})
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("Test service test execution with lro process failure", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosTestTemplatedContainer("container/image/path", ctr)
		exec := NewCrosTestExecutor(cont)
		mocked_client := mocked_services.NewMockExecutionServiceClient(ctrl)
		exec.CrosTestServiceClient = mocked_client
		getMockedExecuteTests(mocked_client).Return(nil, nil)
		resp, err := exec.ExecuteTests(ctx, &testapi.CrosTestRequest{})
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("Test service test execution success", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosTestTemplatedContainer("container/image/path", ctr)
		exec := NewCrosTestExecutor(cont)
		mocked_client := mocked_services.NewMockExecutionServiceClient(ctrl)
		exec.CrosTestServiceClient = mocked_client
		wantResp := &testapi.CrosTestResponse{}
		wantRespAnypb, _ := anypb.New(wantResp)
		getMockedExecuteTests(mocked_client).Return(&longrunning.Operation{
			Done: true,
			Result: &longrunning.Operation_Response{
				Response: wantRespAnypb,
			},
		},
			nil)
		resp, err := exec.ExecuteTests(ctx, &testapi.CrosTestRequest{})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.NotBeNil)
		assert.Loosely(t, proto.Equal(resp, wantResp), should.BeTrue)
	})
}

func TestTestServiceExecuteCommand(t *testing.T) {
	t.Parallel()

	ftt.Run("Test service unsupported cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosTestTemplatedContainer("container/image/path", ctr)
		exec := NewCrosTestExecutor(cont)
		err := exec.ExecuteCommand(ctx, NewUnsupportedCmd())
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Test service start cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosTestTemplatedContainer("container/image/path", ctr)
		exec := NewCrosTestExecutor(cont)
		err := exec.ExecuteCommand(ctx, commands.NewTestServiceStartCmd(exec))
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Test service test execution cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosTestTemplatedContainer("container/image/path", ctr)
		exec := NewCrosTestExecutor(cont)
		err := exec.ExecuteCommand(ctx, commands.NewTestsExecutionCmd(exec))
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func getMockedExecuteTests(mockClient *mocked_services.MockExecutionServiceClient) *gomock.Call {
	return mockClient.EXPECT().RunTests(gomock.Any(),
		gomock.AssignableToTypeOf(&testapi.CrosTestRequest{}),
		gomock.Any())
}
