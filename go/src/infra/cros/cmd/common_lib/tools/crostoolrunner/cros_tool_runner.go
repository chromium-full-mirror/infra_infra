// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package crostoolrunner

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"google.golang.org/grpc"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
)

const (
	Oauth2Username = "oauth2accesstoken"
	Oauth2Password = "$(gcloud auth print-access-token)"
	ImageRegistry  = "us-docker.pkg.dev"
)

// BackoffFunc exposed for test override.
var BackoffFunc = backoffFunc

// CrosToolRunner represents the tool that enables communicating with CTRv2.
type CrosToolRunner struct {
	CtrCipdInfo

	CtrClient         testapi.CrosToolRunnerContainerServiceClient
	EnvVarsToPreserve []string

	wg              *sync.WaitGroup
	isServerRunning bool
	NoSudo          bool
}

// -- CTR server related methods --

// StartCTRServer starts the server and exports service metadata to
// already created temp dir.
func (ctr *CrosToolRunner) StartCTRServer(ctx context.Context) error {
	var err error
	step, ctx := build.StartStep(ctx, fmt.Sprintf("CrosToolRunner: Start cros-tool-runner server"))
	defer func() { step.End(err) }()

	// Initialize if not already initialized.
	if !ctr.IsInitialized {
		if err = ctr.Initialize(ctx); err != nil {
			return errors.Annotate(err, "CTR initialization error: ").Err()
		}
	}

	// Start the server preserving provided environment vars.
	writer := step.Log("CTR Stdout")
	logging.Infof(ctx, "Starting CTR server...")
	cmd := ctr.sudoCommand(ctx, ctr.EnvVarsToPreserve, ctr.CtrPath, "server", "--port", "0", "--export-metadata", ctr.CtrTempDirLoc)
	err = common.RunCommandWithCustomWriter(ctx, cmd, "ctr-start", writer)
	if err != nil {
		if strings.Contains(err.Error(), common.CtrCancelingCmdErrString) {
			logging.Infof(ctx, "Warning: non-critical error during ctr-start command: %s", err.Error())
			err = nil
		} else {
			logging.Infof(ctx, "error during ctr-start command: %s", err.Error())
			return errors.Annotate(err, "error during ctr-start command: ").Err()
		}
	}

	return nil
}

// StartCTRServerAsync starts the server asynchronously.
// This is necessary as we would want the server to run in background.
func (ctr *CrosToolRunner) StartCTRServerAsync(ctx context.Context) (err error) {
	// Do not start a server if an existing server is running.
	if ctr.wg != nil {
		return fmt.Errorf("Stop existing server connection before starting a new one!")
	}

	ctr.wg = &sync.WaitGroup{}
	ctr.wg.Add(1)

	go func() {
		innerErr := ctr.StartCTRServer(ctx)
		if innerErr != nil {
			logging.Infof(ctx, "error during starting ctr server: %s", innerErr.Error())
		}
		ctr.wg.Done()
	}()

	return err
}

// GetServerAddressFromServiceMetadata waits for the service metadata file and
// gets ctr server address from it.
func (ctr *CrosToolRunner) GetServerAddressFromServiceMetadata(ctx context.Context) (string, error) {
	if ctr.CtrTempDirLoc == "" {
		return "", fmt.Errorf("Cannot retrieve ctr server address with empty temp dir.")
	}

	var err error
	step, ctx := build.StartStep(ctx, fmt.Sprintf("CrosToolRunner: Retrieve service metadata"))
	defer func() { step.End(err) }()

	metaFilePath := path.Join(ctr.CtrTempDirLoc, common.CftServiceMetadataFileName)

	metadataLog := step.Log("Ctr service metadata")
	serverAddress, err := common.GetCftLocalServerAddress(ctx, metaFilePath, metadataLog)
	if err != nil {
		return "", errors.Annotate(err, "Error during getting ctr server address: ").Err()
	}

	return serverAddress, nil
}

// ConnectToCTRServer connects to the CTR server in provided server address.
func (ctr *CrosToolRunner) ConnectToCTRServer(
	ctx context.Context,
	serverAddress string) (testapi.CrosToolRunnerContainerServiceClient, error) {
	var err error
	step, ctx := build.StartStep(ctx, fmt.Sprintf("CrosToolRunner: Connect to cros-tool-runner server"))
	defer func() { step.End(err) }()

	if serverAddress == "" {
		return nil, fmt.Errorf("Ctr service connection is not possible without server address.")
	}

	if ctr.CtrClient != nil {
		logging.Infof(ctx, "Skipping connecting to server is an existing server is already running.")
		return ctr.CtrClient, nil
	}

	// Connect with service
	conn, err := common.ConnectWithService(ctx, serverAddress)
	if err != nil {
		return nil, errors.Annotate(err, "Error during connecting to ctr server: ").Err()
	}

	// Successful connection confirms that the server is running.
	ctr.isServerRunning = true
	logging.Infof(ctx, "Successfully connected to CTR service!")

	// Construct CTR client
	ctrClient := testapi.NewCrosToolRunnerContainerServiceClient(conn)
	if ctrClient == nil {
		return nil, fmt.Errorf("CrosToolRunnerContainerServiceClient is nil")
	}

	ctr.CtrClient = ctrClient
	return ctrClient, nil
}

// StopCTRServer stops currently running CTR server.
func (ctr *CrosToolRunner) StopCTRServer(ctx context.Context) error {
	var err error
	step, ctx := build.StartStep(ctx, fmt.Sprintf("CrosToolRunner: Stop cros-tool-runner server"))
	defer func() { step.End(err) }()

	if !ctr.isServerRunning {
		logging.Infof(ctx, "Warning: CTR server is not running so nothing to stop. Exiting stop command.")
		return nil
	}

	if ctr.CtrClient == nil {
		return fmt.Errorf("Cannot stop CTR server when there is no established client.")
	}

	// Stop CTR server
	req := testapi.ShutdownRequest{}
	common.WriteProtoToStepLog(ctx, step, &req, "StopServerRequest")
	resp, err := ctr.CtrClient.Shutdown(ctx, &req, grpc.EmptyCallOption{})
	if err != nil {
		return errors.Annotate(err, "error during stopping ctr server").Err()
	}
	common.WriteProtoToStepLog(ctx, step, resp, "StopServerResponse")

	ctr.isServerRunning = false
	logging.Infof(ctx, "Successfully stopped CTR server!")

	if ctr.wg != nil {
		logging.Infof(ctx, "Waiting for CTR start command step to exit...")
		ctr.wg.Wait()
		logging.Infof(ctx, "Waiting is over.")
		ctr.wg = nil
	}

	return nil
}

// -- Container commands --

// StartContainer starts a non-templated container using ctr client.
func (ctr *CrosToolRunner) StartContainer(
	ctx context.Context,
	startContainerReq *testapi.StartContainerRequest) (*testapi.StartContainerResponse, error) {
	if startContainerReq == nil {
		return nil, fmt.Errorf("start container request cannot be nil for start container command.")
	}

	var err error
	step, ctx := build.StartStep(ctx, fmt.Sprintf("CrosToolRunner: Start container %s", startContainerReq.Name))
	defer func() { step.End(err) }()

	if ctr.CtrClient == nil {
		return nil, fmt.Errorf("Ctr client not found. Please start the server if not done already.")
	}

	common.LogExecutionDetails(ctx, step, startContainerReq.StartCommand)
	common.WriteProtoToStepLog(ctx, step, startContainerReq, "StartContainerRequest")

	// Start container
	resp, err := ctr.CtrClient.StartContainer(ctx, startContainerReq, grpc.EmptyCallOption{})
	if err != nil {
		return nil, errors.Annotate(err, "error during starting container").Err()
	}

	common.WriteProtoToStepLog(ctx, step, resp, "StartContainerResponse")
	logging.Infof(ctx, "Successfully started container %s!", startContainerReq.Name)
	return resp, nil
}

// StartContainer starts a templated container using ctr client.
func (ctr *CrosToolRunner) StartTemplatedContainer(
	ctx context.Context,
	startContainerReq *testapi.StartTemplatedContainerRequest) (*testapi.StartContainerResponse, error) {

	if startContainerReq == nil {
		return nil, fmt.Errorf("start templated container request cannot be nil for start templated container command.")
	}

	var err error
	step, ctx := build.StartStep(ctx, fmt.Sprintf("CrosToolRunner: Start templated container %s", startContainerReq.Name))
	defer func() { step.End(err) }()

	if ctr.CtrClient == nil {
		return nil, fmt.Errorf("Ctr client not found. Please start the server if not done already.")
	}

	// Start the container
	common.WriteProtoToStepLog(ctx, step, startContainerReq, "StartTemplatedContainerRequest")
	resp, err := ctr.CtrClient.StartTemplatedContainer(ctx, startContainerReq, grpc.EmptyCallOption{})
	if err != nil {
		return nil, errors.Annotate(err, "error during starting templated container").Err()
	}

	common.WriteProtoToStepLog(ctx, step, resp, "StartTemplatedContainerResponse")
	logging.Infof(ctx, "Successfully started templated container %s!", startContainerReq.Name)

	return resp, nil
}

// StopContainer stops the container with provided name.
func (ctr *CrosToolRunner) StopContainer(ctx context.Context, containerName string) error {
	if containerName == "" {
		return fmt.Errorf("Cannot stop container with empty container name.")
	}

	var err error
	step, ctx := build.StartStep(ctx, fmt.Sprintf("Docker: Stop container %s", containerName))
	defer func() { step.End(err) }()

	// Stop container
	cmd := ctr.sudoCommand(ctx, nil, "docker", "stop", containerName)
	common.LogExecutionDetails(ctx, step, cmd.Args)
	_, _, err = common.RunCommand(ctx, cmd, "docker-stop-container", nil, false)
	if err != nil {
		return fmt.Errorf("error during stopping container %s: %s", containerName, err.Error())
	}

	logging.Infof(ctx, "Successfully stopped container %s!", containerName)
	return nil
}

// CreateNetwork creates a bridge network with the provided name.
func (ctr *CrosToolRunner) CreateNetwork(ctx context.Context, networkName string) (*testapi.CreateNetworkResponse, error) {
	if networkName == "" {
		return nil, fmt.Errorf("Cannot create network with empty network name.")
	}
	if networkName == "host" {
		return nil, fmt.Errorf("Cannot overwrite default host network.")
	}

	var err error
	step, ctx := build.StartStep(ctx, fmt.Sprintf("Docker: Create network %s", networkName))
	defer func() { step.End(err) }()

	if ctr.CtrClient == nil {
		return nil, fmt.Errorf("Ctr client not found. Please start the server if not done already.")
	}

	createNetworkReq := &testapi.CreateNetworkRequest{Name: networkName}
	common.WriteProtoToStepLog(ctx, step, createNetworkReq, "CreateNetworkRequest")

	resp, err := ctr.CtrClient.CreateNetwork(ctx, createNetworkReq, grpc.EmptyCallOption{})
	if err != nil {
		return nil, errors.Annotate(err, "error during create network: ").Err()
	}

	common.WriteProtoToStepLog(ctx, step, resp, "CreateNetworkResponse")
	logging.Infof(ctx, "Successfully started network %s!", networkName)
	return resp, nil
}

// GetNetwork provides the network ID of the provided name if it exists.
func (ctr *CrosToolRunner) GetNetwork(ctx context.Context, networkName string) (*testapi.GetNetworkResponse, error) {
	if networkName == "" {
		return nil, fmt.Errorf("Cannot get network with empty network name.")
	}
	if networkName == "host" {
		return nil, fmt.Errorf("Cannot fetch default host network. Known to exist.")
	}

	var err error
	step, ctx := build.StartStep(ctx, fmt.Sprintf("Docker: Get network %s", networkName))
	defer func() { step.End(err) }()

	if ctr.CtrClient == nil {
		return nil, fmt.Errorf("Ctr client not found. Please start the server if not done already.")
	}

	getNetworkReq := &testapi.GetNetworkRequest{Name: networkName}
	common.WriteProtoToStepLog(ctx, step, getNetworkReq, "GetNetworkRequest")

	resp, err := ctr.CtrClient.GetNetwork(ctx, getNetworkReq, grpc.EmptyCallOption{})
	if err != nil {
		return nil, errors.Annotate(err, "error during get network: ").Err()
	}

	common.WriteProtoToStepLog(ctx, step, resp, "GetNetworkResponse")
	logging.Infof(ctx, "Successfully got network %s", networkName)
	return resp, nil
}

// GetContainer gets the container with provided name.
func (ctr *CrosToolRunner) GetContainer(
	ctx context.Context,
	containerName string) (*testapi.GetContainerResponse, error) {

	if containerName == "" {
		return nil, fmt.Errorf("Cannot execute get container with empty container name.")
	}

	var err error
	step, ctx := build.StartStep(ctx, fmt.Sprintf("CrosToolRunner: Get container %s", containerName))
	defer func() { step.End(err) }()

	if ctr.CtrClient == nil {
		return nil, fmt.Errorf("Ctr client not found. Please start the server if not done already.")
	}

	// Get container info
	getContainerReq := &testapi.GetContainerRequest{Name: containerName}
	common.WriteProtoToStepLog(ctx, step, getContainerReq, "GetContainerRequest")

	portFound := false
	// Retry finding the container, as it may still be starting up.
	retryCount := 30
	timeout := 5 * time.Second

	resp := &testapi.GetContainerResponse{}
	for !portFound && retryCount > 0 {
		resp, err = ctr.CtrClient.GetContainer(ctx, getContainerReq, grpc.EmptyCallOption{})
		if err != nil {
			return nil, errors.Annotate(err, "error during getting container: ").Err()
		}

		if resp.GetContainer().GetPortBindings() != nil && len(resp.Container.GetPortBindings()) > 0 {
			portFound = true
		}
		retryCount = retryCount - 1
		time.Sleep(timeout)
	}

	logging.Infof(ctx, "portfound: %v, remainingretrycount: %v, timeout: %v", portFound, retryCount, timeout)

	common.WriteProtoToStepLog(ctx, step, resp, "GetContainerResponse")
	logging.Infof(ctx, "Successfully got container %s.", getContainerReq.GetName())
	return resp, nil
}

// GcloudAuth does auth to the registry.
func (ctr *CrosToolRunner) GcloudAuth(
	ctx context.Context,
	dockerFileLocation string,
	useDockerKeyDirectly bool) (*testapi.LoginRegistryResponse, error) {
	username := Oauth2Username
	password := Oauth2Password
	if useDockerKeyDirectly {
		username = "_json_key"
		password = dockerFileLocation
	}
	step, ctx := build.StartStep(ctx, fmt.Sprintf("CrosToolRunner: Auth Gcloud with user %s", username))
	var err error
	defer func() { step.End(err) }()

	if ctr.CtrClient == nil {
		return nil, fmt.Errorf("Ctr client not found. Please start the server if not done already.")
	}

	extension := testapi.LoginRegistryExtensions{}
	if dockerFileLocation != "" && !useDockerKeyDirectly {
		extension = testapi.LoginRegistryExtensions{
			GcloudAuthServiceAccountArgs: []string{"--key-file",
				dockerFileLocation}}
	}

	loginReq := testapi.LoginRegistryRequest{Username: username, Password: password, Registry: ImageRegistry, Extensions: &extension}
	common.WriteProtoToStepLog(ctx, step, &loginReq, "LoginRegistryRequest")

	// Login
	retryFunc := func() (*testapi.LoginRegistryResponse, error) {
		return ctr.CtrClient.LoginRegistry(ctx, &loginReq, grpc.EmptyCallOption{})
	}
	notifyFunc := func(e error, t time.Duration) {
		logging.Infof(ctx, "Gcloud Auth failed after %s with error: %s", t, e)
	}
	resp, err := backoff.RetryNotifyWithData(retryFunc, BackoffFunc(), notifyFunc)

	if err != nil {
		return nil, errors.Annotate(err, "error in gcloud auth: ").Err()
	}

	common.WriteProtoToStepLog(ctx, step, resp, "LoginRegistryResponse")
	log.Printf("Successfully logged in!")
	return resp, nil
}

func (ctr *CrosToolRunner) sudoCommand(ctx context.Context, envVarsToPreserve []string, cmd string, args ...string) *exec.Cmd {
	if ctr.NoSudo {
		return exec.CommandContext(ctx, cmd, args...)
	}

	args = append([]string{cmd}, args...)
	if len(envVarsToPreserve) > 0 {
		args = append(
			[]string{fmt.Sprintf("--preserve-env=%s", strings.Join(envVarsToPreserve, ","))},
			args...,
		)
	}
	return exec.CommandContext(ctx, "sudo", args...)
}

// backoffFunc returns the specified exponential backoff
// for retrying in this context.
func backoffFunc() backoff.BackOff {
	return backoff.NewExponentialBackOff(
		backoff.WithInitialInterval(time.Second*2),
		backoff.WithMaxInterval(time.Second*16),
		backoff.WithMaxElapsedTime(time.Minute),
	)
}
