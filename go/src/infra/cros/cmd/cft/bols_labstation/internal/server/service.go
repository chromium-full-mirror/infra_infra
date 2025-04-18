// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package server implement bols-service API.
package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/config/go/test/api/bols"
)

// GetFileStat reads file information from the labstation.
func (s *service) GetFileStat(ctx context.Context, req *bols.GetFileStatRequest) (*bols.GetFileStatResponse, error) {
	fs, err := fstat(req.GetFilepath())
	if err != nil {
		return nil, fmt.Errorf("failed to get file status of %s: %v", req.GetFilepath(), err)
	}
	return &bols.GetFileStatResponse{
		FileStats: fs,
	}, nil
}

// GetFile gets a file from labstation.
func (s *service) GetFile(req *bols.GetFileRequest, stream bols.BolsService_GetFileServer) error {
	fn := req.GetFilename()
	file, err := os.Open(fn)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %v", fn, err)
	}
	defer file.Close()
	data := make([]byte, 1024*1024)
	for {
		n, err := file.Read(data)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read file %s: %v", fn, err)
		}
		stream.Send(&bols.GetFileResponse{
			Data: data[:n],
		})
	}
	return nil
}

// PutFile puts a file on labstation.
// If the directory of destination path does not exist, this service
// will also create the directory.
func (s *service) PutFile(stream bols.BolsService_PutFileServer) error {
	var f *os.File
	var fn string
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err == nil {
			return fmt.Errorf("failed to receive streaming data: %v", err)
		}
		info := req.GetReqInfo()
		if info != nil {
			fn = info.GetFilename()
			f, err = os.OpenFile(fn, os.O_RDWR|os.O_CREATE, 0644)
			if err != nil {
				return fmt.Errorf("failed to open file %s: %v", fn, err)
			}
			defer f.Close()
			continue
		}
		data := req.GetData()
		if data != nil {
			if f == nil {
				return errors.New("data was send before file name")
			}
			if _, err := f.Write(data); err != nil {
				return fmt.Errorf("failed to write file %s: %v", fn, err)
			}
		}
	}
	return nil
}

// DownloadFile downloads a file on labstation based on the specified url
// by sending a http GET request to the url.
func (s *service) DownloadFile(context.Context, *bols.DownloadFileRequest) (*bols.DownloadFileResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DownloadFile not implemented")
}

// RemoveFile removes a file on labstation/container.
func (s *service) RemoveFile(ctx context.Context, req *bols.RemoveFileRequest) (*bols.RemoveFileResponse, error) {
	fn := req.GetFilename()
	if err := os.Remove(fn); err != nil {
		return nil, fmt.Errorf("failed to remove file %s: %v", fn, err)
	}
	return &bols.RemoveFileResponse{}, nil
}

// DirInfo reads a directory info from the labstation.
func (s *service) GetDirInfo(ctx context.Context, req *bols.GetDirInfoRequest) (*bols.GetDirInfoResponse, error) {
	path := req.GetPath()
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get information on directory %s: %v", path, err)
	}
	var stats []*bols.FileStat
	for _, e := range entries {
		stat, err := fstat(e.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to get information on %s: %v", e.Name(), err)
		}
		stats = append(stats, stat)
	}
	return &bols.GetDirInfoResponse{
		Info: &bols.DirectoryInfo{
			Path:      path,
			FileStats: stats,
		},
	}, nil
}

// MakeDir makes a directory on the labstation.
func (s *service) MakeDir(ctx context.Context, req *bols.MakeDirRequest) (*bols.MakeDirResponse, error) {
	dirName := req.GetPath()
	if err := os.MkdirAll(dirName, 0750); err != nil {
		return nil, fmt.Errorf("failed to make directory %s: %v", dirName, err)
	}
	return &bols.MakeDirResponse{}, nil
}

// MakeTempDir makes a directory on the labstation.
func (s *service) MakeTempDir(ctx context.Context, req *bols.MakeTempDirRequest) (*bols.MakeTempDirResponse, error) {
	dirName := req.GetDir()
	pattern := req.GetPattern()
	path, err := os.MkdirTemp(dirName, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to make temporary directory in %s with pattern %q: %v",
			dirName, pattern, err)
	}
	return &bols.MakeTempDirResponse{
		Info: &bols.DirectoryInfo{
			Path: path,
		},
	}, nil
}

// RemoveDir removes a directory from labstation/container.
func (s *service) RemoveDir(ctx context.Context, req *bols.RemoveDirRequest) (*bols.RemoveDirResponse, error) {

	path := req.GetPath()
	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get file status of %s: %v", path, err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("the path %s is not a directory", path)
	}
	if err := os.RemoveAll(path); err != nil {
		return nil, fmt.Errorf("failed to remove directory %s: %v", path, err)
	}
	return &bols.RemoveDirResponse{}, nil
}

// DMesg returns the output from the dmesg command.
func (s *service) DMesg(req *bols.DMesgRequest, stream bols.BolsService_DMesgServer) error {
	ctx := stream.Context()
	data, err := exec.CommandContext(ctx, "dmesg", "-H").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run dmesg: %s : %v", string(data), err)
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
	return nil
}

// WriteFileByBlock write data to a file by blocks.
// For most implementation of this service, it will be simple
// call to "dd <filename> oflag=sync conv=notrunc,nocreat".
func (s *service) WriteFileByBlock(bols.BolsService_WriteFileByBlockServer) error {
	return status.Errorf(codes.Unimplemented, "method WriteFileByBlock not implemented")
}

// ReadFileByBlock reads data by Block
// For most implementation of this service, it will be simple
// call to "dd".
func (s *service) ReadFileByBlock(*bols.ReadFileByBlockRequest, bols.BolsService_ReadFileByBlockServer) error {
	return status.Errorf(codes.Unimplemented, "method ReadFileByBlock not implemented")
}

// RunMount runs the "mount" command on the labstation.
func (s *service) RunMount(context.Context, *bols.RunMountRequest) (*bols.RunMountResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RunMount not implemented")
}

// RunUMount runs the "umount" command on the labstation.
func (s *service) RunUMount(context.Context, *bols.RunUMountRequest) (*bols.RunUMountResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RunUMount not implemented")
}

// StartServod runs a servod daemon.
func (s *service) StartServod(context.Context, *bols.StartServodRequest) (*bols.StartServodResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StartServod not implemented")
}

// StopServod stops the servod daemon.
func (s *service) StopServod(context.Context, *bols.StopServodRequest) (*bols.StopServodResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StopServod not implemented")
}

// GetServodStatus gets the current status of servod.
func (s *service) GetServodStatus(context.Context, *bols.GetServodStatusRequest) (*bols.GetServodStatusResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetServodStatus not implemented")
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
func (s *service) GetServod(context.Context, *bols.GetServodRequest) (*bols.GetServodResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetServod not implemented")
}

// SetServod sets value to a servod control.
func (s *service) SetServod(context.Context, *bols.SetServodRequest) (*bols.SetServodResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetServod not implemented")
}

// GetServodVersion reads version of started servod.
func (s *service) GetServodVersion(context.Context, *bols.GetServodVersionRequest) (*bols.GetServodVersionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetServodVersion not implemented")
}

// EchoServod calls echo method of servod.
func (s *service) EchoServod(context.Context, *bols.EchoServodRequest) (*bols.EchoServodResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method EchoServod not implemented")
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
func (s *service) RunFutility(context.Context, *bols.RunFutilityRequest) (*bols.RunFutilityResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RunFutility not implemented")
}

// RunFlashEC runs EC firmware flashing from the servo.
// In most of implementation, it runs flash_ec tool on labstation.
func (s *service) RunFlashEC(context.Context, *bols.RunFlashECRequest) (*bols.RunFlashECResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RunFlashEC not implemented")
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

func fstat(path string) (*bols.FileStat, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get file status of %s: %v", path, err)
	}
	lstat, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to check if %s is a symlink: %v", path, err)
	}
	return &bols.FileStat{
		Name:      fi.Name(),
		Size:      fi.Size(),
		IsDir:     fi.IsDir(),
		IsSymlink: lstat.Mode()&os.ModeSymlink == os.ModeSymlink,
	}, nil
}
