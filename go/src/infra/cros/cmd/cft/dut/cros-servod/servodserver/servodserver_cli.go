// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servodserver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dc "github.com/docker/docker/client"
	"google.golang.org/grpc"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/infra/proto/go/satlabrpcserver"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/cft/dut/cros-servod/model"
)

const (
	SatlabRPCServer = "satlab_rpcserver:6003"
	dockerHost      = "tcp://192.168.231.1:2375"
	satlab          = "satlab"
)

func (s *ServodService) getDockerClient() (*dc.Client, error) {
	// For TLS create Docker Client from env variables.
	if path := os.Getenv("DOCKER_CERT_PATH"); path != "" {
		// Use the tcp connection, host IP is defined by DOCKER_HOST env variable.
		return dc.NewClientWithOpts(dc.FromEnv, dc.WithAPIVersionNegotiation())
	}
	// TODO(klimkowicz): remove this legacy Docker Client fallback when
	// Satlab with TLS dockerd is fully rolled out.
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
			s.logger.Println("failed to create the docker client.", err)
			return "", fmt.Errorf("failed to create the docker client")
		}
	}
	containers, err := s.dockerClient.ContainerList(ctx, container.ListOptions{Filters: f})
	if err != nil {
		s.logger.Println("\n Error occurred while getting the docker containers list : ", err)
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
		return fmt.Errorf("failed to initiate communication to satlab RPC %w Error %v", err, SatlabRPCServer)
	}
	satlabClient := satlabrpcserver.NewSatlabRpcServiceClient(conn)
	req := &api.StartServodRequest{ServodDockerContainerName: servodDockerContainerName}
	if _, err := satlabClient.StartServod(context.Background(), req); err != nil {
		return fmt.Errorf("SatlabRPCServer's startServo failed: %w", err)
	}
	_, err = s.getSatlabServodContainerIP(context.Background(), servodDockerContainerName)
	if err != nil {
		return fmt.Errorf("could not get ip address of servod container on satlab")
	}
	s.logger.Println("Servod container started via satlabrpc.")
	return nil
}

func (s *ServodService) StartServo(a model.CliArgs) error {
	if strings.Contains(a.ServodDockerContainerName, satlab) {
		return s.startServodOnSatlab(a.ServodDockerContainerName)
	}
	return s.startServoLabStation(a)
}

func (s *ServodService) startServoLabStation(a model.CliArgs) error {

	var bOut bytes.Buffer
	var bErr bytes.Buffer
	var err error
	command, err := s.getStartServodCmdLabstation(a)
	if err != nil {
		return err
	}
	bOut, bErr, err = s.commandexecutor.Run(a.ServoHostPath, command, nil, false)
	if err != nil {
		return fmt.Errorf("error while running command %s\nstdOut: %s\nstdErr: %s\n err: %s", command, bOut.String(), bErr.String(), err.Error())
	}
	command = fmt.Sprintf("servodtool instance wait-for-active --timeout 120 -p %v", a.ServodPort)
	bOut, bErr, err = s.commandexecutor.Run(a.ServoHostPath, command, nil, false)
	if !strings.Contains(bOut.String(), ready) {
		return fmt.Errorf("error while running command %s\nstdOut: %s\nstdErr: %s\n err: %s", command, bOut.String(), bErr.String(), err.Error())
	}
	return nil
}

// RunCli runs servod service as execution by CLI.
func (s *ServodService) RunCli(cs model.CliSubcommand, a model.CliArgs, stdin io.Reader, routeToStd bool) (bytes.Buffer, bytes.Buffer, error) {
	s.logger.Println("Start running the servod service CLI.")

	var bOut bytes.Buffer
	var bErr bytes.Buffer
	var err error

	command := ""
	switch cs {
	case model.CliStopServod:
		if a.ServodDockerContainerName != "" && strings.Contains(a.ServodDockerContainerName, "satlab") {
			return bOut, bErr, s.stopServodOnSatlab(context.Background(), a.ServodDockerContainerName)
		}
		command = fmt.Sprintf("stop servod PORT=%d", a.ServodPort)
	case model.CliExecCmd:
		command = s.getExecCmdCommand(a)
	case model.CliCallServod:
		command = s.getCallServodCommand(a)
	}

	if command != "" {
		s.logger.Printf("Execute command: %s", command)
		bOut, bErr, err = s.commandexecutor.Run(a.ServoHostPath, command, stdin, routeToStd)
		if err != nil {
			return bOut, bErr, err
		}
		s.logger.Println("Finished running the servod service CLI successfully!")
	}

	return bOut, bErr, nil
}

// getStartServodCommand returns either a start servo command for labstation
func (s *ServodService) getStartServodCmdLabstation(a model.CliArgs) (string, error) {
	if a.Board == "" {
		return "", errors.Reason("Board not specified").Err()
	}
	if a.Model == "" {
		return "", errors.Reason("Model not specified").Err()
	}
	if a.SerialName == "" {
		return "", errors.Reason("SerialName not specified").Err()
	}
	return fmt.Sprintf("start servod %s", getStartServodEnv(a, "")), nil
}

// getStartServodEnv returns environment variables as a string.
// envPrefix is applied to each environment variable (e.g. Docker --env parameter).
func getStartServodEnv(a model.CliArgs, envPrefix string) string {
	env := fmt.Sprintf("%sPORT=%d", envPrefix, a.ServodPort)
	env = fmt.Sprintf("%s %sBOARD=%s", env, envPrefix, a.Board)
	env = fmt.Sprintf("%s %sMODEL=%s", env, envPrefix, a.Model)
	env = fmt.Sprintf("%s %sSERIAL=%s", env, envPrefix, a.SerialName)
	if a.AllowDualV4 != "" {
		env = fmt.Sprintf("%s %sDUAL_V4=%s", env, envPrefix, a.AllowDualV4)
	}
	if a.Config != "" {
		env = fmt.Sprintf("%s %sCONFIG=%s", env, envPrefix, a.Config)
	}
	if a.Debug != "" {
		env = fmt.Sprintf("%s %sDEBUG=%s", env, envPrefix, a.Debug)
	}
	if a.RecoveryMode != "" {
		env = fmt.Sprintf("%s %sREC_MODE=%s", env, envPrefix, a.RecoveryMode)
	}
	return env
}

// getExecCmdCommand returns either a "docker exec" command when
// ServodDockerContainerName is specified or the command provided
// when ServodDockerContainerName is empty.
func (s *ServodService) getExecCmdCommand(a model.CliArgs) string {
	if a.ServodDockerContainerName != "" {
		return fmt.Sprintf("docker exec -d %s '%s'",
			a.ServodDockerContainerName, a.Command)
	} else {
		return a.Command
	}
}

// getCallServodCommand returns either a "docker exec" command when
// ServodDockerContainerName is specified or a "dut-control" command
// when ServodDockerContainerName is empty.
func (s *ServodService) getCallServodCommand(a model.CliArgs) string {
	command := ""
	// Generate a "dut-control" command based on the method and args provided.
	switch strings.ToLower(a.Method) {
	case "doc":
		command = fmt.Sprintf("dut-control -p %d -i %s", a.ServodPort, a.Args)
	case "get", "set":
		command = fmt.Sprintf("dut-control -p %d %s", a.ServodPort, a.Args)
	}

	if a.ServodDockerContainerName != "" {
		return fmt.Sprintf("docker exec -d %s '%s'",
			a.ServodDockerContainerName, command)
	} else {
		return command
	}
}
