// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package server implement bols-service API.
package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/config/go/test/api/bols"

	"go.chromium.org/infra/cros/lib/bols/xmlrpc"
	"go.chromium.org/infra/cros/recovery/docker"
	"go.chromium.org/infra/cros/satlab/common/utils/misc"
)

func (s *service) GetFileStat(context.Context, *bols.GetFileStatRequest) (*bols.GetFileStatResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetFileStat not implemented")
}

func (s *service) GetFile(*bols.GetFileRequest, bols.BolsService_GetFileServer) error {
	return status.Errorf(codes.Unimplemented, "method GetFile not implemented")
}

func (s *service) PutFile(bols.BolsService_PutFileServer) error {
	return status.Errorf(codes.Unimplemented, "method PutFile not implemented")
}

func (s *service) DownloadFile(context.Context, *bols.DownloadFileRequest) (*bols.DownloadFileResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DownloadFile not implemented")
}

func (s *service) RemoveFile(context.Context, *bols.RemoveFileRequest) (*bols.RemoveFileResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RemoveFile not implemented")
}

func (s *service) GetDirInfo(context.Context, *bols.GetDirInfoRequest) (*bols.GetDirInfoResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetDirInfo not implemented")
}

func (s *service) MakeDir(context.Context, *bols.MakeDirRequest) (*bols.MakeDirResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method MakeDir not implemented")
}

func (s *service) MakeTempDir(context.Context, *bols.MakeTempDirRequest) (*bols.MakeTempDirResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method MakeTempDir not implemented")
}

func (s *service) RemoveDir(context.Context, *bols.RemoveDirRequest) (*bols.RemoveDirResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RemoveDir not implemented")
}

func (s *service) DMesg(*bols.DMesgRequest, bols.BolsService_DMesgServer) error {
	return status.Errorf(codes.Unimplemented, "method DMesg not implemented")
}

func (s *service) WriteFileByBlock(bols.BolsService_WriteFileByBlockServer) error {
	return status.Errorf(codes.Unimplemented, "method WriteFileByBlock not implemented")
}

func (s *service) ReadFileByBlock(*bols.ReadFileByBlockRequest, bols.BolsService_ReadFileByBlockServer) error {
	return status.Errorf(codes.Unimplemented, "method ReadFileByBlock not implemented")
}

func (s *service) RunMount(context.Context, *bols.RunMountRequest) (*bols.RunMountResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RunMount not implemented")
}

func (s *service) RunUMount(context.Context, *bols.RunUMountRequest) (*bols.RunUMountResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RunUMount not implemented")
}

func (s *service) StartServod(ctx context.Context, req *bols.StartServodRequest) (*bols.StartServodResponse, error) {
	if err := startServod(ctx,
		req.GetStationId().GetContainerName(),
		req.GetBoard(), req.GetModel(), req.GetStationId().GetServoSerial(), req.GetConfig(),
		req.GetRecoveryMode(),
		req.GetStationId().GetServodPort(), s.logger); err != nil {
		return nil, fmt.Errorf("fail to start servod: %w", err)
	}
	return &bols.StartServodResponse{}, nil
}

func (s *service) StopServod(context.Context, *bols.StopServodRequest) (*bols.StopServodResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StopServod not implemented")
}

func (s *service) GetServodStatus(ctx context.Context, req *bols.GetServodStatusRequest) (*bols.GetServodStatusResponse, error) {
	c, err := docker.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to create docker client: %w", err)
	}
	status, err := servodStatus(ctx, c, req.GetStationId().GetContainerName(), req.GetStationId().GetServodPort())
	if err != nil {
		return nil, fmt.Errorf("fail to get servod status: %w", err)
	}
	return &bols.GetServodStatusResponse{Status: status}, nil
}

func (s *service) HWInitServod(context.Context, *bols.HWInitServodRequest) (*bols.HWInitServodResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method HWInitServod not implemented")
}

func (s *service) ReadServod(context.Context, *bols.ReadServodRequest) (*bols.ReadServodResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReadServod not implemented")
}

func (s *service) GetServod(ctx context.Context, req *bols.GetServodRequest) (*bols.GetServodResponse, error) {
	rpsn, err := xmlrpc.GetServod(ctx, req.GetStationId().GetContainerName(),
		req.GetStationId().GetServodPort(), req.GetControl())
	if err != nil {
		return nil, fmt.Errorf("failed to send get %s request to servod at port %d: %w",
			req.GetControl(), req.GetStationId().GetServodPort(), err)
	}
	return rpsn, nil
}

func (s *service) SetServod(ctx context.Context, req *bols.SetServodRequest) (*bols.SetServodResponse, error) {
	rpsn, err := xmlrpc.SetServod(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send set %s request to servod at port %d: %w",
			req.GetControl(), req.GetStationId().GetServodPort(), err)
	}
	return rpsn, nil
}

func (s *service) GetServodVersion(context.Context, *bols.GetServodVersionRequest) (*bols.GetServodVersionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetServodVersion not implemented")
}

func (s *service) EchoServod(context.Context, *bols.EchoServodRequest) (*bols.EchoServodResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method EchoServod not implemented")
}

func (s *service) GetServoTopology(context.Context, *bols.GetServoTopologyRequest) (*bols.GetServoTopologyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetServoTopology not implemented")
}

func (s *service) UpdateServoFirmware(context.Context, *bols.UpdateServoFirmwareRequest) (*bols.UpdateServoFirmwareResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateServoFirmware not implemented")
}

func (s *service) RunFutility(context.Context, *bols.RunFutilityRequest) (*bols.RunFutilityResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RunFutility not implemented")
}

func (s *service) RunFlashEC(context.Context, *bols.RunFlashECRequest) (*bols.RunFlashECResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RunFlashEC not implemented")
}

func (s *service) GetDolosVersion(context.Context, *bols.GetDolosVersionRequest) (*bols.GetDolosVersionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetDolosVersion not implemented")
}

func (s *service) UpdateDolosVersion(context.Context, *bols.UpdateDolosVersionRequest) (*bols.UpdateDolosVersionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateDolosVersion not implemented")
}

func (s *service) GetDolosStatus(context.Context, *bols.GetDolosStatusRequest) (*bols.GetDolosStatusResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetDolosStatus not implemented")
}

func (s *service) FindDolosUART(context.Context, *bols.FindDolosUARTRequest) (*bols.FindDolosUARTResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method FindDolosUART not implemented")
}

func startServod(ctx context.Context, containerName, board, model, serial, config string,
	recoveryMode bool, port int32, logger *log.Logger) error {
	const (
		servodRegistryUri             = "SERVOD_REGISTRY_URI"
		servodContainerlLabel         = "SERVOD_CONTAINER_LABEL"
		servodRegistryUriFallback     = "us-docker.pkg.dev/chromeos-partner-moblab/common-core"
		servodContainerlLabelFallback = "release"
	)
	if containerName == "" {
		return errors.New("servod docker container name is required")
	}
	servodDockerImagePath := fmt.Sprintf(
		"%s/servod:%s",
		misc.GetEnv(servodRegistryUri, servodRegistryUriFallback),
		misc.GetEnv(servodContainerlLabel, servodContainerlLabelFallback),
	)
	c, err := docker.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("fail to create docker client: %w", err)
	}
	// Force remove servod container if existed.
	// Ignore error if container does not exist.
	err = c.Remove(ctx, containerName, true)
	if err != nil {
		logger.Printf("Fail to remove container `%s`. Non-fatal\n", containerName)
	}

	containerEnvVars := []string{
		fmt.Sprintf("BOARD=%s", board),
		fmt.Sprintf("MODEL=%s", model),
		fmt.Sprintf("SERIAL=%s", serial),
		fmt.Sprintf("PORT=%d", port),
	}
	if config != "" {
		containerEnvVars = append(containerEnvVars, fmt.Sprintf("CONFIG=%s", config))
	}
	if recoveryMode {
		containerEnvVars = append(containerEnvVars, "REC_MODE=1")
	}
	execCmd := []string{"bash", "/start_servod.sh"}

	vols := []string{
		"/dev:/dev",
		fmt.Sprintf("%s_log:/var/log/servod_%d/", serial, port),
	}

	containerArgs := &docker.ContainerArgs{
		Detached:   true,
		Network:    "default_satlab",
		Privileged: true,
		ImageName:  servodDockerImagePath,
		EnvVar:     containerEnvVars,
		Exec:       execCmd,
		Volumes:    vols,
	}
	_, err = c.Start(ctx, containerName, containerArgs, time.Minute)
	if err != nil {
		return fmt.Errorf("fail to start docker client: %w", err)
	}
	if err := verifyServodDaemonIsUp(ctx, c, containerName, port, 60); err != nil {
		return fmt.Errorf("fail to verify docker client: %w", err)
	}
	return nil
}

func verifyServodDaemonIsUp(ctx context.Context, dockerClient docker.Client,
	dockerContainerName string, servodPort int32, waitTime int) error {
	args := []string{
		"servodtool",
		"instance",
		"wait-for-active",
		"-p",
		fmt.Sprintf("%d", servodPort),
		"--timeout",
		fmt.Sprintf("%d", waitTime),
	}
	if _, _, err := containerExecCmd(ctx, dockerClient, dockerContainerName, args, 2*time.Minute); err != nil {
		return fmt.Errorf("failed to exec servodtool: %w", err)
	}
	return nil
}

func containerExecCmd(ctx context.Context, dockerClient docker.Client, dockerContainerName string, args []string,
	timeout time.Duration) (stdout, stderr string, err error) {
	eReq := &docker.ExecRequest{
		Timeout: timeout,
		Cmd:     args,
	}
	res, err := dockerClient.Exec(ctx, dockerContainerName, eReq)
	if err != nil {
		return "", "", fmt.Errorf("failed to exec cmd %q: %w", strings.Join(args, " "), err)
	}
	if res != nil && res.ExitCode != 0 {
		return res.Stdout, res.Stderr,
			fmt.Errorf("command %s failed with exit code: %d, response: %s", args[0], res.ExitCode, res.Stderr)
	}
	return res.Stdout, res.Stderr, nil
}

func servodStatus(ctx context.Context, dockerClient docker.Client, dockerContainerName string,
	port int32) (bols.ServodStatus, error) {
	// Check if the docker client is up.
	isUp, err := dockerClient.IsUp(ctx, dockerContainerName)
	if err != nil {
		return bols.ServodStatus_SERVOD_UNKNOWN, fmt.Errorf("fail to check if servod container %s is up: %w",
			dockerContainerName, err)
	}
	if !isUp {
		return bols.ServodStatus_SERVOD_UNKNOWN, fmt.Errorf("servod container %s is not running", dockerContainerName)
	}
	return getServodStatus(ctx, dockerClient, dockerContainerName, port), nil
}

func getServodStatus(ctx context.Context, dockerClient docker.Client, dockerContainerName string,
	port int32) bols.ServodStatus {
	// TODO: Refactor this function to a command function so that we will have one common
	// function for both satlab and labstation.
	if _, _, err := containerExecCmd(ctx, dockerClient, dockerContainerName,
		[]string{"servodtool", "instance", "show", "-p", fmt.Sprintf("%d", port)},
		time.Minute); err == nil {
		return bols.ServodStatus_SERVOD_RUNNING
	}
	return bols.ServodStatus_SERVOD_STOPPED
}
