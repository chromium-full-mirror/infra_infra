// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package queryutils

import (
	"fmt"
	"strings"

	"go.chromium.org/luci/common/data/aip160"
	"go.chromium.org/luci/common/errors"
)

// compositeArgInfo is the info used when the `arg` has composite expression.
// Currently it is limited to simple expressions like (value1 AND value2 OR ...)
type compositeArgInfo struct {
	comparator string
	column     *Column
	fields     []string
}

// WithWhereClause adds Standard SQL WHERE clause to the query based on the filter (including "WHERE").
// All field names are replaced with the safe database column names from the specified table.
// All user input strings are passed via query parameters, so the returned query is SQL injection safe.
func (q *QueryBuilder) WithWhereClause(filter string) (*QueryBuilder, error) {
	ast, err := aip160.ParseFilter(filter)
	if err != nil {
		return q, err
	}

	if ast.Expression == nil {
		return q, nil
	}

	clause, err := q.expressionQuery(ast.Expression, nil)
	if err != nil {
		return q, err
	}

	q.whereClause = fmt.Sprintf("WHERE %s\n", clause)
	return q, nil
}

// expressionQuery returns the SQL expression equivalent to the given
// filter expression.
// An expression is a conjunction (AND) of sequences or a simple
// sequence.
//
// The returned string is an injection-safe SQL expression.
func (q *QueryBuilder) expressionQuery(expression *aip160.Expression, argInfo *compositeArgInfo) (string, error) {
	factors := []string{}
	// As we use exact match semantics, both Sequence and Factor are equivalent to AND.
	for _, sequence := range expression.Sequences {
		for _, factor := range sequence.Factors {
			f, err := q.factorQuery(factor, argInfo)
			if err != nil {
				return "", err
			}
			factors = append(factors, f)
		}
	}
	if len(factors) == 1 {
		return factors[0], nil
	}
	return "(" + strings.Join(factors, " AND ") + ")", nil
}

// factorQuery returns the SQL expression equivalent to the given
// factor. A factor is a disjunction (OR) of terms or a simple term.
//
// The returned string is an injection-safe SQL expression.
func (q *QueryBuilder) factorQuery(factor *aip160.Factor, argInfo *compositeArgInfo) (string, error) {
	terms := []string{}
	for _, term := range factor.Terms {
		tq, err := q.termQuery(term, argInfo)
		if err != nil {
			return "", err
		}
		terms = append(terms, tq)
	}
	if len(terms) == 1 {
		return terms[0], nil
	}
	return "(" + strings.Join(terms, " OR ") + ")", nil
}

// termQuery returns the SQL expression equivalent to the given
// term.
//
// The returned string is an injection-safe SQL expression.
func (q *QueryBuilder) termQuery(term *aip160.Term, argInfo *compositeArgInfo) (string, error) {
	simpleQuery, err := q.simpleQuery(term.Simple, argInfo)
	if err != nil {
		return "", err
	}
	if term.Negated {
		return fmt.Sprintf("(NOT %s)", simpleQuery), nil
	}
	return simpleQuery, nil
}

// simpleQuery returns the SQL expression equivalent to the given simple
// filter.
// The returned string is an injection-safe SQL expression.
func (q *QueryBuilder) simpleQuery(simple *aip160.Simple, argInfo *compositeArgInfo) (string, error) {
	// This is the case when there was an composite `arg`
	if argInfo != nil {
		if simple.Composite != nil {
			return "", fmt.Errorf("`composite` clause is not supported in composite `arg`")
		}
		return q.compositeArgRestrictionQuery(simple.Restriction, argInfo)
	}

	if simple.Restriction != nil {
		return q.restrictionQuery(simple.Restriction, argInfo)
	} else if simple.Composite != nil {
		return q.expressionQuery(simple.Composite, nil)
	} else {
		return "", fmt.Errorf("invalid 'simple' clause in query filter")
	}
}

// restrictionQuery returns the SQL expression equivalent to the given
// restriction.
// The returned string is an injection-safe SQL expression.
func (q *QueryBuilder) restrictionQuery(restriction *aip160.Restriction, argInfo *compositeArgInfo) (string, error) {
	if restriction.Comparable.Member == nil {
		return "", fmt.Errorf("invalid comparable")
	}

	if restriction.Comparator == "" {
		if len(restriction.Comparable.Member.Fields) > 0 {
			value := restriction.Comparable.Member.Value
			fields := strings.Join(restriction.Comparable.Member.Fields, ".")
			return "", fmt.Errorf("fields are not allowed without an operator, try wrapping %s.%s in double quotes: \"%s.%s\"", value, fields, value, fields)
		}

		return "", fmt.Errorf("global properties are not supported. Got: %s", restriction.Comparable.Member.Value)
	}

	column, ok := q.table.columnByExternalName[restriction.Comparable.Member.Value]
	if !ok {
		return "", fmt.Errorf("column `%s` doesn't exist", restriction.Comparable.Member.Value)
	}

	// Process composite arg.
	// Currently it is limited to simple expressions like (value1 AND value2 OR ...)
	if restriction.Arg.Composite != nil && argInfo == nil {
		return q.expressionQuery(restriction.Arg.Composite, &compositeArgInfo{
			comparator: restriction.Comparator,
			column:     column,
			fields:     restriction.Comparable.Member.Fields,
		})
	} else if restriction.Arg.Composite != nil && argInfo != nil {
		return "", fmt.Errorf("composite `args` are limited to simple expressions like (value1 AND value2 OR ...)")
	}

	if len(restriction.Comparable.Member.Fields) > 0 {
		if column.columnType != ColumnTypeJSONB {
			return "", fmt.Errorf("fields are only supported for json columns. Try removing the '.' from after your column named %q", column.externalName)
		}

		// This is the case for labels. I think it should be has (:) rather than equality (=) as for them
		// as labels have multiple values and we basically check the existence of the specified value in the array.
		// Currently leaving it as `=` to comply with the frontend.
		if restriction.Comparator == "=" {
			value, err := q.jsonArrayHasArgValue(restriction.Arg, column, restriction.Comparable.Member.Fields)
			if err != nil {
				return "", errors.Annotate(err, "argument for field %s", column.externalName).Err()
			}
			return fmt.Sprintf("(%s -> %s)", column.name, value), nil
		}

		return "", fmt.Errorf("operator `%s` not implemented for json fields yet", restriction.Comparator)
	}

	if restriction.Comparator == "=" {
		arg, err := q.argValue(restriction.Arg)
		if err != nil {
			return "", errors.Annotate(err, "argument for field %s", column.externalName).Err()
		}
		return fmt.Sprintf("(%s = %s)", column.name, arg), nil
	} else if restriction.Comparator == "!=" {
		arg, err := q.argValue(restriction.Arg)
		if err != nil {
			return "", errors.Annotate(err, "argument for field %s", column.externalName).Err()
		}
		return fmt.Sprintf("(%s <> %s)", column.name, arg), nil
	} else if restriction.Comparator == ":" {
		arg, err := q.likeArgValue(restriction.Arg, column)
		if err != nil {
			return "", errors.Annotate(err, "argument for field %s", column.externalName).Err()
		}
		return fmt.Sprintf("(%s LIKE %s)", column.name, arg), nil
	} else {
		return "", fmt.Errorf("comparator operator not implemented yet")
	}
}

// restrictionQuery returns the SQL expression equivalent to the given
// restriction.
// The returned string is an injection-safe SQL expression.
func (q *QueryBuilder) compositeArgRestrictionQuery(restriction *aip160.Restriction, argInfo *compositeArgInfo) (string, error) {
	if restriction.Comparable.Member == nil {
		return "", fmt.Errorf("invalid comparable")
	}

	if restriction.Comparator != "" {
		return "", fmt.Errorf("comparators are not supported in the composite args")
	}

	if len(restriction.Comparable.Member.Fields) > 0 {
		return "", fmt.Errorf("fields are not supported in the composite args")
	}

	if restriction.Arg != nil {
		return "", fmt.Errorf("arg is not supported in the composite args")
	}

	if argInfo.column.columnType == ColumnTypeJSONB {
		if argInfo.comparator == "=" {
			value, err := q.jsonArrayHasComparableValue(restriction.Comparable, argInfo.column, argInfo.fields)
			if err != nil {
				return "", errors.Annotate(err, "argument for field %s", argInfo.column.externalName).Err()
			}
			return fmt.Sprintf("(%s -> %s)", argInfo.column.name, value), nil
		}

		return "", fmt.Errorf("operator `%s` not implemented for json fields yet", restriction.Comparator)
	}

	if argInfo.comparator == "=" {
		arg, err := q.comparableValue(restriction.Comparable)
		if err != nil {
			return "", errors.Annotate(err, "argument for field %s", argInfo.column.externalName).Err()
		}
		return fmt.Sprintf("(%s = %s)", argInfo.column.name, arg), nil
	} else if argInfo.comparator == "!=" {
		arg, err := q.comparableValue(restriction.Comparable)
		if err != nil {
			return "", errors.Annotate(err, "argument for field %s", argInfo.column.externalName).Err()
		}
		return fmt.Sprintf("(%s <> %s)", argInfo.column.name, arg), nil
	} else if argInfo.comparator == ":" {
		arg, err := q.likeComparableValue(restriction.Comparable)
		if err != nil {
			return "", errors.Annotate(err, "argument for field %s", argInfo.column.externalName).Err()
		}
		return fmt.Sprintf("(%s LIKE %s)", argInfo.column.name, arg), nil
	} else {
		return "", fmt.Errorf("comparator operator not implemented yet")
	}
}

// argValue returns a SQL expression representing the value of the specified
// arg.
// The returned string is an injection-safe SQL expression.
func (q *QueryBuilder) argValue(arg *aip160.Arg) (string, error) {
	if arg.Composite != nil {
		return "", fmt.Errorf("composite expressions in arguments are not implemented yet")
	}
	if arg.Comparable == nil {
		return "", fmt.Errorf("missing comparable in argument")
	}
	return q.comparableValue(arg.Comparable)
}

// argValue returns a SQL expression representing the value of the specified
// comparable.
// The returned string is an injection-safe SQL expression.
func (q *QueryBuilder) comparableValue(comparable *aip160.Comparable) (string, error) {
	if comparable.Member == nil {
		return "", fmt.Errorf("invalid comparable")
	}
	if len(comparable.Member.Fields) > 0 {
		return "", fmt.Errorf("fields not implemented yet")
	}

	return q.bind(comparable.Member.Value), nil
}

// likeArgValue returns a SQL expression that, when passed to the
// right hand side of a LIKE operator, performs substring matching against
// the value of the argument.
// The returned string is an injection-safe SQL expression.
func (q *QueryBuilder) likeArgValue(arg *aip160.Arg, column *Column) (string, error) {
	if arg.Composite != nil {
		return "", fmt.Errorf("composite expressions are not allowed as RHS to has (:) operator")
	}
	if arg.Comparable == nil {
		return "", fmt.Errorf("missing comparable in the argument")
	}
	if column.columnType != ColumnTypeString {
		return "", fmt.Errorf("cannot use has (:) operator on a non-string field")
	}

	return q.likeComparableValue(arg.Comparable)
}

// likeComparableValue returns a SQL expression that, when passed to the
// right hand side of a LIKE operator, performs substring matching against
// the value of the comparable.
// The returned string is an injection-safe SQL expression.
func (q *QueryBuilder) likeComparableValue(comparable *aip160.Comparable) (string, error) {
	if comparable.Member == nil {
		return "", fmt.Errorf("invalid comparable")
	}
	if len(comparable.Member.Fields) > 0 {
		return "", fmt.Errorf("fields are not allowed on the RHS of has (:) operator")
	}
	// Bind unsanitised user input to a parameter to protect against SQL injection.
	return q.bind("%" + quoteLike(comparable.Member.Value) + "%"), nil
}

// Turns a literal string into an escaped like expression.
// This means strings like test_name will only match as expected, rather than
// also matching test3name.
func quoteLike(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "%", "\\%")
	value = strings.ReplaceAll(value, "_", "\\_")
	return value
}

// Checks whether value exist in the array specified in the json path
func (q *QueryBuilder) jsonArrayHasArgValue(arg *aip160.Arg, column *Column, fields []string) (string, error) {
	if arg.Composite != nil {
		return "", fmt.Errorf("composite expressions are not allowed as RHS to has (:) operator")
	}
	if arg.Comparable == nil {
		return "", fmt.Errorf("missing comparable in the argument")
	}

	return q.jsonArrayHasComparableValue(arg.Comparable, column, fields)
}

func (q *QueryBuilder) jsonArrayHasComparableValue(comparable *aip160.Comparable, column *Column, fields []string) (string, error) {
	if comparable.Member == nil {
		return "", fmt.Errorf("invalid comparable")
	}
	if len(comparable.Member.Fields) > 0 {
		return "", fmt.Errorf("fields are not allowed on the RHS of has (:) operator")
	}

	fullPath := column.jsonFullPath(fields...)
	params := make([]string, len(fullPath))
	for i, field := range fullPath {
		params[i] = q.bind(field)
	}

	value := q.bind(comparable.Member.Value)
	return fmt.Sprintf("%s ? %s", strings.Join(params, " -> "), value), nil
}
