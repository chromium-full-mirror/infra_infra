// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"

	"go.chromium.org/luci/common/data/aip132"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/bigqueryclient"
	"go.chromium.org/infra/fleetconsole/internal/consoleserver/rri"
	"go.chromium.org/infra/fleetconsole/internal/database/queryutils"
	"go.chromium.org/infra/fleetconsole/internal/utils"
)

const (
	DefaultPageSize      = 10
	ExpectedEtaColumnKey = "expected_eta" // computed column key, doesn't exist in the db
)

func BigQueryValueToDate(value bigquery.Value) (date *fleetconsolerpc.DateOnly) {
	if value == nil {
		return nil
	}

	return utils.FromCivilDate(value.(civil.Date))
}

func MapRow(row map[string]bigquery.Value) *fleetconsolerpc.ResourceRequest {
	rrID := row[rri.RrIDColumn].(string)
	actualDeliveryDate := BigQueryValueToDate(row[rri.ResourceRequestActualDeliveryDateColumn])
	targetDeliveryDate := BigQueryValueToDate(row[rri.ResourceRequestTargetDeliveryDateColumn])

	var expectedEta *fleetconsolerpc.DateOnly

	if actualDeliveryDate != nil {
		expectedEta = actualDeliveryDate
	} else if targetDeliveryDate != nil {
		expectedEta = targetDeliveryDate
	}

	return &fleetconsolerpc.ResourceRequest{
		RrId:               row[rri.RrIDColumn].(string),
		Name:               "resourceRequests/" + rrID,
		ResourceDetails:    row[rri.ResourceDetailsColumn].(string),
		ExpectedEta:        expectedEta,
		FulfillmentStatus:  rri.MapFulfillmentStatus(row[rri.FulfillmentStatusColumn]),
		ProcurementEndDate: BigQueryValueToDate(row[rri.ProcurementDateColumn]),
		BuildEndDate:       BigQueryValueToDate(row[rri.BuildEndDateColumn]),
		QaEndDate:          BigQueryValueToDate(row[rri.QAEndDateColumn]),
		ConfigEndDate:      BigQueryValueToDate(row[rri.ConfigEndDateColumn]),
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

	query, err := buildListResourceRequestsQuery(ctx, bqClient, req, offset)
	if err != nil {
		logging.Errorf(ctx, "failed to build query: %s", err)
		return nil, err
	}

	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(5*time.Second))
	defer cancel()
	it, err := query.Read(ctx)
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

// buildListResourceRequestsQuery uses queryutils to build a query for listing
// resource requests.
func buildListResourceRequestsQuery(ctx context.Context, bqClient *bigquery.Client, req *fleetconsolerpc.ListResourceRequestsRequest, offset int) (*bigquery.Query, error) {
	queryBuilder := queryutils.NewQueryBuilder(rri.GetResourceRequestsTable())
	queryBuilder = queryBuilder.SetSqlLangType(queryutils.BigQueryLangType)
	queryBuilder = queryBuilder.WithSelectAllClause()

	queryBuilder, err := queryBuilder.WithWhereClause(req.GetFilter())
	if err != nil {
		logging.Errorf(ctx, "failed to build where clause: %s", err)
		return nil, err
	}

	orderByString, err := mapOrderBy(req.GetOrderBy())
	if err != nil {
		logging.Errorf(ctx, "failed to build order by clause: %s", err)
		return nil, err
	}

	// ORDER BY doesn't support parameters, so we need to make sure the column
	// name is correct.
	queryBuilder = queryBuilder.WithCustomOrderByClause(orderByString)

	queryBuilder = queryBuilder.WithOffsetPagination(offset, int(req.GetPageSize())+1)

	return queryBuilder.ToUnboundBigQueryQuery(bqClient)
}

func mapOrderBy(orderByAip string) (string, error) {
	orderBy, err := aip132.ParseOrderBy(orderByAip)
	if err != nil {
		return "", err
	}

	orderByString := ""

	if len(orderBy) == 0 {
		return rri.RrIDColumn, nil
	}

	if len(orderBy) > 1 {
		return "", fmt.Errorf("only one order by is supported")
	}

	if orderBy[0].FieldPath.String() == ExpectedEtaColumnKey {
		orderByString = "CASE WHEN " + rri.ResourceRequestActualDeliveryDateColumn + " IS NULL THEN " + rri.ResourceRequestTargetDeliveryDateColumn + " ELSE " + rri.ResourceRequestActualDeliveryDateColumn + " END"
	} else {
		orderByString = orderBy[0].FieldPath.String()
	}

	if orderBy[0].Descending {
		orderByString += " DESC"
	}

	return orderByString, nil
}
