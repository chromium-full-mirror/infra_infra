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
	ProcurementTargetStartDateColumn        = "material_sourcing_target_start_date"
	ProcurementActualStartDateColumn        = "material_sourcing_actual_start_date"
	ProcurementTargetDeliveryDateColumn     = "material_sourcing_target_delivery_date"
	ProcurementActualDeliveryDateColumn     = "material_sourcing_actual_delivery_date"
	BuildTargetStartDateColumn              = "build_target_start_date"
	BuildActualStartDateColumn              = "build_actual_start_date"
	BuildTargetDeliveryDateColumn           = "build_target_delivery_date"
	BuildActualDeliveryDateColumn           = "build_actual_delivery_date"
	QATargetStartDateColumn                 = "qa_target_start_date"
	QAActualStartDateColumn                 = "qa_actual_start_date"
	QATargetDeliveryDateColumn              = "qa_target_delivery_date"
	QAActualDeliveryDateColumn              = "qa_actual_delivery_date"
	ConfigTargetStartDateColumn             = "config_target_start_date"
	ConfigActualStartDateColumn             = "config_actual_start_date"
	ConfigTargetDeliveryDateColumn          = "config_target_delivery_date"
	ConfigActualDeliveryDateColumn          = "config_actual_delivery_date"
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
		queryutils.NewColumn(ProcurementTargetStartDateColumn).Build(),
		queryutils.NewColumn(ProcurementActualStartDateColumn).Build(),
		queryutils.NewColumn(ProcurementTargetDeliveryDateColumn).Build(),
		queryutils.NewColumn(ProcurementActualDeliveryDateColumn).Build(),
		queryutils.NewColumn(BuildTargetStartDateColumn).Build(),
		queryutils.NewColumn(BuildActualStartDateColumn).Build(),
		queryutils.NewColumn(BuildTargetDeliveryDateColumn).Build(),
		queryutils.NewColumn(BuildActualDeliveryDateColumn).Build(),
		queryutils.NewColumn(QATargetStartDateColumn).Build(),
		queryutils.NewColumn(QAActualStartDateColumn).Build(),
		queryutils.NewColumn(QATargetDeliveryDateColumn).Build(),
		queryutils.NewColumn(QAActualDeliveryDateColumn).Build(),
		queryutils.NewColumn(ConfigTargetStartDateColumn).Build(),
		queryutils.NewColumn(ConfigActualStartDateColumn).Build(),
		queryutils.NewColumn(ConfigTargetDeliveryDateColumn).Build(),
		queryutils.NewColumn(ConfigActualDeliveryDateColumn).Build(),
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
