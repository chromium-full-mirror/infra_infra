// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filtering

import (
	"fmt"

	"go.chromium.org/luci/common/data/aip160"
)

// ValueGetter given path returns member value in an array. If multiple elements are present in an array - matching any of them satisfies the filter
type ValueGetter func(string) (bool, []string)

// SatisfiesGlobalFunc determines if a global is satisfied
type SatisfiesGlobalFunc func(string) (bool, error)

func SatisfiesFilters(filter *aip160.Filter, getter ValueGetter, globalConditionGetter SatisfiesGlobalFunc) (bool, error) {
	if filter.Expression == nil {
		return true, nil
	}
	return satisfiesExpression(filter.Expression, getter, globalConditionGetter)
}

func satisfiesExpression(expression *aip160.Expression, getter ValueGetter, globalConditionGetter SatisfiesGlobalFunc) (bool, error) {
	for _, sequence := range expression.Sequences {
		satisfies, err := satisfiesSequence(sequence, getter, globalConditionGetter)
		if err != nil {
			return false, err
		}

		if !satisfies {
			return false, nil
		}
	}
	return true, nil
}

func satisfiesSequence(sequence *aip160.Sequence, getter ValueGetter, globalConditionGetter SatisfiesGlobalFunc) (bool, error) {
	for _, factor := range sequence.Factors {
		satisfies, err := satisfiesFactor(factor, getter, globalConditionGetter)
		if err != nil {
			return false, err
		}

		if !satisfies {
			return false, nil
		}
	}
	return true, nil
}

func satisfiesFactor(factor *aip160.Factor, getter ValueGetter, globalConditionGetter SatisfiesGlobalFunc) (bool, error) {
	for _, term := range factor.Terms {
		satisfies, err := satisfiesTerm(term, getter, globalConditionGetter)
		if err != nil {
			return false, err
		}

		if satisfies {
			return true, nil
		}
	}
	return false, nil
}

func satisfiesTerm(term *aip160.Term, getter ValueGetter, globalConditionGetter SatisfiesGlobalFunc) (bool, error) {
	if term.Simple == nil {
		return false, fmt.Errorf("unexpected term node without simple child node")
	}

	result, err := satisfiesSimple(term.Simple, getter, globalConditionGetter)

	if err != nil {
		return false, err
	}

	if term.Negated {
		return !result, nil
	}
	return result, nil
}

func satisfiesSimple(simple *aip160.Simple, getter ValueGetter, globalConditionGetter SatisfiesGlobalFunc) (bool, error) {
	if simple.Restriction != nil {
		return satisfiesRestriction(simple.Restriction, getter, globalConditionGetter)
	}
	if simple.Composite != nil {
		return satisfiesExpression(simple.Composite, getter, globalConditionGetter)
	}
	return false, fmt.Errorf("unexpected simple node without restriction or composite child nodes")
}

func satisfiesRestriction(restriction *aip160.Restriction, getter ValueGetter, globalConditionGetter SatisfiesGlobalFunc) (bool, error) {
	comparableValue, err := getValue(restriction.Comparable, getter)
	if err != nil {
		return false, err
	}

	if restriction.Comparator == "" {
		if len(comparableValue) > 1 {
			return false, fmt.Errorf("unexpected array global condition")
		}
		return globalConditionGetter(comparableValue[0])
	}

	comparator, err := convertComparator(restriction.Comparator)
	if err != nil {
		return false, err
	}

	return valueSatisfiesArg(comparableValue, restriction.Arg, getter, comparator)
}

func valueSatisfiesRestriction(values []string, restriction *aip160.Restriction, getter ValueGetter, comparator comparator) (bool, error) {
	if restriction.Arg != nil {
		return false, fmt.Errorf("unexpected restriction as RHS operand. Expected a comparable")
	}

	return valueSatisfiesComparable(values, restriction.Comparable, getter, comparator)
}

func valueSatisfiesArg(values []string, arg *aip160.Arg, getter ValueGetter, comparator comparator) (bool, error) {
	if arg.Composite != nil {
		return valueSatisfiesExpression(values, arg.Composite, getter, comparator)
	}
	if arg.Comparable != nil {
		return valueSatisfiesComparable(values, arg.Comparable, getter, comparator)
	}
	return false, nil
}

func valueSatisfiesExpression(values []string, composite *aip160.Expression, getter ValueGetter, comparator comparator) (bool, error) {
	for _, sequence := range composite.Sequences {
		satisfies, err := valueSatisfiesSequence(values, sequence, getter, comparator)

		if err != nil {
			return false, err
		}

		if !satisfies {
			return false, nil
		}
	}

	return true, nil
}

func valueSatisfiesSequence(values []string, sequence *aip160.Sequence, getter ValueGetter, comparator comparator) (bool, error) {
	for _, factor := range sequence.Factors {
		satisfies, err := valueSatisfiesFactor(values, factor, getter, comparator)
		if err != nil {
			return false, err
		}

		if !satisfies {
			return false, nil
		}
	}
	return true, nil
}

func valueSatisfiesFactor(values []string, factor *aip160.Factor, getter ValueGetter, comparator comparator) (bool, error) {
	for _, term := range factor.Terms {
		satisfies, err := valueSatisfiesTerm(values, term, getter, comparator)
		if err != nil {
			return false, err
		}

		if satisfies {
			return true, nil
		}
	}
	return false, nil
}

func valueSatisfiesTerm(values []string, term *aip160.Term, getter ValueGetter, comparator comparator) (bool, error) {
	if term.Simple == nil {
		return false, nil
	}
	result, err := valueSatisfiesSimple(values, term.Simple, getter, comparator)

	if err != nil {
		return false, err
	}

	if term.Negated {
		return !result, nil
	}
	return result, nil
}

func valueSatisfiesSimple(values []string, simple *aip160.Simple, getter ValueGetter, comparator comparator) (bool, error) {
	if simple.Restriction != nil {
		return valueSatisfiesRestriction(values, simple.Restriction, getter, comparator)
	}
	if simple.Composite != nil {
		return valueSatisfiesExpression(values, simple.Composite, getter, comparator)
	}
	return false, nil
}

func valueSatisfiesComparable(values []string, comparable *aip160.Comparable, getter ValueGetter, comparator comparator) (bool, error) {
	rhsValues, err := getValue(comparable, getter)
	if err != nil {
		return false, err
	}

	if len(rhsValues) != 1 {
		return false, fmt.Errorf("unexpected array rhs operand. Only one value rhs operands are supported")
	}

	for _, value := range values {
		if comparator(value, rhsValues[0]) {
			return true, nil
		}
	}
	return false, nil
}

func getValue(comparable *aip160.Comparable, getter ValueGetter) ([]string, error) {
	if comparable == nil || comparable.Member == nil {
		return []string{""}, fmt.Errorf("unexpected comparable node without member child node")
	}
	return getMember(comparable.Member, getter)
}

func getMember(member *aip160.Member, getter ValueGetter) ([]string, error) {
	path := member.Value
	for _, field := range member.Fields {
		path += "." + field
	}

	found, value := getter(path)

	if !found {
		// according to AIP-160 grammar if { DOT field } is present in the member it means that we expected a field to exist, so we should return an error
		// that is because free form text is only valid without DOT in it
		if len(member.Fields) > 0 {
			return []string{""}, fmt.Errorf("field %s not found", path)
		}

		// returning member value as a free form text
		return []string{member.Value}, nil
	}
	return value, nil
}

// compares two values used for restrictions
type comparator func(string, string) bool

func convertComparator(comparator string) (comparator, error) {
	switch comparator {
	case "<=":
		return func(a, b string) bool {
			return a <= b
		}, nil
	case "<":
		return func(a, b string) bool {
			return a < b
		}, nil
	case ">=":
		return func(a, b string) bool {
			return a >= b
		}, nil
	case ">":
		return func(a, b string) bool {
			return a > b
		}, nil
	case "!=":
		return func(a, b string) bool {
			return a != b
		}, nil
	case "=":
		return func(a, b string) bool {
			return a == b
		}, nil
	case ":":
		return nil, fmt.Errorf("unsupported comparator: %s", comparator) // 'has' operator is not yet supported, but might be supported in future
	default:
		return nil, fmt.Errorf("unknown comparator: %s", comparator)
	}
}
