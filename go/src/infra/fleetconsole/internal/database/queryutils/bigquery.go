// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package queryutils

import (
	"cloud.google.com/go/bigquery"
)

type BigQueryQuery struct {
	Statement  string
	Parameters []bigquery.QueryParameter
}

// ToBigQueryQuery generates a parametrized BigQuery query with a reader bound to bindings provided within Select methods
func (b *QueryBuilder) ToBigQueryQuery(client *bigquery.Client) (*bigquery.Query, *ValueBoundBigQueryReader, error) {
	query, err := b.Build(nil)

	if err != nil {
		return nil, nil, err
	}

	q := client.Query(query.Statement)
	q.Parameters = convertQueryParameters(query.Parameters)

	return q, &ValueBoundBigQueryReader{bindings: b.selectClause.bindings}, nil
}

// ToUnboundBigQueryQuery - similar to ToBigQueryQuery but discards any bindings set with Select. Use it whenever bindings are not convenient
func (b *QueryBuilder) ToUnboundBigQueryQuery(client *bigquery.Client) (*bigquery.Query, error) {
	query, err := b.Build(nil)

	if err != nil {
		return nil, err
	}

	q := client.Query(query.Statement)

	bqParams := make([]bigquery.QueryParameter, len(query.Parameters))
	for i, p := range b.parameters.values {
		if b.table.ColumnExists(p.Name) && b.table.columnByExternalName[p.Name].Type == ColumnTypeInt64 {
			bqParams[i] = bigquery.QueryParameter{
				Value: &bigquery.QueryParameterValue{
					Type: bigquery.StandardSQLDataType{
						TypeKind: "INT64",
					},
					Value: p.Value,
				},
			}
		} else {
			bqParams[i] = bigquery.QueryParameter{
				Value: p.Value,
			}
		}
	}

	q.Parameters = bqParams

	return q, nil
}

type ValueBoundBigQueryReader struct {
	bindings map[string]*any
}

func (r *ValueBoundBigQueryReader) ReadNext(it *bigquery.RowIterator) error { //TODO: don't return iterator?
	var row map[string]bigquery.Value

	err := it.Next(&row)
	if err != nil {
		return err
	}

	for alias, binding := range r.bindings {
		(*binding) = row[alias]
	}
	return nil
}

// convertQueryParameters converts a slice of query parameters from the
// queryutils format to the format bigquery.QueryParameter).
//
// Note: This function assumes that all parameters are strings. If other
// types are needed, this function will need to be updated.
func convertQueryParameters(params []any) []bigquery.QueryParameter { //TODO: make private
	bqParams := make([]bigquery.QueryParameter, len(params))
	for i, p := range params {
		bqParams[i] = bigquery.QueryParameter{
			Value: p,
		}
	}
	return bqParams
}
