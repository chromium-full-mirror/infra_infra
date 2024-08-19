// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package api

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Validate validates input requests of GetDeviceConfig.
func (r *GetDeviceConfigRequest) Validate() error {
	if r == nil || r.GetConfigId() == nil {
		return status.Errorf(codes.InvalidArgument, "Request is empty")
	}
	return nil
}
