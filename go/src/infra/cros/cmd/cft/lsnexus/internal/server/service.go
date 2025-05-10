// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package server implement lsnexus-service API.
package server

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/config/go/test/api/bols"
	"go.chromium.org/chromiumos/config/go/test/api/lsnexus"
)

func (s *service) StartServod(ctx context.Context, req *lsnexus.StartServodRequest) (*lsnexus.StartServodResponse, error) {
	s.log("Serving StartServod request")
	bolsReq := &bols.StartServodRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    s.dutTopology.GetChromeos().GetServo().GetServodAddress().GetPort(),
			ServoSerial:   s.dutTopology.GetChromeos().GetServo().GetSerial(),
			ContainerName: s.dutTopology.GetChromeos().GetServo().GetContainerName(),
		},
		Board: s.dutTopology.GetChromeos().GetDutModel().GetBuildTarget(),
		Model: s.dutTopology.GetChromeos().GetDutModel().GetModelName(),
	}
	if _, err := s.cl.StartServod(ctx, bolsReq); err != nil {
		err = fmt.Errorf("failed to start servod: %w", err)
		s.log(err)
		return nil, err
	}
	s.log("Successfully served StartServod request")
	return &lsnexus.StartServodResponse{}, nil
}

func (s *service) CallServod(ctx context.Context, req *lsnexus.CallServodRequest) (*lsnexus.CallServodResponse, error) {
	s.log("Serving CallServod request")
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
	}
	return nil, status.Error(codes.Unimplemented, "the specified call servod method not implemented")
}

func (s *service) log(args ...any) {
	if s.logger == nil {
		return
	}
	s.logger.Println(args...)
}

func (s *service) getServodRequest(ctx context.Context, req *lsnexus.CallServodRequest) (*lsnexus.CallServodResponse, error) {
	args := req.GetArgs()
	if len(args) != 1 {
		err := fmt.Errorf("get servod request has wrong number of arguments: got: %d expected: 1", len(args))
		s.log(err)
		return nil, err
	}
	if args[0].GetStringValue() == "" {
		err := errors.New("get servod request needs an non-empty string as control value")
		s.log(err)
		return nil, err
	}
	bolsReq := &bols.GetServodRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    s.dutTopology.GetChromeos().GetServo().GetServodAddress().GetPort(),
			ServoSerial:   s.dutTopology.GetChromeos().GetServo().GetSerial(),
			ContainerName: s.dutTopology.GetChromeos().GetServo().GetContainerName(),
		},
		Control: args[0].GetStringValue(),
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

func (s *service) setServodRequest(ctx context.Context, req *lsnexus.CallServodRequest) (*lsnexus.CallServodResponse, error) {
	args := req.GetArgs()
	if len(args) != 2 {
		err := fmt.Errorf("set servod request has wrong number of arguments: got: %d expected: 2", len(args))
		s.log(err)
		return nil, err
	}
	if args[0].GetStringValue() == "" {
		err := errors.New("gst servod request needs an non-empty string as control value")
		s.log(err)
		return nil, err
	}
	bolsReq := &bols.SetServodRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    s.dutTopology.GetChromeos().GetServo().GetServodAddress().GetPort(),
			ServoSerial:   s.dutTopology.GetChromeos().GetServo().GetSerial(),
			ContainerName: s.dutTopology.GetChromeos().GetServo().GetContainerName(),
		},
		Control: args[0].GetStringValue(),
		Value:   args[1],
	}
	if _, err := s.cl.SetServod(ctx, bolsReq); err != nil {
		err = fmt.Errorf("failed to make SetServod request: %w", err)
		s.log(err)
		return nil, err
	}
	return &lsnexus.CallServodResponse{Result: &lsnexus.CallServodResponse_Success_{}}, nil
}
