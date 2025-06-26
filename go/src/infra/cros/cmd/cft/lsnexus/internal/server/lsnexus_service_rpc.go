// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/config/go/test/api/bols"
	"go.chromium.org/chromiumos/config/go/test/api/lsnexus"
)

func (s *LsNexus) StartServod(ctx context.Context, req *lsnexus.StartServodRequest) (*lsnexus.StartServodResponse, error) {
	s.log("Serving StartServod request")
	if s.cl == nil {
		return nil, s.logAndReturnErr(errors.New("BOLS is not available"))
	}
	config := ""
	if pools := s.pools; len(pools) > 0 {
		for _, p := range pools {
			if strings.Contains(p, "faft-cr50") {
				config = "cr50.xml"
				break
			}
		}
	}
	bolsReq := &bols.StartServodRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    int32(s.servodPort),
			ServoSerial:   s.servodSerial,
			ContainerName: s.servodContainer,
		},
		Board:  s.board,
		Model:  s.model,
		Config: config,
	}
	if _, err := s.cl.StartServod(ctx, bolsReq); err != nil {
		s.logAndReturnErr(fmt.Errorf("failed to start servod: %w", err))
		return nil, err
	}
	s.log("Successfully served StartServod request")
	return &lsnexus.StartServodResponse{}, nil
}

func (s *LsNexus) CallServod(ctx context.Context, req *lsnexus.CallServodRequest) (*lsnexus.CallServodResponse, error) {
	s.log("Serving CallServod request")
	if s.cl == nil {
		return nil, s.logAndReturnErr(errors.New("BOLS is not available"))
	}
	switch req.GetMethod() {
	case lsnexus.CallServodRequest_GET:
		rspn, err := s.getServodRequest(ctx, req)
		if err != nil {
			err = fmt.Errorf("failed to run get servod request: %w", err)
			s.log(err)
			return nil, err
		}
		return rspn, nil
	case lsnexus.CallServodRequest_SET:
		rspn, err := s.setServodRequest(ctx, req)
		if err != nil {
			err = fmt.Errorf("failed to run set servod request: %w", err)
			s.log(err)
			return nil, err
		}
		return rspn, nil
	case lsnexus.CallServodRequest_GET_SERVOD_VERSION:
		rspn, err := s.getServodVersion(ctx)
		if err != nil {
			err = fmt.Errorf("failed to run get servod version request: %w", err)
			s.log(err)
			return nil, err
		}
		return rspn, nil
	}
	return nil, status.Error(codes.Unimplemented, "the specified call servod method not implemented")
}

func (s *LsNexus) log(args ...any) {
	if s.logger == nil {
		return
	}
	s.logger.Println(args...)
}

func (s *LsNexus) getServodRequest(ctx context.Context, req *lsnexus.CallServodRequest) (*lsnexus.CallServodResponse, error) {
	if req.GetControl() == "" {
		err := errors.New("get servod request needs an non-empty string as control value")
		s.log(err)
		return nil, err
	}
	bolsReq := &bols.GetServodRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    int32(s.servodPort),
			ServoSerial:   s.servodSerial,
			ContainerName: s.servodContainer,
		},
		Control: req.GetControl(),
	}
	bolsRspn, err := s.cl.GetServod(ctx, bolsReq)
	if err != nil {
		err = fmt.Errorf("failed to make GetServod request: %w", err)
		s.log(err)
		return nil, err
	}
	return &lsnexus.CallServodResponse{
		Result: &lsnexus.CallServodResponse_Success_{
			Success: &lsnexus.CallServodResponse_Success{
				Result: bolsRspn.GetValue(),
			},
		}}, nil
}

func (s *LsNexus) setServodRequest(ctx context.Context, req *lsnexus.CallServodRequest) (*lsnexus.CallServodResponse, error) {
	args := req.GetArgs()
	if len(args) != 1 {
		err := fmt.Errorf("set servod request has wrong number of arguments: got: %d expected: 1", len(args))
		s.log(err)
		return nil, err
	}
	if req.GetControl() == "" {
		err := errors.New("gst servod request needs an non-empty string as control value")
		s.log(err)
		return nil, err
	}
	bolsReq := &bols.SetServodRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    int32(s.servodPort),
			ServoSerial:   s.servodSerial,
			ContainerName: s.servodContainer,
		},
		Control: req.GetControl(),
		Value:   args[0],
	}
	if _, err := s.cl.SetServod(ctx, bolsReq); err != nil {
		err = fmt.Errorf("failed to make SetServod request: %w", err)
		s.log(err)
		return nil, err
	}
	return &lsnexus.CallServodResponse{Result: &lsnexus.CallServodResponse_Success_{}}, nil
}

func (s *LsNexus) getServodVersion(ctx context.Context) (*lsnexus.CallServodResponse, error) {
	bolsReq := &bols.GetServodVersionRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    s.servodPort,
			ServoSerial:   s.servodSerial,
			ContainerName: s.servodContainer,
		},
	}
	bolsRsp, err := s.cl.GetServodVersion(ctx, bolsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get servod version from BOLS: %w", err)
	}
	return &lsnexus.CallServodResponse{
		Result: &lsnexus.CallServodResponse_Success_{
			Success: &lsnexus.CallServodResponse_Success{
				Result: &bols.ServodValue{
					Value: &bols.ServodValue_StringValue{StringValue: bolsRsp.GetVersion()},
				},
			},
		}}, nil
}

func (s *LsNexus) DownloadServoLogs(ctx context.Context, req *lsnexus.DownloadServoLogsRequest) (*lsnexus.DownloadServoLogsResponse, error) {
	if err := s.saveServodLogs(ctx); err != nil {
		s.log("Warning: failed to download servod logs: ", err)
	}
	return &lsnexus.DownloadServoLogsResponse{}, nil
}

func (s *LsNexus) saveServodLogs(ctx context.Context) error {
	servodLogDir := fmt.Sprintf("/var/log/servod_%d", s.servodPort)
	destDir := filepath.Join(s.artifactDir, "log",
		fmt.Sprintf("/servod_%d", s.servodPort))
	fn := "latest.DEBUG"
	src := filepath.Join(servodLogDir, fn)
	dst := filepath.Join(destDir, fn)
	if err := s.getFile(ctx, src, dst); err != nil {
		return fmt.Errorf("failed to get %s: %w", src, err)
	}
	if err := s.getFile(ctx, "/var/log/messages", filepath.Join(destDir, "system.log")); err != nil {
		s.log("Warning: failed to download /var/log/messages: ", err)
	}
	s.extractServodMCULogs(destDir)
	s.downloadServodDMesgLogs(ctx, destDir)
	return nil
}

func (s *LsNexus) logAndReturnErr(err error) error {
	s.log(err)
	return err
}

func (s *LsNexus) getFile(ctx context.Context, src, dest string) error {
	bolsReq := &bols.GetFileRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    int32(s.servodPort),
			ServoSerial:   s.servodSerial,
			ContainerName: s.servodContainer,
		},
		Filename: src,
	}
	os.MkdirAll(filepath.Dir(dest), 0755)
	stream, err := s.cl.GetFile(ctx, bolsReq)
	if err != nil {
		return fmt.Errorf("failed to get file from BOLS: %w", err)
	}
	f, err := os.OpenFile(dest, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", dest, err)
	}
	defer f.Close()
	for {
		data, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to receive content of file %s: %w", src, err)
		}
		f.Write(data.GetData())
	}
	return nil
}

// extractServodMCULogs extract MCU logs from latest.DEBUG.
func (s *LsNexus) extractServodMCULogs(destDir string) {
	s.log("Extracing servod MCU logs")
	mcuFiles := make(map[string]*os.File)
	src := filepath.Join(destDir, "latest.DEBUG")
	f, err := os.Open(src)
	if err != nil {
		s.log(fmt.Sprintf("Failed to open %s: %v\n", src, err))
		return
	}
	defer f.Close()

	regExpr := `(?P<time>[\d\-]+(( [\d:,]+ )|(T[\d:.+]+ )))` +
		`- (?P<mcu>[\w/]+) - ` +
		`EC3PO\.Console[\s\-\w\d:.]+LogConsoleOutput - /dev/pts/\d+ - ` +
		`(?P<line>.+$)`

	re, err := regexp.Compile(regExpr)
	if err != nil {
		fmt.Printf("Fail in compiling expression %v\n", err)
		return
	}

	sc := bufio.NewScanner(f)
	sc.Split(bufio.ScanLines)
	for sc.Scan() {
		text := sc.Text()
		matches := re.FindStringSubmatch(text)
		timeIndex := re.SubexpIndex("time")
		if timeIndex < 0 || timeIndex >= len(matches) {
			continue
		}
		mcuIndex := re.SubexpIndex("mcu")
		if mcuIndex < 0 || mcuIndex >= len(matches) {
			continue
		}
		lineIndex := re.SubexpIndex("line")
		if lineIndex < 0 || lineIndex >= len(matches) {
			continue
		}
		timestamp := matches[timeIndex]
		mcu := strings.ToLower(matches[mcuIndex])
		line := matches[lineIndex]
		mcuFile, ok := mcuFiles[mcu]
		if !ok {
			mcuFile, err = os.Create(filepath.Join(destDir, fmt.Sprintf("%s.txt", mcu)))
			if err != nil {
				s.log(fmt.Sprintf("Failed to create servo log %s.txt: %v", mcu, err))
				mcuFiles[mcu] = nil
				continue
			}
			mcuFiles[mcu] = mcuFile
			defer mcuFile.Close()
		}
		if mcuFile == nil {
			continue
		}
		fmt.Fprintln(mcuFile, timestamp, "- ", line)
	}
}

func (s *LsNexus) downloadServodDMesgLogs(
	ctx context.Context, servoHostDestDir string) {
	s.log("Saving dmesg log")
	bolsReq := &bols.DMesgRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    int32(s.servodPort),
			ServoSerial:   s.servodSerial,
			ContainerName: s.servodContainer,
		},
	}
	stream, err := s.cl.DMesg(ctx, bolsReq)
	if err != nil {
		s.log(fmt.Sprintf("Failed to send dmesg request: %v", err))
		return
	}
	dmesgFile := filepath.Join(servoHostDestDir, "dmesg")
	f, err := os.Create(dmesgFile)
	if err != nil {
		s.log("Failed to create servo log dmesg: ", err)
		return
	}
	defer f.Close()
	for {
		data, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.log(fmt.Sprintf("failed to receive content of dmesg: %v", err))
		}
		f.Write(data.Output.GetStdout())
	}
}
