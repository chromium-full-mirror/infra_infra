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
)

// CacheServerExecutor represents executor for all cache-server related commands.
type CacheServerExecutor struct {
	*interfaces.AbstractExecutor

	Container     interfaces.ContainerInterface
	ServerAddress string
}

func NewCacheServerExecutor(container interfaces.ContainerInterface) *CacheServerExecutor {
	absExec := interfaces.NewAbstractExecutor(CacheServerExecutorType)
	return &CacheServerExecutor{AbstractExecutor: absExec, Container: container}
}

func (ex *CacheServerExecutor) ExecuteCommand(
	ctx context.Context,
	cmdInterface interfaces.CommandInterface) error {

	switch cmd := cmdInterface.(type) {
	case *commands.CacheServerStartCmd:
		return ex.cacheServerStartCommandExecution(ctx, cmd)
	case *commands.DutVmCacheServerStartCmd:
		return ex.vmCacheServerStartCommandExecution(ctx, cmd)
	default:
		return fmt.Errorf("Command type %s is not supported by %s executor type!", cmd.GetCommandType(), ex.GetExecutorType())
	}
}

// cacheServerStartCommandExecution executes the cache server start command.
func (ex *CacheServerExecutor) cacheServerStartCommandExecution(
	ctx context.Context,
	cmd *commands.CacheServerStartCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "Cache server start")
	defer func() { step.End(err) }()

	err = ex.Start(ctx)
	if err != nil {
		return errors.Annotate(err, "Start cache server cmd err: ").Err()
	}

	cmd.CacheServerAddress, err = common.GetIPEndpoint(ex.ServerAddress)

	return err
}

// Start starts the cache server.
func (ex *CacheServerExecutor) Start(
	ctx context.Context) error {

	cacheServerTemplate := &testapi.CacheServerTemplate{}
	template := &api.Template{
		Container: &api.Template_CacheServer{
			CacheServer: cacheServerTemplate,
		},
	}
	serverAddress, err := ex.Container.ProcessContainer(ctx, template)
	if err != nil {
		return errors.Annotate(err, "error processing container: ").Err()
	}
	ex.ServerAddress = serverAddress

	return nil
}

// vmCacheServerStartCommandExecution executes the "VM cache server start" step.
func (ex *CacheServerExecutor) vmCacheServerStartCommandExecution(
	ctx context.Context,
	cmd *commands.DutVmCacheServerStartCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "VM cache server start")
	defer func() { step.End(err) }()

	logging.Infof(ctx, "Starting cache server.")

	csTemplate := &testapi.CacheServerTemplate{
		ApplicationDefaultCredentials: &testapi.CacheServerTemplate_ServiceAccountKeyfile{
			ServiceAccountKeyfile: common.VMLabDockerKeyFileLocation,
		}}
	template := &api.Template{
		Container: &api.Template_CacheServer{
			CacheServer: csTemplate,
		},
	}

	// Process container.
	serverAddress, err := ex.Container.ProcessContainer(ctx, template)
	if err != nil {
		return errors.Annotate(err, "error processing container: ").Err()
	}

	// Process dut server address.
	cacheServerAddress, err := common.GetIPEndpoint(serverAddress)
	if err != nil {
		return errors.Annotate(err, "error while creating ip endpoint from server address: ").Err()
	}

	// Cacheserver need to be called from the DUT, therefore we need the host IP
	if cacheServerAddress.Address == "localhost" {
		hostIp, err := common.GetHostIP()
		if err != nil {
			return errors.Annotate(err, "error while getting host ip address: ").Err()
		}
		cacheServerAddress.Address = hostIp
	}

	logging.Infof(ctx, "Cacheserver started at address: %v", cacheServerAddress)
	cmd.CacheServerAddress = cacheServerAddress
	return err
}
