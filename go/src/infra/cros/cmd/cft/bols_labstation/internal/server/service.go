// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package server implement bols-service API.
package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/config/go/test/api/bols"

	"go.chromium.org/infra/cros/lib/bols/util"
	"go.chromium.org/infra/cros/lib/bols/xmlrpc"
)

// GetFileStat reads file information from the labstation.
func (s *service) GetFileStat(ctx context.Context, req *bols.GetFileStatRequest) (*bols.GetFileStatResponse, error) {
	s.logger.Println("Receive GetFileStat Request")
	fs, err := fstat(req.GetFilepath())
	if err != nil {
		return nil, s.logAndReturnError(
			fmt.Errorf("failed to get file status of %s: %w", req.GetFilepath(), err))
	}
	s.logger.Println("Served GetFileStat Request Successfully")
	return &bols.GetFileStatResponse{
		FileStats: fs,
	}, nil
}

// GetFile gets a file from labstation.
func (s *service) GetFile(req *bols.GetFileRequest, stream bols.BolsService_GetFileServer) error {
	s.logger.Println("Receive GetFile Request")
	fn := req.GetFilename()
	fileInfo, err := os.Lstat(fn)
	if err != nil {
		return s.logAndReturnError(fmt.Errorf("failed to stat file: %w", err))
	}
	// Check if the file mode indicates it's a symbolic link.
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		// os.Readlink returns the path the symbolic link points to.
		resolvedPath, err := os.Readlink(fn)
		if err != nil {
			return s.logAndReturnError(fmt.Errorf("failed to resolve symbolic link: %w", err))
		}
		// In case the symlink is relative, resolve it to an absolute path
		// based on the link's directory.
		if !filepath.IsAbs(resolvedPath) {
			dir := filepath.Dir(fn)
			resolvedPath = filepath.Join(dir, resolvedPath)
		}
		fn = resolvedPath
	}
	file, err := os.Open(fn)
	if err != nil {
		return s.logAndReturnError(fmt.Errorf("failed to open file %s: %w", fn, err))
	}
	defer file.Close()
	data := make([]byte, 1024*1024)
	for {
		n, err := file.Read(data)
		if err == io.EOF {
			break
		}
		if err != nil {
			return s.logAndReturnError(fmt.Errorf("failed to read file %s: %w", fn, err))
		}
		stream.Send(&bols.GetFileResponse{
			Data: data[:n],
		})
	}
	s.logger.Println("Served GetFile Request Successfully")
	return nil
}

// PutFile puts a file on labstation.
// If the directory of destination path does not exist, this service
// will also create the directory.
func (s *service) PutFile(stream bols.BolsService_PutFileServer) error {
	s.logger.Println("Receive PutFile Request")
	var f *os.File
	var fn string
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			s.logger.Println("get EOF")
			break
		}
		if err != nil {
			return s.logAndReturnError(fmt.Errorf("failed to receive streaming data: %w", err))
		}
		switch {
		case req.GetReqInfo() != nil:
			info := req.GetReqInfo()
			fn = info.GetFilename()
			dir := filepath.Dir(fn)
			if err := os.MkdirAll(dir, os.ModePerm); err != nil {
				return s.logAndReturnError(fmt.Errorf("failed to create directory %s: %w", dir, err))
			}
			f, err = os.OpenFile(fn, os.O_RDWR|os.O_CREATE, 0644)
			if err != nil {
				return s.logAndReturnError(fmt.Errorf("failed to open file %s: %w", fn, err))
			}
			defer f.Close()
			s.logger.Println("PutFile Request destination file: ", fn)
		case req.GetData() != nil:
			if f == nil {
				return s.logAndReturnError(errors.New("data was send before file name"))
			}
			if _, err := f.Write(req.GetData()); err != nil {
				return s.logAndReturnError(fmt.Errorf("failed to write file %s: %w", fn, err))
			}
		}
	}
	stream.SendAndClose(&bols.PutFileResponse{})
	s.logger.Println("Served PutFile Request Successfully")
	return nil
}

// DownloadFile downloads a file on labstation based on the specified url
// by sending a http GET request to the url.
func (s *service) DownloadFile(context.Context, *bols.DownloadFileRequest) (*bols.DownloadFileResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DownloadFile not implemented")
}

// RemoveFile removes a file on labstation/container.
func (s *service) RemoveFile(ctx context.Context, req *bols.RemoveFileRequest) (*bols.RemoveFileResponse, error) {
	s.logger.Println("Receive RemoveFile Request")
	fn := req.GetFilename()
	if err := os.Remove(fn); err != nil {
		return nil, s.logAndReturnError(fmt.Errorf("failed to remove file %s: %w", fn, err))
	}
	s.logger.Println("Served RemoveFile Request Successfully")
	return &bols.RemoveFileResponse{}, nil
}

// DirInfo reads a directory info from the labstation.
func (s *service) GetDirInfo(ctx context.Context, req *bols.GetDirInfoRequest) (*bols.GetDirInfoResponse, error) {
	s.logger.Println("Receive GetDirInfo Request")
	path := req.GetPath()
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, s.logAndReturnError(
			fmt.Errorf("failed to get information on directory %s: %w", path, err))
	}
	var stats []*bols.FileStat
	for _, e := range entries {
		stat, err := fstat(e.Name())
		if err != nil {
			return nil, s.logAndReturnError(
				fmt.Errorf("failed to get information on %s: %w", e.Name(), err))
		}
		stats = append(stats, stat)
	}
	s.logger.Println("Served GetDirInfo Request Successfully")
	return &bols.GetDirInfoResponse{
		Info: &bols.DirectoryInfo{
			Path:      path,
			FileStats: stats,
		},
	}, nil
}

// MakeDir makes a directory on the labstation.
func (s *service) MakeDir(ctx context.Context, req *bols.MakeDirRequest) (*bols.MakeDirResponse, error) {
	s.logger.Println("Receive MakeDir Request")
	dirName := req.GetPath()
	if err := os.MkdirAll(dirName, 0750); err != nil {
		return nil, s.logAndReturnError(
			fmt.Errorf("failed to make directory %s: %w", dirName, err))
	}
	s.logger.Println("Served MakeDir Request Successfully")
	return &bols.MakeDirResponse{}, nil
}

// MakeTempDir makes a directory on the labstation.
func (s *service) MakeTempDir(ctx context.Context, req *bols.MakeTempDirRequest) (*bols.MakeTempDirResponse, error) {
	s.logger.Println("Receive MakeTempDir Request")
	dirName := req.GetDir()
	pattern := req.GetPattern()
	path, err := os.MkdirTemp(dirName, pattern)
	if err != nil {
		return nil, s.logAndReturnError(
			fmt.Errorf("failed to make temporary directory in %s with pattern %q: %w",
				dirName, pattern, err))
	}
	s.logger.Println("Served MakeTempDir Request Successfully")
	return &bols.MakeTempDirResponse{
		Info: &bols.DirectoryInfo{
			Path: path,
		},
	}, nil
}

// RemoveDir removes a directory from labstation/container.
func (s *service) RemoveDir(ctx context.Context, req *bols.RemoveDirRequest) (*bols.RemoveDirResponse, error) {
	s.logger.Println("Receive RemoveDir Request")
	path := req.GetPath()
	fi, err := os.Stat(path)
	if err != nil {
		return nil, s.logAndReturnError(
			fmt.Errorf("failed to get file status of %s: %w", path, err))
	}
	if !fi.IsDir() {
		return nil, s.logAndReturnError(
			fmt.Errorf("the path %s is not a directory", path))
	}
	if err := os.RemoveAll(path); err != nil {
		return nil, s.logAndReturnError(
			fmt.Errorf("failed to remove directory %s: %w", path, err))
	}
	s.logger.Println("Served RemoveDir Request Successfully")
	return &bols.RemoveDirResponse{}, nil
}

// DMesg returns the output from the dmesg command.
func (s *service) DMesg(req *bols.DMesgRequest, stream bols.BolsService_DMesgServer) error {
	s.logger.Println("Receive DMesg Request")
	ctx := stream.Context()
	data, err := exec.CommandContext(ctx, "dmesg", "-H").CombinedOutput()
	if err != nil {
		return s.logAndReturnError(
			fmt.Errorf("failed to run dmesg: %s : %w", string(data), err))
	}
	const size int = 1024 * 1024
	for len(data) > 0 {
		n := len(data)
		if n > size {
			n = size
		}
		stream.Send(&bols.DMesgResponse{
			Output: &bols.OutputStream{
				Stdout: data[:n],
			},
		})
		data = data[n:]
	}
	s.logger.Println("Served DMesg Request Successfully")
	return nil
}

// WriteFileByBlock write data to a file by blocks.
// For most implementation of this service, it will be simple
// call to "dd <filename> oflag=sync conv=notrunc,nocreat".
func (s *service) WriteFileByBlock(stream bols.BolsService_WriteFileByBlockServer) error {
	s.logger.Println("Receive WriteFileByBlock Request")
	var fn string
	var byteSize int32
	var stdin io.WriteCloser
	var cmd *exec.Cmd
	ctx := stream.Context()

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			if err := stdin.Close(); err != nil {
				return s.logAndReturnError(
					fmt.Errorf("failed to close stdin to dd: %w", err))
			}
			if err := cmd.Wait(); err != nil {
				return s.logAndReturnError(
					fmt.Errorf("failed to wait for dd to finish: %w", err))
			}
			break
		}
		if err != nil {
			return s.logAndReturnError(
				fmt.Errorf("failed to receive streaming data: %w", err))
		}
		switch {
		case req.GetReqInfo() != nil:
			info := req.GetReqInfo()
			fn = info.GetFilePath()
			byteSize = info.GetByteSize()
			args := []string{
				fmt.Sprintf("of=%s", fn),
				"oflag=sync", "conv=notrunc,nocreat",
			}
			if byteSize > 0 {
				args = append(args, fmt.Sprintf("bs=%d", byteSize))
			}
			cmd = exec.CommandContext(ctx, "dd", args...)
			stdin, err = cmd.StdinPipe()
			if err != nil {
				return s.logAndReturnError(
					fmt.Errorf("failed to create stdin to dd: %w", err))
			}
		case req.GetData() != nil:
			if fn == "" {
				return errors.New("data was send before file name")
			}
			if _, err := io.Writer.Write(stdin, req.GetData()); err != nil {
				stdin.Close()
				return s.logAndReturnError(
					fmt.Errorf("failed to write data to file %s: %w", fn, err))
			}
		}
	}
	s.logger.Println("Served WriteFileByBlock Request Successfully")
	return nil
}

// ReadFileByBlock reads data by Block
// For most implementation of this service, it will be simple
// call to "dd".
func (s *service) ReadFileByBlock(*bols.ReadFileByBlockRequest, bols.BolsService_ReadFileByBlockServer) error {
	// TODO: check if we really need this function.
	return status.Errorf(codes.Unimplemented, "method ReadFileByBlock not implemented")
}

// RunMount runs the "mount" command on the labstation.
func (s *service) RunMount(ctx context.Context, req *bols.RunMountRequest) (*bols.RunMountResponse, error) {
	s.logger.Println("Receive RunMount Request")
	var args []string
	args = append(args, req.GetParams()...)
	if req.GetSrc() != "" {
		args = append(args, req.GetSrc())
	}
	if req.GetDest() != "" {
		args = append(args, req.GetDest())
	}
	if out, err := exec.CommandContext(ctx, "mount", args...).CombinedOutput(); err != nil {
		return nil, s.logAndReturnError(
			fmt.Errorf("failed to mount %s to %s: %s: %w", req.GetSrc(), req.GetDest(), string(out), err))
	}
	s.logger.Println("Served RunMount Request Successfully")
	return &bols.RunMountResponse{}, nil
}

// RunUMount runs the "umount" command on the labstation.
func (s *service) RunUMount(ctx context.Context, req *bols.RunUMountRequest) (*bols.RunUMountResponse, error) {
	s.logger.Println("Receive RunUMount Request")
	if out, err := exec.CommandContext(ctx, "umount", req.GetPath()).CombinedOutput(); err != nil {
		return nil, s.logAndReturnError(
			fmt.Errorf("failed to umount %s: %s: %w", req.GetPath(), string(out), err))
	}
	s.logger.Println("Served RunUMount Request Successfully")
	return &bols.RunUMountResponse{}, nil
}

// StartServod runs a servod daemon.
func (s *service) StartServod(ctx context.Context, req *bols.StartServodRequest) (*bols.StartServodResponse, error) {
	s.logger.Println("Receive StartServod Request")
	port := req.GetStationId().GetServodPort()
	if err := markInUseFile(port); err != nil {
		return nil, s.logAndReturnError(
			fmt.Errorf("failed to mark in use file: %w", err))
	}
	if getServodStatus(ctx, port) == bols.ServodStatus_SERVOD_RUNNING {
		// Since servod has already been started, do not need to start again.
		return &bols.StartServodResponse{}, nil
	}
	args := []string{"servod"}
	args = append(args, fmt.Sprintf("PORT=%d", port))
	if board := req.GetBoard(); board != "" {
		args = append(args, fmt.Sprintf("BOARD=%s", board))
	}
	if model := req.GetModel(); model != "" {
		args = append(args, fmt.Sprintf("MODEL=%s", model))
	}
	if serial := req.GetStationId().GetServoSerial(); serial != "" {
		args = append(args, fmt.Sprintf("SERIAL=%s", serial))
	}
	if config := req.GetConfig(); config != "" {
		args = append(args, fmt.Sprintf("CONFIG=%s", config))
	}
	if req.GetRecoveryMode() {
		args = append(args, "REC_MODE=1")
	}
	if out, err := exec.CommandContext(ctx, "start", args...).CombinedOutput(); err != nil {
		return nil, s.logAndReturnError(
			fmt.Errorf("failed to start servod at %d: %s: %w", port, string(out), err))
	}
	if out, err := exec.CommandContext(ctx, "servodtool", "instance", "wait-for-active",
		"--timeout", "120", "-p", fmt.Sprintf("%d", port)).Output(); err != nil {
		s.logger.Printf("Failed to check if servod is ready: %s: %v", string(out), err)
	}
	s.logger.Println("Served StartServod Request Successfully")
	return &bols.StartServodResponse{}, nil
}

// StopServod stops the servod daemon.
func (s *service) StopServod(context.Context, *bols.StopServodRequest) (*bols.StopServodResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StopServod not implemented")
}

// GetServodStatus gets the current status of servod.
func (s *service) GetServodStatus(ctx context.Context, req *bols.GetServodStatusRequest) (*bols.GetServodStatusResponse, error) {
	s.logger.Println("Receive GetServodStatus Request")
	status := getServodStatus(ctx, req.StationId.GetServodPort())
	s.logger.Println("Served GetServodStatus Request Successfully")
	return &bols.GetServodStatusResponse{Status: status}, nil
}

// HWInitServod calls hwinit of servod.
func (s *service) HWInitServod(context.Context, *bols.HWInitServodRequest) (*bols.HWInitServodResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method HWInitServod not implemented")
}

// ReadServod read a servod control documentation.
func (s *service) ReadServod(context.Context, *bols.ReadServodRequest) (*bols.ReadServodResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReadServod not implemented")
}

// GetServod gets a servod control value.
func (s *service) GetServod(ctx context.Context, req *bols.GetServodRequest) (*bols.GetServodResponse, error) {
	s.logger.Println("Receive GetServod Request")
	rpsn, err := xmlrpc.GetServod(ctx, "localhost", req.GetStationId().GetServodPort(), req.GetControl())
	if err != nil {
		return nil, s.logAndReturnError(
			fmt.Errorf("failed to send get %s request to servod at port %d: %w",
				req.GetControl(), req.GetStationId().GetServodPort(), err))
	}
	s.logger.Println("Served GetServod Request Successfully")
	return rpsn, nil
}

// SetServod sets value to a servod control.
func (s *service) SetServod(ctx context.Context, req *bols.SetServodRequest) (*bols.SetServodResponse, error) {
	s.logger.Println("Receive SetServod Request")
	rpsn, err := xmlrpc.SetServod(ctx, req)
	if err != nil {
		return nil, s.logAndReturnError(
			fmt.Errorf("failed to send set %s request to servod at port %d: %w",
				req.GetControl(), req.GetStationId().GetServodPort(), err))
	}
	s.logger.Println("Served SetServod Request Successfully")
	return rpsn, nil
}

// GetServodVersion reads version of started servod.
func (s *service) GetServodVersion(ctx context.Context, req *bols.GetServodVersionRequest) (*bols.GetServodVersionResponse, error) {
	s.logger.Println("Receive GetServodVersion Request")
	rpsn, err := xmlrpc.GetServodVersion(ctx, req)
	if err != nil {
		return nil, s.logAndReturnError(
			fmt.Errorf("failed to get version of servod at port %d: %w",
				req.GetStationId().GetServodPort(), err))
	}
	s.logger.Println("Served GetServodVersion Request Successfully")
	return rpsn, nil
}

// EchoServod calls echo method of servod.
func (s *service) EchoServod(ctx context.Context, req *bols.EchoServodRequest) (*bols.EchoServodResponse, error) {
	s.logger.Println("Receive EchoServod Request")
	rpsn, err := xmlrpc.EchoServod(ctx, req)
	if err != nil {
		return nil, s.logAndReturnError(
			fmt.Errorf("failed to send echo request to servod at port %d: %w",
				req.GetStationId().GetServodPort(), err))
	}
	s.logger.Println("Served EchoServod Request Successfully")
	return rpsn, nil
}

// GetServoTopology gets the servo topology.
func (s *service) GetServoTopology(context.Context, *bols.GetServoTopologyRequest) (*bols.GetServoTopologyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetServoTopology not implemented")
}

// UpdateServoFirmware update the firmware of a servo device.
func (s *service) UpdateServoFirmware(context.Context, *bols.UpdateServoFirmwareRequest) (*bols.UpdateServoFirmwareResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateServoFirmware not implemented")
}

// RunFutility run futility tool on labstation.
func (s *service) RunFutility(ctx context.Context, req *bols.RunFutilityRequest) (*bols.RunFutilityResponse, error) {
	s.logger.Println("Receive RunFutility Request")
	if err := util.CheckFutilityParams(req); err != nil {
		return nil, s.logAndReturnError(
			fmt.Errorf("failed to validate parameters: %w", err))
	}
	cmd := exec.CommandContext(ctx, "futility", req.GetParams()...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, s.logAndReturnError(
			fmt.Errorf("failed to run futility: %s: %w", stderr.String(), err))
	}
	s.logger.Println("Served RunFutility Request Successfully")
	return &bols.RunFutilityResponse{
		Output: &bols.OutputStream{
			Stdout: stdout.Bytes(),
			Stderr: stderr.Bytes(),
		}}, nil
}

// RunFlashEC runs EC firmware flashing from the servo.
// In most of implementation, it runs flash_ec tool on labstation.
func (s *service) RunFlashEC(ctx context.Context, req *bols.RunFlashECRequest) (*bols.RunFlashECResponse, error) {
	s.logger.Println("Receive RunFlashEC Request")
	if err := util.CheckFlashECParams(req); err != nil {
		return nil, s.logAndReturnError(
			fmt.Errorf("failed to validate parameters: %w", err))
	}
	cmd := exec.CommandContext(ctx, "flash_ec", req.GetParams()...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, s.logAndReturnError(
			fmt.Errorf("failed to run flash_ec: %s: %w", stderr.String(), err))
	}
	s.logger.Println("Served RunFlashEC Request Successfully")
	return &bols.RunFlashECResponse{
		Output: &bols.OutputStream{
			Stdout: stdout.Bytes(),
			Stderr: stderr.Bytes(),
		}}, nil
}

// GetDolosVersion returns the current dolos version.
func (s *service) GetDolosVersion(context.Context, *bols.GetDolosVersionRequest) (*bols.GetDolosVersionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetDolosVersion not implemented")
}

// UpdateDolosVersion will update the Dolos version if version does
// not match expected one.
func (s *service) UpdateDolosVersion(context.Context, *bols.UpdateDolosVersionRequest) (*bols.UpdateDolosVersionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateDolosVersion not implemented")
}

// GetDolosStatus read the status of the Dolos.
func (s *service) GetDolosStatus(context.Context, *bols.GetDolosStatusRequest) (*bols.GetDolosStatusResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetDolosStatus not implemented")
}

// FindDolosUART finds the UART of the Dolos.
func (s *service) FindDolosUART(context.Context, *bols.FindDolosUARTRequest) (*bols.FindDolosUARTResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method FindDolosUART not implemented")
}

// FindDolosUART finds the UART of the Dolos.
func (s *service) logAndReturnError(err error) error {
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

func fstat(path string) (*bols.FileStat, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get file status of %s: %w", path, err)
	}
	lstat, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to check if %s is a symlink: %w", path, err)
	}
	return &bols.FileStat{
		Name:      fi.Name(),
		Size:      fi.Size(),
		IsDir:     fi.IsDir(),
		IsSymlink: lstat.Mode()&os.ModeSymlink == os.ModeSymlink,
	}, nil
}

func markInUseFile(port int32) error {
	inUseFile := fmt.Sprintf("/var/lib/servod/%d_in_use", port)
	// Create the file if the file does not exist.
	file, err := os.OpenFile(inUseFile, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", inUseFile, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close %s: %w", inUseFile, err)
	}

	currentTime := time.Now()
	err = os.Chtimes(inUseFile, currentTime, currentTime)
	if err != nil {
		return fmt.Errorf("failed to chance time for %s: %w", inUseFile, err)
	}
	return nil
}

func getServodStatus(ctx context.Context, port int32) bols.ServodStatus {
	if err := exec.CommandContext(ctx, "servodtool", "instance", "show", "-p",
		fmt.Sprintf("%d", port)).Run(); err == nil {
		return bols.ServodStatus_SERVOD_RUNNING
	}
	return bols.ServodStatus_SERVOD_STOPPED
}
