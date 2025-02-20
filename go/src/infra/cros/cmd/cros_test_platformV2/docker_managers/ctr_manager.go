// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package managers

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
	crostoolrunner "infra/cros/cmd/cros_test_platformV2/tools/ctr"
)

const (
	Username      = "oauth2accesstoken"
	Password      = "$(gcloud auth print-access-token)"
	ImageRegistry = "us-docker.pkg.dev"
)

type CtrManager struct {
	crostoolrunner.CtrCipdInfo

	CtrClient         testapi.CrosToolRunnerContainerServiceClient
	EnvVarsToPreserve []string

	wg              *sync.WaitGroup
	isServerRunning bool
	address         string
}

func NewCtrManager() *CtrManager {
	return &CtrManager{}
}

func (ex *CtrManager) StartContainer(ctx context.Context, req *testapi.StartTemplatedContainerRequest) (*testapi.StartContainerResponse, error) {
	//	startContainerReq *testapi.StartTemplatedContainerRequest) (*testapi.StartContainerResponse, error) {

	if req == nil {
		return nil, fmt.Errorf("start templated container request cannot be nil for start templated container command.")
	}

	var err error
	step, ctx := build.StartStep(ctx, fmt.Sprintf("CrosToolRunner: Start templated container %s", req.Name))
	defer func() { step.End(err) }()

	if ex.CtrClient == nil {
		return nil, fmt.Errorf("Ctr client not found. Please start the server if not done already.")
	}

	// Start the container
	common.WriteProtoToStepLog(ctx, step, req, "StartTemplatedContainerRequest")
	resp, err := ex.CtrClient.StartTemplatedContainer(ctx, req, grpc.EmptyCallOption{})
	if err != nil {
		return nil, errors.Annotate(err, "error during starting templated container").Err()
	}

	common.WriteProtoToStepLog(ctx, step, resp, "StartTemplatedContainerResponse")
	logging.Infof(ctx, "Successfully started templated container %s!", req.Name)

	return resp, nil
}

func (ex *CtrManager) StopContainer(ctx context.Context, foo string) error {
	return nil
}

// Might not be required?
func (ex *CtrManager) StartManager(ctx context.Context, foo string) (err error) {
	fmt.Println("Starting CTR Server")
	ctrCipd := crostoolrunner.CtrCipdInfo{
		Version:        "prod",
		CtrCipdPackage: common.CtrCipdPackage,
	}
	ex.CtrCipdInfo = ctrCipd
	ex.Initialize(ctx)

	if ex.wg != nil {
		return fmt.Errorf("Stop existing server connection before starting a new one!")
	}
	err = ex.startCTRServer(ctx)
	if err != nil {
		fmt.Printf("error during starting ctr server: %s", err.Error())

		logging.Infof(ctx, "error during starting ctr server: %s", err.Error())
	}

	// Step 3. Get the address
	serverAddress, err := ex.getServerAddressFromServiceMetadata(ctx)
	if err != nil {
		fmt.Printf("error during coonect ctr server: %s\n", err.Error())

		return errors.Annotate(err, "cros-tool-runner retrieve server address error: ").Err()
	}

	// Step 4. Connect to the server
	_, err = ex.connectToCTRServer(ctx, serverAddress)
	if err != nil {
		return errors.Annotate(err, "cros-tool-runner connect to server error: ").Err()
	}

	// Step 5. Auth
	dockerKeyFileLocation := "/creds/service_accounts/skylab-drone.json"
	_, err = ex.gcloudAuth(ctx, dockerKeyFileLocation)
	if err != nil {
		return errors.Annotate(err, "error during gcloud cmd: ").Err()
	}
	return err

}

// func (ex *CtrManager) Initialize(ctx context.Context) error {
// 	ex.CtrCipdInfo.Initialize()
// 	return nil
// }

func (ex *CtrManager) startCTRServer(ctx context.Context) (err error) {
	if ex.wg != nil {
		return fmt.Errorf("Stop existing server connection before starting a new one!")
	}

	ex.wg = &sync.WaitGroup{}
	ex.wg.Add(1)

	go func() {
		err := ex.startCTRServerlower(ctx)
		if err != nil {
			fmt.Println("Error starting")

			logging.Infof(ctx, "error during starting ctr server: %s", err.Error())
		}
		ex.wg.Done()
	}()
	return err
}

// StartCTRServer starts the server and exports service metadata to
// already created temp dir.
func (ex *CtrManager) startCTRServerlower(ctx context.Context) error {
	var err error
	step, ctx := build.StartStep(ctx, "CrosToolRunner: Start cros-tool-runner server")
	defer func() { step.End(err) }()

	// Initialize if not already initialized.
	if !ex.IsInitialized {
		if err = ex.Initialize(ctx); err != nil {
			return errors.Annotate(err, "CTR initialization error: ").Err()
		}
	}

	// Start the server preserving provided environment vars.
	writer := step.Log("CTR Stdout")
	cmdArgs := []string{}
	if len(ex.EnvVarsToPreserve) > 0 {
		cmdArgs = append(cmdArgs, fmt.Sprintf(
			"--preserve-env=%s",
			strings.Join(ex.EnvVarsToPreserve, ","),
		))
	}
	cmdArgs = append(cmdArgs, ex.CtrPath, "server", "--port", "0", "--export-metadata", ex.CtrTempDirLoc)
	fmt.Println(ctx, "Starting CTR server...")

	cmd := exec.CommandContext(ctx, "sudo", cmdArgs...)
	err = common.RunCommandWithCustomWriter(ctx, cmd, "ctr-start", writer)
	fmt.Println(ctx, "Finished starting CTR server")

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

// GetServerAddressFromServiceMetadata waits for the service metadata file and
// gets ctr server address from it.
func (ex *CtrManager) getServerAddressFromServiceMetadata(ctx context.Context) (string, error) {
	if ex.CtrTempDirLoc == "" {
		return "", fmt.Errorf("Cannot retrieve ctr server address with empty temp dir.")
	}

	var err error
	step, ctx := build.StartStep(ctx, "CrosToolRunner: Retrieve service metadata")
	defer func() { step.End(err) }()

	metaFilePath := path.Join(ex.CtrTempDirLoc, common.CftServiceMetadataFileName)

	metadataLog := step.Log("Ctr service metadata")
	serverAddress, err := common.GetCftLocalServerAddress(ctx, metaFilePath, metadataLog)
	if err != nil {
		return "", errors.Annotate(err, "Error during getting ctr server address: ").Err()
	}

	return serverAddress, nil
}

// ConnectToCTRServer connects to the CTR server in provided server address.
func (ex *CtrManager) connectToCTRServer(
	ctx context.Context,
	serverAddress string) (testapi.CrosToolRunnerContainerServiceClient, error) {
	var err error
	step, ctx := build.StartStep(ctx, "CrosToolRunner: Connect to cros-tool-runner server")
	defer func() { step.End(err) }()

	if serverAddress == "" {
		return nil, fmt.Errorf("Ctr service connection is not possible without server address.")
	}

	if ex.CtrClient != nil {
		logging.Infof(ctx, "Skipping connecting to server is an existing server is already running.")
		return ex.CtrClient, nil
	}

	// Connect with service
	conn, err := common.ConnectWithService(ctx, serverAddress)
	if err != nil {
		return nil, errors.Annotate(err, "Error during connecting to ctr server: ").Err()
	}

	// Successful connection confirms that the server is running.
	ex.isServerRunning = true
	logging.Infof(ctx, "Successfully connected to CTR service!")

	// Construct CTR client
	ctrClient := testapi.NewCrosToolRunnerContainerServiceClient(conn)
	if ctrClient == nil {
		return nil, fmt.Errorf("CrosToolRunnerContainerServiceClient is nil")
	}

	ex.CtrClient = ctrClient
	return ctrClient, nil
}

// StopCTRServer stops currently running CTR server.
func (ctr *CtrManager) StopManager(ctx context.Context, foo string) error {
	var err error
	step, ctx := build.StartStep(ctx, "CrosToolRunner: Stop cros-tool-runner server")
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

// // -- Container commands --

// // StartContainer starts a non-templated container using ctr client.
// func (ctr *CrosToolRunner) StartContainer(
// 	ctx context.Context,
// 	startContainerReq *testapi.StartContainerRequest) (*testapi.StartContainerResponse, error) {
// 	if startContainerReq == nil {
// 		return nil, fmt.Errorf("start container request cannot be nil for start container command.")
// 	}

// 	var err error
// 	step, ctx := build.StartStep(ctx, fmt.Sprintf("CrosToolRunner: Start container %s", startContainerReq.Name))
// 	defer func() { step.End(err) }()

// 	if ctr.CtrClient == nil {
// 		return nil, fmt.Errorf("Ctr client not found. Please start the server if not done already.")
// 	}

// 	common.LogExecutionDetails(ctx, step, startContainerReq.StartCommand)
// 	common.WriteProtoToStepLog(ctx, step, startContainerReq, "StartContainerRequest")

// 	// Start container
// 	resp, err := ctr.CtrClient.StartContainer(ctx, startContainerReq, grpc.EmptyCallOption{})
// 	if err != nil {
// 		return nil, errors.Annotate(err, "error during starting container").Err()
// 	}

// 	common.WriteProtoToStepLog(ctx, step, resp, "StartContainerResponse")
// 	logging.Infof(ctx, "Successfully started container %s!", startContainerReq.Name)
// 	return resp, nil
// }

// // StopContainer stops the container with provided name.
// func (ctr *CrosToolRunner) StopContainer(ctx context.Context, containerName string) error {
// 	if containerName == "" {
// 		return fmt.Errorf("Cannot stop container with empty container name.")
// 	}

// 	var err error
// 	step, ctx := build.StartStep(ctx, fmt.Sprintf("Docker: Stop container %s", containerName))
// 	defer func() { step.End(err) }()

// 	// Stop container
// 	cmd := exec.CommandContext(ctx, "sudo", "docker", "stop", containerName)
// 	common.LogExecutionDetails(ctx, step, cmd.Args)
// 	_, _, err = common.RunCommand(ctx, cmd, "docker-stop-container", nil, false)
// 	if err != nil {
// 		return fmt.Errorf("error during stopping container %s: %s", containerName, err.Error())
// 	}

// 	logging.Infof(ctx, "Successfully stopped container %s!", containerName)
// 	return nil
// }

// GetContainer gets the container with provided name.
func (ctr *CtrManager) GetContainer(
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

	// TODO (azrahman): use exponential backoff retry
	portFound := false
	retryCount := 15 // This number is currently a bit high due to drone's lower than expected performance
	timeout := 5 * time.Second

	resp := &testapi.GetContainerResponse{}
	for !portFound && retryCount > 0 {
		fmt.Println("Calling Get container")
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
	fmt.Println("Finished calling Get container")

	fmt.Printf("portfound: %v, remainingretrycount: %v, timeout: %v", portFound, retryCount, timeout)

	common.WriteProtoToStepLog(ctx, step, resp, "GetContainerResponse")
	logging.Infof(ctx, "Successfully got container %s.", getContainerReq.GetName())
	return resp, nil
}

// GcloudAuth does auth to the registry.
func (ex *CtrManager) gcloudAuth(
	ctx context.Context,
	dockerFileLocation string) (*testapi.LoginRegistryResponse, error) {
	log.Println("Gcloud Authing")
	log.Printf("CrosToolRunner: Auth Gcloud with user %s", Username)
	var err error

	if ex.CtrClient == nil {
		return nil, fmt.Errorf("Ctr client not found. Please start the server if not done already.")
	}

	extension := testapi.LoginRegistryExtensions{}
	if dockerFileLocation != "" {
		extension = testapi.LoginRegistryExtensions{
			GcloudAuthServiceAccountArgs: []string{"--key-file",
				dockerFileLocation}}
	}

	loginReq := testapi.LoginRegistryRequest{Username: Username, Password: Password, Registry: ImageRegistry, Extensions: &extension}

	// Login
	resp, err := ex.CtrClient.LoginRegistry(ctx, &loginReq, grpc.EmptyCallOption{})
	if err != nil {
		fmt.Printf("error in gcloud auth: %s", err)
		return nil, errors.Annotate(err, "error in gcloud auth: ").Err()
	}

	log.Printf("Successfully logged in!")
	return resp, nil
}
