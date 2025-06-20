// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/grpcutil"
)

func InvalidTokenError(err error) error {
	return grpcutil.InvalidArgumentTag.Apply(errors.Fmt("invalid_page_token: %w", err))
}

func InvalidFilterError(err error) error {
	return grpcutil.InvalidArgumentTag.Apply(errors.Fmt("invalid_filter: %w", err))
}

func InvalidOrderByError(err error) error {
	return grpcutil.InvalidArgumentTag.Apply(errors.Fmt("invalid_order_by: %w", err))
}

func BadRequest(err error, reason string, args ...any) error {
	return grpcutil.InvalidArgumentTag.Apply(errors.WrapIf(err, reason, args...))
}
