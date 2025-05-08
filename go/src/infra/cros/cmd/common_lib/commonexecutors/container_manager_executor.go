// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commonexecutors

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/commoncommands"
	"go.chromium.org/infra/cros/cmd/common_lib/commontypes"
	"go.chromium.org/infra/cros/cmd/common_lib/containers"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/common_lib/tools/crostoolrunner"
)

// ContainerManagerExecutor represents executor
// for all container management related commands.
type ContainerManagerExecutor struct {
	*interfaces.AbstractExecutor

	// Deps
	Ctr *crostoolrunner.CrosToolRunner

	// Data
	RunningContainers     sync.Map
	RunningProducersCount atomic.Int64

	// KnownNetworks maps the network name to its ID.
	KnownNetworks map[string]string
}

func NewContainerManagerExecutor(ctr *crostoolrunner.CrosToolRunner) *ContainerManagerExecutor {
	absExec := interfaces.NewAbstractExecutor(ContainerManagerExecutorType)
	return &ContainerManagerExecutor{AbstractExecutor: absExec, Ctr: ctr, RunningContainers: sync.Map{}, KnownNetworks: map[string]string{}, RunningProducersCount: atomic.Int64{}}
}

func (ex *ContainerManagerExecutor) ExecuteCommand(ctx context.Context, cmdInterface interfaces.CommandInterface) error {
	switch cmd := cmdInterface.(type) {
	case *commoncommands.ContainerManagerStartCmd:
		return ex.StartManager(ctx, cmd)
	case *commoncommands.ContainerManagerStopCmd:
		return ex.StopManager(ctx, cmd)
	default:
		return fmt.Errorf(
			"Command type %s, %T, %v is not supported by %s executor type!",
			cmd.GetCommandType(),
			cmdInterface,
			cmdInterface,
			ex.GetExecutorType())
	}
}

// StopManager indicates to the request channel that a producer
// is no longer using the container manager.
func (ex *ContainerManagerExecutor) StopManager(ctx context.Context, cmd *commoncommands.ContainerManagerStopCmd) error {
	cmd.RequestChannel <- commontypes.ContainerManagementRequest{
		ContainerInstruction: commontypes.FinishedUsingManager,
	}

	return nil
}

// tryCloseManager decrements the running producer count
// and if it reaches zero, performs the closing logic.
func (ex *ContainerManagerExecutor) tryCloseManager(ctx context.Context, requestChannel chan commontypes.ContainerManagementRequest) {
	if ex.RunningProducersCount.Add(-1) == 0 {
		logging.Infof(ctx, "Closing manager as there are no more running producers")
		if requestChannel != nil {
			close(requestChannel)
		}
		common.LogWarningIfErr(ctx, ex.Ctr.StopCTRServer(ctx))
	} else {
		logging.Infof(ctx, "Could not close manager as there are still %d producers running", ex.RunningProducersCount.Load())
	}
}

// StartManager starts the manager process.
func (ex *ContainerManagerExecutor) StartManager(ctx context.Context, cmd *commoncommands.ContainerManagerStartCmd) error {
	stepStarted := make(chan struct{})
	go func(requestChannel chan commontypes.ContainerManagementRequest) {
		var err error
		step, ctx := build.StartStep(ctx, "Container Manager")
		defer func() {
			step.End(err)
		}()
		ex.RunningProducersCount.Store(int64(cmd.RequestProducerCount))
		logging.Infof(ctx, "Starting container manager with %d number of request producers", ex.RunningProducersCount.Load())
		stepStarted <- struct{}{}

		ex.ProcessRequests(ctx, requestChannel)
	}(cmd.RequestChannel)

	<-stepStarted
	return nil
}

// interceptRequest allows capturing metadata about the request channel
// such as how many pending requests there are for each type.
// This info can inform functions such as closing a container as to
// whether there are any current pending requests to use that container.
func (ex *ContainerManagerExecutor) interceptRequest(ctx context.Context, request commontypes.ContainerManagementRequest) {
	if request.Container != nil && request.Container.GetNetwork() == "" {
		request.Container.Network = common.ContainerDefaultNetwork
	}
	hashKey := GetContainerKeyHash(request.Container)
	logging.Infof(ctx, "Intercepted: %s", hashKey)

	switch request.ContainerInstruction {
	case commontypes.ProvideContainer:
		initialContainerInfo := &commontypes.RunningContainerInfo{
			ContainerInstance: nil,
			Address:           nil,
			CountUsers:        atomic.Int64{},
		}
		infoAny, _ := ex.RunningContainers.LoadOrStore(hashKey, initialContainerInfo)
		info := infoAny.(*commontypes.RunningContainerInfo)
		info.CountUsers.Add(1)
	default:
	}
}

// ProcessRequests performs handling logic for each request
// coming in from the channel.
func (ex *ContainerManagerExecutor) ProcessRequests(ctx context.Context, requestChannel chan commontypes.ContainerManagementRequest) {
	// Launch the routine for processing through requests.
	forwardingChannel := make(chan commontypes.ContainerManagementRequest)
	go func(requestChannel, forwardingChannel chan commontypes.ContainerManagementRequest) {
		for request := range forwardingChannel {
			if request.Container != nil && request.Container.GetNetwork() == "" {
				request.Container.Network = common.ContainerDefaultNetwork
			}
			hashKey := GetContainerKeyHash(request.Container)

			switch request.ContainerInstruction {
			case commontypes.ProvideContainer:
				logging.Infof(ctx, "%s provide", hashKey)
				common.LogWarningIfErr(ctx, ex.provideContainer(ctx, &request, hashKey))
			case commontypes.FinishedUsingContainer:
				logging.Infof(ctx, "%s finished", hashKey)
				ex.markContainerFinished(ctx, &request, hashKey)
			case commontypes.FinishedUsingManager:
				ex.tryCloseManager(ctx, forwardingChannel)
			}
		}

		close(requestChannel)
	}(requestChannel, forwardingChannel)

	// Intercept each request for middleman logic processing.
	for request := range requestChannel {
		ex.interceptRequest(ctx, request)
		go func() { forwardingChannel <- request }()
	}
}

// markContainerFinished decrements the amount of users for a running container
// and if it reaches zero, close the container.
func (ex *ContainerManagerExecutor) markContainerFinished(ctx context.Context, request *commontypes.ContainerManagementRequest, hashKey string) {
	defer func() {
		request.ResponseChannel <- nil
	}()

	// Container should exist.
	infoAny, ok := ex.RunningContainers.Load(hashKey)
	if !ok {
		logging.Infof(ctx, "Warning: container %s could not be found to mark as finished", request.Container.DynamicIdentifier)
		return
	}
	info := infoAny.(*commontypes.RunningContainerInfo)
	info.CountUsers.Add(-1)
	if info.CountUsers.Load() == 0 {
		var err error
		step, ctx := build.StartStep(ctx, fmt.Sprintf("Closing %s", request.Container.GetDynamicIdentifier()))
		defer func() {
			step.End(err)
		}()

		// Close the container
		err = (*info.ContainerInstance).StopContainer(ctx)
		common.LogWarningIfErr(ctx, err)
		info.Address = nil
		info.ContainerInstance = nil
	}
}

// provideContainer checks if the container is already running and if so
// returns the address of the running container.
// If not, starts the container and returns the address.
func (ex *ContainerManagerExecutor) provideContainer(ctx context.Context, request *commontypes.ContainerManagementRequest, hashKey string) error {
	var address *labapi.IpEndpoint
	var logLocation string
	defer func() {
		request.ResponseChannel <- &commontypes.ContainerManagementResponse{
			Address:     address,
			LogLocation: logLocation,
		}
	}()

	// Expected to load, should have been initialized in intercept.
	logging.Infof(ctx, "Loading %s", hashKey)
	infoAny, ok := ex.RunningContainers.Load(hashKey)
	if !ok {
		return fmt.Errorf("Could not find container info for %s", hashKey)
	}

	info, ok := infoAny.(*commontypes.RunningContainerInfo)
	if !ok {
		return fmt.Errorf("Failed to cast %s as *commontypes.RunningContainerInfo", hashKey)
	}

	logging.Infof(ctx, "Found %s", hashKey)
	if info.Address != nil {
		address = info.Address
		logLocation = info.LogLocation
		logging.Infof(ctx, "Container %s already running, return address %s", hashKey, address.String())
		return nil
	}

	// Create and store container.
	var err error
	step, ctx := build.StartStep(ctx, fmt.Sprintf("Start %s", request.Container.GetDynamicIdentifier()))
	defer func() {
		step.End(err)
	}()

	// Log container start request.
	common.WriteProtoToStepLog(ctx, step, request.Container, "Container Request")

	// Start the container.
	containerInstance, address, err := ex.startContainer(ctx, request.Container)
	info.ContainerInstance = containerInstance
	info.Address = address
	if err != nil {
		logging.Infof(ctx, err.Error())
		return err
	}
	logLocation, err = (*containerInstance).GetLogsLocation()
	if err != nil {
		logging.Infof(ctx, err.Error())
		return err
	}
	info.LogLocation = logLocation
	return nil
}

// startContainer starts the container being requested.
func (ex *ContainerManagerExecutor) startContainer(
	ctx context.Context,
	contReq *api.ContainerRequest) (*interfaces.ContainerInterface, *labapi.IpEndpoint, error) {

	template := contReq.GetContainer()

	logging.Infof(ctx, "Checking Network")
	if contReq.Network != common.ContainerDefaultNetwork {
		logging.Infof(ctx, "Updating Network")
		id, ok := ex.KnownNetworks[contReq.Network]
		if !ok {
			network, err := ex.Ctr.GetOrCreateNetwork(ctx, contReq.Network)
			id = network.GetId()
			if err != nil {
				return nil, nil, errors.Annotate(err, "error getting/creating network: ").Err()
			}
			ex.KnownNetworks[contReq.Network] = id
		}
	}

	logging.Infof(ctx, "Create new container")
	containerInstance := containers.NewContainer(
		// This value needs to be different for each test-finder instance that is build related.
		interfaces.ContainerType(contReq.DynamicIdentifier),
		contReq.DynamicIdentifier,
		contReq.Network,
		contReq.ContainerImagePath,
		ex.Ctr,
		true,
	)

	logging.Infof(ctx, "Process container")
	serverAddress, err := containerInstance.ProcessContainer(ctx, template)
	if err != nil {
		return nil, nil, errors.Annotate(err, "error processing container:").Err()
	}
	endpoint, err := common.GetIPEndpoint(serverAddress)
	if err != nil {
		return nil, nil, err
	}

	return &containerInstance, endpoint, nil
}

// GetContainerKeyHash generates a string for a container request.
func GetContainerKeyHash(container *api.ContainerRequest) string {
	if container == nil {
		return "Manager"
	}
	hashKey := strings.Join(
		[]string{
			container.GetDynamicIdentifier(),
			container.GetContainerImageKey(),
			container.GetContainerImagePath(),
			container.GetNetwork(),
		},
		"_",
	)
	if container.GetContainer().GetGeneric() != nil {
		generic := container.GetContainer().GetGeneric()
		hashKey += generic.GetBinaryName()
		for _, binaryArg := range generic.GetBinaryArgs() {
			hashKey += binaryArg
		}
	}
	return hashKey
}
