// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"

	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/bigqueryclient"
	"go.chromium.org/infra/fleetconsole/internal/utils"
)

const (
	ResourceRequestTableName                = "fleet_console.resource_requests"
	RrIDColumn                              = "rr_id"
	ResourceDetailsColumn                   = "resource_details"
	ResourceRequestActualDeliveryDateColumn = "resource_request_actual_delivery_date"
	ResourceRequestTargetDeliveryDateColumn = "resource_request_target_delivery_date"
	FulfillmentStatusColumn                 = "fulfillment_status"
	ProcurementDateColumn                   = "material_sourcing_target_end_date"
	BuildEndDateColumn                      = "build_target_end_date"
	QAEndDateColumn                         = "qa_target_end_date"
	ConfigEndDateColumn                     = "config_target_end_date"
)

// currently we are using big query table names as part of a contract with frontend, which is not a good practice
// in future this method should perform an actual mapping from the contract to the big query name
// this method serves us as a guard from SQL Injection, since BigQuery API doesn't allow for parametrized ORDER BY
func MapOrderBy(orderBy string) (string, error) {
	if orderBy == "" {
		return RrIDColumn, nil
	}

	parts := strings.Split(orderBy, " ")

	if !slices.Contains([]string{RrIDColumn, ResourceDetailsColumn, ProcurementDateColumn, BuildEndDateColumn, QAEndDateColumn, ConfigEndDateColumn}, parts[0]) {
		return "", fmt.Errorf("invalid order_by field: %s", orderBy)
	}

	if len(parts) > 1 {
		if len(parts) > 2 {
			return "", fmt.Errorf("invalid order_by field: %s", orderBy)
		}
		direction := strings.ToUpper(parts[1])
		if direction != "ASC" && direction != "DESC" {
			return "", fmt.Errorf("invalid order_by field: %s", orderBy)
		}
		return parts[0] + " " + direction, nil
	}

	return parts[0], nil
}

func BigQueryValueToDate(value bigquery.Value) (date *fleetconsolerpc.DateOnly) {
	if value == nil {
		return nil
	}

	return utils.FromCivilDate(value.(civil.Date))
}

func MapFulfillmentStatus(status bigquery.Value) *fleetconsolerpc.ResourceRequest_Status {
	if status == nil {
		return nil
	}

	switch status.(string) {
	case "NOT_STARTED":
		{
			status := fleetconsolerpc.ResourceRequest_NOT_STARTED
			return &status
		}
	case "INPROGRESS":
		{
			status := fleetconsolerpc.ResourceRequest_IN_PROGRESS
			return &status
		}
	case "COMPLETE":
		{
			status := fleetconsolerpc.ResourceRequest_COMPLETED
			return &status
		}
	default:
		{
			return nil
		}
	}
}

func MapRow(row map[string]bigquery.Value) *fleetconsolerpc.ResourceRequest {
	rrID := row[RrIDColumn].(string)
	actualDeliveryDate := BigQueryValueToDate(row[ResourceRequestActualDeliveryDateColumn])
	targetDeliveryDate := BigQueryValueToDate(row[ResourceRequestTargetDeliveryDateColumn])

	var expectedEta *fleetconsolerpc.DateOnly

	if actualDeliveryDate != nil {
		expectedEta = actualDeliveryDate
	} else if targetDeliveryDate != nil {
		expectedEta = targetDeliveryDate
	}

	return &fleetconsolerpc.ResourceRequest{
		RrId:               row[RrIDColumn].(string),
		Name:               "resourceRequests/" + rrID,
		ResourceDetails:    row[ResourceDetailsColumn].(string),
		ExpectedEta:        expectedEta,
		FulfillmentStatus:  MapFulfillmentStatus(row[FulfillmentStatusColumn]),
		ProcurementEndDate: BigQueryValueToDate(row[ProcurementDateColumn]),
		BuildEndDate:       BigQueryValueToDate(row[BuildEndDateColumn]),
		QaEndDate:          BigQueryValueToDate(row[QAEndDateColumn]),
		ConfigEndDate:      BigQueryValueToDate(row[ConfigEndDateColumn]),
	}
}

// ListResourceRequests lists resource requests.
func (frontend *FleetConsoleFrontend) ListResourceRequests(ctx context.Context, req *fleetconsolerpc.ListResourceRequestsRequest) (*fleetconsolerpc.ListResourceRequestsResponse, error) {
	logging.Infof(ctx, "ListResourceRequests called")

	bqClient, err := bigqueryclient.NewBQClient(ctx, "chrome-fleet-analytics")

	if err != nil {
		logging.Infof(ctx, "Error instantiating a new BigQuery client")
		return nil, err
	}

	offset, err := resourceRequestsPageTokenToOffset(req)

	if err != nil {
		logging.Errorf(ctx, "failed to extract page token: %s", err)
		return nil, err
	}

	orderBy, err := MapOrderBy(req.GetOrderBy())
	if err != nil {
		logging.Errorf(ctx, "failed to extract order by: %s", err)
		return nil, err
	}

	// seems like ORDER BY doesn't support parameters, so we need to make sure the column name is correct ourselves
	q := bqClient.Query("SELECT * FROM " + ResourceRequestTableName + " ORDER BY " + orderBy + " LIMIT @limit OFFSET @offset")
	q.Parameters = []bigquery.QueryParameter{
		{
			Name: "limit", Value: int(req.GetPageSize()) + 1,
		},
		{
			Name: "offset", Value: offset,
		},
	}

	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(5*time.Second))
	defer cancel()
	it, err := q.Read(ctx)
	if err != nil {
		logging.Errorf(ctx, "fetching resource requests from big query failed: %s", err)
		return nil, err
	}

	resourceRequests := []*fleetconsolerpc.ResourceRequest{}

	for {
		var row map[string]bigquery.Value
		err := it.Next(&row)
		if err != nil {
			break
		}

		resourceRequests = append(resourceRequests, MapRow(row))
	}

	nextPageToken := ""

	if len(resourceRequests) > int(req.PageSize) {
		nextPageToken, err = resourceRequestsOffsetToPageToken(offset+int(req.PageSize), req)
		if err != nil {
			logging.Errorf(ctx, "failed to encode next page token: %s", err)
			return nil, err
		}

		resourceRequests = resourceRequests[:req.PageSize]
	}

	return &fleetconsolerpc.ListResourceRequestsResponse{
		ResourceRequests: resourceRequests,
		NextPageToken:    nextPageToken,
	}, nil
}

func resourceRequestsPageTokenToOffset(req *fleetconsolerpc.ListResourceRequestsRequest) (int, error) {
	return utils.PageTokenToOffset(req.GetPageToken(), []string{
		req.GetFilter(),
		req.GetOrderBy(),
	})
}

func resourceRequestsOffsetToPageToken(offset int, req *fleetconsolerpc.ListResourceRequestsRequest) (string, error) {
	return utils.OffsetToPageToken(offset, []string{
		req.GetFilter(),
		req.GetOrderBy(),
	})
}
