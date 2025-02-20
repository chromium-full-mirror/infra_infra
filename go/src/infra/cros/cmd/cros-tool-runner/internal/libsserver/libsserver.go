// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package libserver implements the test_libs_service.proto (see proto for details)
package libsserver

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"sync"

	"google.golang.org/grpc"

	pb "go.chromium.org/chromiumos/config/go/test/api"
)

const (
	connectionTimeout = 10 // How many seconds to wait for a docker container to load.
)

//go:embed registered_libs.json
var defaultRegisteredLibs []byte

// TestLibsServer represents a Test Libs Service Server.
type TestLibsServer struct {
	Port         int32
	listener     *net.Listener
	server       *grpc.Server
	logger       *log.Logger
	outputDir    string
	libsList     []*LibReg              // List of all registered libs.
	running      map[string]*RunningLib // Map of running libs indexed by id.
	cnts         map[string]int         // Map of lib name to number of started instances of that lib.
	uniquePrefix string                 // Unique prefix which references board/build; used to make unique docker names.
	token        string
	peripherals  map[string]string
}

// New creates a new server to listen to rpc requests.
func New(logger *log.Logger, outputDir, token string, req *pb.CrosToolRunnerTestRequest) (*TestLibsServer, error) {
	// Load pre-computed list of registered libs.
	// TODO (kathrelkeld): hardcoding this as the default value for now.
	libsListFilepath := ""
	var bytes []byte
	var err error
	if libsListFilepath == "" {
		bytes = defaultRegisteredLibs
	} else {
		bytes, err = ioutil.ReadFile(libsListFilepath)
		if err != nil {
			logger.Println("Could not read in registered libs:", err)
			return nil, err
		}
	}
	var libsList []*LibReg
	err = json.Unmarshal(bytes, &libsList)
	if err != nil {
		logger.Println("Could not parse registered libs:", err)
		return nil, err
	}

	initialCnts := make(map[string]int)
	for _, lib := range libsList {
		initialCnts[lib.Name] = 0
	}

	s := &TestLibsServer{
		logger:    logger,
		outputDir: outputDir,
		libsList:  libsList,
		running:   make(map[string]*RunningLib),
		cnts:      initialCnts,
		token:     token,
	}
	s.updateServerFromReq(req)

	logger.Println("Successfully created TestLibsServer")
	return s, nil
}

// updateServerFromReq puts information from the test request into the TestLibsServer struct.
func (s *TestLibsServer) updateServerFromReq(req *pb.CrosToolRunnerTestRequest) {
	if req == nil {
		s.uniquePrefix = "local-run"
		return
	}
	s.uniquePrefix = req.PrimaryDut.Dut.Id.Value

	chromeOS := req.PrimaryDut.Dut.GetChromeos()
	if chromeOS == nil {
		return
	}
	fmt.Printf("Servo:\n%s", chromeOS.Servo)
	addrs := make(map[string]string)
	if chromeOS.Servo != nil && chromeOS.Servo.Present {
		addrs["servo"] = fmt.Sprintf("%s:%d", chromeOS.Servo.ServodAddress.Address,
			chromeOS.Servo.ServodAddress.Port)
	}
	s.peripherals = addrs
}

// Serve creates and runs a grpc server with predefined parameters.
func (s *TestLibsServer) Serve(wg *sync.WaitGroup) error {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		fmt.Printf("Fatal err %s", err)
		log.Fatalln(err)
	}
	s.listener = &l
	s.Port = int32(l.Addr().(*net.TCPAddr).Port)

	if s.server != nil {
		s.server.Stop()
	}
	server := grpc.NewServer()
	pb.RegisterTestLibsServiceServer(server, s)
	s.server = server

	s.logger.Println("Running TestLibsServer on port", s.Port)
	wg.Done()
	return server.Serve(*s.listener)
}

// Stop stops a Test Libs Server, including killing all running docker containers.
func (s *TestLibsServer) Stop(ctx context.Context) {
	for id := range s.running {
		s.running[id].kill(ctx)
		defer delete(s.running, id)
	}

	s.listener = nil
	if s.server != nil {
		s.server.Stop()
		s.server = nil
	}

	s.logger.Println("TestLibsServer has been stopped.")
}

// StartLib takes in a request and starts the given library.
func (s *TestLibsServer) StartLib(ctx context.Context, req *pb.GetLibRequest) (*pb.GetLibResponse, error) {
	s.logger.Println("Received start request", req.Name, req.Version)

	for _, lInfo := range s.libsList {
		if lInfo.Name == req.Name {
			rl, err := s.newRunningLib(ctx, lInfo)
			if err != nil {
				s.logger.Println("Startup error:", err)
				return responseFailure(pb.GetLibFailure_REASON_CONTAINER_START_ERROR), err
			}
			return responseSuccess(rl.id, rl.port), nil
		}
	}
	return responseFailure(pb.GetLibFailure_REASON_UNREGISTERED_LIB), errors.New("could not find library to load")
}

// FindLib takes in a request and looks up the given library (or starts one if
// it is not already running).
func (s *TestLibsServer) FindLib(ctx context.Context, req *pb.GetLibRequest) (*pb.GetLibResponse, error) {
	s.logger.Println("Received find request", req.Name, req.Version)

	for _, r := range s.running {
		if r.info.Name == req.Name {
			return responseSuccess(r.id, r.port), nil
		}
	}
	return s.StartLib(ctx, req)
}

// KillLib handles a KillLibRequest to stop the given library.
func (s *TestLibsServer) KillLib(ctx context.Context, req *pb.KillLibRequest) (*pb.KillLibResponse, error) {
	s.logger.Println("Kill request", req.Id)

	lib, ok := s.running[req.Id]
	if ok {
		lib.kill(ctx)
		delete(s.running, req.Id)
	}

	response := &pb.KillLibResponse{}
	return response, nil
}
