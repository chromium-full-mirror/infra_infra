// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servertemplate

import (
	"context"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/test/post_process/cmd/post-process/common"
	"go.chromium.org/luci/common/errors"
)

// GenericFilterServiceServer ...
type GenericFilterServiceServer struct {
	api.GenericFilterServiceServer
	logPath      string
	name         string
	serverLogger *log.Logger
	commonParams *CommonFilterParams
	executor     func(req *api.InternalTestplan, log *log.Logger, commonParams *CommonFilterParams) (*api.InternalTestplan, error)
}

// NewServer creates an execution server.
func NewServer(logger *log.Logger, logPath, name string, commonParams *CommonFilterParams, executor func(req *api.InternalTestplan, log *log.Logger, commonParams *CommonFilterParams) (*api.InternalTestplan, error)) (*grpc.Server, func()) {
	s := &GenericFilterServiceServer{
		logPath:      logPath,
		name:         name,
		serverLogger: logger,
		commonParams: commonParams,
		executor:     executor,
	}

	server := grpc.NewServer(grpc.MaxRecvMsgSize(1024*1024*32), grpc.MaxSendMsgSize(1024*1024*32))
	var conns []*grpc.ClientConn
	closer := func() {
		for _, conn := range conns {
			conn.Close()
		}
		conns = nil
	}
	api.RegisterGenericFilterServiceServer(server, s)
	// Register reflection service on gRPC server.
	reflection.Register(server)

	logger.Println("filterService listening for requests")
	return server, closer
}

// Execute executes s.executor() with the req. The executor method must be provided on startup to NewServer.
func (s *GenericFilterServiceServer) Execute(ctx context.Context, req *api.InternalTestplan) (*api.InternalTestplan, error) {
	t := time.Now()
	suiteName := req.GetSuiteInfo().GetSuiteRequest().GetTestSuite().GetName()
	logPath := filepath.Join(s.logPath, suiteName, s.name, t.Format("20060102-150405"))
	s.serverLogger.Printf("Creating Log File at %s", logPath)
	logFile, err := common.CreateLogFile(logPath)
	if err != nil {
		err = fmt.Errorf("failed to create log file: %s", err)
		s.serverLogger.Println(err.Error())
		return req, err
	}
	defer logFile.Close()
	logger := s.serverLogger
	logger.SetFlags(log.LstdFlags | log.LUTC | log.Lshortfile)
	logger.SetPrefix(fmt.Sprintf("%s: ", suiteName))

	logger.Printf("Received Request: %s", req)

	rspn, err := s.executor(req, logger, s.commonParams)
	if err != nil {
		return nil, errors.Annotate(err, "Executor: failed to run").Err()
	}
	logger.Printf("Execute RPC Command was successful")

	return rspn, nil
}

// ExecuteStream takes in a streamed InternalTestplan and then calls the s.Execute() path.
func (s *GenericFilterServiceServer) ExecuteStream(stream api.GenericFilterService_ExecuteStreamServer) error {
	testplan, err := ReceiveRequest(stream.Recv)
	if err != nil {
		return err
	}

	testplan, err = s.Execute(stream.Context(), testplan)
	if err != nil {
		return err
	}

	err = SendRequest(stream.Send, testplan)
	if err != nil {
		return err
	}
	return nil
}

// ReceiveRequest waits on InternalTestplanFragments and compiles them into a full InternalTestplan.
func ReceiveRequest(recvFunc func() (*api.InternalTestplanFragment, error)) (*api.InternalTestplan, error) {
	// Gather fragments
	streamedBytes := []byte{}
	for {
		fragment, err := recvFunc()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		streamedBytes = append(streamedBytes, fragment.GetFragment()...)
	}

	// Convert fragments into InternalTestplan
	testplan := &api.InternalTestplan{}
	unmarshaler := proto.UnmarshalOptions{
		DiscardUnknown: true,
	}
	err := unmarshaler.Unmarshal(streamedBytes, testplan)
	if err != nil {
		return nil, err
	}

	return testplan, nil
}

// SendRequest breaks the InternalTestplan object into 2MB fragments and sends them off to the caller.
func SendRequest(sendFunc func(*api.InternalTestplanFragment) error, testplan *api.InternalTestplan) error {
	bytes, err := proto.Marshal(testplan)
	if err != nil {
		return err
	}

	// Partition by 2 MB and stream
	for _, byteFragment := range PartitionBytesBySize(bytes, 2*1024*1024) {
		testplanFragment := &api.InternalTestplanFragment{
			Fragment: byteFragment,
		}

		if err := sendFunc(testplanFragment); err != nil {
			return err
		}
	}

	return nil
}

// PartitionBytesBySize splits a byte array into many byte arrays
// according to the maxSize passed in.
func PartitionBytesBySize(inBytes []byte, maxSize uint64) [][]byte {
	outBytes := [][]byte{}

	increment := int(maxSize)
	for start := 0; start < len(inBytes); start += increment {
		end := start + increment
		if len(inBytes) < end {
			end = len(inBytes)
		}
		outBytes = append(outBytes, inBytes[start:end])
	}

	return outBytes
}
