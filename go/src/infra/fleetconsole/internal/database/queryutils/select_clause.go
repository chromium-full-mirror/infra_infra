// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package queryutils

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
		fieldSelectClause: "SUM(CASE WHEN " + c.field + " " + operator + " \"" + value + "\" THEN 1 ELSE 0 END)",
		binding:           c.binding,
	}
}
