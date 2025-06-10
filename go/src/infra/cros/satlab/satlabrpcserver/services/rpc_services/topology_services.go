// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package rpc_services

import (
	"context"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/prototext"

	pb "go.chromium.org/chromiumos/infra/proto/go/satlabrpcserver"

	"go.chromium.org/infra/cros/satlab/common/satlabcommands"
	"go.chromium.org/infra/cros/satlab/common/topology"
)

func (s *SatlabRpcServiceServer) Topology(ctx context.Context, in *pb.TopologyRequest) (*pb.TopologyResponse, error) {
	satlabID, err := satlabcommands.GetDockerHostBoxIdentifier(ctx, s.commandExecutor)
	if err != nil {
		return nil, err
	}

	req := topology.GetTopology{
		SatlabID: satlabID,
		Hostname: in.GetHostname(),
	}

	resp, err := req.TriggerRun(ctx, s.commandExecutor)
	if err != nil {
		return nil, err
	}

	content := ""
	if resp != nil {
		content = prototext.Format(resp)
	}

	return &pb.TopologyResponse{PasitHost: content}, nil
}

func createTopologyFile(content string) (string, error) {
	// use uuid as filename
	id := uuid.New().String()

	// Define the file path in the system's temp dir
	// and use .textproto extension
	tempDir := os.TempDir()
	filePath := filepath.Join(tempDir, id+".textproto")
	// Create and open the file
	file, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		return "", err
	}

	return filePath, nil
}

func (s *SatlabRpcServiceServer) AddTopology(ctx context.Context, in *pb.AddTopologyRequest) (*pb.AddTopologyResponse, error) {
	satlabID, err := satlabcommands.GetDockerHostBoxIdentifier(ctx, s.commandExecutor)
	if err != nil {
		return nil, err
	}

	filePath, err := createTopologyFile(in.GetContent())
	if err != nil {
		return nil, err
	}

	defer os.Remove(filePath)

	req := topology.AddTopology{
		SatlabID:     satlabID,
		Hostname:     in.GetHostname(),
		TopologyPath: filePath,
	}

	if err := req.TriggerRun(ctx, s.commandExecutor); err != nil {
		return nil, err
	}

	// Needed to populate old PasitHost2 to new Pasit.
	// TODO (b/421860961) remove once crrev.com/c/6542328/6 will be merged.
	rdr := &pb.RepairDutsRequest{
		Hostnames: []string{in.GetHostname()},
		Deep:      false,
	}
	if _, err := s.RepairDuts(ctx, rdr); err != nil {
		return nil, err
	}

	return &pb.AddTopologyResponse{}, nil
}

func (s *SatlabRpcServiceServer) DeleteTopology(ctx context.Context, in *pb.DeleteTopologyRequest) (*pb.DeleteTopologyResponse, error) {
	satlabID, err := satlabcommands.GetDockerHostBoxIdentifier(ctx, s.commandExecutor)
	if err != nil {
		return nil, err
	}

	req := topology.DeleteTopology{
		SatlabID: satlabID,
		Hostname: in.GetHostname(),
	}

	if err := req.TriggerRun(ctx, s.commandExecutor); err != nil {
		return nil, err
	}

	return &pb.DeleteTopologyResponse{}, nil
}
