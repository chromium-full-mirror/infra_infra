// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// DLC constants and helpers
package cross_over

import (
	"context"
	"log"
	"strings"
	"time"

	storage_path "go.chromium.org/chromiumos/config/go"
	lab_api "go.chromium.org/chromiumos/config/go/test/lab/api"
	common_utils "go.chromium.org/chromiumos/test/provision/v2/common-utils"

	"go.chromium.org/chromiumos/config/go/test/api"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	servoSSHPort          = ":22"
	satlab                = "satlab"
	bootWaitRetryCount    = 18
	bootWaitRetryInterval = 5 * time.Second
	jobRunning            = "Job is already running"
)

type CrossOverParameters struct {
	Dut                *lab_api.Dut
	TargetImagePath    *storage_path.StoragePath
	DutClient          api.DutServiceClient
	PostProvisionState common_utils.ServiceState
	ServoNexusClient   api.ServodServiceClient
	PrevError          string
	StopServo          bool
	PartnerMetadata    *api.PartnerMetadata
}

// CrossOverInitState starts the servod process.
type CrossOverInitState struct {
	params *CrossOverParameters
}

func NewCrossOverInitState(params *CrossOverParameters) common_utils.ServiceState {
	return &CrossOverInitState{
		params: params,
	}
}

func (s CrossOverInitState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	err := startServod(ctx, log, s.params.ServoNexusClient, s.params.Dut)
	stopServo := true
	if err != nil && strings.Contains(err.Error(), jobRunning) {
		stopServo = false
	} else if err != nil {
		return common_utils.WrapStringInAny("INFRA: unable to start servod process"), api.InstallResponse_STATUS_PRE_PROVISION_SETUP_FAILED, err
	}
	s.params.StopServo = stopServo
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s CrossOverInitState) Next() common_utils.ServiceState {
	return NewSetupUSBState(s.params)
}

func (s CrossOverInitState) Name() string {
	return "Cross Over Init State"
}
