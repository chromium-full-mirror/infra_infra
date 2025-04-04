// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
)

func (frontend *FleetConsoleFrontend) CountResourceRequests(ctx context.Context, req *fleetconsolerpc.CountResourceRequestsRequest) (_ *fleetconsolerpc.CountResourceRequestsResponse, err error) {
	return &fleetconsolerpc.CountResourceRequestsResponse{
		Total:            9,
		InProgress:       5,
		Completed:        4,
		MaterialSourcing: 1,
		Build:            2,
		Qa:               1,
		Config:           1,
	}, nil
}
