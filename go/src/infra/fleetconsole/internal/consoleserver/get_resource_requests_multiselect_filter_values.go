// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"sort"
	"time"

	"cloud.google.com/go/bigquery"

	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/bigqueryclient"
	"go.chromium.org/infra/fleetconsole/internal/consoleserver/rri"
	"go.chromium.org/infra/fleetconsole/internal/database/queryutils"
)

// GetResourceRequestsMultiselectFilterValues lists resource requests.
func (frontend *FleetConsoleFrontend) GetResourceRequestsMultiselectFilterValues(ctx context.Context, req *fleetconsolerpc.GetResourceRequestsMultiselectFilterValuesRequest) (*fleetconsolerpc.GetResourceRequestsMultiselectFilterValuesResponse, error) {
	logging.Infof(ctx, "GetResourceRequestsMultiselectFilterValues called")

	bqClient, err := bigqueryclient.NewBQClient(ctx, "chrome-fleet-analytics")
	if err != nil {
		logging.Infof(ctx, "Error instantiating a new BigQuery client")
		return nil, err
	}

	var (
		rrId                     any
		resourceDetails          any
		materialSourcingStatus   any
		buildStatus              any
		qaStatus                 any
		configStatus             any
		customer                 any
		resourceName             any
		acceptedQuantity         any
		criticality              any
		requestApproval          any
		resourcePm               any
		fulfillmentChannel       any
		executionStatus          any
		resourceGroups           any
		resourceRequestBugStatus any
	)

	queryBuilder := queryutils.NewQueryBuilder(rri.GetResourceRequestsTable(frontend.IsProdEnvironment()))
	queryBuilder = queryBuilder.SetSqlLangType(queryutils.BigQueryLangType)

	// TODO: these will select combinations instead of all possible values
	queryBuilder = queryBuilder.WithCustomSelectDistinctClause(
		queryutils.Select(rri.RrIDColumn, &rrId),
		queryutils.Select(rri.ResourceDetailsColumn, &resourceDetails),
		queryutils.Select(rri.MaterialSourcingStatusColumn, &materialSourcingStatus),
		queryutils.Select(rri.BuildStatusColumn, &buildStatus),
		queryutils.Select(rri.QAStatusColumn, &qaStatus),
		queryutils.Select(rri.ConfigStatusColumn, &configStatus),
		queryutils.Select(rri.CustomerColumn, &customer),
		queryutils.Select(rri.ResourceNameColumn, &resourceName),
		queryutils.Select(rri.AcceptedQuantityColumn, &acceptedQuantity),
		queryutils.Select(rri.CriticalityColumn, &criticality),
		queryutils.Select(rri.RequestApprovalColumn, &requestApproval),
		queryutils.Select(rri.ResourcePmColumn, &resourcePm),
		queryutils.Select(rri.FulfillmentChannelColumn, &fulfillmentChannel),
		queryutils.Select(rri.ExecutionStatusColumn, &executionStatus),
		queryutils.Select(rri.ResourceGroupsColumn, &resourceGroups),
		queryutils.Select(rri.ResourceRequestBugStatusColumn, &resourceRequestBugStatus),
	)

	query, reader, err := queryBuilder.ToBigQueryQuery(bqClient)
	if err != nil {
		logging.Errorf(ctx, "failed to build query: %s", err)
		return nil, err
	}

	// Increased timeout as querying distinct values can take longer.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	it, err := query.Read(ctx)
	if err != nil {
		logging.Errorf(ctx, "fetching resource requests from big query failed: %s", err)
		return nil, err
	}

	// Use sets to collect unique values for each field.
	rrIdSet := make(map[string]struct{})
	resourceDetailsSet := make(map[string]struct{})
	materialSourcingStatusSet := make(map[string]struct{})
	buildStatusSet := make(map[string]struct{})
	qaStatusSet := make(map[string]struct{})
	configStatusSet := make(map[string]struct{})
	customerSet := make(map[string]struct{})
	resourceNameSet := make(map[string]struct{})
	acceptedQuantitySet := make(map[int32]struct{})
	criticalitySet := make(map[string]struct{})
	requestApprovalSet := make(map[string]struct{})
	resourcePmSet := make(map[string]struct{})
	fulfillmentChannelSet := make(map[string]struct{})
	executionStatusSet := make(map[string]struct{})
	resourceGroupsSet := make(map[string]struct{})
	resourceRequestBugStatusSet := make(map[string]struct{})

	addToStrSet := func(set map[string]struct{}, bqVal any) {
		if bqVal == nil {
			return
		}
		if strVal, ok := bqVal.(string); ok && strVal != "" {
			set[strVal] = struct{}{}
		}
	}

	for {
		err = reader.ReadNext(it)
		if err != nil {
			break
		}
		addToStrSet(rrIdSet, rrId)
		addToStrSet(resourceDetailsSet, resourceDetails)
		addToStrSet(materialSourcingStatusSet, materialSourcingStatus)
		addToStrSet(buildStatusSet, buildStatus)
		addToStrSet(qaStatusSet, qaStatus)
		addToStrSet(configStatusSet, configStatus)
		addToStrSet(customerSet, customer)
		addToStrSet(resourceNameSet, resourceName)
		addToStrSet(criticalitySet, criticality)
		addToStrSet(requestApprovalSet, requestApproval)
		addToStrSet(resourcePmSet, resourcePm)
		addToStrSet(fulfillmentChannelSet, fulfillmentChannel)
		addToStrSet(executionStatusSet, executionStatus)
		addToStrSet(resourceRequestBugStatusSet, resourceRequestBugStatus)

		if acceptedQuantity != nil {
			if aqVal, ok := acceptedQuantity.(int64); ok {
				acceptedQuantitySet[int32(aqVal)] = struct{}{}
			}
		}

		if resourceGroups != nil {
			if rgVals, ok := resourceGroups.([]bigquery.Value); ok {
				for _, v := range rgVals {
					if v == nil {
						continue
					}
					if strV, ok := v.(string); ok && strV != "" {
						resourceGroupsSet[strV] = struct{}{}
					}
				}
			}
		}
	}

	mapKeysToSortedStrSlice := func(set map[string]struct{}) []string {
		list := make([]string, 0, len(set))
		for k := range set {
			list = append(list, k)
		}
		sort.Strings(list)
		return list
	}
	mapKeysToSortedInt32Slice := func(set map[int32]struct{}) []int32 {
		list := make([]int32, 0, len(set))
		for k := range set {
			list = append(list, k)
		}
		sort.Slice(list, func(i, j int) bool { return list[i] < list[j] })
		return list
	}

	rrIdList := mapKeysToSortedStrSlice(rrIdSet)
	resourceDetailsList := mapKeysToSortedStrSlice(resourceDetailsSet)

	return &fleetconsolerpc.GetResourceRequestsMultiselectFilterValuesResponse{
		RrIds:                    rrIdList,
		ResourceDetails:          resourceDetailsList,
		MaterialSourcingStatus:   mapKeysToSortedStrSlice(materialSourcingStatusSet),
		BuildStatus:              mapKeysToSortedStrSlice(buildStatusSet),
		QaStatus:                 mapKeysToSortedStrSlice(qaStatusSet),
		ConfigStatus:             mapKeysToSortedStrSlice(configStatusSet),
		Customer:                 mapKeysToSortedStrSlice(customerSet),
		ResourceName:             mapKeysToSortedStrSlice(resourceNameSet),
		AcceptedQuantity:         mapKeysToSortedInt32Slice(acceptedQuantitySet),
		Criticality:              mapKeysToSortedStrSlice(criticalitySet),
		RequestApproval:          mapKeysToSortedStrSlice(requestApprovalSet),
		ResourcePm:               mapKeysToSortedStrSlice(resourcePmSet),
		FulfillmentChannel:       mapKeysToSortedStrSlice(fulfillmentChannelSet),
		ExecutionStatus:          mapKeysToSortedStrSlice(executionStatusSet),
		ResourceGroups:           mapKeysToSortedStrSlice(resourceGroupsSet),
		ResourceRequestBugStatus: mapKeysToSortedStrSlice(resourceRequestBugStatusSet),
	}, nil
}
