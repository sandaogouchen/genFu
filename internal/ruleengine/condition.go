package ruleengine

import (
	"fmt"
	"strings"

	"github.com/expr-lang/expr"
)

// EvaluateConditionGroup evaluates a ConditionGroup against an environment map.
// If group is nil the evaluation vacuously succeeds (no conditions = always pass).
//
// Evaluation modes:
//   - If group.Expression is set, it is compiled and run via expr-lang directly.
//   - Otherwise, structured Conditions and sub-Groups are evaluated with AND/OR logic,
//     and each individual Condition is compiled to an expr expression internally.
func EvaluateConditionGroup(group *ConditionGroup, env map[string]interface{}) (bool, error) {
	if group == nil {
		return true, nil
	}

	// Raw expression mode: compile and run the expression directly.
	if group.Expression != "" {
		return evaluateExpression(group.Expression, env)
	}

	op := strings.ToLower(group.Operator)
	if op == "" {
		op = "and"
	}

	switch op {
	case "and":
		return evaluateAnd(group, env)
	case "or":
		return evaluateOr(group, env)
	default:
		return false, fmt.Errorf("unsupported condition group operator: %q", group.Operator)
	}
}

// evaluateExpression compiles and runs an arbitrary expr-lang expression against
// the provided environment. The expression must evaluate to a boolean.
func evaluateExpression(expression string, env map[string]interface{}) (bool, error) {
	program, err := expr.Compile(expression, expr.Env(env), expr.AsBool())
	if err != nil {
		return false, fmt.Errorf("compile expression %q: %w", expression, err)
	}
	output, err := expr.Run(program, env)
	if err != nil {
		return false, fmt.Errorf("run expression %q: %w", expression, err)
	}
	result, ok := output.(bool)
	if !ok {
		return false, fmt.Errorf("expression %q returned %T, expected bool", expression, output)
	}
	return result, nil
}

// evaluateAnd returns true only when every condition and every sub-group is true.
func evaluateAnd(group *ConditionGroup, env map[string]interface{}) (bool, error) {
	for i := range group.Conditions {
		ok, err := evaluateCondition(&group.Conditions[i], env)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	for _, sub := range group.Groups {
		ok, err := EvaluateConditionGroup(sub, env)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

// evaluateOr returns true when at least one condition or sub-group is true.
func evaluateOr(group *ConditionGroup, env map[string]interface{}) (bool, error) {
	for i := range group.Conditions {
		ok, err := evaluateCondition(&group.Conditions[i], env)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	for _, sub := range group.Groups {
		ok, err := EvaluateConditionGroup(sub, env)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// evaluateCondition evaluates a single structured Condition by compiling it
// into an expr-lang expression and running it against the environment.
func evaluateCondition(cond *Condition, env map[string]interface{}) (bool, error) {
	expression, err := conditionToExpr(cond)
	if err != nil {
		return false, err
	}
	return evaluateExpression(expression, env)
}

// conditionToExpr converts a structured Condition (Field/Operator/Value) to
// an expr-lang expression string. For example:
//
//	{Field: "pnl_pct", Operator: "gte", Value: 5.0} → "pnl_pct >= 5"
func conditionToExpr(cond *Condition) (string, error) {
	op := strings.ToLower(cond.Operator)

	var exprOp string
	switch op {
	case "gte":
		exprOp = ">="
	case "lte":
		exprOp = "<="
	case "gt":
		exprOp = ">"
	case "lt":
		exprOp = "<"
	case "eq":
		exprOp = "=="
	case "neq", "ne":
		exprOp = "!="
	case "cross_above":
		// Placeholder: cross_above requires historical tick data; defaults to true.
		return "true", nil
	case "cross_below":
		// Placeholder: cross_below requires historical tick data; defaults to true.
		return "true", nil
	default:
		return "", fmt.Errorf("unsupported condition operator: %q", cond.Operator)
	}

	// Format the RHS value based on its Go type.
	var valueStr string
	switch v := cond.Value.(type) {
	case float64:
		valueStr = fmt.Sprintf("%g", v)
	case float32:
		valueStr = fmt.Sprintf("%g", v)
	case int:
		valueStr = fmt.Sprintf("%d", v)
	case int64:
		valueStr = fmt.Sprintf("%d", v)
	case string:
		valueStr = fmt.Sprintf("%q", v)
	default:
		valueStr = fmt.Sprintf("%v", v)
	}

	return fmt.Sprintf("%s %s %s", cond.Field, exprOp, valueStr), nil
}
