// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/grpcutil"

	fleet "go.chromium.org/infra/appengine/crosskylabadmin/api/fleet/v1"
	"go.chromium.org/infra/appengine/crosskylabadmin/internal/app/frontend/datastore/stableversion/satlab"
	"go.chromium.org/infra/cros/stableversion"
)

// SetSatlabStableVersion replaces a satlab stable version with a new entry.
func (is *ServerImpl) SetSatlabStableVersion(ctx context.Context, req *fleet.SetSatlabStableVersionRequest) (_ *fleet.SetSatlabStableVersionResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()

	if err := validateSetSatlabStableVersion(req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "set satlab stable version: %s", err)
	}

	newEntry, err := satlab.MakeSatlabStableVersionEntry(req, true)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "set satlab stable version: %s", err)
	}

	if pErr := satlab.PutSatlabStableVersionEntry(ctx, newEntry); pErr != nil {
		return nil, status.Errorf(codes.Aborted, "set satlab stable version: %s", pErr)
	}
	return &fleet.SetSatlabStableVersionResponse{}, nil
}

// DeleteSatlabStableVersion deletes a satlab stable version entry.
func (is *ServerImpl) DeleteSatlabStableVersion(ctx context.Context, req *fleet.DeleteSatlabStableVersionRequest) (_ *fleet.DeleteSatlabStableVersionResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()

	var hostname string
	var board string
	var model string
	switch v := req.GetStrategy().(type) {
	case *fleet.DeleteSatlabStableVersionRequest_SatlabHostnameDeletionCriterion:
		hostname = v.SatlabHostnameDeletionCriterion.GetHostname()
	case *fleet.DeleteSatlabStableVersionRequest_SatlabBoardModelDeletionCriterion:
		model = v.SatlabBoardModelDeletionCriterion.GetModel()
		board = v.SatlabBoardModelDeletionCriterion.GetBoard()
	}

	id := satlab.MakeSatlabStableVersionID(hostname, board, model)
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "delete satlab stable version: failed to produce identifier")
	}

	if err := satlab.DeleteSatlabStableVersionEntryByRawID(ctx, id); err != nil {
		return nil, status.Errorf(codes.Aborted, "delete satlab stable version: %s", err)
	}
	return &fleet.DeleteSatlabStableVersionResponse{}, nil
}

// ValidateSetSatlabStableVersion validates a set satlab stable version request.
func validateSetSatlabStableVersion(req *fleet.SetSatlabStableVersionRequest) error {
	if req == nil {
		return errors.New("validate set satlab stable version: request cannot be nil")
	}
	var hostname string
	var board string
	var model string
	switch v := req.GetStrategy().(type) {
	case *fleet.SetSatlabStableVersionRequest_SatlabBoardAndModelStrategy:
		s := v.SatlabBoardAndModelStrategy
		board = strings.TrimSpace(strings.ToLower(s.GetBoard()))
		model = strings.TrimSpace(strings.ToLower(s.GetModel()))
	case *fleet.SetSatlabStableVersionRequest_SatlabHostnameStrategy:
		hostname = strings.TrimSpace(strings.ToLower(v.SatlabHostnameStrategy.GetHostname()))
	}

	if err := shallowValidateKeyFields(hostname, board, model); err != nil {
		return errors.Fmt("validate set satlab stable version: %w", err)
	}

	osVersion := req.GetCrosVersion()
	fwVersion := req.GetFirmwareVersion()
	fwImage := req.GetFirmwareImage()

	if err := shallowValidateValueFields(osVersion, fwVersion, fwImage); err != nil {
		return errors.Fmt("validate set satlab stable version: %w", err)
	}

	return nil
}

// ShallowValidateKeyFields validates the key fields of a satlab stable version request. These are the fields
// that are used to look up a record.
//
// This is a shallow validation because it does not consult any sources of truth to see if the information is valid.
func shallowValidateKeyFields(hostname string, board string, model string) error {
	if hostname != "" {
		if board == "" && model == "" {
			return nil
		}
		return status.Errorf(codes.FailedPrecondition, "shallow validate key fields: cannot use both hostname %q and board/model %q/%q", hostname, board, model)
	}
	if board != "" && model != "" {
		return nil
	}
	return status.Errorf(codes.FailedPrecondition, "shallow validate key fields: expected board %q and model %q to both be non-empty", board, model)
}

// ShallowValidateValueFields validates the value fields, which correspond to fragments of gs:// URLs.
//
// This is a shallow validation because it does not consult any sources of truth to see if the information is valid.
func shallowValidateValueFields(os string, fw string, fwImage string) error {
	if err := stableversion.ValidateCrOSVersion(os); err != nil {
		return errors.Fmt("shallow validate value fields: %w", err)
	}
	if fw == "" && fwImage != "" {
		return status.Error(codes.InvalidArgument, "fw version is not specified")
	}
	if fw != "" {
		if _, err := stableversion.ParseFirmwareVersion(fw); err != nil {
			return errors.Fmt("shallow validate value fields: %w", err)
		}
	}
	if fwImage != "" {
		if _, err := stableversion.ParseFirmwarePath(fwImage); err != nil {
			return errors.Fmt("shallow validate value fields: %w", err)
		}
	}
	return nil
}
