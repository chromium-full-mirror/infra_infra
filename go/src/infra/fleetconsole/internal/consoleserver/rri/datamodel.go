// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rri

import (
	"cloud.google.com/go/bigquery"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/database/queryutils"
)

const (
	ResourceRequestDevTableName             = "resource_delivery_dev.resource_requests"
	ResourceRequestProdTableName            = "resource_delivery_prod.resource_requests"
	RrIDColumn                              = "rr_id"
	ResourceDetailsColumn                   = "resource_details"
	ResourceRequestActualDeliveryDateColumn = "resource_request_actual_delivery_date"
	ResourceRequestTargetDeliveryDateColumn = "resource_request_target_delivery_date"
	FulfillmentStatusColumn                 = "fulfillment_status"
	ProcurementDateColumn                   = "material_sourcing_target_delivery_date"
	BuildEndDateColumn                      = "build_target_delivery_date"
	QAEndDateColumn                         = "qa_target_delivery_date"
	ConfigEndDateColumn                     = "config_target_delivery_date"
	MaterialSourcingStatusColumn            = "material_sourcing_status"
	BuildStatusColumn                       = "build_status"
	QAStatusColumn                          = "qa_status"
	ConfigStatusColumn                      = "config_status"

	InProgressStatus = "IN_PROGRESS"
	NotStartedStatus = "NOT_STARTED"
	CompleteStatus   = "COMPLETE"
)

func GetResourceRequestsTable(isProd bool) *queryutils.Table {
	tableName := ResourceRequestDevTableName
	if isProd {
		tableName = ResourceRequestProdTableName
	}
	return queryutils.NewTableBuilder(tableName).WithColumns(
		queryutils.NewColumn(RrIDColumn).Build(),
		queryutils.NewColumn(ResourceDetailsColumn).Build(),
		queryutils.NewColumn(ResourceRequestActualDeliveryDateColumn).Build(),
		queryutils.NewColumn(ResourceRequestTargetDeliveryDateColumn).Build(),
		queryutils.NewColumn(FulfillmentStatusColumn).Build(),
		queryutils.NewColumn(ProcurementDateColumn).Build(),
		queryutils.NewColumn(BuildEndDateColumn).Build(),
		queryutils.NewColumn(QAEndDateColumn).Build(),
		queryutils.NewColumn(ConfigEndDateColumn).Build(),
	).Build()
}

func MapFulfillmentStatus(status bigquery.Value) *fleetconsolerpc.ResourceRequest_Status {
	if status == nil {
		return nil
	}

	switch status.(string) {
	case NotStartedStatus:
		status := fleetconsolerpc.ResourceRequest_NOT_STARTED
		return &status
	case InProgressStatus:
		status := fleetconsolerpc.ResourceRequest_IN_PROGRESS
		return &status
	case CompleteStatus:
		status := fleetconsolerpc.ResourceRequest_COMPLETED
		return &status
	default:
		return nil
	}
}
