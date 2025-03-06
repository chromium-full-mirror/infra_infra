// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/grpcutil"
)

func InvalidTokenError(err error) error {
	return errors.Annotate(err, "invalid_page_token").Tag(grpcutil.InvalidArgumentTag).Err()
}

func InvalidFilterError(err error) error {
	return errors.Annotate(err, "invalid_filter").Tag(grpcutil.InvalidArgumentTag).Err()
}

func InvalidOrderByError(err error) error {
	return errors.Annotate(err, "invalid_order_by").Tag(grpcutil.InvalidArgumentTag).Err()
}

func BadRequest(err error, reason string, args ...any) error {
	return errors.Annotate(err, reason, args...).Tag(grpcutil.InvalidArgumentTag).Err()
}
