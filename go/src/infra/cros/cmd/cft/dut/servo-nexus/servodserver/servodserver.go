// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package servodserver implements servod_service.proto (see proto for details)
package servodserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"go.chromium.org/chromiumos/test/util/portdiscovery"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dc "github.com/docker/docker/client"
	lucierr "go.chromium.org/luci/common/errors"
	crypto_ssh "golang.org/x/crypto/ssh"
	"google.golang.org/grpc"

	xmlrpc_value "go.chromium.org/chromiumos/config/go/api/test/xmlrpc"
	"go.chromium.org/chromiumos/config/go/longrunning"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/infra/proto/go/satlabrpcserver"
	"go.chromium.org/chromiumos/lro"
	common_util "go.chromium.org/chromiumos/test/util/common"

	"infra/cros/cmd/cft/dut/servo-nexus/commandexecutor"
	"infra/cros/cmd/cft/dut/servo-nexus/servod"
	"infra/cros/cmd/cft/dut/servo-nexus/ssh"
)

const (
	jobRunning      = "Job is already running"
	ready           = "ready"
	SatlabRPCServer = "satlab_rpcserver:6003"
	dockerHost      = "tcp://192.168.231.1:2375"
	satlab          = "satlab"
)

// ServodService implementation of servod_service.proto
type ServodService struct {
	manager         *lro.Manager
	logger          *log.Logger
	commandexecutor commandexecutor.CommandExecutorInterface
	sshPool         *ssh.Pool
	servodPool      *servod.Pool
	dockerClient    *dc.Client
}

// NewServodService creates a new servod service.
func NewServodService(ctx context.Context, logger *log.Logger, commandexecutor commandexecutor.CommandExecutorInterface) (*ServodService, func(), error) {
	config, err := ssh.NewDefaultConfig()
	if err != nil {
		return nil, nil, err
	}
	if common_util.IsCloudBot() {
		if err = config.Load(defaultSSHConfigPathOnCloudBot); err != nil {
			return nil, nil, err
		}
	}
	servodService := &ServodService{
		manager:         lro.New(),
		logger:          logger,
		commandexecutor: commandexecutor,
		sshPool:         ssh.New(config),
		servodPool:      servod.NewPool(),
	}

	destructor := func() {
		servodService.manager.Close()
	}

	return servodService, destructor, nil
}

// StartServod runs a servod Docker container and starts the servod daemon
// inside the container if servod is containerized. Otherwise, it simply
// starts the servod daemon.
func (s *ServodService) StartServod(ctx context.Context, req *api.StartServodRequest) (*longrunning.Operation, error) {
	s.logger.Printf("Received api.StartServodRequest: %#v\n", req)
	op := s.manager.NewOperation()
	err := s.StartServo(req)
	if err != nil {
		s.logger.Println("Failed to process StartServo request: ", err)
		s.manager.SetResult(op.Name, &api.StartServodResponse{
			Result: &api.StartServodResponse_Failure_{
				Failure: &api.StartServodResponse_Failure{
					ErrorMessage: err.Error(),
				},
			},
		})
	} else {
		s.logger.Println("Successfully processed StartServo request.")
		s.manager.SetResult(op.Name, &api.StartServodResponse{
			Result: &api.StartServodResponse_Success_{},
		})
		err = nil
	}
	return op, err
}

// StopServod stops the servod daemon inside the container and stops the
// servod Docker container if servod is containerized. Otherwise, it simply
// stops the servod daemon.
func (s *ServodService) StopServod(ctx context.Context, req *api.StopServodRequest) (*longrunning.Operation, error) {
	s.logger.Printf("Received api.StopServodRequest: %#v\n", req)
	op := s.manager.NewOperation()
	var err error
	if isSatlab(req.ServodDockerContainerName) {
		err = s.stopServodOnSatlab(context.Background(), req.ServodDockerContainerName)
	} else {
		command := fmt.Sprintf("stop servod PORT=%d", req.ServodPort)
		bOut, bErr, tmpErr := s.commandexecutor.Run(req.ServoHostPath, command, nil, false)
		if tmpErr != nil {
			err = fmt.Errorf("error while stopping servo \nstdout: %s\n stdErr: %s\n err: %s", bOut.String(), bErr.String(), tmpErr.Error())
		}
	}
	if err != nil {
		s.logger.Println("Failed to run CLI: ", err)
		s.manager.SetResult(op.Name, &api.StopServodResponse{
			Result: &api.StopServodResponse_Failure_{
				Failure: &api.StopServodResponse_Failure{
					ErrorMessage: err.Error(),
				},
			},
		})
	} else {
		s.manager.SetResult(op.Name, &api.StopServodResponse{
			Result: &api.StopServodResponse_Success_{},
		})
	}

	return op, err
}

// ExecCmd executes a system command that is provided through the command
// parameter in the request. It allows the user to execute arbitrary commands
// that can't be handled by calling servod (e.g. update firmware through
// "futility", remote file copy through "scp").
// It executes the command inside the servod Docker container if the
// servod_docker_container_name parameter is provided in the request.
// Otherwise, it executes the command directly inside the host that the servo
// is physically connected to.
func (s *ServodService) ExecCmd(ctx context.Context, req *api.ExecCmdRequest) (*api.ExecCmdResponse, error) {
	s.logger.Printf("Received api.ExecCmdRequest: %#v\n", req)
	var err error
	var bOut, bErr *bytes.Buffer
	if isSatlab(req.ServodDockerContainerName) {
		bOut, bErr, err = s.execCmdOnSatlab(ctx, req)
	} else {
		bOut, bErr, err = s.execCmdOnLabstation(req)
	}

	return &api.ExecCmdResponse{
		ExitInfo: getExitInfo(err),
		Stdout:   bOut.Bytes(),
		Stderr:   bErr.Bytes(),
	}, err
}

// CallServod runs a servod command through an XML-RPC call.
// It runs the command inside the servod Docker container if the
// servod_docker_container_name parameter is provided in the request.
// Otherwise, it runs the command directly inside the host that the servo
// is physically connected to.
// Allowed methods: doc, get, set, and hwinit.
func (s *ServodService) CallServod(ctx context.Context, req *api.CallServodRequest) (*api.CallServodResponse, error) {
	s.logger.Printf("Received api.CallServodRequest: %#v\n", req)
	servoHostPath := req.ServoHostPath
	isSatlab := false
	if strings.Contains(req.ServoHostPath, "satlab") {
		isSatlab = true
		containerIp, err := s.getSatlabServodContainerIP(ctx, req.ServodDockerContainerName)
		if err != nil {
			err := fmt.Errorf("Servod container not started on satlab.Did you forget to call StartServod?")
			return &api.CallServodResponse{
				Result: &api.CallServodResponse_Failure_{
					Failure: &api.CallServodResponse_Failure{
						ErrorMessage: err.Error(),
					},
				},
			}, err
		}
		servoHostPath = containerIp
	}
	sd, err := s.servodPool.Get(
		servoHostPath,
		req.ServodPort,
		// This method must return non-nil value for servod.Get to work so return a dummy array.
		func() ([]string, error) {
			return []string{}, nil
		})
	if err != nil {
		return &api.CallServodResponse{
			Result: &api.CallServodResponse_Failure_{
				Failure: &api.CallServodResponse_Failure{
					ErrorMessage: err.Error(),
				},
			},
		}, err
	}
	var val *xmlrpc_value.Value
	if isSatlab {
		val, err = sd.Call(ctx, servoHostPath, int(req.ServodPort), strings.ToLower(req.Method.String()), req.Args)
	} else {
		val, err = sd.CallWithProxy(ctx, s.sshPool, strings.ToLower(req.Method.String()), req.Args)
	}
	if err != nil {
		return &api.CallServodResponse{
			Result: &api.CallServodResponse_Failure_{
				Failure: &api.CallServodResponse_Failure{
					ErrorMessage: err.Error(),
				},
			},
		}, err
	}
	s.logger.Println("Completed CallServodRequest!")
	return &api.CallServodResponse{
		Result: &api.CallServodResponse_Success_{
			Success: &api.CallServodResponse_Success{
				Result: val,
			},
		},
	}, nil
}

// LogCheckPoint will create checkpoint certain files so that some files
// can be saved partially when SaveLogs is called.
// For example, /var/log/messages in a labstation can be
// very big and include information from a few days ago.
// Getting the checkpoint of the current /var/log/messages will
// allow SaveLogs to save the portion only relevant to the current
// testing session.
func (s *ServodService) LogCheckPoint(ctx context.Context, req *api.LogCheckPointRequest) (*api.LogCheckPointResponse, error) {
	s.logger.Printf("Received api.LogCheckPointRequest: %#v\n", req)
	return nil, errors.New("the service LogCheckPoint has not be implemented")
}

// SaveLogs will save servod related logs on the host that this service
// is running.
// Logs include:
//
//	/var/log/message from the servod host.
//	/var/log/servod_<port>/ latest.DEBUG from servod host.
//	/var/log/servod_<port>.STARTUP.log from servod host.
//	The output of  "dmesg -H"  from the servod host.
//	The extraction of the MCU console logs from latest.DEBUG
func (s *ServodService) SaveLogs(ctx context.Context, req *api.SaveLogsRequest) (*api.SaveLogsResponse, error) {
	s.logger.Printf("Received api.SaveLogsRequest: %#v\n", req)
	return nil, errors.New("the service SaveLogs has not be implemented")
}

func (s *ServodService) execCmdOnSatlab(ctx context.Context, req *api.ExecCmdRequest) (*bytes.Buffer, *bytes.Buffer, error) {
	//TODO: Implement this function.
	return nil, nil, fmt.Errorf("exeCmd on satlab is not supported")
}

func (s *ServodService) execCmdOnLabstation(req *api.ExecCmdRequest) (*bytes.Buffer, *bytes.Buffer, error) {
	bOut, bErr, err := s.commandexecutor.Run(req.ServoHostPath, req.Command, nil, false)
	if err != nil {
		s.logger.Printf("\nError while running %s command, stdOut: %s\n stdErr: %s\nErr: %s", req.Command, bOut.String(), bErr.String(), err.Error())
		return bOut, bErr, err
	}
	s.logger.Printf("\nSuccessfully ran %s command with stdOut: %s\n stdErr: %s", req.Command, bOut.String(), bErr.String())
	return bOut, bErr, nil
}

// getExitInfo extracts exit info from Session Run's error
func getExitInfo(runError error) *api.ExecCmdResponse_ExitInfo {
	// If no error, command succeeded
	if runError == nil {
		return createCommandSucceededExitInfo()
	}

	// If ExitError, command ran but did not succeed
	var ee *crypto_ssh.ExitError
	if errors.As(runError, &ee) {
		return createCommandFailedExitInfo(ee)
	}

	// Otherwise we assume command failed to start
	return createFailedToStartExitInfo(runError)
}

func createFailedToStartExitInfo(err error) *api.ExecCmdResponse_ExitInfo {
	return &api.ExecCmdResponse_ExitInfo{
		Status:       42, // Contract dictates arbitrary response, thus 42 is as good as any number
		Signaled:     false,
		Started:      false,
		ErrorMessage: err.Error(),
	}
}

func createCommandSucceededExitInfo() *api.ExecCmdResponse_ExitInfo {
	return &api.ExecCmdResponse_ExitInfo{
		Status:       0,
		Signaled:     false,
		Started:      true,
		ErrorMessage: "",
	}
}

func createCommandFailedExitInfo(err *crypto_ssh.ExitError) *api.ExecCmdResponse_ExitInfo {
	return &api.ExecCmdResponse_ExitInfo{
		Status:       int32(err.ExitStatus()),
		Signaled:     true,
		Started:      true,
		ErrorMessage: "",
	}
}

func (s *ServodService) getDockerClient() (*dc.Client, error) {
	timeout := time.Duration(1 * time.Second)
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: timeout,
		}).DialContext,
	}
	c := http.Client{Transport: transport}
	return dc.NewClientWithOpts(dc.WithHost(dockerHost), dc.WithHTTPClient(&c), dc.WithAPIVersionNegotiation())
}

func (s *ServodService) stopServodOnSatlab(ctx context.Context, servodDockerContainerName string) error {
	s.logger.Println("Calling ContainerKill")
	if err := s.dockerClient.ContainerKill(ctx, servodDockerContainerName, "INT" /*signal*/); err != nil {
		s.logger.Println("Warning! container SIGINT failed", err)
	}

	s.logger.Println("Calling ContainerStop")
	if err := s.dockerClient.ContainerStop(ctx, servodDockerContainerName, container.StopOptions{}); err != nil {
		s.logger.Println("Warning! ContainerStop failed", err)
	}
	s.logger.Println("Waiting 30 seconds for ContainerStop")
	time.Sleep(30 * time.Second)
	return nil
}

func (s *ServodService) getSatlabServodContainerIP(ctx context.Context, containerName string) (ipaddr string, err error) {
	s.logger.Println("Getting the containerIP for ", containerName)
	// Create filter to get container with given hostname and in running state.
	f := filters.NewArgs()
	f.Add("name", containerName)
	f.Add("status", "running")
	// Get the list of containers based on the filter above.
	if s.dockerClient == nil {
		s.logger.Println("dockerClient is null.... creating the docker client now.")
		s.dockerClient, err = s.getDockerClient()
		if err != nil {
			s.logger.Println("failed to create the docker client.")
			return "", fmt.Errorf("failed to create the docker client")
		}
	}

	containers, err := s.dockerClient.ContainerList(ctx, container.ListOptions{Filters: f})
	if err != nil {
		s.logger.Println("\n Error occurred while getting the docker containers list")
		return "", err
	}
	// Return error if the container is not found or is not in running state.
	if len(containers) != 1 {
		return "", fmt.Errorf("%d number of container(s) with name %s found", len(containers), containerName)
	}
	// Get the Docker network set to the container, this is set in the drone env variables.
	// If not found then fall back to default network name.
	cnet := os.Getenv("DOCKER_DEFAULT_NETWORK")
	if cnet == "" {
		cnet = "default_satlab"
	}
	if containers[0].NetworkSettings != nil {
		satNet := containers[0].NetworkSettings.Networks[cnet]
		if satNet != nil {
			return satNet.IPAddress, nil
		}
		return "", fmt.Errorf("could not find the %q network for the container %q. Found networks: [%v]", cnet, containerName, containers[0].NetworkSettings.Networks)
	}
	return "", fmt.Errorf("could not find IP address for the container %q", containerName)
}

func (s *ServodService) startServodOnSatlab(servodDockerContainerName string) error {
	dockerClient, err := s.getDockerClient()
	if err != nil {
		return err
	}
	s.dockerClient = dockerClient
	if ip, err := s.getSatlabServodContainerIP(context.Background(), servodDockerContainerName); err == nil && ip != "" {
		s.logger.Println("Servo Container already running.")
		return errors.New(jobRunning)
	}
	s.logger.Println("Starting servod container on Satlab.")
	conn, err := grpc.Dial(SatlabRPCServer, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("failed to initiate communication to satlab RPC %v Error %v", err, SatlabRPCServer)
	}
	satlabClient := satlabrpcserver.NewSatlabRpcServiceClient(conn)
	req := &api.StartServodRequest{ServodDockerContainerName: servodDockerContainerName}
	if _, err := satlabClient.StartServod(context.Background(), req); err != nil {
		return fmt.Errorf("SatlabRPCServer's startServo failed: %v", err)
	}
	_, err = s.getSatlabServodContainerIP(context.Background(), servodDockerContainerName)
	if err != nil {
		return fmt.Errorf("could not get ip address of servod container on satlab")
	}
	s.logger.Println("Servod container started via satlabrpc.")
	return nil
}

func (s *ServodService) StartServo(req *api.StartServodRequest) error {
	if strings.Contains(req.ServodDockerContainerName, satlab) {
		return s.startServodOnSatlab(req.ServodDockerContainerName)
	}
	return s.startServoLabStation(req)
}

func (s *ServodService) startServoLabStation(req *api.StartServodRequest) error {

	var bOut, bErr *bytes.Buffer
	var err error
	command, err := s.getStartServodCmdLabstation(req)
	if err != nil {
		return err
	}
	bOut, bErr, err = s.commandexecutor.Run(req.ServoHostPath, command, nil, false)
	if err != nil {
		return fmt.Errorf("error while running command %s\nstdOut: %s\nstdErr: %s\n err: %s", command, bOut.String(), bErr.String(), err.Error())
	}
	command = fmt.Sprintf("servodtool instance wait-for-active --timeout 60 -p %v", req.ServodPort)
	bOut, bErr, err = s.commandexecutor.Run(req.ServoHostPath, command, nil, false)
	if !strings.Contains(bOut.String(), ready) {
		return fmt.Errorf("error while running command %s\nstdOut: %s\nstdErr: %s\n err: %s", command, bOut.String(), bErr.String(), err.Error())
	}
	return nil
}

// getStartServodCommand returns either a start servo command for labstation
func (s *ServodService) getStartServodCmdLabstation(req *api.StartServodRequest) (string, error) {
	if req.Board == "" {
		return "", lucierr.Reason("Board not specified").Err()
	}
	if req.Model == "" {
		return "", lucierr.Reason("Model not specified").Err()
	}
	if req.SerialName == "" {
		return "", lucierr.Reason("SerialName not specified").Err()
	}
	return fmt.Sprintf("start servod %s", getStartServodEnv(req, "")), nil
}

// getStartServodEnv returns environment variables as a string.
// envPrefix is applied to each environment variable (e.g. Docker --env parameter).
func getStartServodEnv(req *api.StartServodRequest, envPrefix string) string {
	env := fmt.Sprintf("%sPORT=%d", envPrefix, req.ServodPort)
	env = fmt.Sprintf("%s %sBOARD=%s", env, envPrefix, req.Board)
	env = fmt.Sprintf("%s %sMODEL=%s", env, envPrefix, req.Model)
	env = fmt.Sprintf("%s %sSERIAL=%s", env, envPrefix, req.SerialName)
	if req.AllowDualV4 != "" {
		env = fmt.Sprintf("%s %sDUAL_V4=%s", env, envPrefix, req.AllowDualV4)
	}
	if req.Config != "" {
		env = fmt.Sprintf("%s %sCONFIG=%s", env, envPrefix, req.Config)
	}
	if req.Debug != "" {
		env = fmt.Sprintf("%s %sDEBUG=%s", env, envPrefix, req.Debug)
	}
	if req.RecoveryMode != "" {
		env = fmt.Sprintf("%s %sREC_MODE=%s", env, envPrefix, req.RecoveryMode)
	}
	return env
}

// StartServer starts servod server on requested port
func (s *ServodService) StartServer(port int32) error {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return lucierr.Annotate(err, "Start servod server: failed to create listener at %d", port).Err()
	}

	s.manager = lro.New()
	defer s.manager.Close()
	// Write port number to ~/.cftmeta for go/cft-port-discovery
	err = portdiscovery.WriteServiceMetadata("servo-nexus", l.Addr().String(), s.logger)
	if err != nil {
		s.logger.Println("Warning: error when writing to metadata file: ", err)
	}
	server := grpc.NewServer()

	api.RegisterServodServiceServer(server, s)
	longrunning.RegisterOperationsServer(server, s.manager)

	s.logger.Println("Servod server is listening to request at ", l.Addr().String())
	return server.Serve(l)
}

func isSatlab(servodDockerContainerName string) bool {
	return servodDockerContainerName != "" && strings.Contains(servodDockerContainerName, satlab)
}
