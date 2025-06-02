// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package server

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.chromium.org/chromiumos/config/go/test/api"
)

func (service *LsNexus) Start(ctx context.Context, start *api.GenericStartRequest) (*api.GenericStartResponse, error) {
	req := start.GetMessage().GetValues()

	for key, val := range req {
		service.logger.Printf("%s: %s", key, val.String())
	}

	// Parse out the request to fill in values for LsNexus.
	var bolsAddr string
	if err := extractStringTo(req, "bols_addr", &bolsAddr); err != nil {
		return nil, err
	}
	if bolsAddr != "" {
		service.logger.Println("Connecting to BOLS at address: ", bolsAddr)
		bolsClient, err := connectToBOLS(bolsAddr)
		if err != nil {
			return nil, err
		}
		service.cl = bolsClient
		service.logger.Println("Connected to BOLS")
	}

	if err := extractStringTo(req, "board", &service.board); err != nil {
		return nil, err
	}

	if err := extractStringTo(req, "model", &service.model); err != nil {
		return nil, err
	}

	pools, err := extractList(req, "pools")
	if err != nil {
		return nil, err
	}
	service.pools = make([]string, len(pools))
	for i, pool := range pools {
		service.pools[i] = pool.GetStringValue()
	}

	if err := extractStringTo(req, "servod_serial", &service.servodSerial); err != nil {
		return nil, err
	}

	if err := extractStringTo(req, "servod_container", &service.servodContainer); err != nil {
		return nil, err
	}

	if err := extractIntTo(req, "servod_port", &service.servodPort); err != nil {
		return nil, err
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

func extractIntTo(req map[string]*anypb.Any, key string, to *int) error {
	if untyped, ok := req[key]; ok {
		val := structpb.Value{}
		if err := untyped.UnmarshalTo(&val); err != nil {
			return err
		}
		*to = int(val.GetNumberValue())
	}
	return nil
}

func extractList(req map[string]*anypb.Any, key string) ([]*structpb.Value, error) {
	if untyped, ok := req[key]; ok {
		val := structpb.Value{}
		if err := untyped.UnmarshalTo(&val); err != nil {
			return nil, err
		}
	}
	return nil, nil
}
