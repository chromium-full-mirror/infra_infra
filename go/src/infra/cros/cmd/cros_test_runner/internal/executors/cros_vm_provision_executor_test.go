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

	"go.chromium.org/infra/cros/cmd/common_lib/containers"
	"go.chromium.org/infra/cros/cmd/common_lib/tools/crostoolrunner"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/commands"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/mocked_services"
)

func TestVMProvisionServiceStart(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	ftt.Run("VM Provision service start fails without starting ctr", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosVMProvisionTemplatedContainer("container/image/path", ctr)
		exec := NewCrosVMProvisionExecutor(cont)
		err := exec.Start(ctx)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("VM Provision service start fails on failing StartTemplatedContainer", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		mocked_client := mocked_services.NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mocked_client
		getMockedStartTemplatedContainer(mocked_client).Return(nil, fmt.Errorf("some error"))
		cont := containers.NewCrosVMProvisionTemplatedContainer("container/image/path", ctr)
		exec := NewCrosVMProvisionExecutor(cont)
		err := exec.Start(ctx)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestVMProvisionServiceLeaseDutVM(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	ftt.Run("VM Provision service LeaseDutVM fails with nil install request", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosVMProvisionTemplatedContainer("container/image/path", ctr)
		exec := NewCrosVMProvisionExecutor(cont)
		resp, err := exec.LeaseDutVM(ctx, nil)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("VM Provision service LeaseDutVM fails with no established client", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosVMProvisionTemplatedContainer("container/image/path", ctr)
		exec := NewCrosVMProvisionExecutor(cont)
		resp, err := exec.LeaseDutVM(ctx, &testapi.InstallRequest{})
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("VM Provision service LeaseDutVM fails with install error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosVMProvisionTemplatedContainer("container/image/path", ctr)
		exec := NewCrosVMProvisionExecutor(cont)
		mocked_client := mocked_services.NewMockGenericProvisionServiceClient(ctrl)
		exec.CrosVMProvisionServiceClient = mocked_client
		getMockedVMProvisionInstall(mocked_client).Return(nil, fmt.Errorf("some_error"))
		resp, err := exec.LeaseDutVM(ctx, &testapi.InstallRequest{})
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("VM Provision service LeaseDutVM fails with empty lro response", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosVMProvisionTemplatedContainer("container/image/path", ctr)
		exec := NewCrosVMProvisionExecutor(cont)
		mocked_client := mocked_services.NewMockGenericProvisionServiceClient(ctrl)
		exec.CrosVMProvisionServiceClient = mocked_client
		getMockedVMProvisionInstall(mocked_client).Return(nil, nil)
		resp, err := exec.LeaseDutVM(ctx, &testapi.InstallRequest{})
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("VM Provision service LeaseDutVM success", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosVMProvisionTemplatedContainer("container/image/path", ctr)
		exec := NewCrosVMProvisionExecutor(cont)
		mocked_client := mocked_services.NewMockGenericProvisionServiceClient(ctrl)
		exec.CrosVMProvisionServiceClient = mocked_client
		wantResp := &testapi.InstallResponse{}
		wantRespAnypb, _ := anypb.New(wantResp)
		getMockedVMProvisionInstall(mocked_client).Return(&longrunning.Operation{
			Done: true,
			Result: &longrunning.Operation_Response{
				Response: wantRespAnypb,
			},
		},
			nil)
		resp, err := exec.LeaseDutVM(ctx, &testapi.InstallRequest{})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.NotBeNil)
		//So(resp, ShouldEqual, wantResp)
		assert.Loosely(t, proto.Equal(resp, wantResp), should.BeTrue)
	})
}

func TestVMProvisionServiceExecuteCommand(t *testing.T) {
	t.Parallel()

	ftt.Run("VM Provision service unsupported cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosVMProvisionTemplatedContainer("container/image/path", ctr)
		exec := NewCrosVMProvisionExecutor(cont)
		err := exec.ExecuteCommand(ctx, NewUnsupportedCmd())
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("VM Provision service start cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosVMProvisionTemplatedContainer("container/image/path", ctr)
		exec := NewCrosVMProvisionExecutor(cont)
		err := exec.ExecuteCommand(ctx, commands.NewVMProvisionServiceStartCmd(exec))
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("VM Provision service install cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosVMProvisionTemplatedContainer("container/image/path", ctr)
		exec := NewCrosVMProvisionExecutor(cont)
		err := exec.ExecuteCommand(ctx, commands.NewVMProvisionLeaseCmd(exec))
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestGetDiskSizeByBoard(t *testing.T) {
	ftt.Run("Given an image name", t, func(t *ftt.Test) {
		t.Run("When the image contains 'reven-vmtest'", func(t *ftt.Test) {
			image := "reven-vmtest"
			expectedSize := int64(20)

			size := getDiskSizeByBoard(image)

			t.Run("Then it should return the expected size", func(t *ftt.Test) {
				assert.Loosely(t, size, should.Equal(expectedSize))
			})
		})

		t.Run("When the image does not contain 'reven-vmtest'", func(t *ftt.Test) {
			image := "other-image"
			expectedSize := int64(13)

			size := getDiskSizeByBoard(image)

			t.Run("Then it should return the expected size", func(t *ftt.Test) {
				assert.Loosely(t, size, should.Equal(expectedSize))
			})
		})
	})
}

func getMockedVMProvisionInstall(mockClient *mocked_services.MockGenericProvisionServiceClient) *gomock.Call {
	return mockClient.EXPECT().Install(gomock.Any(),
		gomock.AssignableToTypeOf(&testapi.InstallRequest{}),
		gomock.Any())
}
