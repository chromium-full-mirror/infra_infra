// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostoolrunner

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/golang/mock/gomock"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestStartCtrServer(t *testing.T) {
	t.Parallel()

	ftt.Run("CTR server start initialization error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		err := ctr.StartCTRServer(ctx)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestStartCtrServerAsync(t *testing.T) {
	// This test closed the tree. Make it non-parallel.
	//
	// t.Parallel()

	ftt.Run("CTR server start async error with existing server connection", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		ctr.wg = &sync.WaitGroup{}
		err := ctr.StartCTRServerAsync(ctx)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, ctr.wg, should.NotBeNil)
	})

	ftt.Run("CTR server start async success", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		err := ctr.StartCTRServerAsync(ctx)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, ctr.wg, should.NotBeNil)
	})
}

func TestGetServerAddressFromServiceMetadata(t *testing.T) {
	t.Parallel()
	ftt.Run("CTR get server address without temp dir error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		serverAddress, err := ctr.GetServerAddressFromServiceMetadata(ctx)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, serverAddress, should.BeEmpty)
	})
}

func TestConnectToCtrServer(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	ftt.Run("CTR server connection without server address", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		ctrClient, err := ctr.ConnectToCTRServer(ctx, "")
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, ctrClient, should.BeNil)
	})

	ftt.Run("CTR server connection with existing client", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		ctr.CtrClient = NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctrClient, err := ctr.ConnectToCTRServer(ctx, "localhost:1234")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, ctrClient, should.NotBeNil)
	})
}

func TestStopCtrServer(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	ftt.Run("CTR server stop error while server is not running", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		ctr.isServerRunning = false
		err := ctr.StopCTRServer(ctx)
		assert.Loosely(t, err, should.BeNil)
	})

	ftt.Run("CTR server stop error while no established client exists", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		ctr.isServerRunning = true
		err := ctr.StopCTRServer(ctx)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("CTR server stop grpc failure", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		ctr.isServerRunning = true

		mockedClient := NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mockedClient

		getMockedShutdown(mockedClient).Return(nil, fmt.Errorf("some error"))

		err := ctr.StopCTRServer(ctx)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, ctr.isServerRunning, should.BeTrue)
	})

	ftt.Run("CTR server stop success", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		ctr.wg = &sync.WaitGroup{}
		ctr.isServerRunning = true

		mockedClient := NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mockedClient

		getMockedShutdown(mockedClient).Return(&testapi.ShutdownResponse{}, nil)

		err := ctr.StopCTRServer(ctx)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, ctr.isServerRunning, should.BeFalse)
		assert.Loosely(t, ctr.wg, should.BeNil)
	})
}

func TestStartContainer(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	ftt.Run("CTR start container error with nil request", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		resp, err := ctr.StartContainer(ctx, nil)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("CTR start container error with no established client", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		resp, err := ctr.StartContainer(ctx, &testapi.StartContainerRequest{})
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("CTR start container error with grpc failure", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}

		mockedClient := NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mockedClient

		getMockedStartContainer(mockedClient).Return(nil, fmt.Errorf("some error"))
		resp, err := ctr.StartContainer(ctx, &testapi.StartContainerRequest{})
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("CTR start container success", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}

		mockedClient := NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mockedClient

		getMockedStartContainer(mockedClient).Return(&testapi.StartContainerResponse{}, nil)
		resp, err := ctr.StartContainer(ctx, &testapi.StartContainerRequest{})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.NotBeNil)
	})
}

func TestStartTemplatedContainer(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	ftt.Run("CTR start templated container error with nil request", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		resp, err := ctr.StartTemplatedContainer(ctx, nil)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("CTR start templated container error with no established client", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		resp, err := ctr.StartTemplatedContainer(ctx, &testapi.StartTemplatedContainerRequest{})
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("CTR start templated container error with grpc failure", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}

		mockedClient := NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mockedClient

		getMockedStartTemplatedContainer(mockedClient).Return(nil, fmt.Errorf("some error"))
		resp, err := ctr.StartTemplatedContainer(ctx, &testapi.StartTemplatedContainerRequest{})
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("CTR start templated container success", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}

		mockedClient := NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mockedClient

		getMockedStartTemplatedContainer(mockedClient).Return(&testapi.StartContainerResponse{}, nil)
		resp, err := ctr.StartTemplatedContainer(ctx, &testapi.StartTemplatedContainerRequest{})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.NotBeNil)
	})
}

func TestStopContainer(t *testing.T) {
	t.Parallel()

	ftt.Run("CTR stop container with empty container name", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		ctr.isServerRunning = false
		err := ctr.StopContainer(ctx, "")
		assert.Loosely(t, err, should.NotBeNil)
	})

	// TODO (azrahman): fix test for windows (sudo not in path)

	// Convey("CTR stop container success", t, func() {
	// 	ctx := context.Background()
	// 	ctrCipd := CtrCipdInfo{Version: "prod"}
	// 	ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
	// 	ctr.isServerRunning = false
	// 	err := ctr.StopContainer(ctx, "container-1234")
	// 	So(err, ShouldBeNil)
	// })
}

func TestGetContainer(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	containerName := "container-1234"

	ftt.Run("CTR get container with empty container name", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		ctr.isServerRunning = false
		resp, err := ctr.GetContainer(ctx, "")
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("CTR get container error with no established client", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		resp, err := ctr.GetContainer(ctx, containerName)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("CTR get container error with grpc failure", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}

		mockedClient := NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mockedClient

		getMockedGetContainer(mockedClient).Return(nil, fmt.Errorf("some error"))
		resp, err := ctr.GetContainer(ctx, containerName)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("CTR get templated container success", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}

		mockedClient := NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mockedClient

		getMockedGetContainer(mockedClient).Return(&testapi.GetContainerResponse{
			Container: &testapi.Container{
				PortBindings: []*testapi.Container_PortBinding{
					{
						HostIp:   "localhost",
						HostPort: 1234,
					},
				},
			},
		},
			nil)
		resp, err := ctr.GetContainer(ctx, containerName)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.NotBeNil)
	})
}

func TestGcloudAuth(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	BackoffFunc = getMockedBackoff

	ftt.Run("CTR gcloud auth error with no established client", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		resp, err := ctr.GcloudAuth(ctx, "", false)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("CTR gcloud auth error with grpc failure", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}

		mockedClient := NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mockedClient

		getMockedLoginRegistry(mockedClient).Return(nil, fmt.Errorf("some error"))
		resp, err := ctr.GcloudAuth(ctx, "", false)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("CTR gcloud auth success", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}

		mockedClient := NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mockedClient

		getMockedLoginRegistry(mockedClient).Return(&testapi.LoginRegistryResponse{}, nil)
		resp, err := ctr.GcloudAuth(ctx, "docker/file/location", false)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.NotBeNil)
	})
}

func getMockedShutdown(mctrclient *MockCrosToolRunnerContainerServiceClient) *gomock.Call {
	return mctrclient.EXPECT().Shutdown(
		gomock.Any(),
		gomock.AssignableToTypeOf(&testapi.ShutdownRequest{}),
		gomock.Any())
}

func getMockedStartContainer(mctrclient *MockCrosToolRunnerContainerServiceClient) *gomock.Call {
	return mctrclient.EXPECT().StartContainer(
		gomock.Any(),
		gomock.AssignableToTypeOf(&testapi.StartContainerRequest{}),
		gomock.Any())
}

func getMockedStartTemplatedContainer(mctrclient *MockCrosToolRunnerContainerServiceClient) *gomock.Call {
	return mctrclient.EXPECT().StartTemplatedContainer(
		gomock.Any(),
		gomock.AssignableToTypeOf(&testapi.StartTemplatedContainerRequest{}),
		gomock.Any())
}

func getMockedGetContainer(mctrclient *MockCrosToolRunnerContainerServiceClient) *gomock.Call {
	return mctrclient.EXPECT().GetContainer(
		gomock.Any(),
		gomock.AssignableToTypeOf(&testapi.GetContainerRequest{}),
		gomock.Any())
}

func getMockedLoginRegistry(mctrclient *MockCrosToolRunnerContainerServiceClient) *gomock.Call {
	return mctrclient.EXPECT().LoginRegistry(
		gomock.Any(),
		gomock.AssignableToTypeOf(&testapi.LoginRegistryRequest{}),
		gomock.Any()).AnyTimes()
}

func getMockedBackoff() backoff.BackOff {
	return backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Millisecond), 1)
}
