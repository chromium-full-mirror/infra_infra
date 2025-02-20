// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows

package rpc_services

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"

	pb "go.chromium.org/chromiumos/infra/proto/go/satlabrpcserver"
	"go.chromium.org/luci/common/logging"

	"infra/cros/satlab/common/dut"
	"infra/cros/satlab/satlabrpcserver/models"
)

type Streaming struct {
	server pb.SatlabRpcService_OpenCCDServer
}

func (s *Streaming) Write(data []byte) (n int, err error) {
	res := &pb.OpenCCDReply{
		Message: string(data),
	}

	err = s.server.Send(res)
	if err != nil {
		logging.Errorf(s.server.Context(), "can't send message on streaming, got an error: %s", err.Error())
	}

	return len(data), err
}

// OpenCCD trigger `open CCD`
func (s *SatlabRpcServiceServer) OpenCCD(req *pb.OpenCCDRequest, stream pb.SatlabRpcService_OpenCCDServer) error {
	logging.Infof(stream.Context(), "start open ccd")
	if req.ServoSerial == "" {
		return errors.New("open ccd, servo serial can't be empty")
	}

	ccd := dut.CCDOpenRun{
		ServoSerial: req.ServoSerial,
		UseRmaAuth:  req.RmaAuth,
	}

	writer := Streaming{
		server: stream,
	}

	r, w := io.Pipe()
	reader := bufio.NewReader(r)

	s.ccdSession[req.ServoSerial] = models.CCDSession{
		Writer: w,
	}

	defer func() {
		// Remove the session from dictionary
		delete(s.ccdSession, req.ServoSerial)
	}()

	return ccd.TriggerRun(stream.Context(), s.commandExecutor, &writer, *reader)
}

func (s *SatlabRpcServiceServer) SendMessageToCCDSession(ctx context.Context, req *pb.SendMessageToCCDSessionRequest) (*pb.SendMessageToCCDSessionResponse, error) {
	session, found := s.ccdSession[req.ServoSerial]
	if !found {
		return nil, fmt.Errorf("can't find the session by %s", req.ServoSerial)
	}

	if err := session.Send(req.Message); err != nil {
		return nil, fmt.Errorf("can't send a message to session. Reason: %v", err)
	}

	return &pb.SendMessageToCCDSessionResponse{}, nil
}
