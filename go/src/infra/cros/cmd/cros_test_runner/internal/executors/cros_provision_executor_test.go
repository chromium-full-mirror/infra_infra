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
	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/cmd/common_lib/containers"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/cmd/cros_test_runner/internal/commands"
	"infra/cros/cmd/cros_test_runner/internal/mocked_services"
)

func TestProvisionServiceStart(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	ftt.Run("Provision service start with nil provision request", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosProvisionTemplatedContainer("container/image/path", ctr)
		exec := NewCrosProvisionExecutor(cont)
		err := exec.Start(ctx, nil)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Provision service start without starting ctr", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosProvisionTemplatedContainer("container/image/path", ctr)
		exec := NewCrosProvisionExecutor(cont)
		err := exec.Start(ctx, &api.CrosProvisionRequest{})
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Provision service start process container fails", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		mocked_client := mocked_services.NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mocked_client
		getMockedStartTemplatedContainer(mocked_client).Return(nil, fmt.Errorf("some error"))
		cont := containers.NewCrosProvisionTemplatedContainer("container/image/path", ctr)
		exec := NewCrosProvisionExecutor(cont)
		err := exec.Start(ctx, &api.CrosProvisionRequest{})
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestProvisionServiceInstall(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	ftt.Run("Provision service install with nil install request", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosProvisionTemplatedContainer("container/image/path", ctr)
		exec := NewCrosProvisionExecutor(cont)
		resp, err := exec.Install(ctx, nil)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("Provision service install with no established client", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosProvisionTemplatedContainer("container/image/path", ctr)
		exec := NewCrosProvisionExecutor(cont)
		resp, err := exec.Install(ctx, &testapi.InstallRequest{})
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("Provision service install with install error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosProvisionTemplatedContainer("container/image/path", ctr)
		exec := NewCrosProvisionExecutor(cont)
		mocked_client := mocked_services.NewMockGenericProvisionServiceClient(ctrl)
		exec.CrosProvisionServiceClient = mocked_client
		getMockedProvisionInstall(mocked_client).Return(nil, fmt.Errorf("some_error"))
		resp, err := exec.Install(ctx, &testapi.InstallRequest{})
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("Provision service install with lro process failure", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosProvisionTemplatedContainer("container/image/path", ctr)
		exec := NewCrosProvisionExecutor(cont)
		mocked_client := mocked_services.NewMockGenericProvisionServiceClient(ctrl)
		exec.CrosProvisionServiceClient = mocked_client
		getMockedProvisionInstall(mocked_client).Return(nil, nil)
		resp, err := exec.Install(ctx, &testapi.InstallRequest{})
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("Provision service install success", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosProvisionTemplatedContainer("container/image/path", ctr)
		exec := NewCrosProvisionExecutor(cont)
		mocked_client := mocked_services.NewMockGenericProvisionServiceClient(ctrl)
		exec.CrosProvisionServiceClient = mocked_client
		wantResp := &testapi.InstallResponse{}
		wantRespAnypb, _ := anypb.New(wantResp)
		getMockedProvisionInstall(mocked_client).Return(&longrunning.Operation{
			Done: true,
			Result: &longrunning.Operation_Response{
				Response: wantRespAnypb,
			},
		},
			nil)
		resp, err := exec.Install(ctx, &testapi.InstallRequest{})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.NotBeNil)
		//So(resp, ShouldEqual, wantResp)
		assert.Loosely(t, proto.Equal(resp, wantResp), should.BeTrue)
	})
}

func TestProvisionServiceExecuteCommand(t *testing.T) {
	t.Parallel()

	ftt.Run("Provision service unsupported cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosProvisionTemplatedContainer("container/image/path", ctr)
		exec := NewCrosProvisionExecutor(cont)
		err := exec.ExecuteCommand(ctx, NewUnsupportedCmd())
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Provision service start cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosProvisionTemplatedContainer("container/image/path", ctr)
		exec := NewCrosProvisionExecutor(cont)
		err := exec.ExecuteCommand(ctx, commands.NewProvisionServiceStartCmd(exec))
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Provision service install cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosProvisionTemplatedContainer("container/image/path", ctr)
		exec := NewCrosProvisionExecutor(cont)
		err := exec.ExecuteCommand(ctx, commands.NewProvisionInstallCmd(exec))
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func getMockedProvisionInstall(mockClient *mocked_services.MockGenericProvisionServiceClient) *gomock.Call {
	return mockClient.EXPECT().Install(gomock.Any(),
		gomock.AssignableToTypeOf(&testapi.InstallRequest{}),
		gomock.Any())
}
