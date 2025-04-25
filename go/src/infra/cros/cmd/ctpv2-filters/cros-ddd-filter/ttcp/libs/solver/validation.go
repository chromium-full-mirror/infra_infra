// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package solver

import (
	"reflect"
	"strconv"

	ttcpSyntax "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/protos/ttcp/syntax"
	validation "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datatools/validation"
	errors "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/errors"
)

// isClassExpressionComplet returns nil if the class expression is complete,
// else it returns an error describing what is missing.
//
// A ClassExpression_Value is complet if:
//   - It is not null
//   - It field Value is complet
func isClassExpression_ValueComplet(exp *ttcpSyntax.ClassExpression_Value) error {
	if exp == nil {
		return errors.NewError("Error: The class expression is nil")
	}
	return errors.JoinError("Error in ClassExpression_Value:", isClassComplet(exp.Value))
}

// isClassComplet returns nil if the class is complete, else it returns an
// error describing what is missing.
//
// A class is complete if:
//   - It is not nil
//   - The field name is not an empty string
//   - The expression is not nil and complete
func isClassComplet(cl *ttcpSyntax.Class) error {
	if cl == nil {
		return errors.NewError("The Class must not be nil.")
	}
	if cl.Name == "" {
		return errors.NewError("The Class Name field must not be empty.")
	}
	if cl.Expression == nil {
		return errors.NewError("The Class is missing the field Expression")
	}
	return errors.JoinError("Error in Class:", isExpressionComplet(cl.Expression))
}

func isExpressionComplet(exp *ttcpSyntax.Expression) error {
	if exp == nil {
		return errors.NewError("The expression must not be nil.")
	}
	return errors.JoinError("Error in Expression:", isExpression_OperatorComplet(exp.Operator))
}

// expressionArgsConditions is the structure to hold the optional argument for the function
// areSubexpressions_complet.
type expressionArgsConditions struct {
	hasMin      bool
	minimumArgs int
	hasMax      bool
	maximimArgs int
}

// expConds initializes expressionArgsConditions struct
func expConds() expressionArgsConditions {
	return expressionArgsConditions{
		hasMin: false,
		hasMax: false,
	}
}

// expressionArgsConditions.min sets the minimum allowed number of arguments
func (conds expressionArgsConditions) min(value int) expressionArgsConditions {
	conds.hasMin = true
	conds.minimumArgs = value
	return conds
}

// expressionArgsConditions.max sets the maximum allowed number of arguments
func (conds expressionArgsConditions) max(value int) expressionArgsConditions {
	conds.hasMax = true
	conds.minimumArgs = value
	return conds
}

// areSubexpressions_complet returns nil if subExpressions are complete, else
// it returns an error describing what is missing.
// A subExpressions is complet if:
//   - it is not nil
//   - if conds has a minimumArgs, the subExpressions lenght is larger than it.
//   - if conds has a maximumArgs, the subExpressions length is smaller than it.
//   - each element of subExpressions is complet
func areSubexpressions_complet(subExpressions []*ttcpSyntax.Expression, conds expressionArgsConditions) error {
	if subExpressions == nil {
		return errors.NewError("SubExpressions is nil")
	}
	if conds.hasMin {
		if len(subExpressions) < conds.minimumArgs {
			return errors.NewErrorf("SubExpression need to have at least %d elements instead of:%v", conds.minimumArgs, subExpressions)
		}

	}
	if conds.hasMax {
		if len(subExpressions) > conds.minimumArgs {
			return errors.NewErrorf("SubExpression need to have less than %d elements instead of:%v", conds.minimumArgs, subExpressions)
		}
	}
	return errors.MultiCheck(validation.Map(subExpressions, func(index int, sub *ttcpSyntax.Expression) error {
		return errors.JoinError("Invalid Or subexpression #"+strconv.Itoa(index)+".", isExpressionComplet(sub))
	})...)
}

// isExpression_OperatorComplet returns nil if the operator op is complete, else
// it returns an error describing what is missing.
// If an operator is complet depends on the type of the operator:
//
//	Expression_Or:
//	   - its Or field is not nil
//	   - its Or field is a complet expression and has at least 2 elemets
//	Expression_And:
//	   - its And field is not nil
//	   - its And field is a complet expression and has at least 2 elemets
//	Expression_Not:
//	   - Its Not field is not nil
//	   - Its Not field.SubExpression is complet
//	Expression_Property:
//	   - Its Property field is complet
//	Expression_True:
//	   - Its True field is not nil
//	If op is not of any of the types mentioned above, an "Unexpected operator"
//	error is returned.
func isExpression_OperatorComplet(op interface{}) error {
	if op == nil {
		return errors.NewError("The operator is nil.")
	}
	switch typedOp := op.(type) {
	case *ttcpSyntax.Expression_Or:
		if typedOp.Or == nil {
			return errors.NewError("Expression_Or is missing field Or")
		}
		errors.JoinError(
			"Error in Expression_Or subexpressions",
			areSubexpressions_complet(typedOp.Or.SubExpressions, expConds().min(2)))
		return nil
	case *ttcpSyntax.Expression_And:
		if typedOp.And == nil {
			return errors.NewError("Expression_And is missing field Or")
		}
		errors.JoinError(
			"Error in Expression_And subexpressions",
			areSubexpressions_complet(typedOp.And.SubExpressions, expConds().min(2)))
		return nil
	case *ttcpSyntax.Expression_Not:
		if typedOp.Not == nil {
			return errors.NewError("The Expression_Not has no Not field")
		}
		return errors.JoinError("Invalid Not subexpression:", isExpressionComplet(typedOp.Not.SubExpression))
	case *ttcpSyntax.Expression_Property:
		return errors.JoinError("Expression_Property", isConditionComplet(typedOp.Property))
	case *ttcpSyntax.Expression_True:
		if typedOp.True == nil {
			return errors.NewError("Expression_True is missing True field.")
		}
		return nil
	default:
		return errors.NewErrorf("Unexpected operator:%v", reflect.TypeOf(op))
	}
}

// isConditionComplet
func isConditionComplet(cond *ttcpSyntax.Condition) error {
	if cond == nil {
		return errors.NewError("The Condition must not be nil.")
	}
	if cond.PropertyPath == "" {
		return errors.NewError("The Condition's PropertyPath field must not be empty.")
	}
	if cond.Condition == nil {
		return errors.NewError("The Condition's Condition field must not be nil.")
	}
	switch typedCond := cond.Condition.(type) {
	case *ttcpSyntax.Condition_Present:
		return nil
	case *ttcpSyntax.Condition_StrEqual:
		return nil
	case *ttcpSyntax.Condition_StrRegexMatch:
		return nil
	case *ttcpSyntax.Condition_StrInSet:
		if typedCond.StrInSet == nil {
			return errors.NewError("Missing StringSet field in Condition_StringSet")
		}
		return nil
	case *ttcpSyntax.Condition_IntEqual:
		return nil
	case *ttcpSyntax.Condition_IntInSet:
		if typedCond.IntInSet == nil {
			return errors.NewError("Missing IntSet field in Condition_IntSet")
		}
		return nil
	case *ttcpSyntax.Condition_IntLess:
		return nil
	case *ttcpSyntax.Condition_IntLessOrEqual:
		return nil
	case *ttcpSyntax.Condition_IntGreater:
		return nil
	case *ttcpSyntax.Condition_IntGreaterOrEqual:
		return nil
	default:
		return errors.NewErrorf("Unexpected condition:%s", reflect.TypeOf(cond.Condition))
	}
}
