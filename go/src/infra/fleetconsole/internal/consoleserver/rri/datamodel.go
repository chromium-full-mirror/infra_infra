// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rri

import "go.chromium.org/infra/fleetconsole/internal/database/queryutils"

const (
	ResourceRequestTableName                = "resource_delivery_dev.resource_requests"
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

	InProgressStatus = "INPROGRESS"
	NotStartedStatus = "NOT_STARTED"
	CompleteStatus   = "COMPLETE"
)

func GetResourceRequestsTable() *queryutils.Table {
	return queryutils.NewTableBuilder(ResourceRequestTableName).WithColumns(
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
