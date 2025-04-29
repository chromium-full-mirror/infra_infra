// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executors

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/anypb"

	_go "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	testapi_metadata "go.chromium.org/chromiumos/config/go/test/api/metadata"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/commands"
)

// CrosPublishExecutor represents executor for all cros-publish related commands.
type CrosPublishExecutor struct {
	*interfaces.AbstractExecutor

	Container                 interfaces.ContainerInterface
	GcsPublishServiceClient   testapi.GenericPublishServiceClient
	TkoPublishServiceClient   testapi.GenericPublishServiceClient
	CpconPublishServiceClient testapi.GenericPublishServiceClient
	RdbPublishServiceClient   testapi.GenericPublishServiceClient
	ServerAddress             string
}

func NewCrosPublishExecutor(
	container interfaces.ContainerInterface,
	execType interfaces.ExecutorType) *CrosPublishExecutor {
	if execType != CrosGcsPublishExecutorType &&
		execType != CrosTkoPublishExecutorType &&
		execType != CrosPublishExecutorType &&
		execType != CrosRdbPublishExecutorType {
		return nil
	}
	absExec := interfaces.NewAbstractExecutor(execType)
	return &CrosPublishExecutor{AbstractExecutor: absExec, Container: container}
}

func (ex *CrosPublishExecutor) ExecuteCommand(
	ctx context.Context,
	cmdInterface interfaces.CommandInterface) error {
	var err error

	switch cmd := cmdInterface.(type) {
	case *commands.GcsPublishServiceStartCmd:
		err = ex.gcsPublishStartCommandExecution(ctx, cmd)
	case *commands.GcsPublishUploadCmd:
		err = ex.gcsPublishUploadCommandExecution(ctx, cmd)
	case *commands.RdbPublishServiceStartCmd:
		err = ex.rdbPublishStartCommandExecution(ctx, cmd)
	case *commands.RdbPublishUploadCmd:
		err = ex.rdbPublishUploadCommandExecution(ctx, cmd)
	case *commands.TkoPublishServiceStartCmd:
		err = ex.tkoPublishStartCommandExecution(ctx, cmd)
	case *commands.TkoPublishUploadCmd:
		err = ex.tkoPublishUploadCommandExecution(ctx, cmd)
	case *commands.CpconPublishServiceStartCmd:
		err = ex.cpconPublishStartCommandExecution(ctx, cmd)
	case *commands.CpconPublishUploadCmd:
		err = ex.cpconPublishUploadCommandExecution(ctx, cmd)
	default:
		err = fmt.Errorf(
			"command type %s is not supported by %s executor type",
			cmd.GetCommandType(),
			ex.GetExecutorType())
	}

	if err != nil {
		return &common.TestRunnerError{
			Type: skylab_test_runner.TestRunnerErrorType_PUBLISH,
			Err:  err,
		}
	}
	return nil
}

// -- GCS Commands --

// gcsPublishStartCommandExecution executes the gcs-publish start command.
func (ex *CrosPublishExecutor) gcsPublishStartCommandExecution(
	ctx context.Context,
	cmd *commands.GcsPublishServiceStartCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "gcs-publish service start")
	defer func() { step.End(err) }()

	gcsPublishTemplate := &testapi.CrosPublishTemplate{
		PublishType:   testapi.CrosPublishTemplate_PUBLISH_GCS,
		PublishSrcDir: cmd.GcsPublishSrcDir}
	publishClient, err := ex.Start(
		ctx,
		&api.Template{
			Container: &api.Template_CrosPublish{
				CrosPublish: gcsPublishTemplate,
			},
		},
	)
	logErr := common.WriteContainerLogToStepLog(ctx, ex.Container, step, "gcs-publish log")
	if err != nil {
		return errors.Annotate(err, "Start gcs-publish cmd err: ").Err()
	}
	if logErr != nil {
		logging.Infof(ctx, "error during writing gcs-publish log contents: %s", err)
	}

	ex.GcsPublishServiceClient = publishClient

	return err
}

// gcsPublishUploadCommandExecution executes the gcs-publish upload command.
func (ex *CrosPublishExecutor) gcsPublishUploadCommandExecution(
	ctx context.Context,
	cmd *commands.GcsPublishUploadCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "gcs-publish upload")
	defer func() { step.End(err) }()

	common.AddLinksToStepSummaryMarkdown(step, "", common.GetGcsClickableLink(cmd.GcsURL), "", false)

	// Create request.
	artifactDirPath := &_go.StoragePath{
		HostType: _go.StoragePath_LOCAL,
		Path:     common.GcsPublishTestArtifactsDir}
	gcsPath := &_go.StoragePath{
		HostType: _go.StoragePath_GS,
		Path:     cmd.GcsURL}
	gcsMetadata, err := anypb.New(&testapi.PublishGcsMetadata{
		GcsPath: gcsPath,
		XtsArchiverMetadata: &testapi.XtsArchiverMetadata{
			AlRun:                cmd.IsALRun,
			Product:              cmd.Product,
			Build:                cmd.Build,
			ParentSwarmingTaskId: cmd.ParentSwarmingTaskID,
			ResultsGcsPrefix:     cmd.XTSResultsGCSPrefix,
			ApfeGcsPrefix:        cmd.XTSAPFEGCSPrefix,
		},
		EnableXtsArchiver: cmd.EnableXTSArchiver,
	})
	if err != nil {
		return errors.Annotate(err, "Creating publish gcs metadata err: ").Err()
	}

	gcsPublishReq := &testapi.PublishRequest{
		ArtifactDirPath: artifactDirPath,
		TestResponse:    nil, Metadata: gcsMetadata}

	err = ex.InvokePublishWithAsyncLogging(
		ctx,
		"gcs-publish",
		gcsPublishReq,
		ex.GcsPublishServiceClient,
		step)

	return err
}

// -- RDB Commands --

// rdbPublishStartCommandExecution executes the rdb-publish start command.
func (ex *CrosPublishExecutor) rdbPublishStartCommandExecution(
	ctx context.Context,
	cmd *commands.RdbPublishServiceStartCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "rdb-publish service start")
	defer func() { step.End(err) }()

	rdbPublishTemplate := &testapi.CrosPublishTemplate{
		PublishType:   testapi.CrosPublishTemplate_PUBLISH_RDB,
		PublishSrcDir: cmd.RdbPublishSrcDir}
	publishClient, err := ex.Start(
		ctx,
		&api.Template{
			Container: &api.Template_CrosPublish{
				CrosPublish: rdbPublishTemplate,
			},
		},
	)
	logErr := common.WriteContainerLogToStepLog(ctx, ex.Container, step, "rdb-publish log")
	if err != nil {
		return errors.Annotate(err, "Start rdb-publish cmd err: ").Err()
	}
	if logErr != nil {
		logging.Infof(ctx, "error during writing rdb-publish log contents: %s", err)
	}

	ex.RdbPublishServiceClient = publishClient

	return err
}

// rdbPublishUploadCommandExecution executes the rdb-publish upload command.
func (ex *CrosPublishExecutor) rdbPublishUploadCommandExecution(
	ctx context.Context,
	cmd *commands.RdbPublishUploadCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "rdb-publish upload")
	defer func() { step.End(err) }()

	common.AddLinksToStepSummaryMarkdown(step, cmd.TesthausURL, "", "", false)

	// Create request.
	rdbMetadata, err := anypb.New(&testapi_metadata.PublishRdbMetadata{
		CurrentInvocationId:  cmd.CurrentInvocationId,
		TesthausUrl:          cmd.TesthausURL,
		TestResult:           cmd.TestResultForRdb,
		Sources:              cmd.Sources,
		BaseVariant:          cmd.BaseVariant,
		PostProcessResponses: cmd.PostProcessResponses,
	})
	if err != nil {
		return errors.Annotate(err, "Creating publish rdb metadata err: ").Err()
	}

	// TODO (azrhaman): remove artifactDirPath after unnecessary rdb validation is removed.
	artifactDirPath := &_go.StoragePath{
		HostType: _go.StoragePath_LOCAL,
		Path:     "/tmp/rdb-publish-test-artifacts/",
	}
	rdbPublishReq := &testapi.PublishRequest{
		ArtifactDirPath: artifactDirPath,
		TestResponse:    nil,
		Metadata:        rdbMetadata,
	}
	err = ex.InvokePublishWithAsyncLogging(
		ctx,
		"rdb-publish",
		rdbPublishReq,
		ex.RdbPublishServiceClient,
		step)

	return err
}

// -- TKO Commands --

// tkoPublishStartCommandExecution executes the tko-publish start command.
func (ex *CrosPublishExecutor) tkoPublishStartCommandExecution(
	ctx context.Context,
	cmd *commands.TkoPublishServiceStartCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "tko-publish service start")
	defer func() { step.End(err) }()

	tkoPublishTemplate := &testapi.CrosPublishTemplate{
		PublishType:   testapi.CrosPublishTemplate_PUBLISH_TKO,
		PublishSrcDir: cmd.TkoPublishSrcDir}
	publishClient, err := ex.Start(
		ctx,
		&api.Template{
			Container: &api.Template_CrosPublish{
				CrosPublish: tkoPublishTemplate,
			},
		},
	)
	logErr := common.WriteContainerLogToStepLog(ctx, ex.Container, step, "tko-publish log")
	if err != nil {
		return errors.Annotate(err, "Start tko-publish cmd err: ").Err()
	}
	if logErr != nil {
		logging.Infof(ctx, "error during writing tko-publish log contents: %s", err)
	}

	ex.TkoPublishServiceClient = publishClient

	return err
}

// tkoPublishUploadCommandExecution executes the tko-publish upload command.
func (ex *CrosPublishExecutor) tkoPublishUploadCommandExecution(
	ctx context.Context,
	cmd *commands.TkoPublishUploadCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "tko-publish upload")
	defer func() { step.End(err) }()

	// Create request.
	artifactDirPath := &_go.StoragePath{
		HostType: _go.StoragePath_LOCAL,
		Path:     common.TKOPublishTestArtifactsDir,
	}
	tkoMetadata, err := anypb.New(&testapi.PublishTkoMetadata{
		JobName: cmd.TkoJobName,
	},
	)
	if err != nil {
		return errors.Annotate(err, "Creating publish tko metadata err: ").Err()
	}

	tkoPublishReq := &testapi.PublishRequest{
		ArtifactDirPath: artifactDirPath,
		TestResponse:    nil,
		Metadata:        tkoMetadata,
	}
	err = ex.InvokePublishWithAsyncLogging(
		ctx,
		"tko-publish",
		tkoPublishReq,
		ex.TkoPublishServiceClient,
		step)

	return err
}

// cpconPublishStartCommandExecution executes the cpcon-publish start command.
func (ex *CrosPublishExecutor) cpconPublishStartCommandExecution(
	ctx context.Context,
	cmd *commands.CpconPublishServiceStartCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "cpcon-publish service start")
	defer func() { step.End(err) }()

	var addnEnvironment []string
	if hostname := os.Getenv("DRONE_AGENT_HIVE"); hostname != "" {
		addnEnvironment = append(addnEnvironment, fmt.Sprintf("PUBLISH_HOSTNAME=%s", hostname))
	}
	publishClient, err := ex.Start(
		ctx,
		&api.Template{
			Container: &api.Template_Generic{
				Generic: &api.GenericTemplate{
					BinaryName:        "cpcon-publish",
					DockerArtifactDir: "/tmp/cpcon-publish",
					BinaryArgs:        []string{"server", "--port", "0"},
					AdditionalVolumes: []string{"/creds/service_accounts:/keys/", cmd.CpconPublishSrcDir + ":" + common.CpconPublishTestArtifactsDir},
					Env:               addnEnvironment,
				},
			},
		},
	)
	logErr := common.WriteContainerLogToStepLog(ctx, ex.Container, step, "cpcon-publish log")
	if err != nil {
		return errors.Annotate(err, "Start cpcon-publish cmd err: ").Err()
	}
	if logErr != nil {
		logging.Infof(ctx, "error during writing cpcon-publish log contents: %s", err)
	}

	ex.CpconPublishServiceClient = publishClient

	return err
}

// cpconPublishUploadCommandExecution executes the cpcon-publish upload command.
func (ex *CrosPublishExecutor) cpconPublishUploadCommandExecution(
	ctx context.Context,
	cmd *commands.CpconPublishUploadCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "cpcon-publish upload")
	defer func() { step.End(err) }()

	// Create request.
	artifactDirPath := &_go.StoragePath{
		HostType: _go.StoragePath_LOCAL,
		Path:     common.CpconPublishTestArtifactsDir,
	}
	cpconMetadata, err := anypb.New(&testapi.PublishCpconMetadata{
		GcsPath: &_go.StoragePath{
			HostType: _go.StoragePath_GS,
			Path:     cmd.GcsURL,
		},
		ServiceAccountCredsFilePath: &_go.StoragePath{
			HostType: _go.StoragePath_LOCAL,
			Path:     "/keys/skylab-drone.json",
		},
		Build:                cmd.Build,
		Suite:                cmd.Suite,
		ParentSwarmingTaskId: cmd.ParentSwarmingTaskID,
	},
	)
	if err != nil {
		return errors.Annotate(err, "Creating publish cpcon metadata err: ").Err()
	}

	cpconPublishReq := &testapi.PublishRequest{
		ArtifactDirPath: artifactDirPath,
		TestResponse:    nil,
		Metadata:        cpconMetadata,
	}
	err = ex.InvokePublishWithAsyncLogging(
		ctx,
		"cpcon-publish",
		cpconPublishReq,
		ex.CpconPublishServiceClient,
		step)

	return err
}

// Start starts the cros-publish server.
func (ex *CrosPublishExecutor) Start(
	ctx context.Context,
	template *api.Template) (testapi.GenericPublishServiceClient, error) {
	if template == nil {
		return nil, fmt.Errorf("Cannot start publish service with empty template.")
	}

	// Process container.
	serverAddress, err := ex.Container.ProcessContainer(ctx, template)
	if err != nil {
		return nil, errors.Annotate(err, "error processing container: ").Err()
	}

	ex.ServerAddress = serverAddress

	// Connect with the service.
	conn, err := common.ConnectWithService(ctx, serverAddress)
	if err != nil {
		logging.Infof(
			ctx,
			"error during connecting server at %s: %s",
			serverAddress,
			err.Error())
		return nil, err
	}
	logging.Infof(ctx, "Connected with service.")

	// Create new client.
	publishClient := api.NewGenericPublishServiceClient(conn)
	if publishClient == nil {
		return nil, fmt.Errorf("GenericPublishServiceClient is nil")
	}

	return publishClient, nil
}

// Publish invokes the publish endpoint of cros-publish.
func (ex *CrosPublishExecutor) Publish(
	ctx context.Context,
	publishReq *testapi.PublishRequest,
	publishClient testapi.GenericPublishServiceClient) (*testapi.PublishResponse, error) {
	if publishClient == nil {
		return nil, fmt.Errorf("CrosPublishServiceClient is nil in CrosPublishExecutor")
	}
	if publishReq == nil {
		return nil, fmt.Errorf("Cannot publish results with empty publish request.")
	}

	publishOp, err := publishClient.Publish(ctx, publishReq, grpc.MaxCallRecvMsgSize(common.MaxPublishMsgSize), grpc.MaxCallSendMsgSize(common.MaxPublishMsgSize))
	if err != nil {
		return nil, errors.Annotate(err, "publish failure: ").Err()
	}

	// TODO: Fix this to use common.WaitLro
	opResp, err := common.ProcessDoneLro(ctx, publishOp)
	if err != nil {
		return nil, errors.Annotate(err, "publish lro failure: ").Err()
	}

	publishResp := &testapi.PublishResponse{}
	if err := opResp.UnmarshalTo(publishResp); err != nil {
		logging.Infof(ctx, "publish lro response unmarshalling failed: %s", err.Error())
		return nil, errors.Annotate(err, "publish lro response unmarshalling failed: ").Err()
	}

	return publishResp, nil
}

// InvokePublishWithAsyncLogging invokes publish endpoint of the service with async logging.
func (ex *CrosPublishExecutor) InvokePublishWithAsyncLogging(
	ctx context.Context,
	publishType string,
	request *api.PublishRequest,
	client api.GenericPublishServiceClient,
	step *build.Step) error {
	if request == nil {
		return fmt.Errorf("Cannot publish result for %s with empty publish request.", publishType)
	}
	if client == nil {
		return fmt.Errorf("Cannot publish result for %s with no established publish client.", publishType)
	}
	if ex.Container == nil {
		return fmt.Errorf("Cannot publish result for %s with empty publish container.", publishType)
	}

	// Write request.
	common.WriteProtoToStepLog(ctx, step, request, fmt.Sprintf("%s request", publishType))

	// Get container logs location.
	logsLoc, err := ex.Container.GetLogsLocation()
	if err != nil {
		logging.Infof(ctx, "error during getting %s container log location: %s", publishType, err)
		return err
	}

	// Stream logs async to build step.
	containerLog := step.Log(fmt.Sprintf("%s Log", publishType))
	taskDone, wg, err := common.StreamLogAsync(ctx, logsLoc, containerLog)
	if err != nil {
		logging.Infof(ctx, "Warning: error during reading %s container log: %s", publishType, err)
	}

	// Publish.
	resp, err := ex.Publish(ctx, request, client)
	if taskDone != nil {
		taskDone <- true // Notify logging process that main task is done
	}
	wg.Wait() // wait for the logging to complete

	if err != nil {
		err = errors.Annotate(err, fmt.Sprintf("%s publish cmd err: ", publishType)).Err()
	}

	common.WriteProtoToStepLog(ctx, step, resp, fmt.Sprintf("%s response", publishType))

	return err
}
