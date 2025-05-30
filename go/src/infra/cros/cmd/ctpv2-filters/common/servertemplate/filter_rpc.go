// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servertemplate

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/streaming"
)

// GenericFilterServiceServer ...
type GenericFilterServiceServer struct {
	api.GenericFilterServiceServer
	LogPath            string
	Name               string
	ServerLogger       *log.Logger
	ExecutionGenerator ExecutorGeneratorFunc
}

type Filter interface {
	Init(*flag.FlagSet, []string) error
	Executor(req *api.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) (*api.InternalTestplan, error)
}

type FilterBase struct {
	Filter
}

func (fb *FilterBase) Init(fs *flag.FlagSet, args []string) error {
	return fs.Parse(args)
}

func (fb *FilterBase) Executor(req *api.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) (*api.InternalTestplan, error) {
	return req, fmt.Errorf("not implemented")
}

type ExecutorGeneratorFunc func() Filter

// NewServer creates an execution server.
func NewServer(logger *log.Logger, logPath, name string, executorGenerator ExecutorGeneratorFunc) (*grpc.Server, func()) {
	s := &GenericFilterServiceServer{
		LogPath:            logPath,
		Name:               name,
		ServerLogger:       logger,
		ExecutionGenerator: executorGenerator,
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
	logPath := filepath.Join(s.LogPath, suiteName, s.Name, t.Format("20060102-150405"))
	s.ServerLogger.Printf("Creating Log File at %s", logPath)
	logFile, err := common.CreateLogFile(logPath)
	if err != nil {
		err = fmt.Errorf("failed to create log file: %w", err)
		s.ServerLogger.Println(err.Error())
		return req, err
	}
	defer logFile.Close()
	logger := s.ServerLogger
	logger.SetFlags(log.LstdFlags | log.LUTC | log.Lshortfile)
	logger.SetPrefix(fmt.Sprintf("%s: ", suiteName))

	logger.Printf("Received Request: %s", req)

	commonParams := &common.CommonFilterParams{
		AuthHelper: streaming.NewLocalAuthHandler(),
	}
	commonFlagset := flag.NewFlagSet("filter flags", flag.ContinueOnError)
	commonFlagset.StringVar(&commonParams.FirestoreDatabaseName, "firestore", TestPlatformFireStore, fmt.Sprintf("Firestore database name to pull from. Default value is %s", TestPlatformFireStore))
	commonFlagset.StringVar(&commonParams.Environment, "env", "prod", "Environment of the run. Default value is prod")
	executor := s.ExecutionGenerator()
	err = executor.Init(commonFlagset, os.Args[2:])
	if err != nil {
		return nil, errors.Annotate(err, "Init: failed to parse args").Err()
	}
	rspn, err := executor.Executor(req, logger, commonParams)
	if err != nil {
		return nil, errors.Annotate(err, "Executor: failed to run").Err()
	}
	logger.Printf("Execute RPC Command was successful")

	return rspn, nil
}

func (s *GenericFilterServiceServer) ExecuteWithStream(stream api.GenericFilterService_ExecuteWithStreamServer) (err error) {
	defer CapturePanic(log.Default(), &err)

	clientCommunicationHandler := streaming.NewClientCommunicationHandler(stream)
	defer clientCommunicationHandler.Close()
	go clientCommunicationHandler.HandleStreamFromClient()
	go clientCommunicationHandler.HandleStreamToClient()

	logger := clientCommunicationHandler.GetLogger()
	logger.Println("Client communication established, streaming logs.")

	args, err := clientCommunicationHandler.GetArgs()
	if err != nil {
		return err
	}

	testplan, err := clientCommunicationHandler.GetInternalTestplan()
	if err != nil {
		return err
	}
	logger.Printf("Received InternalTestplan: %s", testplan)

	commonParams := &common.CommonFilterParams{
		AuthHelper: streaming.NewAuthHandler(clientCommunicationHandler),
	}

	testplan, err = s.execute(testplan, logger, args.Args, commonParams)
	if err != nil {
		return errors.Annotate(err, "Executor: failed to run").Err()
	}

	logger.Printf("Execute RPC Command was successful")
	// When InternalTestplan is sent, the client will mark the stream as closed.
	// No more message streaming should occur past this point.
	err = clientCommunicationHandler.SendInternalTestplan(testplan)
	if err != nil {
		return errors.Annotate(err, "Executor: failed to send InternalTestplan").Err()
	}
	// Receive an empty testplan as ack to close server after success.
	clientCommunicationHandler.GetInternalTestplan()
	return nil
}

func (s *GenericFilterServiceServer) execute(req *api.InternalTestplan, logger *log.Logger, args []string, commonParams *common.CommonFilterParams) (resp *api.InternalTestplan, err error) {
	defer CapturePanic(logger, &err)
	resp = req

	executor := s.ExecutionGenerator()

	commonFlagset := flag.NewFlagSet("filter flags", flag.ContinueOnError)
	commonFlagset.StringVar(&commonParams.FirestoreDatabaseName, "firestore", TestPlatformFireStore, fmt.Sprintf("Firestore database name to pull from. Default value is %s", TestPlatformFireStore))
	commonFlagset.StringVar(&commonParams.Environment, "env", "prod", "Environment of the run. Default value is prod")
	err = executor.Init(commonFlagset, args)
	if err != nil {
		return resp, err
	}
	resp, err = executor.Executor(req, logger, commonParams)
	return
}

func CapturePanic(logger *log.Logger, err *error) {
	if r := recover(); r != nil {
		richError := fmt.Errorf("%s\n%s", r, string(debug.Stack()))
		*err = richError
	}
}
