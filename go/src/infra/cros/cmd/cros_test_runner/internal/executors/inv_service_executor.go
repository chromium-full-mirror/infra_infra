// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executors

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/commands"
	"go.chromium.org/infra/libs/skylab/common/heuristics"
)

const (
	defaultLabInventoryServiceAddress = ":1485" // lab inventory service address
)

// CrosProvisionExecutor represents executor for
// all inventory service related commands.
type InvServiceExecutor struct {
	*interfaces.AbstractExecutor

	InventoryServiceAddress string
	InventoryServiceClient  labapi.InventoryServiceClient
	GrpcConn                *grpc.ClientConn
	DutTopology             *labapi.DutTopology
}

// NewInvServiceExecutor creates a new InvServiceExecutor object.
// inventoryServiceAddress argument is optional.
// If not provided, defaultLabInventoryServiceAddress will be used.
func NewInvServiceExecutor(inventoryServiceAddress string) *InvServiceExecutor {
	// Set service address to default lab address if not provided
	if inventoryServiceAddress == "" {
		inventoryServiceAddress = defaultLabInventoryServiceAddress
	}

	abstractExec := interfaces.NewAbstractExecutor(InvServiceExecutorType)
	return &InvServiceExecutor{
		AbstractExecutor:        abstractExec,
		InventoryServiceAddress: inventoryServiceAddress}
}

func (ex *InvServiceExecutor) ExecuteCommand(
	ctx context.Context,
	cmdInterface interfaces.CommandInterface) error {
	switch cmd := cmdInterface.(type) {
	case *commands.InvServiceStartCmd:
		return ex.invServiceStartCommandExecution(ctx, cmd)
	case *commands.LoadDutTopologyCmd:
		return ex.loadDutTopologyCommandExecution(ctx, cmd)
	case *commands.InvServiceStopCmd:
		return ex.invServiceStopCommandExecution(ctx, cmd)
	case *commands.BuildDutTopologyCmd:
		return ex.invServiceBuildDutTopology(ctx, cmd)
	default:
		return fmt.Errorf(
			"Command type %s is not supported by %s executor type!",
			cmd.GetCommandType(),
			ex.GetExecutorType())
	}
}

// invServiceBuildDutTopology builds the dut topology from supplied inputs.
func (ex *InvServiceExecutor) invServiceBuildDutTopology(
	ctx context.Context,
	cmd *commands.BuildDutTopologyCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "Build Dut Topology")
	defer func() { step.End(err) }()

	cmd.DutTopology = &labapi.DutTopology{
		Duts: []*labapi.Dut{
			{
				Id: &labapi.Dut_Id{
					Value: "localhost",
				},
				DutType: &labapi.Dut_Chromeos{
					Chromeos: &labapi.Dut_ChromeOS{
						Ssh: cmd.DutSshAddress,
						DutModel: &labapi.DutModel{
							BuildTarget: cmd.Board,
						},
						Servo: &labapi.Servo{
							Present: false,
						},
					},
				},
				CacheServer: &labapi.CacheServer{
					Address: cmd.CacheServerAddress,
				},
			},
		},
	}

	return nil
}

// invServiceStartCommandExecution executes the inventory service start command.
func (ex *InvServiceExecutor) invServiceStartCommandExecution(
	ctx context.Context,
	cmd *commands.InvServiceStartCmd) error {
	var err error
	step, ctx := build.StartStep(ctx, "Inventory service start")
	defer func() { step.End(err) }()

	err = ex.Start(ctx, ex.InventoryServiceAddress)
	if err != nil {
		return errors.Annotate(err, "Start inventory service cmd err: ").Err()
	}

	return nil
}

func getDUTHostnameAndContextForInventory(ctx context.Context, hostname string) (context.Context, string, error) {
	botPrefix := os.Getenv("DRONE_AGENT_BOT_PREFIX")
	if botPrefix != "" && !common.StringInSlice(botPrefix, heuristics.HwSwarmingBotIDPrefixes) {
		var found bool
		hostname, found = strings.CutPrefix(hostname, botPrefix)
		if found {
			ctx = metadata.AppendToOutgoingContext(ctx, "namespace", "os-partner")
		} else {
			return nil, "", fmt.Errorf("load dut topology cmd for partner domain failed, bot-prefix not found")
		}
	}
	return ctx, hostname, nil
}

// loadDutTopologyCommandExecution executes the load dut topology command.
func (ex *InvServiceExecutor) loadDutTopologyCommandExecution(
	ctx context.Context,
	cmd *commands.LoadDutTopologyCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "Load DutTopology")
	defer func() { step.End(err) }()

	ctx, hostname, err := getDUTHostnameAndContextForInventory(ctx, cmd.HostName)
	if err != nil {
		err = errors.Annotate(err, "Get dut hostname for lookup err: ").Err()
	}

	dutTopology, err := ex.GetDUTTopology(ctx, hostname)
	if err != nil {
		err = errors.Annotate(err, "Load dut topology cmd err: ").Err()
	}
	common.WriteProtoToStepLog(ctx, step, dutTopology, "Dut Topology")
	cmd.DutTopology = dutTopology
	return err
}

// invServiceStopCommandExecution executes the invenotry service stop command.
func (ex *InvServiceExecutor) invServiceStopCommandExecution(
	ctx context.Context,
	cmd *commands.InvServiceStopCmd) error {
	var err error
	step, ctx := build.StartStep(ctx, "Inventory service stop")
	defer func() { step.End(err) }()

	err = ex.Stop(ctx)
	if err != nil {
		return errors.Annotate(err, "Stop inventory service cmd err: ").Err()
	}

	return nil
}

// Start establishes a connection to inventory service.
func (ex *InvServiceExecutor) Start(ctx context.Context, invServerAddress string) error {
	// Don't need to connect if an established connection exists
	if ex.InventoryServiceClient != nil {
		return nil
	}

	// Validate address
	if invServerAddress == "" {
		return fmt.Errorf("Inventory service address is empty!")
	}

	// Connect with service
	conn, err := common.ConnectWithService(ctx, invServerAddress)
	if err != nil {
		logging.Infof(
			ctx,
			"error during connecting with inventory server at %s: %s",
			invServerAddress,
			err.Error())
		return err
	}
	ex.GrpcConn = conn
	logging.Infof(ctx, "Connected with inventory service.")

	// Get client
	invClient := labapi.NewInventoryServiceClient(conn)
	if invClient == nil {
		return fmt.Errorf("InventoryServiceClient is nil")
	}

	ex.InventoryServiceClient = invClient

	return nil
}

// Stop closes the established connection to inventory service.
func (ex *InvServiceExecutor) Stop(ctx context.Context) error {
	// Make it safe to closeClient() more than once
	if ex.InventoryServiceClient == nil {
		return nil
	}
	if ex.GrpcConn == nil {
		return fmt.Errorf("Cannot close inventory service. Connection is nil.")
	}

	err := ex.GrpcConn.Close()
	if err != nil {
		return fmt.Errorf("error during closing inventory service connection!")
	}
	ex.InventoryServiceClient = nil
	return err
}

// GetDUTTopology invokes the get dut topology endpoint of inventory service.
func (ex *InvServiceExecutor) GetDUTTopology(
	ctx context.Context,
	hostName string) (*labapi.DutTopology, error) {

	if hostName == "" {
		return nil, fmt.Errorf("Provided hostname is empty!")
	}
	if ex.InventoryServiceClient == nil {
		return nil, fmt.Errorf("InventoryServiceClient is nil!")
	}

	dutid := &labapi.DutTopology_Id{Value: hostName}
	stream, err := ex.InventoryServiceClient.GetDutTopology(
		ctx,
		&labapi.GetDutTopologyRequest{
			Id: dutid,
		},
	)
	if err != nil {
		return nil, errors.Annotate(err, "error during GetDutTopology: ").Err()
	}
	response := &labapi.GetDutTopologyResponse{}
	err = stream.RecvMsg(response)
	if err != nil {
		return nil, errors.Annotate(err, "inventoryServer get response: ").Err()
	}

	ex.DutTopology = response.GetSuccess().GetDutTopology()

	return response.GetSuccess().GetDutTopology(), nil
}
