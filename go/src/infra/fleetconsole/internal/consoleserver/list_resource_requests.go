// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"

	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/bigqueryclient"
	"go.chromium.org/infra/fleetconsole/internal/database/queryutils"
	"go.chromium.org/infra/fleetconsole/internal/utils"
)

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

	DefaultPageSize = 10
)

var resourceRequestsTable = queryutils.NewTableBuilder(ResourceRequestTableName).WithColumns(
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

	if req.GetPageSize() == 0 {
		req.PageSize = DefaultPageSize
	}

	offset, err := resourceRequestsPageTokenToOffset(req)
	if err != nil {
		logging.Errorf(ctx, "failed to extract page token: %s", err)
		return nil, err
	}

	query, err := buildListResourceRequestsQuery(ctx, req, offset)
	if err != nil {
		logging.Errorf(ctx, "failed to build query: %s", err)
		return nil, err
	}

	q := bqClient.Query(query.Statement)
	q.Parameters = convertQueryParameters(query.Parameters)

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

// convertQueryParameters converts a slice of query parameters from the
// queryutils format to the format bigquery.QueryParameter).
//
// Note: This function assumes that all parameters are strings. If other
// types are needed, this function will need to be updated.
func convertQueryParameters(params []any) []bigquery.QueryParameter {
	bqParams := make([]bigquery.QueryParameter, len(params))
	for i, p := range params {
		bqParams[i] = bigquery.QueryParameter{
			Value: p,
		}
	}
	return bqParams
}

// buildListResourceRequestsQuery uses queryutils to build a query for listing
// resource requests.
func buildListResourceRequestsQuery(ctx context.Context, req *fleetconsolerpc.ListResourceRequestsRequest, offset int) (*queryutils.Query, error) {
	queryBuilder := queryutils.NewQueryBuilder(resourceRequestsTable)
	queryBuilder = queryBuilder.SetSqlLangType(queryutils.BigQueryLangType)
	queryBuilder = queryBuilder.WithSelectAllClause().WithFromClause()

	queryBuilder, err := queryBuilder.WithWhereClause(req.GetFilter())
	if err != nil {
		logging.Errorf(ctx, "failed to build where clause: %s", err)
		return nil, err
	}

	// ORDER BY doesn't support parameters, so we need to make sure the column
	// name is correct.
	queryBuilder, err = queryBuilder.WithOrderByClause(req.GetOrderBy(), "rr_id")
	if err != nil {
		logging.Errorf(ctx, "failed to build order by clause: %s", err)
		return nil, err
	}

	queryBuilder = queryBuilder.WithOffsetPagination(offset, int(req.GetPageSize()))

	return queryBuilder.Build(nil)
}
