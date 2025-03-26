// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicesdb

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/utils"
)

func ExportCSV(ctx context.Context, dbConn *sql.DB, columns []*fleetconsolerpc.Column, filter, orderby string, realms []string) (string, error) {
	query, err := buildListAllDevicesQuery(ctx, filter, orderby, realms)
	if err != nil {
		return "", utils.BadRequest(err, "failed to construct the query")
	}

	rows, err := dbConn.QueryContext(ctx, query.Statement, query.Parameters...)
	if err != nil {
		return "", errors.Annotate(err, "failed to read devices from the db").Err()
	}
	defer rows.Close()

	var buf bytes.Buffer
	csvWriter := csv.NewWriter(&buf)
	defer csvWriter.Flush()

	csvCols := make([]string, 0, len(columns))
	for _, col := range columns {
		csvCols = append(csvCols, col.DisplayName)
	}
	csvWriter.Write(csvCols)

	for rows.Next() {
		row := make([]string, 0, len(columns))

		var (
			id         string
			dutId      string
			host       string
			port       string
			deviceType string
			state      string
			labelsJSON []byte
		)
		err := rows.Scan(
			&id,
			&dutId,
			&host,
			&port,
			&deviceType,
			&state,
			&labelsJSON,
		)

		if err != nil {
			return "", errors.Annotate(err, "ExportCSV").Err()
		}

		labels := make(map[string]*LabelValuesDAO)
		err = json.Unmarshal(labelsJSON, &labels)
		if err != nil {
			return "", errors.Annotate(err, "ExportCSV").Err()
		}

		for _, column := range columns {
			switch column.Name {
			case IdColumn.ExternalName:
				row = append(row, id)
			case DutIdColumn.ExternalName:
				row = append(row, dutId)
			case HostColumn.ExternalName:
				row = append(row, host)
			case PortColumn.ExternalName:
				row = append(row, port)
			case TypeColumn.ExternalName:
				row = append(row, deviceType)
			case StateColumn.ExternalName:
				row = append(row, strings.TrimPrefix(state, "DEVICE_STATE_"))
			// assume it is a label
			default:
				value := ""
				if label, ok := labels[column.Name]; ok {
					value = strings.Join(label.Values, ", ")
				}
				row = append(row, value)
			}

		}
		csvWriter.Write(row)
	}

	if err := rows.Close(); err != nil {
		return "", errors.Annotate(err, "ExportCSV").Err()
	}

	if err := rows.Err(); err != nil {
		return "", errors.Annotate(err, "ExportCSV").Err()
	}

	csvWriter.Flush()
	return buf.String(), nil
}
