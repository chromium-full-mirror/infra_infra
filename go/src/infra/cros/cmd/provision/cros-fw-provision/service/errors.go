// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package firmwareservice

import (
	"fmt"

	"go.chromium.org/chromiumos/config/go/test/api"
)

type FirmwareProvisionError struct {
	Status api.InstallResponse_Status
	Err    error
}

func (fe *FirmwareProvisionError) Error() string {
	return fmt.Sprintf("%v: %v", fe.Status.String(), fe.Err)
}

func InvalidRequestErr(err error) *FirmwareProvisionError {
	return &FirmwareProvisionError{
		Status: api.InstallResponse_STATUS_INVALID_REQUEST,
		Err:    err,
	}
}

func UnreachablePreProvisionErr(err error) *FirmwareProvisionError {
	return &FirmwareProvisionError{
		Status: api.InstallResponse_STATUS_DUT_UNREACHABLE_PRE_PROVISION,
		Err:    err,
	}
}

func UpdateFirmwareFailedErr(err error) *FirmwareProvisionError {
	return &FirmwareProvisionError{
		Status: api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED,
		Err:    err,
	}
}

func FirmwareMismatchPostProvisionErr(err error) *FirmwareProvisionError {
	return &FirmwareProvisionError{
		Status: api.InstallResponse_STATUS_FIRMWARE_MISMATCH_POST_FIRMWARE_UPDATE,
		Err:    err,
	}
}

func UnreachablePostProvisionErr(err error) *FirmwareProvisionError {
	return &FirmwareProvisionError{
		Status: api.InstallResponse_STATUS_DUT_UNREACHABLE_POST_FIRMWARE_UPDATE,
		Err:    err,
	}
}
