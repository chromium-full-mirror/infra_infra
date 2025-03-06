// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicesdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/database/queryutils"
	"go.chromium.org/infra/fleetconsole/internal/utils"
)

// GetLabels gets all the dynamic labels and their possible values
func GetLabels(ctx context.Context, dbConn *sql.DB, realms []string) (map[string]*fleetconsolerpc.LabelValues, error) {
	query, err := buildGetColumnQuery(ctx, false, LabelsColumn, realms)
	if err != nil {
		return nil, err
	}
	rows, err := dbConn.QueryContext(ctx, query.Statement, query.Parameters...)
	if err != nil {
		return nil, fmt.Errorf("failed to read labels: %w", err)
	}
	defer rows.Close()

	results := map[string]utils.Set[string]{}
	for rows.Next() {
		var (
			values []byte
		)
		err := rows.Scan(
			&values,
		)

		if err != nil {
			return nil, fmt.Errorf("GetLabels: %w", err)
		}

		labels := make(map[string]*LabelValuesDAO)
		err = json.Unmarshal(values, &labels)
		if err != nil {
			return nil, fmt.Errorf("GetLabels: %w", err)
		}

		for k, values := range labels {
			if _, ok := results[k]; !ok {
				results[k] = utils.Set[string]{}
			}

			for _, v := range values.Values {
				if v != "" {
					results[k].Add(v)
				}
			}

		}
	}

	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("GetLabels: %w", err)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetLabels: %w", err)
	}

	return utils.ConvertMapValues(results, func(in utils.Set[string]) *fleetconsolerpc.LabelValues {
		out := make([]string, 0, len(in))
		for v := range in {
			out = append(out, v)
		}

		// Ordering to have deterministic results
		slices.Sort(out)
		return &fleetconsolerpc.LabelValues{Values: out}
	}), nil
}

// GetBaseDimensions gets all the base dimensions and their possible values
func GetBaseDimensions(ctx context.Context, dbConn *sql.DB, realms []string) (map[string]*fleetconsolerpc.LabelValues, error) {
	results := map[string]*LabelValuesDAO{}

	for _, column := range DevicesTable.Columns {
		if column.Type == queryutils.ColumnTypeJSONB {
			continue
		}

		query, err := buildGetColumnQuery(ctx, true, column, realms)
		if err != nil {
			return nil, err
		}

		rows, err := dbConn.QueryContext(ctx, query.Statement, query.Parameters...)
		if err != nil {
			return nil, fmt.Errorf("failed to read base dimensions: %w", err)
		}
		defer rows.Close()

		results[column.ExternalName] = &LabelValuesDAO{}
		values := []string{}
		for rows.Next() {
			var (
				value string
			)
			err := rows.Scan(
				&value,
			)

			if err != nil {
				return nil, fmt.Errorf("GetBaseDimensions: %w", err)
			}

			if value != "" && !(column == PortColumn && value == "0") {
				values = append(values, value)
			}
		}

		if err := rows.Close(); err != nil {
			return nil, fmt.Errorf("GetBaseDimensions: %w", err)
		}

		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("GetBaseDimensions: %w", err)
		}

		if len(values) > 0 {
			results[column.ExternalName].Values = values
		}
	}

	return utils.ConvertMapValues(results, func(in *LabelValuesDAO) *fleetconsolerpc.LabelValues {
		out := make([]string, 0, len(in.Values))
		out = append(out, in.Values...)

		// Ordering to have deterministic results
		slices.Sort(out)
		return &fleetconsolerpc.LabelValues{Values: out}
	}), nil
}
