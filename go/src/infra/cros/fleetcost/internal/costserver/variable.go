// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package costserver

import (
	"context"

	"go.chromium.org/luci/common/errors"

	fleetcostAPI "infra/cros/fleetcost/api/rpc"
)

// CreateVariable creates a global variable.
func (f *FleetCostFrontend) CreateVariable(ctx context.Context, request *fleetcostAPI.CreateVariableRequest) (*fleetcostAPI.CreateVariableResponse, error) {
	return nil, errors.New("not yet implemented")
}
