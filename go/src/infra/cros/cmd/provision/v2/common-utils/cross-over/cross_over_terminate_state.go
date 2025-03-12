// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// DLC constants and helpers
package cross_over

import (
	"context"
	"log"
	"strings"

	common_utils "go.chromium.org/chromiumos/test/provision/v2/common-utils"

	"go.chromium.org/chromiumos/config/go/test/api"
	"google.golang.org/protobuf/types/known/anypb"
)

// CrossOverTerminateState stops the servod process and downloads the servod logs.
type CrossOverTerminateState struct {
	params *CrossOverParameters
}

func NewCrossOverTerminateState(params *CrossOverParameters) common_utils.ServiceState {
	return &CrossOverTerminateState{
		params: params,
	}
}

func (s CrossOverTerminateState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	log.Println("Executing " + s.Name())
	var successMsg string
	if strings.Contains(s.params.TargetImagePath.GetPath(), "android-build") {
		successMsg = "INFRA: cross-over provision to ANDROID successful"
	} else {
		successMsg = "INFRA: cross-over provision to CROS successful"
	}
	if s.params.StopServo {
		req := &api.StopServodRequest{}
		req.ServodPort = s.params.Dut.GetChromeos().GetServo().GetServodAddress().GetPort()
		req.ServoHostPath = s.params.Dut.GetChromeos().GetServo().GetServodAddress().GetAddress() + servoSSHPort
		if strings.Contains(req.ServoHostPath, satlab) {
			req.ServodDockerContainerName = s.params.Dut.GetChromeos().GetServo().GetServodAddress().GetAddress()
		}
		log.Printf("\nCalling stop servod with request %v.", req)
		_, err := s.params.ServoNexusClient.StopServod(ctx, req)
		if err != nil {
			log.Println("Warning: failed to stop servod", err)
			saveServoLogs(ctx, log, s.params.ServoNexusClient, s.params.Dut)
			return common_utils.WrapStringInAny(successMsg), api.InstallResponse_STATUS_SUCCESS, nil
		}
		log.Printf("\nStopped servod process")
	}
	return common_utils.WrapStringInAny(successMsg), api.InstallResponse_STATUS_SUCCESS, nil
}

func (s CrossOverTerminateState) Next() common_utils.ServiceState {
	return s.params.PostProvisionState
}

func (s CrossOverTerminateState) Name() string {
	return "Cross Over Terminate State"
}
