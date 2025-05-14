// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// DLC constants and helpers
package cross_over

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/anypb"

	storage_path "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	commonutils "go.chromium.org/infra/cros/cmd/provision/common-utils"
	"go.chromium.org/infra/cros/cmd/provision/foil-provision/constants"
)

const (
	servoSSHPort          = ":22"
	satlab                = "satlab"
	bootWaitRetryCount    = 18
	bootWaitRetryInterval = 5 * time.Second
	jobRunning            = "Job is already running"
)

type CrossOverParameters struct {
	Dut                *labapi.Dut
	TargetImagePath    *storage_path.StoragePath
	DutClient          api.DutServiceClient
	PostProvisionState commonutils.ServiceState
	ServoNexusClient   api.ServodServiceClient
	PrevError          string
	StopServo          bool
	PartnerMetadata    *api.PartnerMetadata
	KernelPrebuilts    *api.KernelPrebuilts
}

// CrossOverInitState starts the servod process.
type CrossOverInitState struct {
	params *CrossOverParameters
}

func NewCrossOverInitState(params *CrossOverParameters) commonutils.ServiceState {
	return &CrossOverInitState{
		params: params,
	}
}

func (s CrossOverInitState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	// Generate the ADB address for the keep alive command
	address := fmt.Sprintf("%s:%v", s.params.Dut.GetChromeos().GetSsh().GetAddress(), "5555")

	// Create the CLI command to be called by bash.
	//
	// NOTE: We need this to be called by `bash -c` because the os/exec library
	// does not handle redirects which we want here. We are not setting
	// redirects using the exec.Command struct because we want this task to be
	// run in background and persist after this provision container/binary
	// completes.
	bashCMD := fmt.Sprintf("adb-logcat -adb-address %s -log-dir %s >%s 2>&1", address, constants.LogFileDir, fmt.Sprintf("%s%s", constants.LogFileDir, "logcat.txt"))
	log.Printf("bashCMD: %s\n", bashCMD)

	// Launch the adb-logcat CIPD binary
	//
	// CLEAN(b/408454320): Remove once adb-logcat is containerized.
	cmd := exec.Command("bash", "-c", bashCMD)
	err := cmd.Start()
	if err != nil {
		log.Printf("Error launching adb-logcat sidecar, ignoring error: %s", err.Error())
	} else {
		log.Printf("Launched sidecar")
	}

	err = startServod(ctx, log, s.params.ServoNexusClient, s.params.Dut)
	stopServo := true
	if err != nil && strings.Contains(err.Error(), jobRunning) {
		stopServo = false
	} else if err != nil {
		return commonutils.WrapStringInAny("INFRA: unable to start servod process"), api.InstallResponse_STATUS_PRE_PROVISION_SETUP_FAILED, err
	}
	s.params.StopServo = stopServo
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s CrossOverInitState) Next() commonutils.ServiceState {
	return NewSetupUSBState(s.params)
}

func (s CrossOverInitState) Name() string {
	return "Cross Over Init State"
}

func (s CrossOverInitState) Retry() bool {
	return false
}
