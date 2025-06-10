// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package server implement bols-service API.
package server

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/config/go/test/api/bols"

	"go.chromium.org/infra/cros/lib/bols/util"
	"go.chromium.org/infra/cros/lib/bols/xmlrpc"
	"go.chromium.org/infra/cros/recovery/docker"
	"go.chromium.org/infra/cros/satlab/common/utils/misc"
)

const bufSize int = 1024 * 1024

func (s *service) GetFileStat(context.Context, *bols.GetFileStatRequest) (*bols.GetFileStatResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetFileStat not implemented")
}

func (s *service) GetFile(req *bols.GetFileRequest, stream bols.BolsService_GetFileServer) error {
	s.logger.Println("Receive GetFile request for file ", req.GetFilename())
	ctx := stream.Context()
	args := []string{"realpath", req.GetFilename()}
	containerName := req.StationId.GetContainerName()
	c, err := dockerClient(ctx, containerName)
	if err != nil {
		return s.logAndReturnErr(fmt.Errorf("fail to create docker client: %w", err))
	}
	timeout := timeRemaining(ctx)
	eReq := &docker.ExecRequest{
		Timeout: timeout,
		Cmd:     args,
	}
	res, err := c.Exec(ctx, req.GetStationId().GetContainerName(), eReq)
	if err != nil {
		return s.logAndReturnErr(fmt.Errorf("failed to exec cmd %q: %w", strings.Join(args, " "), err))
	}
	realpath := strings.TrimSpace(string(res.Stdout))

	tempDir, err := os.MkdirTemp("", "getfile-*")
	if err != nil {
		return s.logAndReturnErr(fmt.Errorf("failed to create temporary directory: %w", err))
	}
	defer os.RemoveAll(tempDir) // Clean up the directory when done
	tarFileName := filepath.Join(tempDir, fmt.Sprintf("%s.*.tar", filepath.Base(realpath)))
	if err := c.CopyFrom(ctx, containerName, realpath, tarFileName); err != nil {
		return s.logAndReturnErr(fmt.Errorf("failed to copy %s from container %s: %v",
			realpath, containerName, err))
	}
	tempFileName := filepath.Join(tempDir, filepath.Base(realpath))
	if err := extractOneFileFromTarFile(tempFileName, tarFileName); err != nil {
		return s.logAndReturnErr(fmt.Errorf("failed to extract %s from tar file: %v",
			realpath, err))
	}

	file, err := os.Open(tempFileName)
	if err != nil {
		return s.logAndReturnErr(fmt.Errorf("failed to open %s: %w", tempFileName, err))
	}
	defer file.Close()
	buffer := make([]byte, bufSize)
	for {
		n, err := file.Read(buffer)
		if err != nil {
			if err != io.EOF {
				return s.logAndReturnErr(fmt.Errorf("failed to read %s: %w", tempFileName, err))
			}
			break // End of file
		}
		stream.Send(&bols.GetFileResponse{
			Data: buffer[:n],
		})
	}
	s.logger.Println("Served GetFile Request Successfully")
	return nil
}

func (s *service) PutFile(stream bols.BolsService_PutFileServer) error {
	s.logger.Println("Receive PutFile Request")
	var f *os.File
	var fn string
	var dir string
	var containerName string
	ctx := stream.Context()

	tempDir, err := os.MkdirTemp("", "putfile-*")
	if err != nil {
		return s.logAndReturnErr(fmt.Errorf("failed to create temporary directory: %w", err))
	}
	defer os.RemoveAll(tempDir) // Clean up the directory when done
	timeout := timeRemaining(ctx)
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			if f == nil {
				return s.logAndReturnErr(errors.New("stream was closed prematurely"))
			}
			f.Close()
			c, err := dockerClient(ctx, containerName)
			if err != nil {
				return s.logAndReturnErr(fmt.Errorf("fail to create docker client: %w", err))
			}
			if err := mkdirInContainer(ctx, c, containerName, dir, timeout); err != nil {
				return s.logAndReturnErr(fmt.Errorf("failed to make directory %s in container %s: %w",
					dir, containerName, err))
			}
			tarFileName := fmt.Sprintf("%s.tar", f.Name())
			if err := copyFileToTar(f.Name(), tarFileName); err != nil {
				return s.logAndReturnErr(fmt.Errorf("failed to create tar file %s: %w",
					tarFileName, err))
			}
			if err := c.CopyTo(ctx, containerName, tarFileName, dir); err != nil {
				return s.logAndReturnErr(fmt.Errorf("failed to copy file %s to container %s as %s: %w",
					f.Name(), containerName, fn, err))
			}
			break
		}
		if err != nil {
			return fmt.Errorf("failed to receive streaming data: %w", err)
		}
		switch {
		case req.GetReqInfo() != nil:
			info := req.GetReqInfo()
			containerName = info.GetStationId().GetContainerName()
			fn = info.GetFilename()
			dir = filepath.Dir(fn)
			base := filepath.Base(fn)
			f, err = os.OpenFile(filepath.Join(tempDir, base), os.O_RDWR|os.O_CREATE, 0644)
			if err != nil {
				return s.logAndReturnErr(
					fmt.Errorf("failed to create temporary file for %s: %w", fn, err))
			}
			s.logger.Println("PutFile Request destination file: ", fn)
		case req.GetData() != nil:
			if f == nil {
				return s.logAndReturnErr(errors.New("data was send before file name"))
			}
			if _, err := f.Write(req.GetData()); err != nil {
				return s.logAndReturnErr(
					fmt.Errorf("failed to write file %s: %w", fn, err))
			}
		}
	}
	stream.SendAndClose(&bols.PutFileResponse{})
	s.logger.Println("Served PutFile Request Successfully")
	return nil
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

func (s *service) DMesg(req *bols.DMesgRequest, stream bols.BolsService_DMesgServer) error {
	ctx := stream.Context()
	args := []string{"dmesg", "-H"}
	c, err := dockerClient(ctx, req.StationId.GetContainerName())
	if err != nil {
		return s.logAndReturnErr(fmt.Errorf("fail to create docker client: %w", err))
	}

	tmpStdout, err := os.CreateTemp("", "dmesg-stdout-*.txt")
	if err != nil {
		return s.logAndReturnErr(
			fmt.Errorf("failed to create temporary file to capture dmesg stdout: %w", err))
	}
	stdoutFileName := tmpStdout.Name()
	defer os.Remove(stdoutFileName)
	timeout := timeRemaining(ctx)
	eReq := &docker.ExecRequest{
		Timeout: timeout,
		Cmd:     args,
		Stdout:  tmpStdout,
	}
	res, err := c.Exec(ctx, req.GetStationId().GetContainerName(), eReq)
	tmpStdout.Close()
	if err != nil {
		return s.logAndReturnErr(
			fmt.Errorf("failed to exec cmd %q: %w", strings.Join(args, " "), err))
	}
	if res != nil && res.ExitCode != 0 {
		return s.logAndReturnErr(
			fmt.Errorf("command %s failed with exit code: %d, response: %s", args[0], res.ExitCode, res.Stderr))
	}
	buffer := make([]byte, bufSize)
	file, err := os.Open(stdoutFileName)
	if err != nil {
		return s.logAndReturnErr(
			fmt.Errorf("failed to open %s for stdout of dmesg: %w", stdoutFileName, err))
	}
	defer file.Close()
	for {
		n, err := file.Read(buffer)
		if err != nil {
			if err != io.EOF {
				return s.logAndReturnErr(
					fmt.Errorf("failed to read %s for stdout of dmesg: %w", stdoutFileName, err))
			}
			break // End of file
		}
		stream.Send(&bols.DMesgResponse{
			Output: &bols.OutputStream{
				Stdout: buffer[:n],
			},
		})
	}
	return nil
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
		return nil, s.logAndReturnErr(fmt.Errorf("fail to start servod: %w", err))
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

func (s *service) GetServodVersion(ctx context.Context, req *bols.GetServodVersionRequest) (*bols.GetServodVersionResponse, error) {
	rpsn, err := xmlrpc.GetServodVersion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get version of servod at port %d: %w",
			req.GetStationId().GetServodPort(), err)
	}
	return rpsn, nil
}

func (s *service) EchoServod(ctx context.Context, req *bols.EchoServodRequest) (*bols.EchoServodResponse, error) {
	rpsn, err := xmlrpc.EchoServod(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send echo request to servod at port %d: %w",
			req.GetStationId().GetServodPort(), err)
	}
	return rpsn, nil
}

func (s *service) GetServoTopology(context.Context, *bols.GetServoTopologyRequest) (*bols.GetServoTopologyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetServoTopology not implemented")
}

func (s *service) UpdateServoFirmware(context.Context, *bols.UpdateServoFirmwareRequest) (*bols.UpdateServoFirmwareResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateServoFirmware not implemented")
}

func (s *service) RunFutility(ctx context.Context, req *bols.RunFutilityRequest) (*bols.RunFutilityResponse, error) {
	if err := util.CheckFutilityParams(req); err != nil {
		return nil, fmt.Errorf("failed to validate parameters: %w", err)
	}
	args := append([]string{"futility"}, req.GetParams()...)
	c, err := docker.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to create docker client: %w", err)
	}
	stdout, stderr, err := containerExecCmd(ctx, c, req.StationId.GetContainerName(), args,
		timeRemaining(ctx))
	if err != nil {
		return nil, fmt.Errorf("fail to execute futility command: %w", err)
	}
	return &bols.RunFutilityResponse{
		Output: &bols.OutputStream{
			Stdout: []byte(stdout),
			Stderr: []byte(stderr),
		}}, nil
}

func (s *service) logAndReturnErr(err error) error {
	if s.logger == nil {
		return err
	}
	prefix := ""
	pc, _, _, ok := runtime.Caller(1) // Skip 1 frame to get the caller
	if ok {
		funcInfo := runtime.FuncForPC(pc)
		if funcInfo != nil {
			prefix = fmt.Sprintf("%s:", funcInfo.Name())
		}
	}
	s.logger.Println(prefix, err)
	return err
}

func (s *service) RunFlashEC(ctx context.Context, req *bols.RunFlashECRequest) (*bols.RunFlashECResponse, error) {
	if err := util.CheckFlashECParams(req); err != nil {
		return nil, fmt.Errorf("failed to validate parameters: %w", err)
	}
	args := append([]string{"flash_ec"}, req.GetParams()...)
	c, err := docker.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to create docker client: %w", err)
	}
	stdout, stderr, err := containerExecCmd(ctx, c, req.StationId.GetContainerName(), args,
		timeRemaining(ctx))
	if err != nil {
		return nil, fmt.Errorf("fail to execute flash_ec command: %w", err)
	}
	return &bols.RunFlashECResponse{
		Output: &bols.OutputStream{
			Stdout: []byte(stdout),
			Stderr: []byte(stderr),
		}}, nil
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

func dockerClient(ctx context.Context, containerName string) (docker.Client, error) {
	if containerName == "" {
		return nil, errors.New("servod docker container name is required")
	}
	c, err := docker.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to create docker client: %w", err)
	}
	return c, nil
}

func timeRemaining(ctx context.Context) time.Duration {
	const defaultTimeout = time.Minute
	if deadline, ok := ctx.Deadline(); ok {
		return time.Until(deadline)
	}
	return defaultTimeout
}

func mkdirInContainer(ctx context.Context, c docker.Client,
	containerName, path string, timeout time.Duration) error {
	args := []string{"mkdir", "-p", path}
	req := &docker.ExecRequest{
		Timeout: timeout,
		Cmd:     args,
	}
	if _, err := c.Exec(ctx, containerName, req); err != nil {
		return fmt.Errorf("failed to exec cmd %q: %w", strings.Join(args, " "), err)
	}
	return nil
}

func copyFileToTar(filePath, tarFileName string) error {
	tarfile, err := os.Create(tarFileName)
	if err != nil {
		return fmt.Errorf("failed to creat tar file %s: %w", tarFileName, err)
	}
	defer tarfile.Close()

	// Create a new tar archive.
	tw := tar.NewWriter(tarfile)
	defer tw.Close()

	// Add a file to the archive.
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", filePath, err)
	}
	defer file.Close()

	// Get the file info.
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file stat from %s: %w", filePath, err)
	}

	// Create a tar header.
	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return fmt.Errorf("failed to create tar header: %w", err)
	}

	// Write the header.
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}

	// Copy the file data to the tar writer.
	if _, err := io.Copy(tw, file); err != nil {
		return fmt.Errorf("failed to copy %s tar file %s archive: %w", filePath, tarFileName, err)
	}
	return nil
}

func extractOneFileFromTarFile(filePath, tarFileName string) error {
	tarFileReader, err := os.Open(tarFileName)
	if err != nil {
		return fmt.Errorf("failed to open tar file %s: %w", tarFileName, err)
	}
	outFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	tr := tar.NewReader(tarFileReader)
	if _, err := tr.Next(); err != nil {
		return fmt.Errorf("failed to get content from tar file %s: %w", tarFileName, err)
	}
	if _, err = io.Copy(outFile, tr); err != nil {
		return fmt.Errorf("failed to copy content from tar file %s to %s: %w", tarFileName, filePath, err)
	}
	return outFile.Close()
}
