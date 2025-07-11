// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package queryutils

import (
	"fmt"
	"strings"
)

type SelectClause struct {
	selectClause string

	//bindings between the aliases used within the query to variables used for reading the data
	bindings map[string]*any
}

type FieldSelectClause struct {
	fieldSelectClause string
	binding           *any
}

func Select(field string, binding *any) *FieldSelectClause {
	return &FieldSelectClause{
		fieldSelectClause: field,
		binding:           binding,
	}
}
func CountAll(binding *any) *FieldSelectClause {
	return &FieldSelectClause{
		fieldSelectClause: "COUNT(*)",
		binding:           binding,
	}
}

type ConditionalCountClause struct {
	field   string
	binding *any
}

func CountIf(field string, binding *any) *ConditionalCountClause {
	return &ConditionalCountClause{
		field:   field,
		binding: binding,
	}
}

func (c *ConditionalCountClause) Equals(value string) *FieldSelectClause {
	return c.toFieldSelectClause("=", value)
}

func (c *ConditionalCountClause) GreaterThan(value string) *FieldSelectClause {
	return c.toFieldSelectClause(">", value)
}

func (c *ConditionalCountClause) LessThan(value string) *FieldSelectClause {
	return c.toFieldSelectClause("<", value)
}

func (c *ConditionalCountClause) toFieldSelectClause(operator string, value string) *FieldSelectClause {
	return &FieldSelectClause{
		fieldSelectClause: fmt.Sprintf("SUM(CASE WHEN %s %s \"%s\" THEN 1 ELSE 0 END)", c.field, operator, value),
		binding:           c.binding,
	}
}

func (c *ConditionalCountClause) In(values ...string) *FieldSelectClause {
	if len(values) == 0 {
		return &FieldSelectClause{
			fieldSelectClause: "0", // No values to match.
			binding:           c.binding,
		}
	}

	quotedValues := make([]string, len(values))
	for i, v := range values {
		quotedValues[i] = fmt.Sprintf("'%s'", v)
	}

	inClause := strings.Join(quotedValues, ", ")

	return &FieldSelectClause{
		fieldSelectClause: fmt.Sprintf("SUM(CASE WHEN %s IN (%s) THEN 1 ELSE 0 END)", c.field, inClause),
		binding:           c.binding,
	}
}
