// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package statemachine contains the individual states representing the kernel
// provision state machine.

package statemachine

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cft/common/adb"
	commonutils "go.chromium.org/infra/cros/cmd/provision/common-utils"
	"go.chromium.org/infra/cros/cmd/provision/kernel-provision/service"
)

// DisableVerityState represents the tasks needed to disable verity on the DUT.
//
// Verity is that it checks to make sure certain partitions contain expected contents.
// However, when we overwrite kernel partitions, the contents no longer match the
// hashes saved in the device image's vbmeta, which will cause verity to fail,
// and the device will refuse to boot.
//
// There are basically two ways to handle this: either disable verity, or update
// the device's vbmeta so that verity passes. For now, we have taken the approach
// of disabling verity, as it is simpler. However, this is not ideal: verity runs
// on kernel code, so by disabling verity we make it impossible to test verity
// during kernel tests.
//
// TODO: b/425976596 - Kernel provisioning should write a new vbmeta.img to the
// device so we can re-enable verity.
//
// For more info on Verity, see https://source.android.com/docs/security/features/verifiedboot.
type DisableVerityState struct {
	service *service.KernelProvisionService
}

func NewDisableVerityState(service *service.KernelProvisionService) commonutils.ServiceState {
	return &DisableVerityState{service: service}
}

func (s *DisableVerityState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	log.Println("Disabling verity on the DUT...")
	dutAddress := s.service.DUT.GetChromeos().GetSsh().GetAddress()
	if err := adb.RetrySetupAdb(log, dutAddress, 3*time.Minute); err != nil {
		return nil, api.InstallResponse_STATUS_PRE_PROVISION_SETUP_FAILED, fmt.Errorf("connecting to adb (host=%s): %w", dutAddress, err)
	}
	if out, err := adb.AdbCmd([]string{"-s", adb.FmtAddr(dutAddress), "root"}, log, 3, 30); err != nil {
		log.Printf("Failed to run `adb root`: %s", out)
		return nil, api.InstallResponse_STATUS_PRE_PROVISION_SETUP_FAILED, fmt.Errorf("sending adb command `root`: %w", err)
	}
	if out, err := adb.AdbCmd([]string{"-s", adb.FmtAddr(dutAddress), "disable-verity"}, log, 5, 30); err != nil {
		log.Printf("Failed to run `adb disable-verity`: %s", out)
		return nil, api.InstallResponse_STATUS_PRE_PROVISION_SETUP_FAILED, fmt.Errorf("sending adb command `disable-verity`: %w", err)
	}
	log.Println("Verity disabled.")
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s *DisableVerityState) Next() commonutils.ServiceState {
	return NewEnableDevSwitchState(s.service)
}

func (s *DisableVerityState) Name() string {
	return "Kernel-Provision Disable Verity"
}

func (s *DisableVerityState) Retry() bool {
	return false
}
