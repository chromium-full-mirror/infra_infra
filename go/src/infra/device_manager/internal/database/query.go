// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package database

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"unicode"

	"go.chromium.org/luci/common/logging"
)

// DefaultPageSize is the default page size for paginated APIs.
const DefaultPageSize = 1000

// PageToken is a string containing a page token to a database query.
type PageToken string

// EncodePageToken encodes a string as a base64 PageToken.
func EncodePageToken(ctx context.Context, key string) PageToken {
	return PageToken(base64.StdEncoding.EncodeToString([]byte(key)))
}

// DecodePageToken decodes a base64 PageToken as a string.
func DecodePageToken(ctx context.Context, token PageToken) (string, error) {
	key, err := base64.StdEncoding.DecodeString(string(token))
	if err != nil {
		return "", fmt.Errorf("DecodePageToken: %w", err)
	}
	return string(key), nil
}

// BuildQueryFilter builds a WHERE clause based on a filter string.
//
// Parse filter from string. Assume filters are separated by "AND" based on
// AIP-160.
func BuildQueryFilter(ctx context.Context, filter string) (string, []interface{}) {
	var (
		filterExprs []string
		filterArgs  []interface{}
		queryFilter string
	)

	logging.Debugf(ctx, "BuildQueryFilter: processing filter string %q", filter)
	for _, filterPart := range strings.Split(filter, "AND") {
		trimmedFilter := strings.TrimSpace(filterPart)
		parts := strings.FieldsFunc(trimmedFilter, func(r rune) bool {
			return r == '=' || r == '!' || r == '>' || r == '<' || unicode.IsSpace(r)
		})

		if len(parts) < 2 {
			logging.Warningf(ctx, "BuildQueryFilter: invalid filter format: %q", trimmedFilter)
			break
		}

		// parse field, value, and operator
		field := parts[0]
		var value string
		switch len(parts) {
		case 2:
			value = parts[1] // operator with one operand; e.g. is_active = true
		default:
			value = strings.Join(parts[2:], " ") // operator with multiple operands; e.g. IS NOT NULL where NOT NULL is the operand
		}
		operator := strings.TrimSpace(trimmedFilter[len(field):(len(trimmedFilter) - len(value))])

		logging.Debugf(ctx, "BuildQueryFilter: %q %q %q", field, operator, value)
		position := fmt.Sprintf("$%d", len(filterArgs)+1)

		switch operator {
		case "=":
			filterExprs = append(filterExprs, fmt.Sprintf("%s = %s", field, position))
		case "!=":
			filterExprs = append(filterExprs, fmt.Sprintf("%s != %s", field, position))
		case ">":
			filterExprs = append(filterExprs, fmt.Sprintf("%s > %s", field, position))
		case "<":
			filterExprs = append(filterExprs, fmt.Sprintf("%s < %s", field, position))
		case ">=":
			filterExprs = append(filterExprs, fmt.Sprintf("%s >= %s", field, position))
		case "<=":
			filterExprs = append(filterExprs, fmt.Sprintf("%s <= %s", field, position))
		case "IS":
			// only support NULL check cases for filtering with IS
			if strings.ToUpper(value) == "NULL" {
				filterExprs = append(filterExprs, fmt.Sprintf("%s IS NULL", field))
			} else if strings.ToUpper(value) == "NOT NULL" {
				filterExprs = append(filterExprs, fmt.Sprintf("%s IS NOT NULL", field))
			}
			continue
		default:
			logging.Warningf(ctx, "BuildQueryFilter: unsupported filter: %q %q %q", field, operator, value)
			continue
		}

		filterArgs = append(filterArgs, value)
	}

	if len(filterExprs) > 0 {
		queryFilter += `
		WHERE ` + strings.Join(filterExprs, " AND ")
	}
	return queryFilter, filterArgs
}
