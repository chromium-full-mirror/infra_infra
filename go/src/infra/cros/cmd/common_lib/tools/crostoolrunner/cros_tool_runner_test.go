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
	. "github.com/smartystreets/goconvey/convey"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
)

func TestStartCtrServer(t *testing.T) {
	t.Parallel()

	Convey("CTR server start initialization error", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		err := ctr.StartCTRServer(ctx)
		So(err, ShouldNotBeNil)
	})
}

func TestStartCtrServerAsync(t *testing.T) {
	// This test closed the tree. Make it non-parallel.
	//
	// t.Parallel()

	Convey("CTR server start async error with existing server connection", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		ctr.wg = &sync.WaitGroup{}
		err := ctr.StartCTRServerAsync(ctx)
		So(err, ShouldNotBeNil)
		So(ctr.wg, ShouldNotBeNil)
	})

	Convey("CTR server start async success", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		err := ctr.StartCTRServerAsync(ctx)
		So(err, ShouldBeNil)
		So(ctr.wg, ShouldNotBeNil)
	})
}

func TestGetServerAddressFromServiceMetadata(t *testing.T) {
	t.Parallel()
	Convey("CTR get server address without temp dir error", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		serverAddress, err := ctr.GetServerAddressFromServiceMetadata(ctx)
		So(err, ShouldNotBeNil)
		So(serverAddress, ShouldEqual, "")
	})
}

func TestConnectToCtrServer(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	Convey("CTR server connection without server address", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		ctrClient, err := ctr.ConnectToCTRServer(ctx, "")
		So(err, ShouldNotBeNil)
		So(ctrClient, ShouldBeNil)
	})

	Convey("CTR server connection with existing client", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		ctr.CtrClient = NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctrClient, err := ctr.ConnectToCTRServer(ctx, "localhost:1234")
		So(err, ShouldBeNil)
		So(ctrClient, ShouldNotBeNil)
	})
}

func TestStopCtrServer(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	Convey("CTR server stop error while server is not running", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		ctr.isServerRunning = false
		err := ctr.StopCTRServer(ctx)
		So(err, ShouldBeNil)
	})

	Convey("CTR server stop error while no established client exists", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		ctr.isServerRunning = true
		err := ctr.StopCTRServer(ctx)
		So(err, ShouldNotBeNil)
	})

	Convey("CTR server stop grpc failure", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		ctr.isServerRunning = true

		mocked_client := NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mocked_client

		getMockedShutdown(mocked_client).Return(nil, fmt.Errorf("some error"))

		err := ctr.StopCTRServer(ctx)
		So(err, ShouldNotBeNil)
		So(ctr.isServerRunning, ShouldBeTrue)
	})

	Convey("CTR server stop success", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		ctr.wg = &sync.WaitGroup{}
		ctr.isServerRunning = true

		mocked_client := NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mocked_client

		getMockedShutdown(mocked_client).Return(&testapi.ShutdownResponse{}, nil)

		err := ctr.StopCTRServer(ctx)
		So(err, ShouldBeNil)
		So(ctr.isServerRunning, ShouldBeFalse)
		So(ctr.wg, ShouldBeNil)
	})
}

func TestStartContainer(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	Convey("CTR start container error with nil request", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		resp, err := ctr.StartContainer(ctx, nil)
		So(err, ShouldNotBeNil)
		So(resp, ShouldBeNil)
	})

	Convey("CTR start container error with no established client", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		resp, err := ctr.StartContainer(ctx, &testapi.StartContainerRequest{})
		So(err, ShouldNotBeNil)
		So(resp, ShouldBeNil)
	})

	Convey("CTR start container error with grpc failure", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}

		mocked_client := NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mocked_client

		getMockedStartContainer(mocked_client).Return(nil, fmt.Errorf("some error"))
		resp, err := ctr.StartContainer(ctx, &testapi.StartContainerRequest{})
		So(err, ShouldNotBeNil)
		So(resp, ShouldBeNil)
	})

	Convey("CTR start container success", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}

		mocked_client := NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mocked_client

		getMockedStartContainer(mocked_client).Return(&testapi.StartContainerResponse{}, nil)
		resp, err := ctr.StartContainer(ctx, &testapi.StartContainerRequest{})
		So(err, ShouldBeNil)
		So(resp, ShouldNotBeNil)
	})
}

func TestStartTemplatedContainer(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	Convey("CTR start templated container error with nil request", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		resp, err := ctr.StartTemplatedContainer(ctx, nil)
		So(err, ShouldNotBeNil)
		So(resp, ShouldBeNil)
	})

	Convey("CTR start templated container error with no established client", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		resp, err := ctr.StartTemplatedContainer(ctx, &testapi.StartTemplatedContainerRequest{})
		So(err, ShouldNotBeNil)
		So(resp, ShouldBeNil)
	})

	Convey("CTR start templated container error with grpc failure", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}

		mocked_client := NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mocked_client

		getMockedStartTemplatedContainer(mocked_client).Return(nil, fmt.Errorf("some error"))
		resp, err := ctr.StartTemplatedContainer(ctx, &testapi.StartTemplatedContainerRequest{})
		So(err, ShouldNotBeNil)
		So(resp, ShouldBeNil)
	})

	Convey("CTR start templated container success", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}

		mocked_client := NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mocked_client

		getMockedStartTemplatedContainer(mocked_client).Return(&testapi.StartContainerResponse{}, nil)
		resp, err := ctr.StartTemplatedContainer(ctx, &testapi.StartTemplatedContainerRequest{})
		So(err, ShouldBeNil)
		So(resp, ShouldNotBeNil)
	})
}

func TestStopContainer(t *testing.T) {
	t.Parallel()

	Convey("CTR stop container with empty container name", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		ctr.isServerRunning = false
		err := ctr.StopContainer(ctx, "")
		So(err, ShouldNotBeNil)
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

	Convey("CTR get container with empty container name", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		ctr.isServerRunning = false
		resp, err := ctr.GetContainer(ctx, "")
		So(err, ShouldNotBeNil)
		So(resp, ShouldBeNil)
	})

	Convey("CTR get container error with no established client", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		resp, err := ctr.GetContainer(ctx, containerName)
		So(err, ShouldNotBeNil)
		So(resp, ShouldBeNil)
	})

	Convey("CTR get container error with grpc failure", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}

		mocked_client := NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mocked_client

		getMockedGetContainer(mocked_client).Return(nil, fmt.Errorf("some error"))
		resp, err := ctr.GetContainer(ctx, containerName)
		So(err, ShouldNotBeNil)
		So(resp, ShouldBeNil)
	})

	Convey("CTR get templated container success", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}

		mocked_client := NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mocked_client

		getMockedGetContainer(mocked_client).Return(&testapi.GetContainerResponse{
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
		So(err, ShouldBeNil)
		So(resp, ShouldNotBeNil)
	})
}

func TestGcloudAuth(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	BackoffFunc = getMockedBackoff

	Convey("CTR gcloud auth error with no established client", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}
		resp, err := ctr.GcloudAuth(ctx, "", false)
		So(err, ShouldNotBeNil)
		So(resp, ShouldBeNil)
	})

	Convey("CTR gcloud auth error with grpc failure", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}

		mocked_client := NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mocked_client

		getMockedLoginRegistry(mocked_client).Return(nil, fmt.Errorf("some error"))
		resp, err := ctr.GcloudAuth(ctx, "", false)
		So(err, ShouldNotBeNil)
		So(resp, ShouldBeNil)
	})

	Convey("CTR gcloud auth success", t, func() {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "prod"}
		ctr := CrosToolRunner{CtrCipdInfo: ctrCipd}

		mocked_client := NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mocked_client

		getMockedLoginRegistry(mocked_client).Return(&testapi.LoginRegistryResponse{}, nil)
		resp, err := ctr.GcloudAuth(ctx, "docker/file/location", false)
		So(err, ShouldBeNil)
		So(resp, ShouldNotBeNil)
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
