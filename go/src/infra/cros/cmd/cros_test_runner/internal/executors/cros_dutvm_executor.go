// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executors

import (
	"context"
	"fmt"

	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/commands"
	"go.chromium.org/infra/libs/vmlab"
	vmlabapi "go.chromium.org/infra/libs/vmlab/api"
)

// CrosDutVmExecutor implements the execution of the steps defined in supported
// DutVm-related commands.
type CrosDutVmExecutor struct {
	*interfaces.AbstractExecutor

	// Dependencies for Injection
	Container   interfaces.ContainerInterface
	ImageApi    vmlabapi.ImageApi
	InstanceApi vmlabapi.InstanceApi
}

func NewCrosDutVmExecutor(container interfaces.ContainerInterface) *CrosDutVmExecutor {
	absExec := interfaces.NewAbstractExecutor(CrosDutVmExecutorType)
	return &CrosDutVmExecutor{AbstractExecutor: absExec, Container: container}
}

// getImageApi takes a provider id and returns the image API interface
func (ex *CrosDutVmExecutor) getImageApi() (vmlabapi.ImageApi, error) {
	var err error
	if ex.ImageApi == nil {
		ex.ImageApi, err = vmlab.NewImageApi(vmlabapi.ProviderId_CLOUDSDK)
	}
	return ex.ImageApi, err
}

func (ex *CrosDutVmExecutor) ExecuteCommand(
	ctx context.Context,
	cmdInterface interfaces.CommandInterface) error {

	switch cmd := cmdInterface.(type) {
	case *commands.DutVmGetImageCmd:
		return ex.vmGetImageCommandExecution(ctx, cmd)
	// For MVP, cros-dut container is the same for both HW and VM tests. In future
	// VM tests may have a dedicated cros-dut-vm container.
	case *commands.DutServiceStartCmd:
		return ex.dutStartCommandExecution(ctx, cmd)
	default:
		return fmt.Errorf("Command type %s is not supported by %s executor type!", cmd.GetCommandType(), ex.GetExecutorType())
	}
}

// vmGetImageCommandExecution executes the "Get Dut VM GCE image" step.
func (ex *CrosDutVmExecutor) vmGetImageCommandExecution(
	ctx context.Context,
	cmd *commands.DutVmGetImageCmd) error {
	var err error
	step, ctx := build.StartStep(ctx, "Get Dut VM GCE image")
	defer func() { step.End(err) }()

	imageApi, err := ex.getImageApi()
	if err != nil {
		return err
	}

	buildPath := cmd.Build
	logging.Infof(ctx, "call VMLab to get GCE image based on build path in CftTestRequest: %s, ", buildPath)
	// For MVP, this call may take minutes: If image doesn't exist, GetImage
	// will create an image on the fly.
	gceImage, err := imageApi.GetImage(buildPath, true)

	if gceImage != nil {
		logging.Infof(ctx, "got gceImage from VMLab: %v", gceImage)
	}
	cmd.DutVmGceImage = gceImage
	return err
}

// dutStartCommandExecution executes the "Start Dut service" step. This is
// mostly the same as how to start dut service for HW tests, with the exception
// that the cache server address needs to be switched from localhost to an IP
// address in order to be accessible from the Dut VM.
func (ex *CrosDutVmExecutor) dutStartCommandExecution(
	ctx context.Context,
	cmd *commands.DutServiceStartCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "Dut service start")
	defer func() { step.End(err) }()

	template := &api.Template{
		Container: &api.Template_CrosDut{
			CrosDut: &testapi.CrosDutTemplate{
				CacheServer: cmd.CacheServerAddress,
				DutAddress:  cmd.DutSshAddress,
			},
		},
	}

	// Process container.
	serverAddress, err := ex.Container.ProcessContainer(ctx, template)
	if err != nil {
		return errors.Annotate(err, "error processing container: ").Err()
	}

	// Process dut server address.
	dutServerAddress, err := common.GetIPEndpoint(serverAddress)

	logging.Infof(ctx, "Cros-dut started at address: %v", dutServerAddress)
	cmd.DutServerAddress = dutServerAddress
	return err
}
