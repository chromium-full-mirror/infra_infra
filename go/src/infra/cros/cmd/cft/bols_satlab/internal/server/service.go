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
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/config/go/test/api/bols"

	"go.chromium.org/infra/cros/lib/bols/util"
	"go.chromium.org/infra/cros/lib/bols/xmlrpc"
	"go.chromium.org/infra/cros/recovery/docker"
	"go.chromium.org/infra/cros/satlab/common/utils/misc"
	servod_xmlrpc "go.chromium.org/infra/cros/servo/xmlrpc"
)

const bufSize int = 1024 * 1024

func (s *service) GetFileStat(ctx context.Context, req *bols.GetFileStatRequest) (*bols.GetFileStatResponse, error) {
	s.logger.Printf("Receive GetFileStat request for file %q in container %q", req.GetFilepath(), req.GetStationId().GetContainerName())
	containerName := req.GetStationId().GetContainerName()
	filePath := req.GetFilepath()
	if filePath == "" {
		return nil, s.logAndReturnErrorf("filepath is required")
	}
	c, err := dockerClient(ctx, containerName)
	if err != nil {
		return nil, s.logAndReturnErrorf("fail to create docker client: %w", err)
	}
	stat, err := c.ContainerStatPath(ctx, containerName, filePath)
	if err != nil {
		return nil, s.logAndReturnErrorf("failed to stat path %q in container %q: %w", filePath, containerName, err)
	}

	s.logger.Println("Served GetFileStat Request Successfully")
	return &bols.GetFileStatResponse{
		FileStats: &bols.FileStat{
			Name:      stat.Name,
			Path:      filePath,
			Size:      stat.Size,
			IsDir:     stat.Mode.IsDir(),
			IsSymlink: (stat.Mode & os.ModeSymlink) != 0,
		},
	}, nil
}

func (s *service) GetFile(req *bols.GetFileRequest, stream bols.BolsService_GetFileServer) error {
	s.logger.Println("Receive GetFile request for file ", req.GetFilename())
	ctx := stream.Context()
	args := []string{"realpath", req.GetFilename()}
	containerName := req.StationId.GetContainerName()
	c, err := dockerClient(ctx, containerName)
	if err != nil {
		return s.logAndReturnErrorf("fail to create docker client: %w", err)
	}
	timeout := timeRemaining(ctx)
	eReq := &docker.ExecRequest{
		Timeout: timeout,
		Cmd:     args,
	}
	res, err := c.Exec(ctx, req.GetStationId().GetContainerName(), eReq)
	if err != nil {
		return s.logAndReturnErrorf("failed to exec cmd %q: %w", strings.Join(args, " "), err)
	}
	realpath := strings.TrimSpace(string(res.Stdout))

	tempDir, err := createTempDir("getfile-*")
	if err != nil {
		return s.logAndReturnErrorf("failed to create temporary directory for GetFile: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up the directory when done

	tarFileName := filepath.Join(tempDir, fmt.Sprintf("%s.*.tar", filepath.Base(realpath)))
	if err := c.CopyFrom(ctx, containerName, realpath, tarFileName); err != nil {
		return s.logAndReturnErrorf("failed to copy %s from container %s: %v",
			realpath, containerName, err)
	}
	tempFileName := filepath.Join(tempDir, filepath.Base(realpath))
	if err := extractOneFileFromTarFile(tempFileName, tarFileName); err != nil {
		return s.logAndReturnErrorf("failed to extract %s from tar file: %v",
			realpath, err)
	}

	file, err := os.Open(tempFileName)
	if err != nil {
		return s.logAndReturnErrorf("failed to open %s: %w", tempFileName, err)
	}
	defer file.Close()
	buffer := make([]byte, bufSize)
	for {
		n, err := file.Read(buffer)
		if err != nil {
			if err != io.EOF {
				return s.logAndReturnErrorf("failed to read %s: %w", tempFileName, err)
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

	tempDir, err := createTempDir("putfile-*")
	if err != nil {
		return s.logAndReturnErrorf("failed to create temporary directory for PutFile: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up the directory when done

	timeout := timeRemaining(ctx)
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			if f == nil {
				return s.logAndReturnErrorf("stream was closed prematurely")
			}
			f.Close()
			c, err := dockerClient(ctx, containerName)
			if err != nil {
				return s.logAndReturnErrorf("fail to create docker client: %w", err)
			}
			if err := mkdirInContainer(ctx, c, containerName, dir, timeout); err != nil {
				return s.logAndReturnErrorf("failed to make directory %s in container %s: %w",
					dir, containerName, err)
			}
			tarFileName := fmt.Sprintf("%s.tar", f.Name())
			if err := copyFileToTar(f.Name(), tarFileName); err != nil {
				return s.logAndReturnErrorf("failed to create tar file %s: %w",
					tarFileName, err)
			}
			if err := c.CopyTo(ctx, containerName, tarFileName, dir); err != nil {
				return s.logAndReturnErrorf("failed to copy file %s to container %s as %s: %w",
					f.Name(), containerName, fn, err)
			}
			break
		}
		if err != nil {
			return s.logAndReturnErrorf("failed to receive streaming data: %w", err)
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
				return s.logAndReturnErrorf("failed to create temporary file for %s: %w", fn, err)
			}
			s.logger.Println("PutFile Request destination file: ", fn)
		case req.GetData() != nil:
			if f == nil {
				return s.logAndReturnErrorf("data was send before file name")
			}
			if _, err := f.Write(req.GetData()); err != nil {
				return s.logAndReturnErrorf("failed to write file %s: %w", fn, err)
			}
		}
	}
	stream.SendAndClose(&bols.PutFileResponse{})
	s.logger.Println("Served PutFile Request Successfully")
	return nil
}

func (s *service) DownloadFile(ctx context.Context, req *bols.DownloadFileRequest) (*bols.DownloadFileResponse, error) {
	s.logger.Println("Receive DownloadFile Request for url", req.GetUrl())
	containerName := req.GetStationId().GetContainerName()
	destDir := req.GetDest()
	url := req.GetUrl()
	if destDir == "" || url == "" {
		return nil, s.logAndReturnErrorf("DownloadFile: destination directory, and URL are required")
	}

	c, err := dockerClient(ctx, containerName)
	if err != nil {
		return nil, s.logAndReturnErrorf("DownloadFile: fail to create docker client: %w", err)
	}

	// Ensure destination directory exists.
	if err := mkdirInContainer(ctx, c, containerName, destDir, timeRemaining(ctx)); err != nil {
		return nil, s.logAndReturnErrorf("DownloadFile: failed to create destination directory %s: %w", destDir, err)
	}

	fileName := filepath.Base(url)
	fullDestPath := filepath.Join(destDir, fileName)
	args := []string{"curl", "-f", "-L", "-sS", "-o", fullDestPath}
	for _, header := range req.GetHeaders() {
		args = append(args, "-H", fmt.Sprintf("%s: %s", header.GetKey(), header.GetValue()))
	}
	args = append(args, url)

	if _, _, err := containerExecCmd(ctx, c, containerName, args, timeRemaining(ctx)); err != nil {
		return nil, s.logAndReturnErrorf("DownloadFile: failed to execute curl: %w", err)
	}

	s.logger.Println("Served DownloadFile Request Successfully, file at", fullDestPath)
	return &bols.DownloadFileResponse{File: fullDestPath}, nil
}

func (s *service) RemoveFile(ctx context.Context, req *bols.RemoveFileRequest) (*bols.RemoveFileResponse, error) {
	s.logger.Println("Receive RemoveFile Request for file", req.GetFilename())
	containerName := req.GetStationId().GetContainerName()
	filename := req.GetFilename()
	if filename == "" {
		return nil, s.logAndReturnErrorf("RemoveFile: file name is required")
	}
	c, err := dockerClient(ctx, containerName)
	if err != nil {
		return nil, s.logAndReturnErrorf("RemoveFile: fail to create docker client: %w", err)
	}

	args := []string{"rm", filename}
	if _, _, err := containerExecCmd(ctx, c, containerName, args, timeRemaining(ctx)); err != nil {
		return nil, s.logAndReturnErrorf("RemoveFile: failed to execute %q: %w", strings.Join(args, " "), err)
	}

	s.logger.Println("Served RemoveFile Request Successfully")
	return &bols.RemoveFileResponse{}, nil
}

func (s *service) GetDirInfo(ctx context.Context, req *bols.GetDirInfoRequest) (*bols.GetDirInfoResponse, error) {
	s.logger.Println("Receive GetDirInfo Request for path", req.GetPath())
	containerName := req.GetStationId().GetContainerName()
	path := req.GetPath()
	if path == "" {
		return nil, s.logAndReturnErrorf("GetDirInfo: path is required")
	}
	c, err := dockerClient(ctx, containerName)
	if err != nil {
		return nil, s.logAndReturnErrorf("GetDirInfo: fail to create docker client: %w", err)
	}

	// Using find  stat is more robust than parsing 'ls -l'.
	// -maxdepth 1 and -mindepth 1 ensures we only get the immediate contents of the directory.
	// %F - file type
	// %s - size in bytes
	// %n - file name
	args := []string{"find", path, "-maxdepth", "1", "-mindepth", "1", "-exec", "stat", "--format=%F|%s|%n", "{}", "+"}
	stdout, _, err := containerExecCmd(ctx, c, containerName, args, timeRemaining(ctx))
	if err != nil {
		// containerExecCmd already annotates the error well enough.
		return nil, s.logAndReturnErrorf("GetDirInfo: failed to list directory contents: %w", err)
	}

	fileStats, err := parseDirInfo(stdout)
	if err != nil {
		return nil, s.logAndReturnErrorf("GetDirInfo: failed to parse directory info: %w", err)
	}

	s.logger.Println("Served GetDirInfo Request Successfully")
	return &bols.GetDirInfoResponse{
		Info: &bols.DirectoryInfo{
			Path:      path,
			FileStats: fileStats,
		},
	}, nil
}

func (s *service) MakeDir(ctx context.Context, req *bols.MakeDirRequest) (*bols.MakeDirResponse, error) {
	s.logger.Println("Receive MakeDir Request for path", req.GetPath())
	containerName := req.GetStationId().GetContainerName()
	path := req.GetPath()
	if containerName == "" || path == "" {
		return nil, s.logAndReturnErrorf("MakeDir: container name and path are required")
	}

	c, err := dockerClient(ctx, containerName)
	if err != nil {
		return nil, s.logAndReturnErrorf("MakeDir: fail to create docker client: %w", err)
	}

	if err := mkdirInContainer(ctx, c, containerName, path, timeRemaining(ctx)); err != nil {
		return nil, s.logAndReturnErrorf("MakeDir: failed to make directory %s in container %s: %w",
			path, containerName, err)
	}

	s.logger.Println("Served MakeDir Request Successfully")
	return &bols.MakeDirResponse{}, nil
}

func (s *service) MakeTempDir(ctx context.Context, req *bols.MakeTempDirRequest) (*bols.MakeTempDirResponse, error) {
	s.logger.Println("Receive MakeTempDir Request")
	containerName := req.GetStationId().GetContainerName()

	c, err := dockerClient(ctx, containerName)
	if err != nil {
		return nil, s.logAndReturnErrorf("MakeTempDir: fail to create docker client: %w", err)
	}

	args := []string{"mktemp", "-d"}
	if d := req.GetDir(); d != "" {
		// Use --tmpdir for mktemp to specify the parent directory.
		args = append(args, fmt.Sprintf("--tmpdir=%s", d))
	}
	if p := req.GetPattern(); p != "" {
		// mktemp uses a template that must end in X's.
		// If the pattern contains a '*', replace it with 'XXXXX' to create
		// a valid mktemp template. Otherwise, use the pattern as is.
		p = strings.ReplaceAll(p, "*", "XXXXX")
		args = append(args, p)
	}

	stdout, _, err := containerExecCmd(ctx, c, containerName, args, timeRemaining(ctx))
	if err != nil {
		return nil, s.logAndReturnErrorf("MakeTempDir: failed to execute mktemp: %w", err)
	}

	tempDirPath := strings.TrimSpace(stdout)
	if tempDirPath == "" {
		return nil, s.logAndReturnErrorf("MakeTempDir: mktemp did not return a path")
	}

	s.logger.Println("Served MakeTempDir Request Successfully, created", tempDirPath)
	return &bols.MakeTempDirResponse{
		Info: &bols.DirectoryInfo{
			Path:      tempDirPath,
			FileStats: []*bols.FileStat{}, // A new temp directory is empty.
		},
	}, nil
}

func (s *service) RemoveDir(ctx context.Context, req *bols.RemoveDirRequest) (*bols.RemoveDirResponse, error) {
	s.logger.Println("Receive RemoveDir Request for path", req.GetPath())
	containerName := req.GetStationId().GetContainerName()
	path := req.GetPath()
	if containerName == "" || path == "" {
		return nil, s.logAndReturnErrorf("RemoveDir: container name and path are required")
	}
	c, err := dockerClient(ctx, containerName)
	if err != nil {
		return nil, s.logAndReturnErrorf("RemoveDir: fail to create docker client: %w", err)
	}
	if err := rmdirInContainer(ctx, c, containerName, path, req.GetRemoveAll(), timeRemaining(ctx)); err != nil {
		return nil, s.logAndReturnErrorf("RemoveDir: failed to remove directory: %w", err)
	}

	s.logger.Println("Served RemoveDir Request Successfully")
	return &bols.RemoveDirResponse{}, nil
}

func (s *service) DMesg(req *bols.DMesgRequest, stream bols.BolsService_DMesgServer) error {
	s.logger.Println("Receive DMesg Request")
	ctx := stream.Context()
	args := []string{"dmesg", "-H"}
	c, err := dockerClient(ctx, req.StationId.GetContainerName())
	if err != nil {
		return s.logAndReturnErrorf("fail to create docker client: %w", err)
	}

	tmpStdout, err := createTempDir("dmesg-stdout-*.txt") // Using createTempDir for consistency, though it creates a dir
	if err != nil {
		return s.logAndReturnErrorf("failed to create temporary directory for dmesg stdout: %w", err)
	}
	defer os.RemoveAll(tmpStdout) // Clean up the directory
	// We need a file, not a directory for os.CreateTemp behavior. Let's revert this part for dmesg.
	// Reverting dmesg temp file creation to os.CreateTemp as it needs a file, not a dir.
	// The createTempDir helper is for directories.

	stdoutFile, err := os.CreateTemp("", "dmesg-stdout-*.txt")
	if err != nil {
		return s.logAndReturnErrorf("failed to create temporary file to capture dmesg stdout: %w", err)
	}
	stdoutFileName := stdoutFile.Name()
	defer os.Remove(stdoutFileName) // Clean up the file

	timeout := timeRemaining(ctx)
	eReq := &docker.ExecRequest{
		Timeout: timeout,
		Cmd:     args,
		Stdout:  stdoutFile, // Pass the *os.File object
	}
	res, err := c.Exec(ctx, req.GetStationId().GetContainerName(), eReq)
	stdoutFile.Close() // Close the file after exec

	if err != nil {
		return s.logAndReturnErrorf("failed to exec cmd %q: %w", strings.Join(args, " "), err)
	}
	if res != nil && res.ExitCode != 0 {
		return s.logAndReturnErrorf("command %s failed with exit code: %d, stderr: %s", args[0], res.ExitCode, res.Stderr)
	}
	buffer := make([]byte, bufSize)
	file, err := os.Open(stdoutFileName)
	if err != nil {
		return s.logAndReturnErrorf("failed to open %s for stdout of dmesg: %w", stdoutFileName, err)
	}
	defer file.Close()
	for {
		n, err := file.Read(buffer)
		if err != nil {
			if err != io.EOF {
				return s.logAndReturnErrorf("failed to read %s for stdout of dmesg: %w", stdoutFileName, err)
			}
			break // End of file
		}
		stream.Send(&bols.DMesgResponse{
			Output: &bols.OutputStream{
				Stdout: buffer[:n],
			},
		})
	}
	s.logger.Println("Served DMesg Request Successfully")
	return nil
}

func (s *service) WriteFileByBlock(bols.BolsService_WriteFileByBlockServer) error {
	return status.Errorf(codes.Unimplemented, "method WriteFileByBlock not implemented")
}

func (s *service) ReadFileByBlock(*bols.ReadFileByBlockRequest, bols.BolsService_ReadFileByBlockServer) error {
	return status.Errorf(codes.Unimplemented, "method ReadFileByBlock not implemented")
}

func (s *service) RunMount(ctx context.Context, req *bols.RunMountRequest) (*bols.RunMountResponse, error) {
	s.logger.Printf("Receive RunMount Request: src=%s, dest=%s", req.GetSrc(), req.GetDest())

	containerName := req.GetStationId().GetContainerName()
	src := req.GetSrc()
	dest := req.GetDest()
	if src == "" || dest == "" {
		return nil, s.logAndReturnErrorf("RunMount: container name, source, and destination are required")
	}

	c, err := dockerClient(ctx, containerName)
	if err != nil {
		return nil, s.logAndReturnErrorf("RunMount: fail to create docker client: %w", err)
	}

	args := []string{"mount"}
	args = append(args, req.GetParams()...)
	args = append(args, src, dest)

	if _, _, err := containerExecCmd(ctx, c, containerName, args, timeRemaining(ctx)); err != nil {
		return nil, s.logAndReturnErrorf("RunMount: failed to execute mount command: %w", err)
	}
	s.logger.Println("Served RunMount Request Successfully")
	return &bols.RunMountResponse{}, nil
}

func (s *service) RunUMount(ctx context.Context, req *bols.RunUMountRequest) (*bols.RunUMountResponse, error) {
	s.logger.Printf("Receive RunUMount Request for path %s", req.GetPath())

	containerName := req.GetStationId().GetContainerName()
	path := req.GetPath()
	if path == "" {
		return nil, s.logAndReturnErrorf("RunUMount: container name and path are required")
	}

	c, err := dockerClient(ctx, containerName)
	if err != nil {
		return nil, s.logAndReturnErrorf("RunUMount: fail to create docker client: %w", err)
	}

	args := []string{"umount", path}

	if _, _, err := containerExecCmd(ctx, c, containerName, args, timeRemaining(ctx)); err != nil {
		return nil, s.logAndReturnErrorf("RunUMount: failed to execute umount command: %w", err)
	}
	s.logger.Println("Served RunUMount Request Successfully")
	return &bols.RunUMountResponse{}, nil
}

func (s *service) StartServod(ctx context.Context, req *bols.StartServodRequest) (*bols.StartServodResponse, error) {
	s.logger.Println("Receive StartServod Request")
	if err := startServod(ctx,
		req.GetStationId().GetContainerName(),
		req.GetBoard(), req.GetModel(), req.GetStationId().GetServoSerial(), req.GetConfig(),
		req.GetRecoveryMode(),
		req.GetStationId().GetServodPort(), s.logger); err != nil {
		return nil, s.logAndReturnErrorf("fail to start servod: %w", err)
	}
	s.logger.Println("Served StartServod Request Successfully")
	return &bols.StartServodResponse{}, nil
}

func (s *service) StopServod(ctx context.Context, req *bols.StopServodRequest) (*bols.StopServodResponse, error) {
	s.logger.Println("Receive StopServod Request")
	containerName := req.GetStationId().GetContainerName()

	c, err := dockerClient(ctx, containerName)
	if err != nil {
		return nil, s.logAndReturnErrorf("StopServod: fail to create docker client: %w", err)
	}

	isUp, err := c.IsUp(ctx, containerName)
	if err != nil {
		return nil, s.logAndReturnErrorf("StopServod: failed to check status of container %q: %w", containerName, err)
	}
	if !isUp {
		s.logger.Printf("StopServod: container %q is not running, considering it already stopped.", containerName)
		return &bols.StopServodResponse{}, nil
	}

	if err := c.Remove(ctx, containerName, true); err != nil {
		return nil, s.logAndReturnErrorf("StopServod: failed to remove container %q: %w", containerName, err)
	}

	s.logger.Println("Served StopServod Request Successfully")
	return &bols.StopServodResponse{}, nil
}

func (s *service) GetServodStatus(ctx context.Context, req *bols.GetServodStatusRequest) (*bols.GetServodStatusResponse, error) {
	s.logger.Println("Receive GetServodStatus Request")
	c, err := docker.NewClient(ctx) // Not using dockerClient helper here, direct call
	if err != nil {
		return nil, s.logAndReturnErrorf("fail to create docker client: %w", err)
	}
	statusVal, err := servodStatus(ctx, c, req.GetStationId().GetContainerName(), req.GetStationId().GetServodPort())
	if err != nil {
		return nil, s.logAndReturnErrorf("fail to get servod status: %w", err)
	}
	s.logger.Println("Served GetServodStatus Request Successfully")
	return &bols.GetServodStatusResponse{Status: statusVal}, nil
}

// HWInitServod calls hwinit of servod by delegating to the xmlrpc package.
func (s *service) HWInitServod(ctx context.Context, req *bols.HWInitServodRequest) (*bols.HWInitServodResponse, error) {
	s.logger.Println("Receive HWInitServod Request")

	cl, err := xmlrpcClient(req.GetStationId())
	if err != nil {
		return nil, s.logAndReturnErrorf("failed create xmlrpc client: %w", err)
	}
	if err := xmlrpc.HWInitServod(ctx, cl); err != nil {
		return nil, s.logAndReturnErrorf("failed to execute hwinit on servod via xmlrpc client: %w", err)
	}

	s.logger.Println("Served HWInitServod Request Successfully")
	return &bols.HWInitServodResponse{}, nil
}

// DocServod reads a servod control documentation.
func (s *service) DocServod(ctx context.Context, req *bols.DocServodRequest) (*bols.DocServodResponse, error) {
	s.logger.Printf("Receive DocServod Request for control %q", req.GetControl())
	control := req.GetControl()

	cl, err := xmlrpcClient(req.GetStationId())
	if err != nil {
		return nil, s.logAndReturnErrorf("failed create xmlrpc client: %w", err)
	}
	docString, err := xmlrpc.DocServod(ctx, control, cl)
	if err != nil {
		return nil, s.logAndReturnErrorf("failed to get doc for control %q from servod: %w", control, err)
	}

	s.logger.Println("Served DocServod Request Successfully")
	return &bols.DocServodResponse{
		Control:     control,
		Description: docString,
	}, nil
}

func (s *service) GetServod(ctx context.Context, req *bols.GetServodRequest) (*bols.GetServodResponse, error) {
	s.logger.Println("Receive GetServod Request")
	rpsn, err := xmlrpc.GetServod(ctx, req.GetStationId().GetContainerName(),
		req.GetStationId().GetServodPort(), req.GetControl())
	if err != nil {
		return nil, s.logAndReturnErrorf("failed to send get %s request to servod at port %d: %w",
			req.GetControl(), req.GetStationId().GetServodPort(), err)
	}
	s.logger.Println("Served GetServod Request Successfully")
	return rpsn, nil
}

func (s *service) SetServod(ctx context.Context, req *bols.SetServodRequest) (*bols.SetServodResponse, error) {
	s.logger.Println("Receive SetServod Request")
	rpsn, err := xmlrpc.SetServod(ctx, req)
	if err != nil {
		return nil, s.logAndReturnErrorf("failed to send set %s request to servod at port %d: %w",
			req.GetControl(), req.GetStationId().GetServodPort(), err)
	}
	s.logger.Println("Served SetServod Request Successfully")
	return rpsn, nil
}

func (s *service) GetServodVersion(ctx context.Context, req *bols.GetServodVersionRequest) (*bols.GetServodVersionResponse, error) {
	s.logger.Println("Receive GetServodVersion Request")
	rpsn, err := xmlrpc.GetServodVersion(ctx, req)
	if err != nil {
		return nil, s.logAndReturnErrorf("failed to get version of servod at port %d: %w",
			req.GetStationId().GetServodPort(), err)
	}
	s.logger.Println("Served GetServodVersion Request Successfully")
	return rpsn, nil
}

func (s *service) EchoServod(ctx context.Context, req *bols.EchoServodRequest) (*bols.EchoServodResponse, error) {
	s.logger.Println("Receive EchoServod Request")
	rpsn, err := xmlrpc.EchoServod(ctx, req)
	if err != nil {
		return nil, s.logAndReturnErrorf("failed to send echo request to servod at port %d: %w",
			req.GetStationId().GetServodPort(), err)
	}
	s.logger.Println("Served EchoServod Request Successfully")
	return rpsn, nil
}

func (s *service) GetServoTopology(context.Context, *bols.GetServoTopologyRequest) (*bols.GetServoTopologyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetServoTopology not implemented")
}

func (s *service) UpdateServoFirmware(context.Context, *bols.UpdateServoFirmwareRequest) (*bols.UpdateServoFirmwareResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateServoFirmware not implemented")
}

func (s *service) RunFutility(ctx context.Context, req *bols.RunFutilityRequest) (*bols.RunFutilityResponse, error) {
	s.logger.Println("Receive RunFutility Request")
	if err := util.CheckFutilityParams(req); err != nil {
		return nil, s.logAndReturnErrorf("failed to validate parameters: %w", err)
	}
	args := append([]string{"futility"}, req.GetParams()...)
	c, err := docker.NewClient(ctx) // Not using dockerClient helper
	if err != nil {
		return nil, s.logAndReturnErrorf("fail to create docker client: %w", err)
	}
	stdout, stderr, err := containerExecCmd(ctx, c, req.StationId.GetContainerName(), args,
		timeRemaining(ctx))
	if err != nil {
		return nil, s.logAndReturnErrorf("fail to execute futility command: %w", err)
	}
	s.logger.Println("Served RunFutility Request Successfully")
	return &bols.RunFutilityResponse{
			Output: &bols.OutputStream{
				Stdout: []byte(stdout),
				Stderr: []byte(stderr),
			}},
		nil
}

func (s *service) RunFlashEC(ctx context.Context, req *bols.RunFlashECRequest) (*bols.RunFlashECResponse, error) {
	s.logger.Println("Receive RunFlashEC Request")
	if err := util.CheckFlashECParams(req); err != nil {
		return nil, s.logAndReturnErrorf("failed to validate parameters: %w", err)
	}
	args := append([]string{"flash_ec"}, req.GetParams()...)
	c, err := docker.NewClient(ctx) // Not using dockerClient helper
	if err != nil {
		return nil, s.logAndReturnErrorf("fail to create docker client: %w", err)
	}
	stdout, stderr, err := containerExecCmd(ctx, c, req.StationId.GetContainerName(), args,
		timeRemaining(ctx))
	if err != nil {
		return nil, s.logAndReturnErrorf("fail to execute flash_ec command: %w", err)
	}
	s.logger.Println("Served RunFlashEC Request Successfully")
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

// Helper functions (unchanged from original, unless noted)

func startServod(ctx context.Context, containerName, board, model, serial, config string,
	recoveryMode bool, port int32, logger *log.Logger) error {
	const (
		servodRegistryUri             = "SERVOD_REGISTRY_URI"
		servodContainerlLabel         = "SERVOD_CONTAINER_LABEL"
		servodRegistryUriFallback     = "us-docker.pkg.dev/chromeos-partner-moblab/common-core"
		servodContainerlLabelFallback = "release"
	)
	if containerName == "" {
		return errors.New("servod docker container name is required") // Not using s.logAndReturnErrorf as logger might not be s.logger
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
			fmt.Errorf("command %s failed with exit code: %d, stderr: %s", args[0], res.ExitCode, res.Stderr)
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
		// This error won't be logged by s.logAndReturnErrorf as it's a package-level helper potentially.
		// However, callers (service methods) will use s.logAndReturnErrorf.
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
	res, err := c.Exec(ctx, containerName, req) // Modified to capture res
	if err != nil {
		return fmt.Errorf("failed to exec cmd %q: %w", strings.Join(args, " "), err)
	}
	// Check for non-zero exit code, as c.Exec might not return error for it
	if res != nil && res.ExitCode != 0 {
		return fmt.Errorf("failed to exec cmd %q, exit code %d, stderr: %s", strings.Join(args, " "), res.ExitCode, res.Stderr)
	}
	return nil
}

func rmdirInContainer(ctx context.Context, c docker.Client,
	containerName, path string, removeAll bool, timeout time.Duration) error {
	var args []string
	if removeAll {
		args = []string{"rm", "-rf", path}
	} else {
		args = []string{"rmdir", path}
	}
	if _, _, err := containerExecCmd(ctx, c, containerName, args, timeout); err != nil {
		return fmt.Errorf("failed to execute %q: %w", strings.Join(args, " "), err)
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
	defer tarFileReader.Close() // Added defer

	outFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer outFile.Close() // Added defer

	tr := tar.NewReader(tarFileReader)
	if _, err := tr.Next(); err != nil {
		return fmt.Errorf("failed to get content from tar file %s: %w", tarFileName, err)
	}
	if _, err = io.Copy(outFile, tr); err != nil {
		return fmt.Errorf("failed to copy content from tar file %s to %s: %w", tarFileName, filePath, err)
	}
	return nil // outFile.Close() is handled by defer
}

// parseDirInfo parses the output of `find ... -exec stat --format='%F|%s|%n' {} `
func parseDirInfo(output string) ([]*bols.FileStat, error) {
	var stats []*bols.FileStat
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 1 && lines[0] == "" {
		// Handle empty directory case
		return stats, nil
	}

	for _, line := range lines {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("unexpected stat output format: %q", line)
		}

		fileTypeStr := parts[0]
		sizeStr := parts[1]
		pathStr := parts[2]

		size, err := strconv.ParseInt(sizeStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse size %q from line %q: %w", sizeStr, line, err)
		}

		// The path from stat will handle symlinks correctly for our needs.
		// e.g., "symlink -> target"
		nameAndLink := strings.SplitN(pathStr, " -> ", 2)
		fullPath := nameAndLink[0]

		stat := &bols.FileStat{
			Name:      filepath.Base(fullPath),
			Path:      fullPath,
			Size:      size,
			IsDir:     fileTypeStr == "directory",
			IsSymlink: fileTypeStr == "symbolic link",
		}
		stats = append(stats, stat)
	}

	return stats, nil
}

func xmlrpcClient(s *bols.StationIdentifier) (*servod_xmlrpc.XMLRpc, error) {
	if s.GetContainerName() == "" {
		return nil, errors.New("DocServod: container name is required for servod host")
	}
	return servod_xmlrpc.New(s.GetContainerName(), int(s.GetServodPort())), nil
}

// logAndReturnErrorf logs an error with the caller's function name and returns it.
func (s *service) logAndReturnErrorf(format string, a ...interface{}) error {
	err := fmt.Errorf(format, a...)
	if s.logger == nil {
		// If logger is not initialized, just return the error.
		// This might happen if an error occurs before logger setup in Run.
		return err
	}
	prefix := ""
	pc, _, _, ok := runtime.Caller(1) // Skip 1 frame to get the caller (e.g., GetFileStat)
	if ok {
		funcInfo := runtime.FuncForPC(pc)
		if funcInfo != nil {
			// Extract only the function name, not the full path
			parts := strings.Split(funcInfo.Name(), ".")
			funcName := parts[len(parts)-1]
			// Add service name prefix if it's a method
			if strings.HasPrefix(funcInfo.Name(), "go.chromium.org/infra/cros/cmd/cft/bols_satlab/internal/server.(*service).") {
				prefix = fmt.Sprintf("%s:", funcName)
			} else {
				prefix = fmt.Sprintf("%s:", funcInfo.Name()) // Fallback to full name if not a service method
			}
		}
	}
	s.logger.Println(prefix, err)
	return err
}

// createTempDir creates a new temporary directory in the default temporary directory.
// The caller is responsible for cleaning up the directory using os.RemoveAll.
func createTempDir(pattern string) (string, error) {
	dir, err := os.MkdirTemp("", pattern)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory with pattern %q: %w", pattern, err)
	}
	return dir, nil
}
