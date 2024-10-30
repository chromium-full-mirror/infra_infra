// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows

package rpc_services

import (
	"context"
	"errors"

	pb "go.chromium.org/chromiumos/infra/proto/go/satlabrpcserver"
	"go.chromium.org/luci/common/logging"

	"infra/cros/satlab/common/dut"
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
		logging.Infof(s.server.Context(), "Get an error when streaming. Reason: %s", err.Error())
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
	}

	writer := Streaming{
		server: stream,
	}

	return ccd.TriggerRun(context.Background(), s.commandExecutor, &writer)
}
