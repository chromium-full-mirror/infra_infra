// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package server implement lsnexus-service API.
package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/bols"
	"go.chromium.org/chromiumos/config/go/test/api/lsnexus"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

func TestStartServod(t *testing.T) {
	var port int32 = 9999
	serial := "serial"
	containerName := "container"
	board := "board"
	model := "model"
	handler := func(ctx context.Context, req *bols.StartServodRequest) (*bols.StartServodResponse, error) {
		if req.GetStationId().GetServodPort() != port {
			return nil, fmt.Errorf("port number mismatched: got: %d wamted: %d",
				req.GetStationId().GetServodPort(), port)
		}
		if req.GetStationId().GetContainerName() != containerName {
			return nil, fmt.Errorf("container name mismatched: got: %s wamted: %s",
				req.GetStationId().GetContainerName(), containerName)
		}
		if req.GetBoard() != board {
			return nil, fmt.Errorf("board mismatched: got: %s wamted: %s",
				req.GetBoard(), board)
		}
		if req.GetModel() != model {
			return nil, fmt.Errorf("model mismatched: got: %s wamted: %s",
				req.GetModel(), model)
		}
		return &bols.StartServodResponse{}, nil
	}
	bolsService := &mockBolsService{
		startServodHandler: handler,
	}
	stopBols, bolsAddr, err := startMockBolsServer(bolsService)
	if err != nil {
		t.Fatalf("failed to start mock BOLS server: %v", err)
	}
	defer stopBols()
	ctx := context.Background()
	stopLsNexus, lsNexusAddr, err := startLSNexusServer(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("failed to start LSNexus server: %v", err)
	}
	defer stopLsNexus()

	conn, err := grpc.NewClient(lsNexusAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to create LSNexus client: %v", err)
	}

	genericClient := testapi.NewGenericServiceClient(conn)
	startRequest, err := createGenericStartRequest(map[string]proto.Message{
		"bols_addr":        structpb.NewStringValue(bolsAddr),
		"board":            structpb.NewStringValue(board),
		"model":            structpb.NewStringValue(model),
		"servod_serial":    structpb.NewStringValue(serial),
		"servod_container": structpb.NewStringValue(containerName),
		"servod_port":      structpb.NewNumberValue(float64(port)),
	})
	if err != nil {
		t.Fatalf("failed to create generic start request: %v", err)
	}
	if _, err := genericClient.Start(ctx, startRequest); err != nil {
		t.Fatalf("Failed to call Start: %v", err)
	}

	cl := lsnexus.NewLSNexusServiceClient(conn)
	if _, err := cl.StartServod(ctx, &lsnexus.StartServodRequest{}); err != nil {
		t.Fatalf("failed to call StartServod: %v", err)
	}
}

func TestCallServodSet(t *testing.T) {
	var port int32 = 9999
	serial := "serial"
	containerName := "container"
	board := "board"
	model := "model"
	control := "control"
	value := "value"
	handler := func(ctx context.Context, req *bols.SetServodRequest) (*bols.SetServodResponse, error) {
		if req.GetStationId().GetServodPort() != port {
			return nil, fmt.Errorf("port number mismatched: got: %d wamted: %d",
				req.GetStationId().GetServodPort(), port)
		}
		if req.GetStationId().GetContainerName() != containerName {
			return nil, fmt.Errorf("container name mismatched: got: %s wamted: %s",
				req.GetStationId().GetContainerName(), containerName)
		}
		if req.GetControl() != control {
			return nil, fmt.Errorf("control mismatched: got: %s wamted: %s",
				req.GetControl(), control)
		}
		if req.GetValue().GetStringValue() != value {
			return nil, fmt.Errorf("value mismatched: got: %s wamted: %s",
				req.GetValue().GetStringValue(), value)

		}
		return &bols.SetServodResponse{}, nil
	}
	bolsService := &mockBolsService{
		setServodHandler: handler,
	}
	stopBols, bolsAddr, err := startMockBolsServer(bolsService)
	if err != nil {
		t.Fatalf("failed to start mock BOLS server: %v", err)
	}
	defer stopBols()
	ctx := context.Background()
	stopLsNexus, lsNexusAddr, err := startLSNexusServer(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("failed to start LSNexus server: %v", err)
	}
	defer stopLsNexus()

	conn, err := grpc.NewClient(lsNexusAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to create LSNexus client: %v", err)
	}

	genericClient := testapi.NewGenericServiceClient(conn)
	startRequest, err := createGenericStartRequest(map[string]proto.Message{
		"bols_addr":        structpb.NewStringValue(bolsAddr),
		"board":            structpb.NewStringValue(board),
		"model":            structpb.NewStringValue(model),
		"servod_serial":    structpb.NewStringValue(serial),
		"servod_container": structpb.NewStringValue(containerName),
		"servod_port":      structpb.NewNumberValue(float64(port)),
	})
	if err != nil {
		t.Fatalf("failed to create generic start request: %v", err)
	}
	if _, err := genericClient.Start(ctx, startRequest); err != nil {
		t.Fatalf("Failed to call Start: %v", err)
	}

	cl := lsnexus.NewLSNexusServiceClient(conn)
	rspn, err := cl.CallServod(ctx, &lsnexus.CallServodRequest{
		Method:  lsnexus.CallServodRequest_SET,
		Control: control,
		Args: []*bols.ServodValue{
			{Value: &bols.ServodValue_StringValue{StringValue: value}},
		},
	})
	if err != nil {
		t.Fatalf("failed to call CallServod: %v", err)
	}
	result := rspn.GetSuccess()
	if result == nil {
		t.Fatal("failed to get success result from CallServod response")
	}
}

func TestCallServodGet(t *testing.T) {
	var port int32 = 9999
	serial := "serial"
	containerName := "container"
	board := "board"
	model := "model"
	control := "control"
	value := "value"
	handler := func(ctx context.Context, req *bols.GetServodRequest) (*bols.GetServodResponse, error) {
		if req.GetStationId().GetServodPort() != port {
			return nil, fmt.Errorf("port number mismatched: got: %d wamted: %d",
				req.GetStationId().GetServodPort(), port)
		}
		if req.GetStationId().GetContainerName() != containerName {
			return nil, fmt.Errorf("container name mismatched: got: %s wamted: %s",
				req.GetStationId().GetContainerName(), containerName)
		}
		if req.GetControl() != control {
			return nil, fmt.Errorf("control mismatched: got: %s wamted: %s",
				req.GetControl(), control)
		}
		return &bols.GetServodResponse{
			Control: control,
			Value:   &bols.ServodValue{Value: &bols.ServodValue_StringValue{StringValue: value}},
		}, nil
	}
	bolsService := &mockBolsService{
		getServodHandler: handler,
	}
	stopBols, bolsAddr, err := startMockBolsServer(bolsService)
	if err != nil {
		t.Fatalf("failed to start mock BOLS server: %v", err)
	}
	defer stopBols()
	ctx := context.Background()
	stopLsNexus, lsNexusAddr, err := startLSNexusServer(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("failed to start LSNexus server: %v", err)
	}
	defer stopLsNexus()

	conn, err := grpc.NewClient(lsNexusAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to create LSNexus client: %v", err)
	}

	genericClient := testapi.NewGenericServiceClient(conn)
	startRequest, err := createGenericStartRequest(map[string]proto.Message{
		"bols_addr":        structpb.NewStringValue(bolsAddr),
		"board":            structpb.NewStringValue(board),
		"model":            structpb.NewStringValue(model),
		"servod_serial":    structpb.NewStringValue(serial),
		"servod_container": structpb.NewStringValue(containerName),
		"servod_port":      structpb.NewNumberValue(float64(port)),
	})
	if err != nil {
		t.Fatalf("failed to create generic start request: %v", err)
	}
	if _, err := genericClient.Start(ctx, startRequest); err != nil {
		t.Fatalf("Failed to call Start: %v", err)
	}

	cl := lsnexus.NewLSNexusServiceClient(conn)
	rspn, err := cl.CallServod(ctx, &lsnexus.CallServodRequest{
		Method:  lsnexus.CallServodRequest_GET,
		Control: control,
	})
	if err != nil {
		t.Fatalf("failed to call CallServod: %v", err)
	}
	result := rspn.GetSuccess()
	if result == nil {
		t.Fatal("failed to get success result from CallServod response")
	}
	if result.GetResult().GetStringValue() != value {
		t.Fatalf("failed to get correct result; got %q: wanted: %q",
			result.GetResult().GetStringValue(), value)
	}
}

// service holds info for server.
type mockBolsService struct {
	bols.UnimplementedBolsServiceServer
	startServodHandler func(context.Context, *bols.StartServodRequest) (*bols.StartServodResponse, error)
	getServodHandler   func(context.Context, *bols.GetServodRequest) (*bols.GetServodResponse, error)
	setServodHandler   func(context.Context, *bols.SetServodRequest) (*bols.SetServodResponse, error)
}

func (s *mockBolsService) StartServod(ctx context.Context, req *bols.StartServodRequest) (*bols.StartServodResponse, error) {
	if s.startServodHandler != nil {
		return s.startServodHandler(ctx, req)
	}
	return nil, status.Errorf(codes.Unimplemented, "method StartServod not implemented")
}

func (s *mockBolsService) GetServod(ctx context.Context, req *bols.GetServodRequest) (*bols.GetServodResponse, error) {
	if s.getServodHandler != nil {
		return s.getServodHandler(ctx, req)
	}
	return nil, status.Errorf(codes.Unimplemented, "method GetServod not implemented")
}

func (s *mockBolsService) SetServod(ctx context.Context, req *bols.SetServodRequest) (*bols.SetServodResponse, error) {
	if s.setServodHandler != nil {
		return s.setServodHandler(ctx, req)
	}
	return nil, status.Errorf(codes.Unimplemented, "method GetServod not implemented")
}

func startMockBolsServer(mockServer *mockBolsService) (stopFunc func(), addr string, err error) {
	srv := grpc.NewServer()
	bols.RegisterBolsServiceServer(srv, mockServer)
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, "", fmt.Errorf("Failed to listen: %w", err)
	}

	go srv.Serve(lis)

	return func() {
		srv.Stop()
	}, lis.Addr().String(), nil
}

func createGenericStartRequest(protoMap map[string]proto.Message) (*testapi.GenericStartRequest, error) {
	values := map[string]*anypb.Any{}
	for key, protoObj := range protoMap {
		asAny, err := anypb.New(protoObj)
		if err != nil {
			return nil, err
		}
		values[key] = asAny
	}
	return &testapi.GenericStartRequest{
		Message: &testapi.GenericMessage{
			Values: values,
		},
	}, nil
}

func startLSNexusServer(ctx context.Context, dir string) (stopFunc func(), addr string, err error) {
	logFile, err := common.CreateLogFile(dir)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()
	logger := common.NewLogger(logFile)
	log.SetOutput(logger.Writer())

	server := NewServer(logger)

	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create a net listener: %w", err)
	}
	go server.Serve(l)

	return func() {
		server.Stop()
	}, l.Addr().String(), nil
}
