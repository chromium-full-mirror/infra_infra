// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package containers

import (
	"context"
	"fmt"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
)

func TestGetContainerType(t *testing.T) {
	t.Parallel()

	ftt.Parallel("GetContainerType", t, func(t *ftt.Test) {
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		absContainer := NewAbstractContainer(wantContType, "test-container", "host", "container-image", ctr)
		gotContType := absContainer.GetContainerType()
		assert.Loosely(t, gotContType, should.Equal(wantContType))
	})
}

func TestGetLogsLocation(t *testing.T) {
	t.Parallel()

	ftt.Parallel("GetLogsLocation_error", t, func(t *ftt.Test) {
		contType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		absContainer := NewAbstractContainer(contType, "test-container", "host", "container-image", ctr)
		logsLoc, err := absContainer.GetLogsLocation()
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, logsLoc, should.BeEmpty)
	})

	ftt.Parallel("GetLogsLocation_success", t, func(t *ftt.Test) {
		contType := CrosProvisionTemplatedContainerType
		wantLogsLoc := "temp/dir/loc"
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		absContainer := NewAbstractContainer(contType, "test-container", "host", "container-image", ctr)
		absContainer.TempDirLoc = wantLogsLoc
		gotLogsLoc, err := absContainer.GetLogsLocation()
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, gotLogsLoc, should.Equal(wantLogsLoc))
	})
}

func TestInitializeBase(t *testing.T) {
	t.Parallel()

	ftt.Parallel("InitializeBase_wrong_state", t, func(t *ftt.Test) {
		ctx := context.Background()
		contType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		absContainer := NewAbstractContainer(contType, "test-container", "host", "container-image", ctr)
		absContainer.state = ContainerStateStarted
		err := absContainer.InitializeBase(ctx)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Parallel("InitializeBase_missing_namePrefix", t, func(t *ftt.Test) {
		ctx := context.Background()
		contType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		absContainer := NewAbstractContainer(contType, "", "host", "container-image", ctr)
		err := absContainer.InitializeBase(ctx)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Parallel("InitializeBase_missing_containerImage", t, func(t *ftt.Test) {
		ctx := context.Background()
		contType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		absContainer := NewAbstractContainer(contType, "test-container", "host", "", ctr)
		err := absContainer.InitializeBase(ctx)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Parallel("InitializeBase_success", t, func(t *ftt.Test) {
		ctx := context.Background()
		contType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		absContainer := NewAbstractContainer(contType, "test-container", "host", "container-image", ctr)
		err := absContainer.InitializeBase(ctx)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, absContainer.TempDirLoc, should.NotEqual(""))
	})
}

func TestGetContainer(t *testing.T) {
	t.Parallel()

	ftt.Parallel("GetContainer_wrong_state", t, func(t *ftt.Test) {
		ctx := context.Background()
		contType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		absContainer := NewAbstractContainer(contType, "test-container", "host", "container-image", ctr)
		resp, err := absContainer.GetContainer(ctx)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Parallel("GetContainer_empty_container_name", t, func(t *ftt.Test) {
		ctx := context.Background()
		contType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		absContainer := NewAbstractContainer(contType, "test-container", "host", "container-image", ctr)
		absContainer.state = ContainerStateStarted
		resp, err := absContainer.GetContainer(ctx)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Parallel("GetContainer_without_starting_client", t, func(t *ftt.Test) {
		ctx := context.Background()
		contType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		absContainer := NewAbstractContainer(contType, "test-container", "host", "container-image", ctr)
		absContainer.state = ContainerStateStarted
		absContainer.Name = "container-1234"
		resp, err := absContainer.GetContainer(ctx)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})
}

func TestStopContainer(t *testing.T) {
	t.Parallel()

	ftt.Parallel("StopContainer_wrong_state", t, func(t *ftt.Test) {
		ctx := context.Background()
		contType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		absContainer := NewAbstractContainer(contType, "test-container", "host", "container-image", ctr)
		err := absContainer.StopContainer(ctx)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Parallel("StopContainer_empty_container_name", t, func(t *ftt.Test) {
		ctx := context.Background()
		contType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		absContainer := NewAbstractContainer(contType, "test-container", "host", "container-image", ctr)
		absContainer.state = ContainerStateStarted
		err := absContainer.StopContainer(ctx)
		assert.Loosely(t, err, should.NotBeNil)
	})
	// TODO (azrahman): fix test for windows (sudo not in path)
	//
	//	Convey("GetContainer_with_no_client", t, func() {
	//		ctx := context.Background()
	//		contType := CrosProvisionTemplatedContainerType
	//		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
	//		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
	//		absContainer := NewAbstractContainer(contType, "test-container", "host", "container-image", ctr)
	//		absContainer.state = ContainerStateStarted
	//		absContainer.Name = "container-1234"
	//		err := absContainer.StopContainer(ctx)
	//		So(err, ShouldBeNil)
	//	})
}

func TestProcessContainer(t *testing.T) {
	t.Parallel()

	ftt.Parallel("ProcessContainer_initialization_error", t, func(t *ftt.Test) {
		ctx := context.Background()
		contType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		absContainer := NewAbstractContainer(contType, "test-container", "host", "container-image", ctr)
		absContainer.ConcreteContainer = &MockContainer{InitializeErr: fmt.Errorf("some err")}
		address, err := absContainer.ProcessContainer(ctx, nil)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, address, should.BeEmpty)
	})

	ftt.Parallel("ProcessContainer_startcontainer_error", t, func(t *ftt.Test) {
		ctx := context.Background()
		contType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		absContainer := NewAbstractContainer(contType, "test-container", "host", "container-image", ctr)
		absContainer.ConcreteContainer = &MockContainer{InitializeErr: nil, StartContainerResp: nil, StartContainerErr: fmt.Errorf("some err")}
		address, err := absContainer.ProcessContainer(ctx, nil)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, address, should.BeEmpty)
	})

	ftt.Parallel("ProcessContainer_getcontainer_error", t, func(t *ftt.Test) {
		ctx := context.Background()
		contType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		absContainer := NewAbstractContainer(contType, "test-container", "host", "container-image", ctr)
		absContainer.ConcreteContainer = &MockContainer{InitializeErr: nil, StartContainerResp: &api.StartContainerResponse{}, StartContainerErr: nil, GetContainerResp: nil, GetContainerErr: fmt.Errorf("some err")}
		address, err := absContainer.ProcessContainer(ctx, nil)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, address, should.BeEmpty)
	})

	ftt.Parallel("ProcessContainer_server_address_retrieval_error", t, func(t *ftt.Test) {
		ctx := context.Background()
		contType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		absContainer := NewAbstractContainer(contType, "test-container", "host", "container-image", ctr)
		absContainer.ConcreteContainer = &MockContainer{InitializeErr: nil, StartContainerResp: &api.StartContainerResponse{}, StartContainerErr: nil, GetContainerResp: nil, GetContainerErr: nil}
		address, err := absContainer.ProcessContainer(ctx, nil)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, address, should.BeEmpty)
	})

	ftt.Parallel("ProcessContainer_success", t, func(t *ftt.Test) {
		ctx := context.Background()
		hostIP := "localhost"
		hostPort := int32(1234)
		contType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		absContainer := NewAbstractContainer(contType, "test-container", "host", "container-image", ctr)
		getResp := &api.GetContainerResponse{Container: &api.Container{
			PortBindings: []*api.Container_PortBinding{
				{
					HostIp:   hostIP,
					HostPort: hostPort,
				},
			},
		},
		}
		absContainer.ConcreteContainer = &MockContainer{
			InitializeErr:      nil,
			StartContainerResp: &api.StartContainerResponse{},
			StartContainerErr:  nil,
			GetContainerResp:   getResp,
			GetContainerErr:    nil}
		address, err := absContainer.ProcessContainer(ctx, nil)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, address, should.Equal(fmt.Sprintf("%s:%v", hostIP, hostPort)))
	})
}

type MockContainer struct {
	ContainerType                 interfaces.ContainerType
	InitializeErr                 error
	ProcessContainerServerAddress string
	ProcessContainerErr           error
	StopContainerErr              error
	LogsLoc                       string
	LogsLocErr                    error
	StartContainerResp            *api.StartContainerResponse
	StartContainerErr             error
	GetContainerResp              *api.GetContainerResponse
	GetContainerErr               error
}

func (cont *MockContainer) GetContainerType() interfaces.ContainerType {
	return cont.ContainerType
}

func (cont *MockContainer) Initialize(context.Context, *api.Template) error {
	return cont.InitializeErr
}

func (cont *MockContainer) ProcessContainer(context.Context, *api.Template) (string, error) {
	return cont.ProcessContainerServerAddress, cont.ProcessContainerErr
}

func (cont *MockContainer) StopContainer(context.Context) error {
	return cont.StopContainerErr
}

func (cont *MockContainer) GetLogsLocation() (string, error) {
	return cont.LogsLoc, cont.LogsLocErr
}

func (cont *MockContainer) StartContainer(context.Context) (*api.StartContainerResponse, error) {
	return cont.StartContainerResp, cont.StartContainerErr
}

func (cont *MockContainer) GetContainer(context.Context) (*api.GetContainerResponse, error) {
	return cont.GetContainerResp, cont.GetContainerErr
}
