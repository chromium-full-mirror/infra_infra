// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostoolrunner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/hardcoded/chromeinfra"

	"infra/cros/cmd/common_lib/common"
)

// CtrCipdInfo represents Ctr cipd related info.
type CtrCipdInfo struct {
	Version        string
	IsInitialized  bool
	CtrPath        string
	CtrCipdPackage string
	CtrTempDirLoc  string
}

// Validate validates required ctr cipd info.
func (ctrCipd *CtrCipdInfo) Validate(ctx context.Context) error {
	if ctrCipd.Version == "" {
		logging.Infof(ctx, "cros-tool-runner cipd version is missing")
		return fmt.Errorf("cros-tool-runner cipd version is required!")
	}

	return nil
}

// Initialize initializes Ctr.
func (ctrCipd *CtrCipdInfo) Initialize(ctx context.Context) error {
	// Create temp dir for ctr if necessary
	if ctrCipd.CtrTempDirLoc == "" {
		var err error
		ctrCipd.CtrTempDirLoc, err = common.CreateTempDir(ctx, "ctr")
		if err != nil {
			return errors.Annotate(err, "Error while creating temp dir for ctr: ").Err()
		}
	}
	if ctrCipd.IsInitialized {
		return nil
	}

	// Validation
	if err := ctrCipd.Validate(ctx); err != nil {
		return errors.Annotate(err, "Ctr validation error: ").Err()
	}

	// Ensure CTR
	if err := ctrCipd.ensure(ctx); err != nil {
		return errors.Annotate(err, "Ctr ensure error: ").Err()
	}

	logging.Infof(ctx, "CTR initialization succeeded.")
	ctrCipd.IsInitialized = true
	return nil
}

// ensure ensures the ctr cipd binary is locally available.
func (ctrCipd *CtrCipdInfo) ensure(ctx context.Context) error {
	if ctrCipd.CtrCipdPackage == "" {
		return fmt.Errorf("Cannot ensure ctr with empty package.")
	}

	path, err := os.Executable()
	if err != nil {
		return errors.Annotate(err, "error getting path for current executable: ").Err()
	}

	cipdRoot := filepath.Dir(path)
	cipdHost := chromeinfra.CIPDServiceURL
	authOpts := chromeinfra.DefaultAuthOptions()
	cipdClient, err := common.CreateCIPDClient(ctx, authOpts, cipdHost, cipdRoot)
	if err != nil {
		return errors.Annotate(err, "error creating CIPD client: ").Err()
	}

	_, err = common.EnsureCIPDPackage(
		ctx,
		cipdClient,
		authOpts,
		cipdHost,
		common.CtrCipdPackage,
		ctrCipd.Version,
		"")
	if err != nil {
		return errors.Annotate(err, "CIPD ensure package error: ").Err()
	}

	ctrCipd.CtrPath = filepath.Join(cipdRoot, "cros-tool-runner")
	return nil
}
