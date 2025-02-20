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

func TestPublishServiceStart(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	ftt.Run("Publish service with invalid type", t, func(t *ftt.Test) {
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := NewCrosPublishExecutor(cont, NoExecutorType)
		assert.Loosely(t, exec, should.BeNil)
	})

	ftt.Run("Publish service start with no template", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := NewCrosPublishExecutor(cont, CrosGcsPublishExecutorType)
		resp, err := exec.Start(ctx, nil)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("Publish service start process container fails", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		mocked_client := mocked_services.NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mocked_client
		getMockedStartTemplatedContainer(mocked_client).Return(nil, fmt.Errorf("some error"))
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := NewCrosPublishExecutor(cont, CrosGcsPublishExecutorType)
		gcsPublishTemplate := &testapi.CrosPublishTemplate{
			PublishType:   testapi.CrosPublishTemplate_PUBLISH_GCS,
			PublishSrcDir: "publish/src/dir"}
		contTemplate := &testapi.Template{
			Container: &testapi.Template_CrosPublish{
				CrosPublish: gcsPublishTemplate,
			},
		}
		resp, err := exec.Start(ctx, contTemplate)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})
}

func TestPublishServicePublishResults(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	ftt.Run("Publish service publish results with no client", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := NewCrosPublishExecutor(cont, CrosGcsPublishExecutorType)
		resp, err := exec.Publish(ctx, nil, nil)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("Publish service publish results with empty request", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := NewCrosPublishExecutor(cont, CrosGcsPublishExecutorType)
		mocked_client := mocked_services.NewMockGenericPublishServiceClient(ctrl)
		resp, err := exec.Publish(ctx, nil, mocked_client)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("Publish service publish results with publish results error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := NewCrosPublishExecutor(cont, CrosGcsPublishExecutorType)
		mocked_client := mocked_services.NewMockGenericPublishServiceClient(ctrl)
		getMockedPublishResults(mocked_client).Return(nil, fmt.Errorf("some_error"))
		resp, err := exec.Publish(ctx, &testapi.PublishRequest{}, mocked_client)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("Publish service publish results with lro process failure", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := NewCrosPublishExecutor(cont, CrosGcsPublishExecutorType)
		mocked_client := mocked_services.NewMockGenericPublishServiceClient(ctrl)
		getMockedPublishResults(mocked_client).Return(nil, nil)
		resp, err := exec.Publish(ctx, &testapi.PublishRequest{}, mocked_client)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("Publish service publish results success", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := NewCrosPublishExecutor(cont, CrosGcsPublishExecutorType)
		mocked_client := mocked_services.NewMockGenericPublishServiceClient(ctrl)
		wantResp := &testapi.PublishResponse{}
		wantRespAnypb, _ := anypb.New(wantResp)
		getMockedPublishResults(mocked_client).Return(&longrunning.Operation{
			Done:   true,
			Result: &longrunning.Operation_Response{Response: wantRespAnypb}},
			nil)
		resp, err := exec.Publish(ctx, &testapi.PublishRequest{}, mocked_client)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.NotBeNil)
		assert.Loosely(t, proto.Equal(resp, wantResp), should.BeTrue)
	})
}

func TestInvokePublishWithAsyncLogging(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	ftt.Run("Publish service invoke publish with async logging error with empty request", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := NewCrosPublishExecutor(cont, CrosGcsPublishExecutorType)
		err := exec.InvokePublishWithAsyncLogging(ctx, "testing-publish", nil, nil, nil)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Publish service invoke publish with async logging error with empty client", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := NewCrosPublishExecutor(cont, CrosGcsPublishExecutorType)
		err := exec.InvokePublishWithAsyncLogging(
			ctx,
			"testing-publish",
			&testapi.PublishRequest{},
			nil,
			nil)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Publish service invoke publish with async logging error with empty container", t, func(t *ftt.Test) {
		ctx := context.Background()
		exec := NewCrosPublishExecutor(nil, CrosGcsPublishExecutorType)
		err := exec.InvokePublishWithAsyncLogging(
			ctx,
			"testing-publish",
			&testapi.PublishRequest{},
			mocked_services.NewMockGenericPublishServiceClient(ctrl),
			nil)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Publish service invoke publish with async logging error with no logs location", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := NewCrosPublishExecutor(cont, CrosGcsPublishExecutorType)
		err := exec.InvokePublishWithAsyncLogging(
			ctx,
			"testing-publish",
			&testapi.PublishRequest{},
			mocked_services.NewMockGenericPublishServiceClient(ctrl),
			nil)
		assert.Loosely(t, err, should.NotBeNil)
	})

}
func TestPublishServiceExecuteCommand(t *testing.T) {
	t.Parallel()

	ftt.Run("Publish service unsupported cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := NewCrosPublishExecutor(cont, CrosGcsPublishExecutorType)
		err := exec.ExecuteCommand(ctx, NewUnsupportedCmd())
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Publish service gcs start cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := NewCrosPublishExecutor(cont, CrosGcsPublishExecutorType)
		err := exec.ExecuteCommand(ctx, commands.NewGcsPublishServiceStartCmd(exec))
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Publish service gcs publish results cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := NewCrosPublishExecutor(cont, CrosGcsPublishExecutorType)
		err := exec.ExecuteCommand(ctx, commands.NewGcsPublishUploadCmd(exec))
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Publish service tko start cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := NewCrosPublishExecutor(cont, CrosTkoPublishExecutorType)
		err := exec.ExecuteCommand(ctx, commands.NewTkoPublishServiceStartCmd(exec))
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Publish service tko publish results cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := NewCrosPublishExecutor(cont, CrosTkoPublishExecutorType)
		err := exec.ExecuteCommand(ctx, commands.NewTkoPublishUploadCmd(exec))
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Publish service cpcon start cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := NewCrosPublishExecutor(cont, CrosPublishExecutorType)
		err := exec.ExecuteCommand(ctx, commands.NewCpconPublishServiceStartCmd(exec))
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Publish service cpcon publish results cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := NewCrosPublishExecutor(cont, CrosPublishExecutorType)
		err := exec.ExecuteCommand(ctx, commands.NewCpconPublishUploadCmd(exec))
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Publish service rdb start cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := NewCrosPublishExecutor(cont, CrosRdbPublishExecutorType)
		err := exec.ExecuteCommand(ctx, commands.NewRdbPublishServiceStartCmd(exec))
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Publish service rdb publish results cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosPublishTemplatedContainer(
			containers.CrosGcsPublishTemplatedContainerType,
			"container/image/path",
			ctr)
		exec := NewCrosPublishExecutor(cont, CrosRdbPublishExecutorType)
		err := exec.ExecuteCommand(ctx, commands.NewRdbPublishUploadCmd(exec))
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func getMockedPublishResults(mockClient *mocked_services.MockGenericPublishServiceClient) *gomock.Call {
	return mockClient.EXPECT().Publish(gomock.Any(),
		gomock.AssignableToTypeOf(&testapi.PublishRequest{}),
		gomock.Any())
}
