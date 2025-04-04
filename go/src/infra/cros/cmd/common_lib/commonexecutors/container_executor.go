// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commonexecutors

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/analytics"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/commoncommands"
	"go.chromium.org/infra/cros/cmd/common_lib/containers"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/common_lib/tools/crostoolrunner"
)

// ContainerExecutor represents executor
// for all container related commands.
type ContainerExecutor struct {
	*interfaces.AbstractExecutor

	Ctr              *crostoolrunner.CrosToolRunner
	WaitGroups       []*sync.WaitGroup
	LogChannels      []chan<- bool
	ContainerChannel chan struct {
		string
		interfaces.ContainerInterface
	}
	// KnownNetworks maps the network name to its ID.
	KnownNetworks map[string]string
	isClosed      bool
}

func NewContainerExecutor(ctr *crostoolrunner.CrosToolRunner) *ContainerExecutor {
	absExec := interfaces.NewAbstractExecutor(ContainerExecutorType)
	return &ContainerExecutor{AbstractExecutor: absExec, Ctr: ctr, WaitGroups: []*sync.WaitGroup{}, LogChannels: []chan<- bool{}, ContainerChannel: make(chan struct {
		string
		interfaces.ContainerInterface
	}), isClosed: false, KnownNetworks: map[string]string{}}
}

func (ex *ContainerExecutor) ExecuteCommand(ctx context.Context, cmdInterface interfaces.CommandInterface) error {
	switch cmd := cmdInterface.(type) {
	case *commoncommands.ContainerStartCmd:

		key := fmt.Sprintf("%s-start", cmd.ContainerRequest.DynamicIdentifier)
		analytics.SoftInsertStepWInternalPlan(ctx, cmd.BQClient, &analytics.BqData{Step: key, Status: analytics.Start}, cmd.Req, cmd.BuildState)

		start := time.Now()

		status := analytics.Success
		err := ex.startContainerCommandExecution(ctx, cmd)
		if err != nil {
			status = analytics.Fail
		}
		analytics.SoftInsertStepWInternalPlan(ctx, cmd.BQClient, &analytics.BqData{Step: key, Status: status, Duration: float32(time.Since(start).Seconds())}, cmd.Req, cmd.BuildState)
		return err
	case *commoncommands.ContainerCloseLogsCmd:
		return ex.CloseLogs()
	case *commoncommands.ContainerReadLogsCmd:
		return ex.ReadLogs(ctx)

	default:
		return fmt.Errorf(
			"command type %s, %T, %v is not supported by %s executor type",
			cmd.GetCommandType(),
			cmdInterface,
			cmdInterface,
			ex.GetExecutorType())
	}
}

// startContainerCommandExecution executes the container start command.
func (ex *ContainerExecutor) startContainerCommandExecution(
	ctx context.Context,
	cmd *commoncommands.ContainerStartCmd) error {
	if cmd.SkipStartingContainer {
		// skip starting the container
		logging.Infof(ctx, "skipping starting container %s", cmd.ContainerRequest.DynamicIdentifier)
		return nil
	}
	var err error
	step, ctx := build.StartStep(ctx, fmt.Sprintf("Container Start: %s", cmd.ContainerRequest.DynamicIdentifier))
	defer func() {
		step.End(err)
	}()

	if cmd.ContainerRequest == nil {
		return fmt.Errorf("cannot start container with nil container request")
	}

	common.WriteProtoToStepLog(ctx, step, cmd.ContainerRequest, "container service request")

	containerInstance, endpoint, err := ex.Start(
		ctx,
		cmd.ContainerRequest,
		cmd.ContainerRequest.Container,
		cmd.ContainerImage,
		cmd.BuildState.Build().GetId())

	if err != nil {
		return errors.Annotate(err, "start container cmd err: ").Err()
	}
	cmd.ContainerInstance = containerInstance
	cmd.Endpoint = endpoint

	go func() {
		// Send container for logs to be read.
		ex.ContainerChannel <- struct {
			string
			interfaces.ContainerInterface
		}{cmd.ContainerRequest.DynamicIdentifier, containerInstance}
	}()

	return err
}

func (ex *ContainerExecutor) ReadLogs(ctx context.Context) error {
	stepStarted := make(chan struct{})
	go func() {
		var err error
		step, ctx := build.StartStep(ctx, "Read Container Logs")

		stepStarted <- struct{}{}

		defer func() {
			step.End(err)
		}()

		for recv := range ex.ContainerChannel {
			ex.streamLogAsync(ctx, step, recv.string, recv.ContainerInterface)
		}
	}()

	// Confirm the step has started.
	<-stepStarted
	return nil
}

// Start starts the container.
func (ex *ContainerExecutor) Start(
	ctx context.Context,
	contReq *api.ContainerRequest,
	template *api.Template,
	containerImage string,
	bbID int64) (interfaces.ContainerInterface, *labapi.IpEndpoint, error) {

	if contReq.Network == "" {
		contReq.Network = common.ContainerDefaultNetwork
	}
	if contReq.Network != common.ContainerDefaultNetwork {
		// Assigning id is ineffectual because it gets overwritten before use in
		// each branch.
		var id string
		_, ok := ex.KnownNetworks[contReq.Network]
		if !ok {
			getResp, err := ex.Ctr.GetNetwork(ctx, contReq.Network)
			if err != nil {
				logging.Infof(ctx, err.Error())
			}
			if getResp != nil {
				id = getResp.GetNetwork().GetId()
			} else {
				resp, err := ex.Ctr.CreateNetwork(ctx, contReq.Network)
				if err != nil {
					return nil, nil, errors.Annotate(err, "error processing container: ").Err()
				}
				id = resp.GetNetwork().GetId()
			}
			ex.KnownNetworks[contReq.Network] = id
		}
	}

	containerInstance := containers.NewContainer(
		interfaces.ContainerType(fmt.Sprintf("%s-%d", contReq.DynamicIdentifier, bbID)),
		contReq.DynamicIdentifier,
		contReq.Network,
		containerImage,
		ex.Ctr,
		true)

	// Process container.
	serverAddress, err := containerInstance.ProcessContainer(ctx, template)
	if err != nil {
		return nil, nil, errors.Annotate(err, "error processing container: ").Err()
	}
	endpoint, err := common.GetIPEndpoint(serverAddress)
	if err != nil {
		return nil, nil, err
	}

	return containerInstance, endpoint, nil
}

// streamLog kicks off streaming the containers log and stores its channel and waitgroup.
func (ex *ContainerExecutor) streamLogAsync(ctx context.Context, step *build.Step, identifier string, containerInstance interfaces.ContainerInterface) (wg *sync.WaitGroup) {
	logsLoc, err := containerInstance.GetLogsLocation()
	if err != nil {
		logging.Infof(ctx, "error during getting container log location: %s", err)
	}
	containerLog := step.Log(fmt.Sprintf("%s Log", identifier))

	taskDone, wg, err := common.StreamLogAsync(ctx, logsLoc, containerLog)
	if err != nil {
		logging.Infof(ctx, "Warning: error during reading container log: %s", err)
		return
	}

	ex.LogChannels = append(ex.LogChannels, taskDone)
	ex.WaitGroups = append(ex.WaitGroups, wg)

	// Foil-provision is marked as cros-provision so for now we are adding an
	// extra file stream log for the logcat logs.
	//
	// CLEAN(b/408454320): Remove once adb-logcat is containerized.
	if identifier == "cros-provision_primary" {
		containerLog := step.Log(fmt.Sprintf("%s-logcat Log", identifier))

		logCatTaskDone, logCatWG, err := common.StreamLogcatAsync(ctx, logsLoc, containerLog)
		if err != nil {
			logging.Infof(ctx, "Warning: error during reading container log: %s", err)
			return
		}

		ex.LogChannels = append(ex.LogChannels, logCatTaskDone)
		ex.WaitGroups = append(ex.WaitGroups, logCatWG)

	}

	return
}

// CloseLogs signals to the streaming logs through their channels that they can close.
func (ex *ContainerExecutor) CloseLogs() error {
	// Already closed
	if ex.isClosed {
		return nil
	}
	for _, logChannel := range ex.LogChannels {
		logChannel <- true
	}
	ex.LogChannels = []chan<- bool{}
	for _, waitGroup := range ex.WaitGroups {
		waitGroup.Wait()
	}
	ex.WaitGroups = []*sync.WaitGroup{}

	close(ex.ContainerChannel)
	ex.isClosed = true

	return nil
}
