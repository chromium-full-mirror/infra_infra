// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"go.chromium.org/chromiumos/test/provision/v2/cros-provision/service"
	"context"
	"errors"
	"log"

	"go.chromium.org/chromiumos/config/go/test/api"
)

// CheckVersionMatches is the commands interface struct.
type CheckVersionMatches struct {
	ctx context.Context
	cs  *service.CrOSService
}

// NewCheckVersionMatches is the commands interface to CheckVersionMatches
func NewCheckVersionMatches(ctx context.Context, cs *service.CrOSService) *CheckVersionMatches {
	return &CheckVersionMatches{
		ctx: ctx,
		cs:  cs,
	}

}

// Execute is the executor for the command. Will check if the current DUT version == target, if so set the skip install flag.
func (c *CheckVersionMatches) Execute(log *log.Logger) error {
	log.Printf("RUNNING CheckVersionMatches Execute")
	targetBuilderPath, err := getTargetBuilderPath(c.cs.ImagePath.GetPath())
	if err != nil {
		return err
	}
	if c.cs.MachineMetadata.Version == targetBuilderPath {
		log.Printf("POST INSTALL VERSION MATCHES")
		c.cs.UpdateCros = false
	} else {
		log.Printf("POST INSTALL DOES NOT MATCH.")
		return errors.New("post install version does not match target image")
	}
	log.Printf("RUNNING CheckVersionMatches Success")

	return nil
}

// Revert interface command. None needed as nothing has happened yet.
func (c *CheckVersionMatches) Revert() error {
	// Thought this method has side effects to the service it does not to the OS,
	// as such Revert here is unneeded
	return nil
}

// GetErrorMessage provides the failed to check install err string.
func (c *CheckVersionMatches) GetErrorMessage() string {
	return "image could not be validated as correct post install"
}

// GetStatus provides API Error reason.
func (c *CheckVersionMatches) GetStatus() api.InstallResponse_Status {
	return api.InstallResponse_STATUS_IMAGE_MISMATCH_POST_PROVISION_UPDATE
}
