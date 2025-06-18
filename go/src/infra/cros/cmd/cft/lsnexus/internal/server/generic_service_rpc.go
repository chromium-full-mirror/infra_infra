// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package server

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

func (service *LsNexus) Start(ctx context.Context, start *api.GenericStartRequest) (*api.GenericStartResponse, error) {
	req := start.GetMessage().GetValues()

	for key, val := range req {
		service.logger.Printf("%s: %s", key, val.String())
	}

	dut := labapi.Dut{}
	if err := extractDUTTo(req, "dut", &dut); err != nil {
		service.logger.Println("Failed to extract DUT topology: ", err)
		// Do not return error until lsnexus code is stabilized.
		return &api.GenericStartResponse{}, nil
	}

	if dut.GetChromeos().GetServo().GetState() == labapi.PeripheralState_BROKEN {
		service.logger.Println("LSNexus will not be operational because here is no working servo")
		return &api.GenericStartResponse{}, nil
	}

	// Parse out the request to fill in values for LsNexus.
	if err := extractStringTo(req, "board", &service.board); err != nil {
		service.logger.Println("Failed to parse board argument: ", err)
	}
	if service.board == "" {
		service.board = dut.GetChromeos().GetDutModel().GetBuildTarget()
	}

	if err := extractStringTo(req, "model", &service.model); err != nil {
		service.logger.Println("Failed to parse model argument: ", err)
	}
	if service.model == "" {
		service.model = dut.GetChromeos().GetDutModel().GetModelName()
	}

	pools, err := extractList(req, "pools")
	if err != nil {
		service.logger.Println("Failed to parse pools argument: ", err)
	}
	if len(pools) > 0 {
		service.pools = make([]string, len(pools))
		for i, pool := range pools {
			service.pools[i] = pool.GetStringValue()
		}
	} else {
		service.pools = dut.GetPools()
	}

	if err := extractStringTo(req, "servod_serial", &service.servodSerial); err != nil {
		service.logger.Println("Failed to parse servod_serial argument: ", err)
	}
	if service.servodSerial == "" {
		service.servodSerial = dut.GetChromeos().GetServo().GetSerial()
	}

	if err := extractStringTo(req, "servod_container", &service.servodContainer); err != nil {
		service.logger.Println("Failed to parse servod_container argument: ", err)
	}
	if service.servodContainer == "" {
		service.servodContainer = dut.GetChromeos().GetServo().GetContainerName()
	}

	if err := extractIntTo(req, "servod_port", &service.servodPort); err != nil {
		service.logger.Println("Failed to parse servod_port argument: ", err)
	}
	if service.servodPort == 0 {
		service.servodPort = dut.GetChromeos().GetServo().GetServodAddress().GetPort()
	}
	var bolsAddr string
	if err := extractStringTo(req, "bols_addr", &bolsAddr); err != nil {
		service.logger.Println("Failed to parse bols_addr argument: ", err)
	}
	if bolsAddr == "" {
		if service.servodContainer != "" {
			// BOLS is in a satlab. The BOLS address should be bols:9100
			bolsAddr = "bols:9100"
		} else if dut.GetChromeos().GetServo().GetServodAddress().GetAddress() != "" {
			// BOLS is in labstation. The port should be 9000.
			bolsAddr = fmt.Sprintf("%s:9000",
				dut.GetChromeos().GetServo().GetServodAddress().GetAddress())
		}
	}
	if bolsAddr != "" {
		service.logger.Println("Connecting to BOLS at address: ", bolsAddr)
		bolsClient, err := connectToBOLS(bolsAddr)
		if err != nil {
			service.logger.Printf("Failed to connect to BOLS at %s: %v\n", bolsAddr, err)
		} else {
			service.cl = bolsClient
			service.logger.Println("Connected to BOLS")
		}
	}

	service.logger.Println("Successfully served Start request")
	service.logger.Println("board: ", service.board)
	service.logger.Println("model: ", service.model)
	service.logger.Println("pools: ", service.pools)
	service.logger.Println("servod_serial: ", service.servodSerial)
	service.logger.Println("servod_container: ", service.servodContainer)
	service.logger.Println("servod_port: ", service.servodPort)

	return &api.GenericStartResponse{}, nil
}

func (service *LsNexus) Stop(ctx context.Context, req *api.GenericStopRequest) (*api.GenericStopResponse, error) {
	if service.cl == nil {
		service.logger.Println("LSNexus will not save servod log because there is no BOLS client")
		return &api.GenericStopResponse{}, nil
	}
	if err := service.saveServodLogs(ctx); err != nil {
		service.logger.Println("Warning: failed to download servod logs: ", err)
	}
	return &api.GenericStopResponse{}, nil
}

func extractStringTo(req map[string]*anypb.Any, key string, to *string) error {
	if untyped, ok := req[key]; ok {
		val := structpb.Value{}
		if err := untyped.UnmarshalTo(&val); err != nil {
			return err
		}
		*to = val.GetStringValue()
	}
	return nil
}

func extractIntTo(req map[string]*anypb.Any, key string, to *int32) error {
	if untyped, ok := req[key]; ok {
		val := structpb.Value{}
		if err := untyped.UnmarshalTo(&val); err != nil {
			return err
		}
		*to = int32(val.GetNumberValue())
	}
	return nil
}

func extractList(req map[string]*anypb.Any, key string) ([]*structpb.Value, error) {
	if untyped, ok := req[key]; ok {
		val := structpb.Value{}
		if err := untyped.UnmarshalTo(&val); err != nil {
			return nil, err
		}
		return val.GetListValue().GetValues(), nil
	}
	return nil, nil
}

func extractDUTTo(req map[string]*anypb.Any, key string, to *labapi.Dut) error {
	if untyped, ok := req[key]; ok {
		if err := untyped.UnmarshalTo(to); err != nil {
			return err
		}
	}
	return nil
}
