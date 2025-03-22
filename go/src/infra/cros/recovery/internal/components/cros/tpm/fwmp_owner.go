// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tpm provides function to work with TPM.
package tpm

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/components/cros"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/internal/retry"
)

type fwmpCleaner struct {
	// FWMP clientCLI path used for flags manipulation.
	clientCLI string
	// Default timeout to run commands
	timeout time.Duration
}

const (
	fwmpNewClient              = "/usr/sbin/device_management_client"
	fwmpOldClient              = "/usr/sbin/cryptohome"
	fwmpClientGetActionGlob    = "%s --action=get_firmware_management_parameters | grep flags="
	fwmpClientremoveActionGlob = "%s --action=remove_firmware_management_parameters"
)

// NewFWMPCleaner creates a new FWMP cleaner.
func NewFWMPCleaner(ctx context.Context, ha components.HostAccess, timeout time.Duration) *fwmpCleaner {
	client := fwmpNewClient
	if _, err := ha.Run(ctx, timeout, "test -f "+client); err != nil {
		client = fwmpOldClient
	}
	log.Debugf(ctx, "Use %q client for FWMP!", client)
	return &fwmpCleaner{
		clientCLI: client,
		timeout:   timeout,
	}
}

// IsClean checks is FWMP is clean based on the flag.
func (c *fwmpCleaner) IsClean(ctx context.Context, ha components.HostAccess) (isClean bool, _ error) {

	// Expected response is `0`, if not then they need to be removed
	readFlag := fmt.Sprintf(fwmpClientGetActionGlob, c.clientCLI)
	flagsRes, err := ha.Run(ctx, c.timeout, readFlag)
	if err != nil {
		return false, errors.Annotate(err, "is clean up FWMP").Err()
	}
	out := strings.TrimSpace(flagsRes.GetStdout())
	parts := strings.Split(out, "=")
	if len(parts) != 2 {
		return false, errors.Reason("is clean up FWMP: fail to read flags, value is missed %q", out).Err()
	}
	hexString := parts[1]
	flags, err := strconv.ParseInt(hexString, 16, 64)
	if err != nil {
		return false, errors.Annotate(err, "is clean up FWMP: fail to parse flags").Err()
	} else if flags == 0 {
		log.Infof(ctx, "FWMP flag is 0, no need to clean up!")
		return true, nil
	}
	log.Infof(ctx, "FWMP falg: %d is bad!", flags)
	return false, nil
}

// Clean cleans FWMP.
func (c *fwmpCleaner) Clean(ctx context.Context, ha components.HostAccess, skipReboot bool, ownerWaitTimeout time.Duration) error {
	log.Debugf(ctx, "Start clear TPM owner and FWMP")
	if skipReboot {
		log.Debugf(ctx, "Clean FWMP reboot skipped!")
	} else {
		if _, err := ha.Run(ctx, c.timeout, "crossystem clear_tpm_owner_request=1"); err != nil {
			return errors.Annotate(err, "clean up FWMP").Err()
		}
		if err := cros.RebootWithCheck(ctx, ha, cros.WaitTimeToDownAtRestart, cros.WaitTimeToBootAfterRestart); err != nil {
			return errors.Annotate(err, "clean up FWMP").Err()
		}
	}
	tpmServices := []string{
		"org.chromium.UserDataAuth",
		"org.chromium.TpmManager",
	}
	for _, service := range tpmServices {
		cmd := "gdbus wait --system --timeout 15 " + service
		if _, err := ha.Run(ctx, 20*time.Second, cmd); err != nil {
			log.Debugf(ctx, "Clean up FWMP: fail to wait for service: %s", err)
		}
	}
	if _, err := ha.Run(ctx, c.timeout, "tpm_manager_client take_ownership"); err != nil {
		log.Debugf(ctx, "Clean up FWMP: fail to wait for service: %s", err)
	}
	if err := retry.WithTimeout(ctx, 2*time.Second, ownerWaitTimeout, func() error {
		s := NewTpmStatus(ctx, ha, c.timeout)
		if owner, err := s.IsOwned(); err != nil {
			return err
		} else if owner {
			log.Debugf(ctx, "Clean up FWMP: owner:true")
			return nil
		} else {
			return errors.Reason("owner:false").Err()
		}
	}, "waiting for owner is true"); err != nil {
		return errors.Annotate(err, "clean up FWMP").Err()
	}
	removeFlag := fmt.Sprintf(fwmpClientremoveActionGlob, c.clientCLI)
	if _, err := ha.Run(ctx, c.timeout, removeFlag); err != nil {
		return errors.Annotate(err, "clean up FWMP").Err()
	}
	log.Infof(ctx, "Successfully clean up FWMP!")
	return nil
}
