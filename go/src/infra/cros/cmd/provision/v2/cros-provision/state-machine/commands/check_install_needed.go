// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"go.chromium.org/chromiumos/test/provision/v2/cros-provision/service"
	"context"
	"fmt"
	"log"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"go.chromium.org/chromiumos/config/go/test/api"
)

// CheckInstallNeeded is the commands interface struct.
type CheckInstallNeeded struct {
	ctx context.Context
	cs  *service.CrOSService
}

// NewCheckInstallNeeded is the commands interface to CheckInstallNeeded
func NewCheckInstallNeeded(ctx context.Context, cs *service.CrOSService) *CheckInstallNeeded {
	return &CheckInstallNeeded{
		ctx: ctx,
		cs:  cs,
	}

}

// Execute is the executor for the command. Will check if the current DUT version == target, if so set the skip install flag.
func (c *CheckInstallNeeded) Execute(log *log.Logger) error {
	log.Printf("RUNNING CheckInstallNeeded Execute")
	targetBuilderPath, err := getTargetBuilderPath(c.cs.ImagePath.GetPath())
	if err != nil {
		return err
	}

	// Long term, we might want to consider a flag to be used here, as someone might provision a CQ image locally and be ok with a skip.
	isCq, err := isCQImage(targetBuilderPath)
	if err != nil || isCq {
		// TODO, investigate following TLS logic for only provisioing stateful
		log.Printf("Forcing provision on CQ build.")
		c.cs.UpdateCros = true
	} else if exists, _ := c.cs.Connection.PathExists(c.ctx, "/mnt/stateful_partition/.force_provision"); exists {
		log.Printf("Forcing provision as requested per provision marker.")
		c.cs.UpdateCros = true
	} else if c.cs.MachineMetadata.Version == targetBuilderPath {
		log.Printf("SKIPPING PROVISION AS TARGET VERSION MATCHES CURRENT")
		c.cs.UpdateCros = false
	} else {
		c.cs.UpdateCros = true
	}

	if !c.cs.UpdateCros && c.canClearTPM(c.ctx) {
		oid, err := c.cs.Connection.RunCmd(c.ctx, "hwsec-ownership-id", []string{"id"})
		oid = strings.TrimSpace(oid)
		if err != nil {
			// We need to reinstall the stateful image for the missing utils.
			log.Printf("Missing test utils.")
			c.cs.QuickResetDevice = true
		} else if !isHexDigest(oid) {
			// The device is not in a correct state, we should clear the TPM.
			log.Printf("Need to clear the TPM: %v", oid)
			c.cs.QuickResetDevice = true
		}
	}

	log.Printf("RUNNING CheckInstallNeeded Success")

	return nil
}

// CanClearTPM determines whether the current board can clear TPM
func (c *CheckInstallNeeded) canClearTPM(ctx context.Context) bool {
	return !strings.HasPrefix(c.cs.MachineMetadata.Board, "reven")
}

func isCQImage(imageName string) (bool, error) {
	return regexp.MatchString(`.*-cq/.*`, imageName)
}

func isHexDigest(data string) bool {
	isHex, err := regexp.MatchString(`^[0-9a-fA-F]+$`, data)
	return err != nil || isHex
}

// Revert interface command. None needed as nothing has happened yet.
func (c *CheckInstallNeeded) Revert() error {
	// Thought this method has side effects to the service it does not to the OS,
	// as such Revert here is unneeded
	return nil
}

// GetErrorMessage provides the failed to check install err string.
func (c *CheckInstallNeeded) GetErrorMessage() string {
	return "failed to check if install is needed."
}

// GetStatus provides API Error reason.
func (c *CheckInstallNeeded) GetStatus() api.InstallResponse_Status {
	return api.InstallResponse_STATUS_PRE_PROVISION_SETUP_FAILED
}

func trimFirst(s string) string {
	_, i := utf8.DecodeRuneInString(s)
	return s[i:]
}

func trimPath(s string) string {
	// Sometimes parse has a leading slash, remove it if present.
	if strings.HasPrefix(s, "/") {
		return trimFirst(s)
	}
	return s

}

func getTargetBuilderPath(targetPath string) (string, error) {
	u, uErr := url.Parse(targetPath)
	if uErr != nil {
		return "", fmt.Errorf("failed to parse image path, %s", uErr)
	}
	p := trimPath(u.Path)

	d, version := path.Split(p)
	targetBuilderPath := path.Join(d, version)
	return targetBuilderPath, nil
}
