// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common_utils

import (
	"os"

	"github.com/golang/protobuf/jsonpb"
	"github.com/pkg/errors"
	"go.chromium.org/chromiumos/config/go/test/api"
)

// ParseCrosProvisionRequest parses CrosProvisionRequest input request data from
// the input file.
func ParseCrosProvisionRequest(path string) (*api.CrosProvisionRequest, error) {
	in := &api.CrosProvisionRequest{}
	r, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "open file %q", path)
	}

	umrsh := jsonpb.Unmarshaler{}
	umrsh.AllowUnknownFields = true
	err = umrsh.Unmarshal(r, in)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid json in %q", path)
	}

	return in, nil
}

// ParseAndroidProvisionRequest parses AndroidProvisionRequest input request data from
// the input file.
func ParseAndroidProvisionRequest(path string) (*api.AndroidProvisionRequest, error) {
	in := &api.AndroidProvisionRequest{}
	r, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "open file %q", path)
	}

	umrsh := jsonpb.Unmarshaler{}
	umrsh.AllowUnknownFields = true
	err = umrsh.Unmarshal(r, in)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid json in %q", path)
	}

	return in, nil
}

// ParseInstallRequest parses InstallRequest input request data from
// the input file.
func ParseInstallRequest(path string) (*api.InstallRequest, error) {
	in := &api.InstallRequest{}
	r, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "open file %q", path)
	}

	umrsh := jsonpb.Unmarshaler{}
	umrsh.AllowUnknownFields = false
	err = umrsh.Unmarshal(r, in)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid json in %q", path)
	}

	return in, nil
}

// ParseProvisionStartupRequest parses ProvisionStartupRequest input request data from
// the input file.
func ParseProvisionStartupRequest(path string) (*api.ProvisionStartupRequest, error) {
	in := &api.ProvisionStartupRequest{}
	r, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "open file %q", path)
	}

	umrsh := jsonpb.Unmarshaler{}
	umrsh.AllowUnknownFields = false
	err = umrsh.Unmarshal(r, in)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid json in %q", path)
	}

	return in, nil
}
