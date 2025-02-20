// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package service

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"regexp"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
)

type CrosTestRunnerServer struct {
	skylab_test_runner.UnimplementedCrosTestRunnerServiceServer

	metadata *ServerMetadata
	server   *grpc.Server

	sk *data.LocalTestStateKeeper
}

func NewCrosTestRunnerServer(metadata *ServerMetadata) (*CrosTestRunnerServer, func(), error) {
	var conns []*grpc.ClientConn
	closer := func() {
		for _, conn := range conns {
			conn.Close()
		}
		conns = nil
	}

	if err := ValidateExecuteRequest(metadata.InputProto); err != nil {
		return nil, closer, err
	}

	return &CrosTestRunnerServer{metadata: metadata}, closer, nil
}

func (server *CrosTestRunnerServer) Start() error {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", server.metadata.Port))
	if err != nil {
		return fmt.Errorf("failed to create listener at %d", server.metadata.Port)
	}

	// Write port number to ~/.cftmeta for go/cft-port-discovery
	err = exportMetadata(l, server.metadata.ServiceMetadataExportPath)
	if err != nil {
		log.Printf("Failed to write service metadata at provided path %s", server.metadata.ServiceMetadataExportPath)
	}

	// Construct state keeper to be used throughout the whole server session
	server.sk = server.ConstructStateKeeper()

	server.server = grpc.NewServer()
	skylab_test_runner.RegisterCrosTestRunnerServiceServer(server.server, server)
	reflection.Register(server.server)

	log.Println("cros-test-runner-service listen to request at ", l.Addr().String())
	return server.server.Serve(l)
}

func (server *CrosTestRunnerServer) ConstructStateKeeper() *data.LocalTestStateKeeper {
	sk := &data.LocalTestStateKeeper{}
	req := server.metadata.InputProto

	if req.GetHostName() != "" {
		sk.HostName = req.GetHostName()
	}

	if req.GetDutTopology() != nil {
		sk.DutTopology = req.GetDutTopology()
	}

	if req.GetDockerKeyFileLocation() != "" {
		sk.DockerKeyFileLocation = req.GetDockerKeyFileLocation()
	}

	if req.GetLogDataGsRoot() != "" {
		gcsurl := common.GetGcsURL(req.GetLogDataGsRoot())
		sk.GcsURL = gcsurl
		sk.TesthausURL = common.GetTesthausURL("", gcsurl)
	}

	sk.GcsPublishSrcDir = server.metadata.LogPath
	sk.UseDockerKeyDirectly = req.GetUseDockerKeyDirectly()

	return sk
}

func (server *CrosTestRunnerServer) Execute(ctx context.Context, req *skylab_test_runner.ExecuteRequest) (*skylab_test_runner.ExecuteResponse, error) {
	log.Println("Received ExecuteRequest: ", req)
	out := &skylab_test_runner.ExecuteResponse{}

	service, err := NewCrosTestRunnerService(req, server.sk)
	if err != nil {
		log.Printf("failed to create new cros-test-runner service: %s", err)
		return out, fmt.Errorf("failed to create new cros-test-runner service: %w", err)
	}

	logPath := path.Join(server.metadata.LogPath, req.ArtifactsPath)

	out, err = service.Execute(ctx, logPath, server.metadata.NoSudo)
	if err != nil {
		log.Printf("execution failed: %s", err)
		return out, fmt.Errorf("execution failed: %w", err)
	}

	log.Println("Execution finished successfully!")
	return out, nil
}

// ValidateExecuteRequest validates provided request.
func ValidateExecuteRequest(req *skylab_test_runner.CrosTestRunnerServerStartRequest) error {
	// TODO : Add all validations.
	return nil
}

// ParseServerStartReq parses CrosTestRunnerServerStartRequest input request data from
// the input file.
func ParseServerStartReq(path string) (*skylab_test_runner.CrosTestRunnerServerStartRequest, error) {
	in := &skylab_test_runner.CrosTestRunnerServerStartRequest{}
	r, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error while opening file at %s: %w", path, err)
	}

	data, err := os.ReadFile(r.Name())
	if err != nil {
		return nil, fmt.Errorf("error while reading file %s: %w", r.Name(), err)
	}

	umrsh := protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
	err = umrsh.Unmarshal(data, in)
	if err != nil {
		return nil, fmt.Errorf("err while unmarshalling: %w", err)
	}

	return in, nil
}

// exportMetadata exports cft service metadata.
func exportMetadata(address net.Listener, exportTo string) error {
	metaFile := path.Join(exportTo, ".cftmeta")

	f, err := os.OpenFile(metaFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("error: cannot open metadata file %v", err)
		return err
	}
	defer f.Close()

	r := regexp.MustCompile(`.*:(\d+)$`)
	match := r.FindStringSubmatch(address.Addr().String())
	if match == nil {
		log.Printf("error: cannot find port from address %v", address)
		return fmt.Errorf("cannot find port from address %v", address)
	}

	port := match[1]
	content := fmt.Sprintf("%s=%s\n%s=%s\n%s=%s\n",
		"SERVICE_PORT", port,
		"SERVICE_NAME", "cros_test_runner",
		"SERVICE_START_TIME", time.Now().Format(time.RFC3339))
	_, err = f.WriteString(content)
	if err != nil {
		log.Printf("error: cannot write to metadata file %v", err)
		return fmt.Errorf("cannot write to metadata file %w", err)
	}

	log.Printf("service metadata has been exported to %v", metaFile)
	return nil
}
