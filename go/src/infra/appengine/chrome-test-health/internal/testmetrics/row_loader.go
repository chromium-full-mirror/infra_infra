// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testmetrics

import (
	"strings"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/appengine/chrome-test-health/api"
)

// rowLoader provides a way of marshalling a BigQuery row.
// The intended usage is as follows (where it is the bigquery iterator):
//
// var loader rowLoader
// err := it.Next(&loader) // 'it' is a bigquery iterator.
// ... // handle err
// someField := loader.NullString("some_field")
// otherField := loader.Int64("other_field")
//
// if err := loader.Error(); err != nil {
// ... // handle err
// }
type rowLoader struct {
	vals   []bigquery.Value
	schema bigquery.Schema
	err    error
}

func (r *rowLoader) Load(v []bigquery.Value, s bigquery.Schema) error {
	r.vals = v
	r.schema = s
	r.err = nil
	return nil
}

func (r *rowLoader) fieldIndex(fieldName string) (index int, ok bool) {
	for i, field := range r.schema {
		if field.Name == fieldName {
			return i, true
		}
	}
	return -1, false
}

func (r *rowLoader) valueWithType(fieldName string, expectedType bigquery.FieldType, repeated bool) (bigquery.Value, error) {
	i, ok := r.fieldIndex(fieldName)
	if !ok {
		return nil, errors.Fmt("field %s is not defined", fieldName)
	}
	fieldType := r.schema[i]
	if fieldType.Type != expectedType {
		return nil, errors.Fmt("field %s has type %s, expected type %s", fieldName, fieldType.Type, expectedType)
	}
	if fieldType.Repeated != repeated {
		return nil, errors.Fmt("field %s repeated=%v, expected repeated=%v", fieldName, fieldType.Repeated, repeated)
	}
	return r.vals[i], nil
}

// NullDate returns the value of a field of type bigquery.NullDate.
// If the field does not exist or is of an incorrect type, the default
// bigquery.NullDate is returned and an error will be available from
// rowLoader.Error().
func (r *rowLoader) NullDate(fieldName string) bigquery.NullDate {
	repeated := false
	val, err := r.valueWithType(fieldName, bigquery.DateFieldType, repeated)
	if err != nil {
		r.reportError(err)
		return bigquery.NullDate{}
	}
	if val == nil {
		return bigquery.NullDate{}
	}
	return bigquery.NullDate{Valid: true, Date: val.(civil.Date)}
}

// Date returns the value of a field of type civil.Date.
// If the field does not exist or is of an incorrect type, the default
// civil.Date is returned and an error will be available from
// rowLoader.Error().
func (r *rowLoader) Date(fieldName string) civil.Date {
	val := r.NullDate(fieldName)
	if !val.Valid {
		r.reportError(errors.Fmt("field %s value is NULL, expected non-null date", fieldName))
		return civil.Date{}
	}
	return val.Date
}

// NullBool returns the value of a field of type bigquery.NullBool.
// If the field does not exist or is of an incorrect type, the default
// bigquery.NullBool is returned and an error will be available from
// rowLoader.Error().
func (r *rowLoader) NullBool(fieldName string) bigquery.NullBool {
	repeated := false
	val, err := r.valueWithType(fieldName, bigquery.BooleanFieldType, repeated)
	if err != nil {
		r.reportError(err)
		return bigquery.NullBool{}
	}
	if val == nil {
		return bigquery.NullBool{}
	}
	return bigquery.NullBool{Valid: true, Bool: val.(bool)}
}

// Bool returns the value of a field of type bool.
// If the field does not exist or is of an incorrect type, false
// is returned and an error will be available from rowLoader.Error().
func (r *rowLoader) Bool(fieldName string) bool {
	val := r.NullBool(fieldName)
	if !val.Valid {
		r.reportError(errors.Fmt("field %s value is NULL, expected non-null string", fieldName))
		return false
	}
	return val.Bool
}

// NullString returns the value of a field of type bigquery.NullString.
// If the field does not exist or is of an incorrect type, the default
// bigquery.NullString is returned and an error will be available from
// rowLoader.Error().
func (r *rowLoader) NullString(fieldName string) bigquery.NullString {
	repeated := false
	val, err := r.valueWithType(fieldName, bigquery.StringFieldType, repeated)
	if err != nil {
		r.reportError(err)
		return bigquery.NullString{}
	}
	if val == nil {
		return bigquery.NullString{}
	}
	return bigquery.NullString{Valid: true, StringVal: val.(string)}
}

// String returns the value of a field of type string.
// If the field does not exist or is of an incorrect type, an empty
// string is returned and an error will be available from rowLoader.Error().
func (r *rowLoader) String(fieldName string) string {
	val := r.NullString(fieldName)
	if !val.Valid {
		r.reportError(errors.Fmt("field %s value is NULL, expected non-null string", fieldName))
		return ""
	}
	return val.String()
}

// NullFloat64 returns the value of a field of type NullFloat64.
// If the field does not exist or is of an incorrect type, the default
// NullFloat64 is returned and an error will be available from rowLoader.Error()
func (r *rowLoader) NullFloat64(fieldName string) bigquery.NullFloat64 {
	repeated := false
	val, err := r.valueWithType(fieldName, bigquery.FloatFieldType, repeated)
	if err != nil {
		r.reportError(err)
		return bigquery.NullFloat64{}
	}
	if val == nil {
		return bigquery.NullFloat64{}
	}
	return bigquery.NullFloat64{Valid: true, Float64: val.(float64)}
}

func (r *rowLoader) Float64(fieldName string) float64 {
	val := r.NullFloat64(fieldName)
	if !val.Valid {
		r.reportError(errors.Fmt("field %s value is NULL, expected non-null float", fieldName))
		return -1
	}
	return val.Float64
}

// NullInt64 returns the value of a field of type NullInt64.
// If the field does not exist or is of an incorrect type, the default
// NullInt64 is returned and an error will be available from rowLoader.Error().
func (r *rowLoader) NullInt64(fieldName string) bigquery.NullInt64 {
	repeated := false
	val, err := r.valueWithType(fieldName, bigquery.IntegerFieldType, repeated)
	if err != nil {
		r.reportError(err)
		return bigquery.NullInt64{}
	}
	if val == nil {
		return bigquery.NullInt64{}
	}
	return bigquery.NullInt64{Valid: true, Int64: val.(int64)}
}

// Int64 returns the value of a field of type Int64.
// If the field does not exist or is of an incorrect type, the value
// -1 is returned and an error will be available from rowLoader.Error().
func (r *rowLoader) Int64(fieldName string) int64 {
	val := r.NullInt64(fieldName)
	if !val.Valid {
		r.reportError(errors.Fmt("field %s value is NULL, expected non-null integer", fieldName))
		return -1
	}
	return val.Int64
}

// MetricSqlName converts a metricType to the corresponding name in our
// table coumns. These names are lowercase versions of api.MetricType.
func MetricSqlName(metricType api.MetricType) string {
	return strings.ToLower(metricType.String())
}

// Metrics returns the value of a field of type []*api.TestMetricsData.
// The expected metric/field names are required. If the provided metric name
// is not found, it is not included in the return array and an error will be
// available from rowLoader.Error().
func (r *rowLoader) Metrics(metrics []api.MetricType) []*api.TestMetricsData {
	retMetrics := make([]*api.TestMetricsData, len(metrics))
	for metricIndex := range metrics {
		columnName := MetricSqlName(metrics[metricIndex])

		i, ok := r.fieldIndex(columnName)
		if !ok {
			r.reportError(errors.New("metric field is not defined"))
			continue
		}
		fieldType := r.schema[i]
		var val float64
		if fieldType.Type == bigquery.FloatFieldType {
			val = r.NullFloat64(columnName).Float64
		} else {
			val = float64(r.NullInt64(columnName).Int64)
		}
		retMetrics[metricIndex] = &api.TestMetricsData{
			MetricType:  metrics[metricIndex],
			MetricValue: val,
		}
	}
	return retMetrics
}

func (r *rowLoader) reportError(err error) {
	// Keep the first error that was reported.
	if r.err == nil {
		r.err = err
	}
}

// Error returns the first error that occurred while marshalling the
// row (if any). It is exposed here to avoid needing boilerplate error
// handling code around every field marshalling operation.
func (r *rowLoader) Error() error {
	return r.err
}
