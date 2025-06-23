// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package vpd provide ability to read and update VPD values.
package vpd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// Read read vpd value of VPD.
func Read(ctx context.Context, ha components.HostAccess, timeout time.Duration, key string) (string, error) {
	return readValue(ctx, ha, "", timeout, key)
}

// ReadRO read vpd value of VPD from RO_VPD partition.
func ReadRO(ctx context.Context, ha components.HostAccess, timeout time.Duration, key string) (string, error) {
	return readValue(ctx, ha, "RO_VPD", timeout, key)
}

func readValue(ctx context.Context, ha components.HostAccess, partition string, timeout time.Duration, key string) (string, error) {
	errorMessage := "read vpd"
	cmd := "vpd"
	if partition != "" {
		cmd = fmt.Sprintf("%s -i %s", cmd, partition)
		errorMessage = fmt.Sprintf("%s of %q partition", errorMessage, partition)
	}
	cmd = fmt.Sprintf("%s -g %s", cmd, key)
	res, err := ha.Run(ctx, timeout, cmd)
	if err != nil {
		return "", errors.Annotate(err, "%s for %q", errorMessage, key).Err()
	}
	return strings.TrimSpace(res.GetStdout()), nil
}

// Set sets vpd value of VPD by key.
func Set(ctx context.Context, ha components.HostAccess, timeout time.Duration, key, value string) error {
	cmd := fmt.Sprintf("vpd -s %s=%s", key, value)
	_, err := ha.Run(ctx, timeout, cmd)
	return errors.WrapIf(err, "set vpd for %q:%q", key, value)
}

// IsEnrollmentInClean checks that the device's is not in enrollment state.
//
// Verify that the device's enrollment state is clean.
//
// The validation based on the value of "check_enrollment" from VPD.
// Which can be obtained by running:
//
//	vpd -g check_enrollment -i RW_VPD
//
//	If the value is check_enrollment=1 then the device will perform forced
//	re-enrollment check and depending on the response from the server might
//	force the device to enroll again. If the value is check_enrollment=0,
//	then device can be used like a new device.
func IsEnrollmentInClean(ctx context.Context, ha components.HostAccess, timeout time.Duration) (bool, error) {
	res, err := ha.Run(ctx, time.Minute, "vpd -g check_enrollment -i RW_VPD")
	if err != nil {
		// In any case it returns a non zero value, it means we can't verify enrollment state, but we cannot say the device is enrolled
		// Only trigger the enrollment in clean state when we can confirm the device is enrolled.
		log.Errorf(ctx, "Unexpected error occurred during verify enrollment state in VPD cache, skipping verify process.")
		return false, errors.Annotate(err, "is enrollment is clean: fail to run command").Err()
	}
	result := strings.TrimSpace(res.GetStdout())
	log.Debugf(ctx, "Enrollment state: %s", result)
	if result == "0" {
		// We are good!.
		log.Debugf(ctx, "Device is not enrolled.")
		return true, nil
	}
	log.Errorf(ctx, "Device is enrolled!")
	return false, nil
}
