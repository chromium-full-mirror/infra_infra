// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"runtime/debug"

	"google.golang.org/grpc"

	"go.chromium.org/chromiumos/config/go/longrunning"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/lro"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/cft/execution/cros-test/internal/common"
)

// ExecutionServiceServer implementation of dut_service.proto
type ExecutionServiceServer struct {
	manager       *lro.Manager
	logger        *log.Logger
	resultRootDir string
	tlwAddr       string
	metadata      *api.TestCaseMetadataList
}

// NewServer creates an execution server.
func NewServer(logger *log.Logger, resultRootDir, tlwAddr string, metadataList *api.TestCaseMetadataList) (*grpc.Server, func()) {
	s := &ExecutionServiceServer{
		manager: lro.New(),
		logger:  logger,

		resultRootDir: resultRootDir,
		tlwAddr:       tlwAddr,
		metadata:      metadataList,
	}

	server := grpc.NewServer()
	var conns []*grpc.ClientConn
	closer := func() {
		for _, conn := range conns {
			conn.Close()
		}
		conns = nil
	}
	api.RegisterExecutionServiceServer(server, s)
	longrunning.RegisterOperationsServer(server, s.manager)
	logger.Println("crostestservice listen to request at ")
	return server, closer
}

// RunTests calls the runTests flow in cros-test.
func (s *ExecutionServiceServer) RunTests(ctx context.Context, req *api.CrosTestRequest) (*longrunning.Operation, error) {
	op := s.manager.NewOperation()

	defer func() {
		if r := recover(); r != nil {
			s.logger.Println("Recovered from panic: ", r)

			stack := string(debug.Stack())

			s.logger.Println(stack)
		}
	}()

	s.logger.Println("Received api.CrosTestRequest: ", req)

	resultsDir, err := s.loadResultsDir(req)
	if err != nil {
		return op, errors.Annotate(err, "RunTests: unable to determine results directory path").Err()
	}
	if err = updateShadowConfig(); err != nil {
		return op, err
	}

	rspn, err := runTests(ctx, s.logger, resultsDir, s.tlwAddr, s.metadata, req)
	if err != nil {
		s.logger.Printf("Error running tests, %s", err)
		return op, errors.Annotate(err, "RunTests: failed to run test").Err()
	}
	s.logger.Printf("Test RPC Command was successful")
	// Note: We are setting the response on the LRO, rather than writing to a resultJson like CLI mode.
	_ = s.manager.SetResult(op.Name, rspn)
	return op, nil
}

func (s *ExecutionServiceServer) loadResultsDir(req *api.CrosTestRequest) (string, error) {
	_, resultsSubDir, err := common.UnpackMetadata(req)
	if err != nil {
		return "", err
	}

	if resultsSubDir == "" {
		return s.resultRootDir, nil
	}

	resultsDir := path.Join(s.resultRootDir, resultsSubDir)
	s.logger.Printf("WARNING: overriding default results directory path with %v", resultsDir)
	return resultsDir, nil
}

func updateShadowConfig() error {
	if imageStorageServer := os.Getenv("DRONE_AGENT_GCS_IMAGE_STORAGE_SERVER"); imageStorageServer != "" {
		shadowConfig := "/usr/local/autotest/shadow_config.ini"
		f, err := os.Create(shadowConfig)
		if err != nil {
			return errors.Annotate(err, fmt.Sprintf("Unable to create %s", shadowConfig)).Err()
		}
		_, err = f.WriteString(fmt.Sprintf("\n[CROS]\nimage_storage_server: gs://%s\n", imageStorageServer))
		if err != nil {
			f.Close()
			return errors.Annotate(err, fmt.Sprintf("Failed to write to %s", shadowConfig)).Err()
		}
		f.Close()
	}
	return nil
}
