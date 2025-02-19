// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"

	"infra/fleetconsole/api/fleetconsolerpc"
	"infra/fleetconsole/internal/utils"
)

// ListResourceRequests lists resource requests.
func (frontend *FleetConsoleFrontend) ListResourceRequests(ctx context.Context, req *fleetconsolerpc.ListResourceRequestsRequest) (*fleetconsolerpc.ListResourceRequestsResponse, error) {
	return &fleetconsolerpc.ListResourceRequestsResponse{
		ResourceRequests: []*fleetconsolerpc.ResourceRequest{
			{
				RrId:               "RR-123",
				Name:               "resourceRequests/RR-123",
				ResourceDetails:    "MacBook Pro 8GB",
				ProcurementEndDate: utils.NewDateOnly(2025, 4, 12),
				BuildEndDate:       utils.NewDateOnly(2025, 4, 24),
				QaEndDate:          utils.NewDateOnly(2025, 5, 2),
				ConfigEndDate:      utils.NewDateOnly(2025, 5, 16),
			},
		},
	}, nil
}
