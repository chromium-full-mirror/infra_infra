// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rpc

import (
	"context"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/appstatus"
	"go.chromium.org/luci/server/auth"
)

const somAccessGroup = "googlers"

// Checks if this call is allowed, returns an error if it is.
func checkAllowedPrelude(ctx context.Context, methodName string, req proto.Message) (context.Context, error) {
	if err := checkAllowed(ctx, somAccessGroup); err != nil {
		return ctx, err
	}
	return ctx, nil
}

// Logs and converts the errors to GRPC type errors.
func gRPCifyAndLogPostlude(ctx context.Context, methodName string, rsp proto.Message, err error) error {
	return appstatus.GRPCifyAndLog(ctx, err)
}

func checkAllowed(ctx context.Context, allowedGroup string) error {
	switch yes, err := auth.IsMember(ctx, allowedGroup); {
	case err != nil:
		return errors.Fmt("failed to check ACL: %w", err)
	case !yes:
		return appstatus.Errorf(codes.PermissionDenied, "not a member of %s", allowedGroup)
	default:
		return nil
	}
}
