// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executors

import (
	"context"
	"fmt"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/commands"
)

// AndroidDutExecutor represents executor for all android-dut related commands.
type AndroidDutExecutor struct {
	*interfaces.AbstractExecutor

	NamePrefix              string
	Container               interfaces.ContainerInterface
	AndroidDutServiceClient testapi.DutServiceClient
	AndroidDutServerAddress *labapi.IpEndpoint
	ServerAddress           string
}

func NewAndroidDutExecutor(container interfaces.ContainerInterface) *AndroidDutExecutor {
	absExec := interfaces.NewAbstractExecutor(AndroidDutExecutorType)
	return &AndroidDutExecutor{AbstractExecutor: absExec, Container: container}
}

func (ex *AndroidDutExecutor) ExecuteCommand(
	ctx context.Context,
	cmdInterface interfaces.CommandInterface) error {

	switch cmd := cmdInterface.(type) {
	case *commands.AndroidCompanionDutServiceStartCmd:
		return ex.androidDutStartCommandExecution(ctx, cmd)
	default:
		return fmt.Errorf("Command type %s is not supported by %s executor type!", cmd.GetCommandType(), ex.GetExecutorType())
	}
}

// androidDutStartCommandExecution executes the dut start command.
func (ex *AndroidDutExecutor) androidDutStartCommandExecution(
	ctx context.Context,
	cmd *commands.AndroidCompanionDutServiceStartCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "Android dut service start")
	defer func() { step.End(err) }()

	err = ex.Start(ctx, cmd.CacheServerAddress, cmd.AndroidDutSshAddress)
	logErr := common.WriteContainerLogToStepLog(ctx, ex.Container, step, "android-dut log")
	if err != nil {
		return errors.Annotate(err, "Start android dut cmd err: ").Err()
	}
	if logErr != nil {
		logging.Infof(ctx, "error during writing android-dut log contents: %s", err)
	}

	cmd.AndroidDutServerAddress = ex.AndroidDutServerAddress

	return err
}

// Start starts the android-dut server.
func (ex *AndroidDutExecutor) Start(
	ctx context.Context,
	cacheServerAddress *labapi.IpEndpoint,
	androidDutSshAddress *labapi.IpEndpoint) error {

	if cacheServerAddress == nil {
		return fmt.Errorf("Cannot start android-dut service with nil cache server address.")
	}

	if androidDutSshAddress == nil {
		return fmt.Errorf("Cannot start android-dut service with nil android dut ssh address.")
	}

	dutTemplate := &testapi.CrosDutTemplate{
		CacheServer: cacheServerAddress,
		DutAddress:  androidDutSshAddress}
	template := &testapi.Template{
		Container: &testapi.Template_CrosDut{
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
		logging.Infof(ctx, "error during connecting with android dut server at %s: %s", serverAddress, err.Error())
		return err
	}
	logging.Infof(ctx, "Connected with android dut service.")

	// Process dut server address.
	ex.AndroidDutServerAddress, err = common.GetIPEndpoint(serverAddress)
	if err != nil {
		return errors.Annotate(err, "error while creating ip endpoint from server address: ").Err()
	}

	// Create new client.
	dutClient := testapi.NewDutServiceClient(conn)
	if dutClient == nil {
		return fmt.Errorf("AndroidDutServiceClient is nil")
	}

	ex.AndroidDutServiceClient = dutClient

	return nil
}
