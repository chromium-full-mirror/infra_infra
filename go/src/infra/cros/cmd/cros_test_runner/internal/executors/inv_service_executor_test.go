// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executors

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"google.golang.org/grpc/metadata"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/cros_test_runner/internal/commands"
	"infra/cros/cmd/cros_test_runner/internal/mocked_services"
)

type UnsupportedCmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor
}

func NewUnsupportedCmd() interfaces.CommandInterface {
	absCmd := interfaces.NewAbstractCmd(commands.UnSupportedCmdType)
	absSingleCmdByNoExec := &interfaces.AbstractSingleCmdByNoExecutor{AbstractCmd: absCmd}
	return &UnsupportedCmd{AbstractSingleCmdByNoExecutor: absSingleCmdByNoExec}
}

func TestInvServiceStart(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	ftt.Run("Inventory service start with an already established connection", t, func(t *ftt.Test) {
		ctx := context.Background()
		exec := NewInvServiceExecutor("")
		exec.InventoryServiceClient = mocked_services.NewMockInventoryServiceClient(ctrl)
		err := exec.Start(ctx, exec.InventoryServiceAddress)
		assert.Loosely(t, err, should.BeNil)
	})

	ftt.Run("Inventory service start with empty server address", t, func(t *ftt.Test) {
		ctx := context.Background()
		exec := NewInvServiceExecutor("")
		err := exec.Start(ctx, "")
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestInvServiceStop(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	ftt.Run("Inventory service stop with no established server", t, func(t *ftt.Test) {
		ctx := context.Background()
		exec := NewInvServiceExecutor("")
		err := exec.Stop(ctx)
		assert.Loosely(t, err, should.BeNil)
	})

	ftt.Run("Inventory service stop with no grpc connection", t, func(t *ftt.Test) {
		ctx := context.Background()
		exec := NewInvServiceExecutor("")
		exec.InventoryServiceClient = mocked_services.NewMockInventoryServiceClient(ctrl)
		err := exec.Stop(ctx)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestInvServiceGetDutTopology(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	hostName := "DUT-1234"

	ftt.Run("GetDutTopology with empty host name", t, func(t *ftt.Test) {
		ctx := context.Background()
		exec := NewInvServiceExecutor("")
		dutTopology, err := exec.GetDUTTopology(ctx, "")
		assert.Loosely(t, dutTopology, should.BeNil)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("GetDutTopology with no service client", t, func(t *ftt.Test) {
		ctx := context.Background()
		exec := NewInvServiceExecutor("")
		dutTopology, err := exec.GetDUTTopology(ctx, hostName)
		assert.Loosely(t, dutTopology, should.BeNil)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("GetDutTopology_Success", t, func(t *ftt.Test) {
		ctx := context.Background()
		exec := NewInvServiceExecutor("")
		mockInvServiceClient := mocked_services.NewMockInventoryServiceClient(ctrl)
		mockInvServiceGDTClient := mocked_services.NewMockInventoryService_GetDutTopologyClient(ctrl)
		exec.InventoryServiceClient = mockInvServiceClient

		getMockedGetDutTopologyRcvMsgSuccess(mockInvServiceGDTClient, hostName)
		getMockedGetDutTopology(mockInvServiceClient, hostName).Return(mockInvServiceGDTClient, nil)

		dutTopology, err := exec.GetDUTTopology(ctx, hostName)
		assert.Loosely(t, dutTopology, should.NotBeNil)
		assert.Loosely(t, err, should.BeNil)
	})

	ftt.Run("GetDutTopology_grpc_failure", t, func(t *ftt.Test) {
		ctx := context.Background()
		exec := NewInvServiceExecutor("")
		mockInvServiceClient := mocked_services.NewMockInventoryServiceClient(ctrl)
		exec.InventoryServiceClient = mockInvServiceClient

		getMockedGetDutTopology(mockInvServiceClient, hostName).Return(nil, fmt.Errorf("some error"))

		dutTopology, err := exec.GetDUTTopology(ctx, hostName)
		assert.Loosely(t, dutTopology, should.BeNil)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("GetDutTopology_grpc_response_failure", t, func(t *ftt.Test) {
		ctx := context.Background()
		exec := NewInvServiceExecutor("")
		mockInvServiceClient := mocked_services.NewMockInventoryServiceClient(ctrl)
		mockInvServiceGDTClient := mocked_services.NewMockInventoryService_GetDutTopologyClient(ctrl)
		exec.InventoryServiceClient = mockInvServiceClient

		getMockedGetDutTopologyRcvMsgFailure(mockInvServiceGDTClient)
		getMockedGetDutTopology(mockInvServiceClient, hostName).Return(mockInvServiceGDTClient, nil)

		dutTopology, err := exec.GetDUTTopology(ctx, hostName)
		assert.Loosely(t, dutTopology, should.BeNil)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestInvServiceExecuteCommand(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	ftt.Run("Inventory service unsupported cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		exec := NewInvServiceExecutor("")
		err := exec.ExecuteCommand(ctx, NewUnsupportedCmd())
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Inventory service start cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		exec := NewInvServiceExecutor("")
		exec.InventoryServiceAddress = ""
		startCmd := commands.NewInvServiceStartCmd(exec)
		err := exec.ExecuteCommand(ctx, startCmd)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Inventory service load dut topology cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		exec := NewInvServiceExecutor("")
		loadDuttopologyCmd := commands.NewLoadDutTopologyCmd(exec)
		err := exec.ExecuteCommand(ctx, loadDuttopologyCmd)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Inventory service stop cmd execution error", t, func(t *ftt.Test) {
		ctx := context.Background()
		exec := NewInvServiceExecutor("")
		exec.InventoryServiceClient = mocked_services.NewMockInventoryServiceClient(ctrl)
		stopCmd := commands.NewInvServiceStopCmd(exec)
		err := exec.ExecuteCommand(ctx, stopCmd)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func getMockedGetDutTopology(
	mis *mocked_services.MockInventoryServiceClient,
	hostName string) *gomock.Call {
	return mis.EXPECT().GetDutTopology(
		gomock.Any(),
		gomock.Eq(&labapi.GetDutTopologyRequest{
			Id: &labapi.DutTopology_Id{
				Value: hostName,
			},
		},
		),
	)
}

func TestGetDUTHostnameAndContextForInventory(t *testing.T) {

	for _, testcase := range []struct {
		droneAgentBotPrefix string
		commandHostname     string
		wantHostname        string
		wantNamespace       string
		wantError           bool
	}{
		{
			droneAgentBotPrefix: "crossk-",
			commandHostname:     "crossk-this-is-good",
			wantHostname:        "crossk-this-is-good",
			wantNamespace:       "",
			wantError:           false,
		},
		{
			droneAgentBotPrefix: "extcro-",
			commandHostname:     "extcro-this-is-good",
			wantHostname:        "this-is-good",
			wantNamespace:       "os-partner",
			wantError:           false,
		},
		{
			droneAgentBotPrefix: "extcro-",
			commandHostname:     "extnotcro-this-is-good",
			wantNamespace:       "",
			wantError:           true,
		},
	} {
		t.Setenv("DRONE_AGENT_BOT_PREFIX", testcase.droneAgentBotPrefix)
		ctx := context.Background()

		ctx, hostname, err := getDUTHostnameAndContextForInventory(ctx, testcase.commandHostname)
		if err != nil {
			if testcase.wantError {
				continue
			}
			t.Error(err)
			continue
		}

		// check altered hostname (minus bot-prefix)
		if hostname != testcase.wantHostname {
			t.Errorf("produced hostname does not match, want: %s got: %s", testcase.wantHostname, hostname)
			continue
		}
		if md, found := metadata.FromOutgoingContext(ctx); !found {
			// okay not to be found if the wanted namespace is blank
			if testcase.wantNamespace == "" {
				continue
			}
			t.Errorf("metadata not found in outgoing context, want: %s for namespace", testcase.wantNamespace)
			continue
		} else {
			// in this case we should have exactly one namespace populated
			namespace := md.Get("namespace")
			if len(namespace) != 1 {
				t.Errorf("metadata for key namespace does not match expected, got: %s", namespace)
				continue
			}
			if namespace[0] != testcase.wantNamespace {
				t.Errorf("incorrect metadata for namespace, want: %s, got: %s", testcase.wantNamespace, namespace[0])
				continue
			}
		}
	}
}

func getMockedGetDutTopologyRcvMsgSuccess(
	misgtc *mocked_services.MockInventoryService_GetDutTopologyClient,
	hostName string) *gomock.Call {

	return misgtc.EXPECT().RecvMsg(gomock.Eq(&labapi.GetDutTopologyResponse{})).DoAndReturn(
		func(resp *labapi.GetDutTopologyResponse) error {
			resp.Result = &labapi.GetDutTopologyResponse_Success_{
				Success: &labapi.GetDutTopologyResponse_Success{
					DutTopology: &labapi.DutTopology{
						Id: &labapi.DutTopology_Id{
							Value: hostName,
						},
					},
				},
			}
			return nil
		})
}

func getMockedGetDutTopologyRcvMsgFailure(
	misgtc *mocked_services.MockInventoryService_GetDutTopologyClient) *gomock.Call {
	return misgtc.EXPECT().RecvMsg(gomock.Eq(&labapi.GetDutTopologyResponse{})).DoAndReturn(
		func(resp *labapi.GetDutTopologyResponse) error {
			return fmt.Errorf("some error")
		})
}
