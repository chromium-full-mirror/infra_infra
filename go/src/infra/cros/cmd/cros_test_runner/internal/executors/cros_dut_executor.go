// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executors

import (
	"context"
	"fmt"

	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/commands"
)

// CrosDutExecutor represents executor for all cros-dut related commands.
type CrosDutExecutor struct {
	*interfaces.AbstractExecutor

	NamePrefix           string
	Container            interfaces.ContainerInterface
	CrosDutServiceClient testapi.DutServiceClient
	DutServerAddress     *labapi.IpEndpoint
	ServerAddress        string
}

func NewCrosDutExecutor(container interfaces.ContainerInterface) *CrosDutExecutor {
	absExec := interfaces.NewAbstractExecutor(CrosDutExecutorType)
	return &CrosDutExecutor{AbstractExecutor: absExec, Container: container}
}

func (ex *CrosDutExecutor) ExecuteCommand(
	ctx context.Context,
	cmdInterface interfaces.CommandInterface) error {

	switch cmd := cmdInterface.(type) {
	case *commands.DutServiceStartCmd:
		return ex.dutStartCommandExecution(ctx, cmd)
	default:
		return fmt.Errorf("Command type %s is not supported by %s executor type!", cmd.GetCommandType(), ex.GetExecutorType())
	}
}

// dutStartCommandExecution executes the dut start command.
func (ex *CrosDutExecutor) dutStartCommandExecution(
	ctx context.Context,
	cmd *commands.DutServiceStartCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "Dut service start")
	defer func() { step.End(err) }()

	err = ex.Start(ctx, cmd.CacheServerAddress, cmd.DutSshAddress)
	logErr := common.WriteContainerLogToStepLog(ctx, ex.Container, step, "cros-dut log")
	if err != nil {
		return errors.Annotate(err, "Start dut cmd err: ").Err()
	}
	if logErr != nil {
		logging.Infof(ctx, "error during writing cros-dut log contents: %s", err)
	}

	cmd.DutServerAddress = ex.DutServerAddress

	return err
}

// Start starts the cros-dut server.
func (ex *CrosDutExecutor) Start(
	ctx context.Context,
	cacheServerAddress *labapi.IpEndpoint,
	dutSshAddress *labapi.IpEndpoint) error {

	if cacheServerAddress == nil {
		return fmt.Errorf("Cannot start dut service with nil cache server address.")
	}

	if dutSshAddress == nil {
		return fmt.Errorf("Cannot start dut service with nil dut ssh address.")
	}

	dutTemplate := &testapi.CrosDutTemplate{
		CacheServer: cacheServerAddress,
		DutAddress:  dutSshAddress}
	template := &api.Template{
		Container: &api.Template_CrosDut{
			CrosDut: dutTemplate,
		},
	}

	// Process container.
	serverAddress, err := ex.Container.ProcessContainer(ctx, template)
	if err != nil {
		return errors.Annotate(err, "error processing container: ").Err()
	}
	ex.ServerAddress = serverAddress

	// Connect with the service.
	conn, err := common.ConnectWithService(ctx, serverAddress)
	if err != nil {
		logging.Infof(ctx, "error during connecting with dut server at %s: %s", serverAddress, err.Error())
		return err
	}
	logging.Infof(ctx, "Connected with dut service.")

	// Process dut server address.
	ex.DutServerAddress, err = common.GetIPEndpoint(serverAddress)
	if err != nil {
		return errors.Annotate(err, "error while creating ip endpoint from server address: ").Err()
	}

	// Create new client.
	dutClient := api.NewDutServiceClient(conn)
	if dutClient == nil {
		return fmt.Errorf("DutServiceClient is nil")
	}

	ex.CrosDutServiceClient = dutClient

	return nil
}
